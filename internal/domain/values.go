package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// UserID represents a unique identifier for a user.
// wrapping uuid to enforce type safety and prevent mixing with other ids.
type UserID struct {
	value uuid.UUID
}

// NewUserID creates a new random UserID.
func NewUserID() UserID {
	return UserID{value: uuid.New()}
}

// ParseUserID parses a string into a UserID.
func ParseUserID(s string) (UserID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserID{}, fmt.Errorf("invalid user id: %w", err)
	}
	return UserID{value: id}, nil
}

// UserIDFromUUID creates a UserID from an existing uuid.
func UserIDFromUUID(id uuid.UUID) UserID {
	return UserID{value: id}
}

// String returns the string representation of the UserID.
func (id UserID) String() string {
	return id.value.String()
}

// UUID returns the underlying uuid value.
func (id UserID) UUID() uuid.UUID {
	return id.value
}

// IsZero returns true if the UserID is not set.
func (id UserID) IsZero() bool {
	return id.value == uuid.Nil
}

// CommunityID represents a unique identifier for a community.
type CommunityID struct {
	value uuid.UUID
}

// NewCommunityID creates a new random CommunityID.
func NewCommunityID() CommunityID {
	return CommunityID{value: uuid.New()}
}

// ParseCommunityID parses a string into a CommunityID.
func ParseCommunityID(s string) (CommunityID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CommunityID{}, fmt.Errorf("invalid community id: %w", err)
	}
	return CommunityID{value: id}, nil
}

// CommunityIDFromUUID creates a CommunityID from an existing uuid.
func CommunityIDFromUUID(id uuid.UUID) CommunityID {
	return CommunityID{value: id}
}

// String returns the string representation of the CommunityID.
func (id CommunityID) String() string {
	return id.value.String()
}

// UUID returns the underlying uuid value.
func (id CommunityID) UUID() uuid.UUID {
	return id.value
}

// IsZero returns true if the CommunityID is not set.
func (id CommunityID) IsZero() bool {
	return id.value == uuid.Nil
}

// EventID represents a unique identifier for an activity event.
type EventID struct {
	value uuid.UUID
}

// NewEventID creates a new random EventID.
func NewEventID() EventID {
	return EventID{value: uuid.New()}
}

// ParseEventID parses a string into an EventID.
func ParseEventID(s string) (EventID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return EventID{}, fmt.Errorf("invalid event id: %w", err)
	}
	return EventID{value: id}, nil
}

// EventIDFromUUID creates an EventID from an existing uuid.
func EventIDFromUUID(id uuid.UUID) EventID {
	return EventID{value: id}
}

// String returns the string representation of the EventID.
func (id EventID) String() string {
	return id.value.String()
}

// UUID returns the underlying uuid value.
func (id EventID) UUID() uuid.UUID {
	return id.value
}

// IsZero returns true if the EventID is not set.
func (id EventID) IsZero() bool {
	return id.value == uuid.Nil
}

// Slug represents a url-friendly identifier.
// must be lowercase, alphanumeric with hyphens, 3-100 chars.
type Slug struct {
	value string
}

var (
	ErrSlugEmpty    = errors.New("slug cannot be empty")
	ErrSlugTooShort = errors.New("slug must be at least 3 characters")
	ErrSlugTooLong  = errors.New("slug must be at most 100 characters")
	ErrSlugInvalid  = errors.New("slug must contain only lowercase letters, numbers, and hyphens")
)

// NewSlug creates a new Slug from a string, validating the format.
func NewSlug(s string) (Slug, error) {
	if s == "" {
		return Slug{}, ErrSlugEmpty
	}
	if len(s) < 3 {
		return Slug{}, ErrSlugTooShort
	}
	if len(s) > 100 {
		return Slug{}, ErrSlugTooLong
	}

	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return Slug{}, ErrSlugInvalid
		}
	}

	return Slug{value: s}, nil
}

// SlugFromTrusted creates a Slug without validation.
// only use this when loading from database where data is already validated.
func SlugFromTrusted(s string) Slug {
	return Slug{value: s}
}

// String returns the string representation of the Slug.
func (s Slug) String() string {
	return s.value
}

// Username represents a validated username.
// must be 3-50 chars, alphanumeric with underscores.
type Username struct {
	value string
}

var (
	ErrUsernameEmpty    = errors.New("username cannot be empty")
	ErrUsernameTooShort = errors.New("username must be at least 3 characters")
	ErrUsernameTooLong  = errors.New("username must be at most 50 characters")
	ErrUsernameInvalid  = errors.New("username must contain only letters, numbers, and underscores")
)

// NewUsername creates a new Username from a string, validating the format.
func NewUsername(s string) (Username, error) {
	if s == "" {
		return Username{}, ErrUsernameEmpty
	}
	if len(s) < 3 {
		return Username{}, ErrUsernameTooShort
	}
	if len(s) > 50 {
		return Username{}, ErrUsernameTooLong
	}

	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return Username{}, ErrUsernameInvalid
		}
	}

	return Username{value: s}, nil
}

// UsernameFromTrusted creates a Username without validation.
// only use this when loading from database where data is already validated.
func UsernameFromTrusted(s string) Username {
	return Username{value: s}
}

// String returns the string representation of the Username.
func (u Username) String() string {
	return u.value
}

// Momentum represents a momentum score value.
// always non-negative, represents rate of activity change.
type Momentum struct {
	value float64
}

// NewMomentum creates a new Momentum, clamping negative values to zero.
func NewMomentum(v float64) Momentum {
	if v < 0 {
		v = 0
	}
	return Momentum{value: v}
}

// Value returns the numeric momentum value.
func (m Momentum) Value() float64 {
	return m.value
}

// Add returns a new Momentum with the given value added.
func (m Momentum) Add(delta float64) Momentum {
	return NewMomentum(m.value + delta)
}

// IsZero returns true if momentum is zero.
func (m Momentum) IsZero() bool {
	return m.value == 0
}

// Weight represents the importance weight of an event.
// must be between 0.1 and 10.0.
type Weight struct {
	value float64
}

const (
	MinWeight     = 0.1
	MaxWeight     = 10.0
	DefaultWeight = 1.0
)

var ErrWeightOutOfRange = errors.New("weight must be between 0.1 and 10.0")

// NewWeight creates a new Weight, validating the range.
func NewWeight(v float64) (Weight, error) {
	if v < MinWeight || v > MaxWeight {
		return Weight{}, ErrWeightOutOfRange
	}
	return Weight{value: v}, nil
}

// DefaultEventWeight returns the default weight for events.
func DefaultEventWeight() Weight {
	return Weight{value: DefaultWeight}
}

// Value returns the numeric weight value.
func (w Weight) Value() float64 {
	return w.value
}
