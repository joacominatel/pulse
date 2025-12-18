package cache

import (
	"context"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// CommunityRepositoryWithCache wraps a CommunityRepository and adds Redis caching.
// uses redis for the hot path (ListByMomentum) and falls back to postgres on errors.
type CommunityRepositoryWithCache struct {
	repo   domain.CommunityRepository
	redis  *RedisClient
	logger *logging.Logger
}

// NewCommunityRepositoryWithCache creates a cached community repository.
// if redis is nil, all calls go directly to the underlying repository.
func NewCommunityRepositoryWithCache(
	repo domain.CommunityRepository,
	redis *RedisClient,
	logger *logging.Logger,
) *CommunityRepositoryWithCache {
	return &CommunityRepositoryWithCache{
		repo:   repo,
		redis:  redis,
		logger: logger.WithComponent("community_cache"),
	}
}

// FindByID delegates directly to the underlying repository.
// single entity lookups don't benefit much from caching here.
func (r *CommunityRepositoryWithCache) FindByID(ctx context.Context, id domain.CommunityID) (*domain.Community, error) {
	return r.repo.FindByID(ctx, id)
}

// FindByIDs delegates directly to the underlying repository.
func (r *CommunityRepositoryWithCache) FindByIDs(ctx context.Context, ids []domain.CommunityID) ([]*domain.Community, error) {
	return r.repo.FindByIDs(ctx, ids)
}

// FindBySlug delegates directly to the underlying repository.
func (r *CommunityRepositoryWithCache) FindBySlug(ctx context.Context, slug domain.Slug) (*domain.Community, error) {
	return r.repo.FindBySlug(ctx, slug)
}

// Save delegates directly to the underlying repository.
func (r *CommunityRepositoryWithCache) Save(ctx context.Context, community *domain.Community) error {
	return r.repo.Save(ctx, community)
}

// Exists delegates directly to the underlying repository.
func (r *CommunityRepositoryWithCache) Exists(ctx context.Context, id domain.CommunityID) (bool, error) {
	return r.repo.Exists(ctx, id)
}

// UpdateMomentum delegates directly to the underlying repository.
// redis sync is handled by the use case, not here.
func (r *CommunityRepositoryWithCache) UpdateMomentum(ctx context.Context, id domain.CommunityID, momentum domain.Momentum) error {
	return r.repo.UpdateMomentum(ctx, id, momentum)
}

// ListByMomentum returns active communities ordered by momentum.
// tries redis first for sub-millisecond response, falls back to postgres on error.
func (r *CommunityRepositoryWithCache) ListByMomentum(ctx context.Context, limit, offset int) ([]*domain.Community, error) {
	// if redis is not configured, go straight to postgres
	if r.redis == nil {
		return r.repo.ListByMomentum(ctx, limit, offset)
	}

	// try to get community IDs from redis leaderboard
	communityIDs, err := r.redis.GetTopCommunities(ctx, int64(limit), int64(offset))
	if err != nil {
		// redis failed or empty - fall back to postgres
		r.logger.Debug("leaderboard cache miss, falling back to postgres",
			"limit", limit,
			"offset", offset,
			"reason", err.Error(),
		)
		return r.repo.ListByMomentum(ctx, limit, offset)
	}

	r.logger.Debug("leaderboard cache hit",
		"limit", limit,
		"offset", offset,
		"cached_count", len(communityIDs),
	)

	// convert string IDs to domain IDs
	ids := make([]domain.CommunityID, 0, len(communityIDs))
	for _, idStr := range communityIDs {
		id, err := domain.ParseCommunityID(idStr)
		if err != nil {
			// corrupted data in redis? log and skip
			r.logger.Warn("invalid community id in leaderboard cache",
				"id", idStr,
				"error", err.Error(),
			)
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		// all IDs were invalid? fall back to postgres
		r.logger.Warn("all leaderboard cache entries invalid, falling back to postgres")
		return r.repo.ListByMomentum(ctx, limit, offset)
	}

	// fetch full community details from postgres
	// FindByIDs preserves the order from redis (momentum descending)
	communities, err := r.repo.FindByIDs(ctx, ids)
	if err != nil {
		// postgres failed after redis success - this is a real error
		return nil, err
	}

	return communities, nil
}
