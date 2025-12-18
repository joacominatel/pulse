package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joacominatel/pulse/internal/domain"
)

// WebhookSubscriptionRepository implements domain.WebhookSubscriptionRepository using Postgres.
type WebhookSubscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewWebhookSubscriptionRepository creates a new WebhookSubscriptionRepository.
func NewWebhookSubscriptionRepository(pool *pgxpool.Pool) *WebhookSubscriptionRepository {
	return &WebhookSubscriptionRepository{pool: pool}
}

// Save persists a webhook subscription (insert or update).
func (r *WebhookSubscriptionRepository) Save(ctx context.Context, sub *domain.WebhookSubscription) error {
	const query = `
		INSERT INTO pulse.webhook_subscriptions (id, user_id, community_id, target_url, secret, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, community_id) DO UPDATE SET
			target_url = EXCLUDED.target_url,
			secret = EXCLUDED.secret,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.pool.Exec(ctx, query,
		sub.ID().String(),
		sub.UserID().UUID(),
		sub.CommunityID().UUID(),
		sub.TargetURL(),
		sub.Secret(),
		sub.IsActive(),
		sub.CreatedAt(),
		sub.UpdatedAt(),
	)
	return err
}

// FindByCommunity retrieves all active subscriptions for a community.
func (r *WebhookSubscriptionRepository) FindByCommunity(ctx context.Context, communityID domain.CommunityID) ([]*domain.WebhookSubscription, error) {
	const query = `
		SELECT id, user_id, community_id, target_url, secret, is_active, created_at, updated_at
		FROM pulse.webhook_subscriptions
		WHERE community_id = $1 AND is_active = true
	`

	rows, err := r.pool.Query(ctx, query, communityID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSubscriptions(rows)
}

// FindByUser retrieves all subscriptions for a user.
func (r *WebhookSubscriptionRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.WebhookSubscription, error) {
	const query = `
		SELECT id, user_id, community_id, target_url, secret, is_active, created_at, updated_at
		FROM pulse.webhook_subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSubscriptions(rows)
}

// Delete removes a subscription.
func (r *WebhookSubscriptionRepository) Delete(ctx context.Context, id domain.WebhookSubscriptionID) error {
	const query = `DELETE FROM pulse.webhook_subscriptions WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id.String())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// scanSubscriptions scans multiple rows into subscription slice.
func (r *WebhookSubscriptionRepository) scanSubscriptions(rows pgx.Rows) ([]*domain.WebhookSubscription, error) {
	var subs []*domain.WebhookSubscription

	for rows.Next() {
		var (
			id          string
			userID      string
			communityID string
			targetURL   string
			secret      string
			isActive    bool
			createdAt   time.Time
			updatedAt   time.Time
		)

		err := rows.Scan(&id, &userID, &communityID, &targetURL, &secret, &isActive, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		sub, err := r.buildSubscription(id, userID, communityID, targetURL, secret, isActive, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

// buildSubscription constructs a domain subscription from raw values.
func (r *WebhookSubscriptionRepository) buildSubscription(
	id, userID, communityID, targetURL, secret string,
	isActive bool,
	createdAt, updatedAt time.Time,
) (*domain.WebhookSubscription, error) {
	subID, err := domain.NewWebhookSubscriptionID(id)
	if err != nil {
		return nil, err
	}

	domainUserID, err := domain.ParseUserID(userID)
	if err != nil {
		return nil, err
	}

	domainCommunityID, err := domain.ParseCommunityID(communityID)
	if err != nil {
		return nil, err
	}

	return domain.ReconstructWebhookSubscription(
		subID,
		domainUserID,
		domainCommunityID,
		targetURL,
		secret,
		isActive,
		createdAt,
		updatedAt,
	), nil
}
