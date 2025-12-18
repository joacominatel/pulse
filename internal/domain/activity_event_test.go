package domain

import (
	"testing"
)

func TestNewActivityEvent_ValidInput(t *testing.T) {
	communityID := NewCommunityID()
	userID := NewUserID()
	eventType := EventTypeJoin
	weight := DefaultEventWeight()
	metadata := map[string]any{"source": "web"}

	event, err := NewActivityEvent(communityID, &userID, eventType, weight, metadata)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.CommunityID() != communityID {
		t.Errorf("expected community id %s, got %s", communityID, event.CommunityID())
	}
	if event.EventType() != eventType {
		t.Errorf("expected event type %s, got %s", eventType, event.EventType())
	}
	if event.ID().IsZero() {
		t.Error("expected non-zero event id")
	}
}

func TestNewActivityEvent_EmptyCommunityID(t *testing.T) {
	userID := NewUserID()

	_, err := NewActivityEvent(CommunityID{}, &userID, EventTypeJoin, DefaultEventWeight(), nil)

	if err != ErrEventCommunityEmpty {
		t.Errorf("expected ErrEventCommunityEmpty, got %v", err)
	}
}

func TestNewActivityEvent_InvalidEventType(t *testing.T) {
	communityID := NewCommunityID()
	userID := NewUserID()

	_, err := NewActivityEvent(communityID, &userID, EventType("invalid"), DefaultEventWeight(), nil)

	if err != ErrEventTypeEmpty {
		t.Errorf("expected ErrEventTypeEmpty, got %v", err)
	}
}

func TestActivityEvent_MetadataImmutability(t *testing.T) {
	communityID := NewCommunityID()
	userID := NewUserID()
	originalMetadata := map[string]any{"key": "original"}

	event, err := NewActivityEvent(communityID, &userID, EventTypePost, DefaultEventWeight(), originalMetadata)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// modify original map after creation
	originalMetadata["key"] = "modified"
	originalMetadata["new_key"] = "new_value"

	// event's internal state should not be affected
	eventMetadata := event.Metadata()
	if eventMetadata["key"] != "original" {
		t.Errorf("expected 'original', got %v", eventMetadata["key"])
	}
	if _, exists := eventMetadata["new_key"]; exists {
		t.Error("new_key should not exist in event metadata")
	}
}

func TestActivityEvent_MetadataGetterReturnsDefensiveCopy(t *testing.T) {
	communityID := NewCommunityID()
	userID := NewUserID()
	metadata := map[string]any{"key": "value"}

	event, err := NewActivityEvent(communityID, &userID, EventTypePost, DefaultEventWeight(), metadata)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// modify returned metadata
	returned := event.Metadata()
	returned["key"] = "modified"
	returned["extra"] = "extra_value"

	// original should be unchanged
	fresh := event.Metadata()
	if fresh["key"] != "value" {
		t.Errorf("expected 'value', got %v", fresh["key"])
	}
	if _, exists := fresh["extra"]; exists {
		t.Error("extra key should not exist")
	}
}

func TestActivityEvent_MomentumContribution(t *testing.T) {
	communityID := NewCommunityID()
	userID := NewUserID()

	tests := []struct {
		name         string
		eventType    EventType
		weight       float64
		expectedSign string
	}{
		{"join_positive", EventTypeJoin, 2.0, "positive"},
		{"leave_negative", EventTypeLeave, 2.0, "negative"},
		{"post_positive", EventTypePost, 1.5, "positive"},
		{"reaction_positive", EventTypeReaction, 0.5, "positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := NewWeight(tt.weight)
			if err != nil {
				t.Fatalf("unexpected error creating weight: %v", err)
			}
			event, err := NewActivityEvent(communityID, &userID, tt.eventType, weight, nil)
			if err != nil {
				t.Fatalf("unexpected error creating event: %v", err)
			}

			contribution := event.MomentumContribution()

			if tt.expectedSign == "positive" && contribution <= 0 {
				t.Errorf("expected positive contribution, got %f", contribution)
			}
			if tt.expectedSign == "negative" && contribution >= 0 {
				t.Errorf("expected negative contribution, got %f", contribution)
			}
		})
	}
}

func TestActivityEvent_AnonymousEvents(t *testing.T) {
	communityID := NewCommunityID()

	event, err := NewActivityEvent(communityID, nil, EventTypeView, DefaultEventWeight(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !event.IsAnonymous() {
		t.Error("expected event to be anonymous")
	}
	if event.UserID() != nil {
		t.Error("expected nil user id")
	}
}
