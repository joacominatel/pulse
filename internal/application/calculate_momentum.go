package application

import (
	"context"
	"fmt"
	"time"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

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

// CalculateMomentumUseCase handles momentum calculation for communities.
type CalculateMomentumUseCase struct {
	eventRepo     domain.ActivityEventRepository
	communityRepo domain.CommunityRepository
	config        MomentumConfig
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
		logger:        logger.WithComponent("calculate_momentum"),
	}
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
	since := time.Now().UTC().Add(-uc.config.TimeWindow)

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

	// apply decay factor based on time window
	// simpler model: just use the weighted sum directly for now
	// more sophisticated decay can be added later by fetching individual events
	newMomentum := domain.NewMomentum(weightedSum * uc.config.DecayFactor)

	// update community momentum
	if err := uc.communityRepo.UpdateMomentum(ctx, communityID, newMomentum); err != nil {
		uc.logger.Error("momentum update failed",
			"community_id", communityID.String(),
			"old_momentum", oldMomentum,
			"new_momentum", newMomentum.Value(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("updating momentum: %w", err)
	}

	uc.logger.Info("momentum calculated",
		"community_id", communityID.String(),
		"old_momentum", oldMomentum,
		"new_momentum", newMomentum.Value(),
		"event_count", eventCount,
		"time_window", uc.config.TimeWindow.String(),
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
