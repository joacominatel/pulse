package domain

import (
	"context"
	"time"
)

// WebhookSubscription represents a user's subscription to community momentum notifications.
type WebhookSubscription struct {
	id          WebhookSubscriptionID
	userID      UserID
	communityID CommunityID
	targetURL   string
	secret      string
	isActive    bool
	createdAt   time.Time
	updatedAt   time.Time
}

// WebhookSubscriptionID uniquely identifies a webhook subscription.
type WebhookSubscriptionID struct {
	value string
}

// NewWebhookSubscriptionID creates a new webhook subscription ID from a string.
func NewWebhookSubscriptionID(id string) (WebhookSubscriptionID, error) {
	if id == "" {
		return WebhookSubscriptionID{}, ErrInvalidInput
	}
	return WebhookSubscriptionID{value: id}, nil
}

// String returns the string representation.
func (id WebhookSubscriptionID) String() string {
	return id.value
}

// NewWebhookSubscription creates a new webhook subscription.
func NewWebhookSubscription(
	id WebhookSubscriptionID,
	userID UserID,
	communityID CommunityID,
	targetURL string,
	secret string,
) (*WebhookSubscription, error) {
	if targetURL == "" {
		return nil, ErrInvalidInput
	}
	if secret == "" {
		return nil, ErrInvalidInput
	}

	now := time.Now().UTC()
	return &WebhookSubscription{
		id:          id,
		userID:      userID,
		communityID: communityID,
		targetURL:   targetURL,
		secret:      secret,
		isActive:    true,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

// ReconstructWebhookSubscription rebuilds a subscription from persistence.
// bypasses validation for trusted data from database.
func ReconstructWebhookSubscription(
	id WebhookSubscriptionID,
	userID UserID,
	communityID CommunityID,
	targetURL string,
	secret string,
	isActive bool,
	createdAt time.Time,
	updatedAt time.Time,
) *WebhookSubscription {
	return &WebhookSubscription{
		id:          id,
		userID:      userID,
		communityID: communityID,
		targetURL:   targetURL,
		secret:      secret,
		isActive:    isActive,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// Getters

func (s *WebhookSubscription) ID() WebhookSubscriptionID { return s.id }
func (s *WebhookSubscription) UserID() UserID            { return s.userID }
func (s *WebhookSubscription) CommunityID() CommunityID  { return s.communityID }
func (s *WebhookSubscription) TargetURL() string         { return s.targetURL }
func (s *WebhookSubscription) Secret() string            { return s.secret }
func (s *WebhookSubscription) IsActive() bool            { return s.isActive }
func (s *WebhookSubscription) CreatedAt() time.Time      { return s.createdAt }
func (s *WebhookSubscription) UpdatedAt() time.Time      { return s.updatedAt }

// Deactivate disables the subscription without deleting it.
func (s *WebhookSubscription) Deactivate() {
	s.isActive = false
	s.updatedAt = time.Now().UTC()
}

// Activate enables a previously deactivated subscription.
func (s *WebhookSubscription) Activate() {
	s.isActive = true
	s.updatedAt = time.Now().UTC()
}

// WebhookSubscriptionRepository defines persistence for webhook subscriptions.
type WebhookSubscriptionRepository interface {
	// Save persists a webhook subscription (insert or update).
	Save(ctx context.Context, sub *WebhookSubscription) error

	// FindByCommunity retrieves all active subscriptions for a community.
	FindByCommunity(ctx context.Context, communityID CommunityID) ([]*WebhookSubscription, error)

	// FindByUser retrieves all subscriptions for a user.
	FindByUser(ctx context.Context, userID UserID) ([]*WebhookSubscription, error)

	// Delete removes a subscription.
	Delete(ctx context.Context, id WebhookSubscriptionID) error
}

// MomentumSpike represents a significant momentum change event.
type MomentumSpike struct {
	CommunityID   CommunityID
	CommunityName string
	OldMomentum   float64
	NewMomentum   float64
	PercentChange float64
	Timestamp     time.Time
}

// NotificationService defines the interface for sending momentum notifications.
// implementations handle the actual delivery mechanism (webhooks, etc).
type NotificationService interface {
	// NotifyMomentumSpike sends notifications when momentum crosses a threshold.
	// returns the number of notifications sent.
	NotifyMomentumSpike(ctx context.Context, spike *MomentumSpike) (int, error)
}

// MomentumSpikeThresholds defines when a spike is considered significant.
type MomentumSpikeThresholds struct {
	// AbsoluteThreshold is the minimum momentum value to trigger (e.g., > 10.0).
	AbsoluteThreshold float64

	// GrowthPercentage is the minimum growth rate to trigger (e.g., 0.20 = 20%).
	GrowthPercentage float64
}

// DefaultSpikeThresholds returns sensible defaults.
func DefaultSpikeThresholds() MomentumSpikeThresholds {
	return MomentumSpikeThresholds{
		AbsoluteThreshold: 10.0,
		GrowthPercentage:  0.20, // 20% growth
	}
}

// IsSpike determines if the momentum change constitutes a spike.
func (t MomentumSpikeThresholds) IsSpike(oldMomentum, newMomentum float64) bool {
	// must exceed absolute threshold
	if newMomentum <= t.AbsoluteThreshold {
		return false
	}

	// must be growing (not shrinking)
	if newMomentum <= oldMomentum {
		return false
	}

	// check percentage growth
	if oldMomentum <= 0 {
		// from zero or negative, any positive growth counts
		return newMomentum > t.AbsoluteThreshold
	}

	growth := (newMomentum - oldMomentum) / oldMomentum
	return growth >= t.GrowthPercentage
}
