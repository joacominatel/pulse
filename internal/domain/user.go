package domain

import (
	"errors"
	"time"
)

// User represents a user profile in the pulse system.
// users generate signals through their interactions.
type User struct {
	id          UserID
	externalID  string // identifier from external auth provider
	username    Username
	displayName string
	avatarURL   string
	bio         string
	createdAt   time.Time
	updatedAt   time.Time
}

var (
	ErrUserExternalIDEmpty = errors.New("external id cannot be empty")
)

// NewUser creates a new User with the required fields.
func NewUser(externalID string, username Username) (*User, error) {
	if externalID == "" {
		return nil, ErrUserExternalIDEmpty
	}

	now := time.Now().UTC()
	return &User{
		id:         NewUserID(),
		externalID: externalID,
		username:   username,
		createdAt:  now,
		updatedAt:  now,
	}, nil
}

// ReconstructUser recreates a User from stored data.
// use this when loading from database, not for creating new users.
func ReconstructUser(
	id UserID,
	externalID string,
	username Username,
	displayName string,
	avatarURL string,
	bio string,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	return &User{
		id:          id,
		externalID:  externalID,
		username:    username,
		displayName: displayName,
		avatarURL:   avatarURL,
		bio:         bio,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// ID returns the user's unique identifier.
func (u *User) ID() UserID {
	return u.id
}

// ExternalID returns the external auth provider identifier.
func (u *User) ExternalID() string {
	return u.externalID
}

// Username returns the user's username.
func (u *User) Username() Username {
	return u.username
}

// DisplayName returns the user's display name.
func (u *User) DisplayName() string {
	return u.displayName
}

// AvatarURL returns the user's avatar URL.
func (u *User) AvatarURL() string {
	return u.avatarURL
}

// Bio returns the user's bio.
func (u *User) Bio() string {
	return u.bio
}

// CreatedAt returns when the user was created.
func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

// UpdatedAt returns when the user was last updated.
func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}

// UpdateProfile updates the user's profile fields.
func (u *User) UpdateProfile(displayName, avatarURL, bio string) {
	u.displayName = displayName
	u.avatarURL = avatarURL
	u.bio = bio
	u.updatedAt = time.Now().UTC()
}
