package application

import (
	"context"
	"fmt"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// IngestEventInput contains the data needed to ingest an activity event.
type IngestEventInput struct {
	CommunityID string
	UserID      *string // optional
	EventType   string
	Weight      *float64       // optional, uses default if not provided
	Metadata    map[string]any // optional
}

// IngestEventOutput contains the result of ingesting an event.
type IngestEventOutput struct {
	EventID     string
	CommunityID string
	EventType   string
	Weight      float64
	Accepted    bool
	Queued      bool // true if event was queued for async processing
}

// IngestEventUseCase handles the ingestion of activity events.
// supports both synchronous (direct save) and asynchronous (buffered channel) modes.
type IngestEventUseCase struct {
	eventRepo        domain.ActivityEventRepository
	communityRepo    domain.CommunityRepository
	userRepo         domain.UserRepository
	communityChecker CommunityChecker
	logger           *logging.Logger

	// async mode: if eventChan is set, events are pushed to the channel
	// instead of being saved directly to the repository
	eventChan chan<- *domain.ActivityEvent
}

// CommunityChecker abstracts community existence checks.
// allows using a cache instead of hitting the database every time.
type CommunityChecker interface {
	CheckActive(ctx context.Context, id domain.CommunityID) (exists bool, isActive bool, err error)
}

// NewIngestEventUseCase creates a new IngestEventUseCase in synchronous mode.
func NewIngestEventUseCase(
	eventRepo domain.ActivityEventRepository,
	communityRepo domain.CommunityRepository,
	userRepo domain.UserRepository,
	logger *logging.Logger,
) *IngestEventUseCase {
	return &IngestEventUseCase{
		eventRepo:     eventRepo,
		communityRepo: communityRepo,
		userRepo:      userRepo,
		logger:        logger.WithComponent("ingest_event"),
	}
}

// WithEventChannel sets the async event channel.
// when set, events will be pushed to the channel instead of saved directly.
// returns the use case for chaining.
func (uc *IngestEventUseCase) WithEventChannel(ch chan<- *domain.ActivityEvent) *IngestEventUseCase {
	uc.eventChan = ch
	return uc
}

// WithCommunityChecker sets the community existence checker.
// when set, uses the checker (typically a cache) instead of the repository.
func (uc *IngestEventUseCase) WithCommunityChecker(checker CommunityChecker) *IngestEventUseCase {
	uc.communityChecker = checker
	return uc
}

// Execute ingests a new activity event.
func (uc *IngestEventUseCase) Execute(ctx context.Context, input IngestEventInput) (*IngestEventOutput, error) {
	// parse and validate community id
	communityID, err := domain.ParseCommunityID(input.CommunityID)
	if err != nil {
		uc.logger.Warn("event rejected: invalid community id",
			"community_id", input.CommunityID,
			"reason", err.Error(),
		)
		return nil, fmt.Errorf("invalid community id: %w", err)
	}

	// verify community exists and is active
	// use cache if available for high-throughput scenarios
	var exists, isActive bool
	if uc.communityChecker != nil {
		exists, isActive, err = uc.communityChecker.CheckActive(ctx, communityID)
		if err != nil {
			uc.logger.Warn("event rejected: community check failed",
				"community_id", communityID.String(),
				"reason", err.Error(),
			)
			return nil, fmt.Errorf("community check: %w", err)
		}
	} else {
		// fallback to direct repository lookup
		community, err := uc.communityRepo.FindByID(ctx, communityID)
		if err != nil {
			uc.logger.Warn("event rejected: community lookup failed",
				"community_id", communityID.String(),
				"reason", err.Error(),
			)
			return nil, fmt.Errorf("community lookup: %w", err)
		}
		exists = true
		isActive = community.IsActive()
	}

	if !exists {
		uc.logger.Warn("event rejected: community not found",
			"community_id", communityID.String(),
			"outcome", "rejected",
		)
		return nil, fmt.Errorf("community %s not found", communityID.String())
	}
	if !isActive {
		uc.logger.Warn("event rejected: community inactive",
			"community_id", communityID.String(),
			"outcome", "rejected",
		)
		return nil, fmt.Errorf("community %s is not active", communityID.String())
	}

	// parse and validate event type
	eventType, err := domain.ParseEventType(input.EventType)
	if err != nil {
		uc.logger.Warn("event rejected: invalid event type",
			"community_id", communityID.String(),
			"event_type", input.EventType,
			"reason", err.Error(),
		)
		return nil, fmt.Errorf("invalid event type: %w", err)
	}

	// parse optional user id
	var userID *domain.UserID
	if input.UserID != nil {
		parsed, err := domain.ParseUserID(*input.UserID)
		if err != nil {
			uc.logger.Warn("event rejected: invalid user id",
				"community_id", communityID.String(),
				"user_id", *input.UserID,
				"reason", err.Error(),
			)
			return nil, fmt.Errorf("invalid user id: %w", err)
		}

		// verify user exists
		exists, err := uc.userRepo.Exists(ctx, parsed)
		if err != nil {
			return nil, fmt.Errorf("user lookup: %w", err)
		}
		if !exists {
			uc.logger.Warn("event rejected: user not found",
				"community_id", communityID.String(),
				"user_id", parsed.String(),
				"outcome", "rejected",
			)
			return nil, fmt.Errorf("user %s not found", parsed.String())
		}
		userID = &parsed
	}

	// determine weight
	var weight domain.Weight
	if input.Weight != nil {
		weight, err = domain.NewWeight(*input.Weight)
		if err != nil {
			uc.logger.Warn("event rejected: invalid weight",
				"community_id", communityID.String(),
				"weight", *input.Weight,
				"reason", err.Error(),
			)
			return nil, fmt.Errorf("invalid weight: %w", err)
		}
	} else {
		weight = eventType.DefaultWeight()
	}

	// create the domain event
	event, err := domain.NewActivityEvent(communityID, userID, eventType, weight, input.Metadata)
	if err != nil {
		uc.logger.Error("event creation failed",
			"community_id", communityID.String(),
			"event_type", eventType.String(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("creating event: %w", err)
	}

	// async mode: push to channel (non-blocking with select)
	if uc.eventChan != nil {
		select {
		case uc.eventChan <- event:
			uc.logger.Debug("event queued",
				"event_id", event.ID().String(),
				"community_id", communityID.String(),
				"event_type", eventType.String(),
			)
			return &IngestEventOutput{
				EventID:     event.ID().String(),
				CommunityID: communityID.String(),
				EventType:   eventType.String(),
				Weight:      weight.Value(),
				Accepted:    true,
				Queued:      true,
			}, nil
		default:
			// channel full, log warning but don't block
			uc.logger.Warn("event buffer full, dropping event",
				"event_id", event.ID().String(),
				"community_id", communityID.String(),
			)
			return nil, fmt.Errorf("event buffer full, try again later")
		}
	}

	// sync mode: persist directly
	if err := uc.eventRepo.Save(ctx, event); err != nil {
		uc.logger.Error("event save failed",
			"community_id", communityID.String(),
			"event_id", event.ID().String(),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("saving event: %w", err)
	}

	uc.logger.Info("event ingested",
		"event_id", event.ID().String(),
		"community_id", communityID.String(),
		"event_type", eventType.String(),
		"weight", weight.Value(),
		"outcome", "accepted",
	)

	return &IngestEventOutput{
		EventID:     event.ID().String(),
		CommunityID: communityID.String(),
		EventType:   eventType.String(),
		Weight:      weight.Value(),
		Accepted:    true,
		Queued:      false,
	}, nil
}
