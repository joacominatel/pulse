package application

import (
	"context"
	"fmt"
	"time"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// TimeProvider abstracts time acquisition for testability.
// inject a custom implementation to control time in tests.
type TimeProvider func() time.Time

// RealTime returns the current UTC time.
// use this in production.
func RealTime() time.Time {
	return time.Now().UTC()
}

// MomentumConfig contains parameters for momentum calculation.
type MomentumConfig struct {
	// TimeWindow is the sliding window for counting activity.
	// events older than this are not considered.
	TimeWindow time.Duration

	// DecayFactor controls how quickly old events lose weight.
	// 1.0 means no decay, 0.5 means events at window edge count half.
	DecayFactor float64
}

// DefaultMomentumConfig returns sensible defaults.
func DefaultMomentumConfig() MomentumConfig {
	return MomentumConfig{
		TimeWindow:  1 * time.Hour, // 1 hour sliding window
		DecayFactor: 0.7,           // 30% decay at window edge
	}
}

// CalculateMomentumInput contains the data needed to calculate momentum.
type CalculateMomentumInput struct {
	CommunityID string
}

// CalculateMomentumOutput contains the result of momentum calculation.
type CalculateMomentumOutput struct {
	CommunityID string
	OldMomentum float64
	NewMomentum float64
	EventCount  int64
	TimeWindow  time.Duration
	WasUpdated  bool
}

// LeaderboardUpdater abstracts the cache layer for momentum rankings.
// allows the use case to remain decoupled from redis specifics.
type LeaderboardUpdater interface {
	UpdateLeaderboardScore(ctx context.Context, communityID string, momentum float64) error
}

// SpikeNotifier abstracts the notification layer for momentum spikes.
// allows the use case to remain decoupled from webhook specifics.
type SpikeNotifier interface {
	NotifyMomentumSpike(ctx context.Context, spike domain.MomentumSpike) (int, error)
	Thresholds() domain.MomentumSpikeThresholds
}

// CalculateMomentumUseCase handles momentum calculation for communities.
type CalculateMomentumUseCase struct {
	eventRepo     domain.ActivityEventRepository
	communityRepo domain.CommunityRepository
	leaderboard   LeaderboardUpdater
	notifier      SpikeNotifier
	config        MomentumConfig
	timeProvider  TimeProvider
	logger        *logging.Logger
}

// NewCalculateMomentumUseCase creates a new CalculateMomentumUseCase.
func NewCalculateMomentumUseCase(
	eventRepo domain.ActivityEventRepository,
	communityRepo domain.CommunityRepository,
	config MomentumConfig,
	logger *logging.Logger,
) *CalculateMomentumUseCase {
	return &CalculateMomentumUseCase{
		eventRepo:     eventRepo,
		communityRepo: communityRepo,
		config:        config,
		timeProvider:  RealTime,
		logger:        logger.WithComponent("calculate_momentum"),
	}
}

// WithTimeProvider sets a custom time provider for testing.
func (uc *CalculateMomentumUseCase) WithTimeProvider(tp TimeProvider) *CalculateMomentumUseCase {
	uc.timeProvider = tp
	return uc
}

// WithLeaderboard sets the leaderboard updater (redis cache).
// when set, momentum updates are also pushed to the cache.
func (uc *CalculateMomentumUseCase) WithLeaderboard(lb LeaderboardUpdater) *CalculateMomentumUseCase {
	uc.leaderboard = lb
	return uc
}

// WithNotifier sets the spike notifier (webhook dispatcher).
// when set, momentum spikes trigger webhook notifications.
func (uc *CalculateMomentumUseCase) WithNotifier(n SpikeNotifier) *CalculateMomentumUseCase {
	uc.notifier = n
	return uc
}

// Execute calculates and updates momentum for a community.
func (uc *CalculateMomentumUseCase) Execute(ctx context.Context, input CalculateMomentumInput) (*CalculateMomentumOutput, error) {
	// parse and validate community id
	communityID, err := domain.ParseCommunityID(input.CommunityID)
	if err != nil {
		uc.logger.Warn("momentum calculation rejected: invalid community id",
			"community_id", input.CommunityID,
			"reason", err.Error(),
		)
		return nil, fmt.Errorf("invalid community id: %w", err)
	}

	// load community
	community, err := uc.communityRepo.FindByID(ctx, communityID)
	if err != nil {
		uc.logger.Warn("momentum calculation failed: community lookup failed",
			"community_id", communityID.String(),
			"reason", err.Error(),
		)
		return nil, fmt.Errorf("community lookup: %w", err)
	}

	oldMomentum := community.CurrentMomentum().Value()

	// use injected time provider for testability
	now := uc.timeProvider()
	since := now.Add(-uc.config.TimeWindow)

	// get event count for logging context
	eventCount, err := uc.eventRepo.CountByCommunity(ctx, communityID, since)
	if err != nil {
		uc.logger.Error("momentum calculation failed: event count failed",
			"community_id", communityID.String(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("counting events: %w", err)
	}

	// calculate weighted sum of events in window
	weightedSum, err := uc.eventRepo.SumWeightsByCommunity(ctx, communityID, since)
	if err != nil {
		uc.logger.Error("momentum calculation failed: weight sum failed",
			"community_id", communityID.String(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("summing weights: %w", err)
	}

	// use pure domain function for momentum calculation
	// using simpler model with pre-aggregated weights from db
	newMomentum := domain.SimpleMomentum(weightedSum, uc.config.DecayFactor)

	// update community momentum in postgres
	if err := uc.communityRepo.UpdateMomentum(ctx, communityID, newMomentum); err != nil {
		uc.logger.Error("momentum update failed",
			"community_id", communityID.String(),
			"old_momentum", oldMomentum,
			"new_momentum", newMomentum.Value(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("updating momentum: %w", err)
	}

	// sync to redis leaderboard (best-effort, don't fail on cache errors)
	if uc.leaderboard != nil {
		if err := uc.leaderboard.UpdateLeaderboardScore(ctx, communityID.String(), newMomentum.Value()); err != nil {
			// log but don't fail - postgres is the source of truth
			uc.logger.Warn("leaderboard sync failed",
				"community_id", communityID.String(),
				"momentum", newMomentum.Value(),
				"error", err.Error(),
			)
		}
	}

	// check for spike and notify (best-effort, don't fail on notification errors)
	if uc.notifier != nil {
		thresholds := uc.notifier.Thresholds()
		if thresholds.IsSpike(oldMomentum, newMomentum.Value()) {
			percentChange := 0.0
			if oldMomentum > 0 {
				percentChange = (newMomentum.Value() - oldMomentum) / oldMomentum
			}

			spike := domain.MomentumSpike{
				CommunityID:   communityID,
				CommunityName: community.Name(),
				OldMomentum:   oldMomentum,
				NewMomentum:   newMomentum.Value(),
				PercentChange: percentChange,
				Timestamp:     now,
			}

			if _, err := uc.notifier.NotifyMomentumSpike(ctx, spike); err != nil {
				uc.logger.Warn("spike notification failed",
					"community_id", communityID.String(),
					"error", err.Error(),
				)
			} else {
				uc.logger.Info("momentum spike detected",
					"community_id", communityID.String(),
					"old_momentum", oldMomentum,
					"new_momentum", newMomentum.Value(),
					"percent_change", percentChange,
				)
			}
		}
	}

	uc.logger.Info("momentum calculated",
		"community_id", communityID.String(),
		"old_momentum", oldMomentum,
		"new_momentum", newMomentum.Value(),
		"event_count", eventCount,
		"time_window", uc.config.TimeWindow.String(),
		"leaderboard_enabled", uc.leaderboard != nil,
		"notifier_enabled", uc.notifier != nil,
		"outcome", "updated",
	)

	return &CalculateMomentumOutput{
		CommunityID: communityID.String(),
		OldMomentum: oldMomentum,
		NewMomentum: newMomentum.Value(),
		EventCount:  eventCount,
		TimeWindow:  uc.config.TimeWindow,
		WasUpdated:  true,
	}, nil
}

// CalculateAllInput is empty as we process all active communities.
type CalculateAllInput struct {
	Limit int // max communities to process, 0 for all
}

// CalculateAllOutput contains the result of batch momentum calculation.
type CalculateAllOutput struct {
	Processed int
	Succeeded int
	Failed    int
}

// ExecuteAll calculates momentum for all active communities.
// useful for background jobs.
func (uc *CalculateMomentumUseCase) ExecuteAll(ctx context.Context, input CalculateAllInput) (*CalculateAllOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 1000 // reasonable default
	}

	communities, err := uc.communityRepo.ListByMomentum(ctx, limit, 0)
	if err != nil {
		uc.logger.Error("batch momentum calculation failed: listing communities",
			"error", err.Error(),
		)
		return nil, fmt.Errorf("listing communities: %w", err)
	}

	output := &CalculateAllOutput{
		Processed: len(communities),
	}

	for _, community := range communities {
		_, err := uc.Execute(ctx, CalculateMomentumInput{
			CommunityID: community.ID().String(),
		})
		if err != nil {
			output.Failed++
			// don't fail the whole batch, continue with others
			continue
		}
		output.Succeeded++
	}

	uc.logger.Info("batch momentum calculation completed",
		"processed", output.Processed,
		"succeeded", output.Succeeded,
		"failed", output.Failed,
	)

	return output, nil
}
