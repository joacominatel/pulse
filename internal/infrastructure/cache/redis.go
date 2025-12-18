package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

const (
	// LeaderboardKey is the sorted set key for momentum rankings.
	// using a single key keeps things simple for now.
	LeaderboardKey = "pulse:leaderboard"

	// default connection timeout
	defaultConnectTimeout = 10 * time.Second
)

var (
	ErrRedisNotConnected = errors.New("redis not connected")
	ErrRedisEmpty        = errors.New("redis leaderboard is empty")
)

// RedisConfig holds configuration for Redis connection.
type RedisConfig struct {
	URL string
}

// RedisClient wraps the go-redis client with pulse-specific operations.
// focused on leaderboard functionality for now.
type RedisClient struct {
	client *redis.Client
	logger *logging.Logger
}

// NewRedisClient creates a new Redis client from the config.
// returns nil if the URL is empty (redis disabled).
func NewRedisClient(cfg RedisConfig, logger *logging.Logger) (*RedisClient, error) {
	if cfg.URL == "" {
		logger.Info("redis disabled: no REDIS_URL configured")
		return nil, nil
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}

	// pool size tuned for high concurrency
	// redis is fast, but we need enough connections for parallel reads
	opts.DialTimeout = defaultConnectTimeout
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	opts.PoolSize = 100
	opts.MinIdleConns = 10

	client := redis.NewClient(opts)

	rc := &RedisClient{
		client: client,
		logger: logger.WithComponent("redis"),
	}

	return rc, nil
}

// Connect tests the connection to Redis.
func (r *RedisClient) Connect(ctx context.Context) error {
	if r.client == nil {
		return ErrRedisNotConnected
	}

	ctx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	r.logger.Info("redis connected")
	return nil
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

// Client returns the underlying redis client.
// exposed for advanced usage, but prefer using the wrapped methods.
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// UpdateLeaderboardScore updates the momentum score for a community.
// uses ZADD to upsert the score in the sorted set.
func (r *RedisClient) UpdateLeaderboardScore(ctx context.Context, communityID string, momentum float64) error {
	if r.client == nil {
		return ErrRedisNotConnected
	}

	err := r.client.ZAdd(ctx, LeaderboardKey, redis.Z{
		Score:  momentum,
		Member: communityID,
	}).Err()

	if err != nil {
		r.logger.Error("failed to update leaderboard",
			"community_id", communityID,
			"momentum", momentum,
			"error", err.Error(),
		)
		return fmt.Errorf("zadd failed: %w", err)
	}

	r.logger.Debug("leaderboard updated",
		"community_id", communityID,
		"momentum", momentum,
	)

	return nil
}

// GetTopCommunities returns the top N community IDs ordered by momentum (descending).
// returns community IDs only, use these to fetch full details from postgres.
func (r *RedisClient) GetTopCommunities(ctx context.Context, limit, offset int64) ([]string, error) {
	if r.client == nil {
		return nil, ErrRedisNotConnected
	}

	// ZREVRANGE returns members ordered by score (high to low)
	start := offset
	stop := offset + limit - 1

	members, err := r.client.ZRevRange(ctx, LeaderboardKey, start, stop).Result()
	if err != nil {
		r.logger.Error("failed to get top communities",
			"limit", limit,
			"offset", offset,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("zrevrange failed: %w", err)
	}

	if len(members) == 0 {
		return nil, ErrRedisEmpty
	}

	r.logger.Debug("leaderboard queried",
		"limit", limit,
		"offset", offset,
		"returned", len(members),
	)

	return members, nil
}

// GetTopCommunitiesWithScores returns top N community IDs with their momentum scores.
// useful for debugging or when you need both values.
func (r *RedisClient) GetTopCommunitiesWithScores(ctx context.Context, limit, offset int64) ([]redis.Z, error) {
	if r.client == nil {
		return nil, ErrRedisNotConnected
	}

	start := offset
	stop := offset + limit - 1

	results, err := r.client.ZRevRangeWithScores(ctx, LeaderboardKey, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangewithscores failed: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrRedisEmpty
	}

	return results, nil
}

// RemoveFromLeaderboard removes a community from the leaderboard.
// useful when a community is deactivated.
func (r *RedisClient) RemoveFromLeaderboard(ctx context.Context, communityID string) error {
	if r.client == nil {
		return ErrRedisNotConnected
	}

	err := r.client.ZRem(ctx, LeaderboardKey, communityID).Err()
	if err != nil {
		return fmt.Errorf("zrem failed: %w", err)
	}

	r.logger.Debug("removed from leaderboard", "community_id", communityID)
	return nil
}

// GetCommunityRank returns the rank of a community (0-based, highest momentum = 0).
// returns -1 if community is not in the leaderboard.
func (r *RedisClient) GetCommunityRank(ctx context.Context, communityID string) (int64, error) {
	if r.client == nil {
		return -1, ErrRedisNotConnected
	}

	rank, err := r.client.ZRevRank(ctx, LeaderboardKey, communityID).Result()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("zrevrank failed: %w", err)
	}

	return rank, nil
}

// LeaderboardSize returns the number of communities in the leaderboard.
func (r *RedisClient) LeaderboardSize(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, ErrRedisNotConnected
	}

	count, err := r.client.ZCard(ctx, LeaderboardKey).Result()
	if err != nil {
		return 0, fmt.Errorf("zcard failed: %w", err)
	}

	return count, nil
}

// HealthCheck verifies Redis is responding.
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	if r.client == nil {
		return ErrRedisNotConnected
	}

	return r.client.Ping(ctx).Err()
}
