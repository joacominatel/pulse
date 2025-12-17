package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// ActivityEvent represents a single user activity signal.
// events are append-only and immutable once created.
type ActivityEvent struct {
	id          EventID
	communityID CommunityID
	userID      *UserID // optional, some events can be anonymous
	eventType   EventType
	weight      Weight
	metadata    map[string]any
	createdAt   time.Time
}

var (
	ErrEventCommunityEmpty = errors.New("event must have a community id")
	ErrEventTypeEmpty      = errors.New("event must have an event type")
)

// NewActivityEvent creates a new ActivityEvent with the required fields.
func NewActivityEvent(
	communityID CommunityID,
	userID *UserID,
	eventType EventType,
	weight Weight,
	metadata map[string]any,
) (*ActivityEvent, error) {
	if communityID.IsZero() {
		return nil, ErrEventCommunityEmpty
	}
	if !eventType.IsValid() {
		return nil, ErrEventTypeEmpty
	}

	return &ActivityEvent{
		id:          NewEventID(),
		communityID: communityID,
		userID:      userID,
		eventType:   eventType,
		weight:      weight,
		metadata:    metadata,
		createdAt:   time.Now().UTC(),
	}, nil
}

// NewActivityEventWithDefaultWeight creates an event using the default weight for its type.
func NewActivityEventWithDefaultWeight(
	communityID CommunityID,
	userID *UserID,
	eventType EventType,
	metadata map[string]any,
) (*ActivityEvent, error) {
	return NewActivityEvent(communityID, userID, eventType, eventType.DefaultWeight(), metadata)
}

// ReconstructActivityEvent recreates an ActivityEvent from stored data.
// use this when loading from database, not for creating new events.
func ReconstructActivityEvent(
	id EventID,
	communityID CommunityID,
	userID *UserID,
	eventType EventType,
	weight Weight,
	metadata map[string]any,
	createdAt time.Time,
) *ActivityEvent {
	return &ActivityEvent{
		id:          id,
		communityID: communityID,
		userID:      userID,
		eventType:   eventType,
		weight:      weight,
		metadata:    metadata,
		createdAt:   createdAt,
	}
}

// ID returns the event's unique identifier.
func (e *ActivityEvent) ID() EventID {
	return e.id
}

// CommunityID returns the community this event belongs to.
func (e *ActivityEvent) CommunityID() CommunityID {
	return e.communityID
}

// UserID returns the user who generated this event, if any.
func (e *ActivityEvent) UserID() *UserID {
	return e.userID
}

// EventType returns the type of this event.
func (e *ActivityEvent) EventType() EventType {
	return e.eventType
}

// Weight returns the momentum weight of this event.
func (e *ActivityEvent) Weight() Weight {
	return e.weight
}

// Metadata returns the event-specific metadata.
func (e *ActivityEvent) Metadata() map[string]any {
	return e.metadata
}

// CreatedAt returns when this event was created.
func (e *ActivityEvent) CreatedAt() time.Time {
	return e.createdAt
}

// MetadataJSON returns the metadata as a JSON byte slice.
// useful for database storage.
func (e *ActivityEvent) MetadataJSON() ([]byte, error) {
	if e.metadata == nil {
		return []byte("null"), nil
	}
	return json.Marshal(e.metadata)
}

// MomentumContribution returns the signed momentum contribution of this event.
// positive events add to momentum, leave events subtract.
func (e *ActivityEvent) MomentumContribution() float64 {
	if e.eventType.IsPositiveSignal() {
		return e.weight.Value()
	}
	return -e.weight.Value()
}

// IsAnonymous returns true if this event has no associated user.
func (e *ActivityEvent) IsAnonymous() bool {
	return e.userID == nil
}
