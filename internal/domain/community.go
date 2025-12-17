package domain

import (
	"errors"
	"time"
)

// Community represents a thematic grouping in pulse.
// communities are lightweight containers for discussion and activity.
type Community struct {
	id                CommunityID
	slug              Slug
	name              string
	description       string
	creatorID         UserID
	avatarURL         string
	isActive          bool
	currentMomentum   Momentum
	momentumUpdatedAt *time.Time
	createdAt         time.Time
	updatedAt         time.Time
}

var (
	ErrCommunityNameEmpty    = errors.New("community name cannot be empty")
	ErrCommunityNameTooLong  = errors.New("community name must be at most 255 characters")
	ErrCommunityCreatorEmpty = errors.New("community must have a creator")
)

// NewCommunity creates a new Community with the required fields.
func NewCommunity(slug Slug, name string, creatorID UserID) (*Community, error) {
	if name == "" {
		return nil, ErrCommunityNameEmpty
	}
	if len(name) > 255 {
		return nil, ErrCommunityNameTooLong
	}
	if creatorID.IsZero() {
		return nil, ErrCommunityCreatorEmpty
	}

	now := time.Now().UTC()
	return &Community{
		id:              NewCommunityID(),
		slug:            slug,
		name:            name,
		creatorID:       creatorID,
		isActive:        true,
		currentMomentum: NewMomentum(0),
		createdAt:       now,
		updatedAt:       now,
	}, nil
}

// ReconstructCommunity recreates a Community from stored data.
// use this when loading from database, not for creating new communities.
func ReconstructCommunity(
	id CommunityID,
	slug Slug,
	name string,
	description string,
	creatorID UserID,
	avatarURL string,
	isActive bool,
	currentMomentum Momentum,
	momentumUpdatedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *Community {
	return &Community{
		id:                id,
		slug:              slug,
		name:              name,
		description:       description,
		creatorID:         creatorID,
		avatarURL:         avatarURL,
		isActive:          isActive,
		currentMomentum:   currentMomentum,
		momentumUpdatedAt: momentumUpdatedAt,
		createdAt:         createdAt,
		updatedAt:         updatedAt,
	}
}

// ID returns the community's unique identifier.
func (c *Community) ID() CommunityID {
	return c.id
}

// Slug returns the community's URL-friendly slug.
func (c *Community) Slug() Slug {
	return c.slug
}

// Name returns the community's name.
func (c *Community) Name() string {
	return c.name
}

// Description returns the community's description.
func (c *Community) Description() string {
	return c.description
}

// CreatorID returns the ID of the user who created this community.
func (c *Community) CreatorID() UserID {
	return c.creatorID
}

// AvatarURL returns the community's avatar URL.
func (c *Community) AvatarURL() string {
	return c.avatarURL
}

// IsActive returns whether the community is active.
func (c *Community) IsActive() bool {
	return c.isActive
}

// CurrentMomentum returns the precomputed momentum score.
func (c *Community) CurrentMomentum() Momentum {
	return c.currentMomentum
}

// MomentumUpdatedAt returns when momentum was last calculated.
func (c *Community) MomentumUpdatedAt() *time.Time {
	return c.momentumUpdatedAt
}

// CreatedAt returns when the community was created.
func (c *Community) CreatedAt() time.Time {
	return c.createdAt
}

// UpdatedAt returns when the community was last updated.
func (c *Community) UpdatedAt() time.Time {
	return c.updatedAt
}

// UpdateMomentum sets the current momentum score.
// this is called by the momentum calculation job.
func (c *Community) UpdateMomentum(momentum Momentum) {
	c.currentMomentum = momentum
	now := time.Now().UTC()
	c.momentumUpdatedAt = &now
	c.updatedAt = now
}

// Deactivate marks the community as inactive.
func (c *Community) Deactivate() {
	c.isActive = false
	c.updatedAt = time.Now().UTC()
}

// Activate marks the community as active.
func (c *Community) Activate() {
	c.isActive = true
	c.updatedAt = time.Now().UTC()
}

// UpdateDetails updates the community's descriptive fields.
func (c *Community) UpdateDetails(name, description, avatarURL string) error {
	if name == "" {
		return ErrCommunityNameEmpty
	}
	if len(name) > 255 {
		return ErrCommunityNameTooLong
	}

	c.name = name
	c.description = description
	c.avatarURL = avatarURL
	c.updatedAt = time.Now().UTC()
	return nil
}
