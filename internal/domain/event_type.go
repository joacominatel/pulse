package domain

import "errors"

// EventType represents the type of activity event.
// defined as enum to enforce valid values at compile time.
type EventType string

const (
	EventTypeView     EventType = "view"
	EventTypeJoin     EventType = "join"
	EventTypeLeave    EventType = "leave"
	EventTypePost     EventType = "post"
	EventTypeComment  EventType = "comment"
	EventTypeReaction EventType = "reaction"
	EventTypeShare    EventType = "share"
)

var ErrInvalidEventType = errors.New("invalid event type")

// validEventTypes for quick lookup.
var validEventTypes = map[EventType]bool{
	EventTypeView:     true,
	EventTypeJoin:     true,
	EventTypeLeave:    true,
	EventTypePost:     true,
	EventTypeComment:  true,
	EventTypeReaction: true,
	EventTypeShare:    true,
}

// ParseEventType validates and returns an EventType from a string.
func ParseEventType(s string) (EventType, error) {
	et := EventType(s)
	if !validEventTypes[et] {
		return "", ErrInvalidEventType
	}
	return et, nil
}

// String returns the string representation of the EventType.
func (e EventType) String() string {
	return string(e)
}

// IsValid returns true if the event type is valid.
func (e EventType) IsValid() bool {
	return validEventTypes[e]
}

// DefaultWeight returns the default momentum weight for this event type.
// different events contribute differently to momentum.
// these weights reflect relative importance for discovery.
func (e EventType) DefaultWeight() Weight {
	weights := map[EventType]float64{
		EventTypeView:     0.5,  // passive, low signal
		EventTypeJoin:     3.0,  // strong commitment signal
		EventTypeLeave:    -2.0, // negative signal (will be clamped in momentum)
		EventTypePost:     5.0,  // high engagement signal
		EventTypeComment:  3.0,  // active participation
		EventTypeReaction: 1.0,  // lightweight engagement
		EventTypeShare:    4.0,  // distribution signal
	}

	w, ok := weights[e]
	if !ok {
		return DefaultEventWeight()
	}

	// weights can be negative for modeling, but Weight type clamps
	// so we use absolute value here - negative effects handled in momentum calc
	if w < 0 {
		w = -w
	}
	weight, _ := NewWeight(w)
	return weight
}

// IsPositiveSignal returns true if this event type contributes positively to momentum.
func (e EventType) IsPositiveSignal() bool {
	return e != EventTypeLeave
}
