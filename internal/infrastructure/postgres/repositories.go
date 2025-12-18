package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joacominatel/pulse/internal/domain"
)

// UserRepository implements domain.UserRepository using Postgres.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// FindByID retrieves a user by their internal ID.
func (r *UserRepository) FindByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	const query = `
		SELECT id, external_id, username, display_name, avatar_url, bio, created_at, updated_at
		FROM pulse.users_profile
		WHERE id = $1
	`

	return r.scanUser(ctx, query, id.UUID())
}

// FindByExternalID retrieves a user by their external auth provider ID.
func (r *UserRepository) FindByExternalID(ctx context.Context, externalID string) (*domain.User, error) {
	const query = `
		SELECT id, external_id, username, display_name, avatar_url, bio, created_at, updated_at
		FROM pulse.users_profile
		WHERE external_id = $1
	`

	return r.scanUser(ctx, query, externalID)
}

// FindByUsername retrieves a user by their username.
func (r *UserRepository) FindByUsername(ctx context.Context, username domain.Username) (*domain.User, error) {
	const query = `
		SELECT id, external_id, username, display_name, avatar_url, bio, created_at, updated_at
		FROM pulse.users_profile
		WHERE username = $1
	`

	return r.scanUser(ctx, query, username.String())
}

// Save persists a user (insert or update).
func (r *UserRepository) Save(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO pulse.users_profile (id, external_id, username, display_name, avatar_url, bio, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			bio = EXCLUDED.bio,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.pool.Exec(ctx, query,
		user.ID().UUID(),
		user.ExternalID(),
		user.Username().String(),
		nullableString(user.DisplayName()),
		nullableString(user.AvatarURL()),
		nullableString(user.Bio()),
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if err != nil {
		return fmt.Errorf("saving user: %w", err)
	}
	return nil
}

// Exists checks if a user with the given ID exists.
func (r *UserRepository) Exists(ctx context.Context, id domain.UserID) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM pulse.users_profile WHERE id = $1)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, id.UUID()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking user existence: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) scanUser(ctx context.Context, query string, args ...any) (*domain.User, error) {
	var (
		id          string
		externalID  string
		username    string
		displayName *string
		avatarURL   *string
		bio         *string
		createdAt   time.Time
		updatedAt   time.Time
	)

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&id, &externalID, &username, &displayName, &avatarURL, &bio, &createdAt, &updatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	// database stores trusted data, but we still validate for safety
	// if parsing fails, we have data corruption
	userID, err := domain.ParseUserID(id)
	if err != nil {
		return nil, fmt.Errorf("corrupted user id in database: %w", err)
	}

	return domain.ReconstructUser(
		userID,
		externalID,
		domain.UsernameFromTrusted(username),
		derefString(displayName),
		derefString(avatarURL),
		derefString(bio),
		createdAt,
		updatedAt,
	), nil
}

// CommunityRepository implements domain.CommunityRepository using Postgres.
type CommunityRepository struct {
	pool *pgxpool.Pool
}

// NewCommunityRepository creates a new CommunityRepository.
func NewCommunityRepository(pool *pgxpool.Pool) *CommunityRepository {
	return &CommunityRepository{pool: pool}
}

// FindByID retrieves a community by its ID.
func (r *CommunityRepository) FindByID(ctx context.Context, id domain.CommunityID) (*domain.Community, error) {
	const query = `
		SELECT id, slug, name, description, creator_id, avatar_url, is_active, 
		       current_momentum, momentum_updated_at, created_at, updated_at
		FROM pulse.communities
		WHERE id = $1
	`

	return r.scanCommunity(ctx, query, id.UUID())
}

// FindBySlug retrieves a community by its URL-friendly slug.
func (r *CommunityRepository) FindBySlug(ctx context.Context, slug domain.Slug) (*domain.Community, error) {
	const query = `
		SELECT id, slug, name, description, creator_id, avatar_url, is_active,
		       current_momentum, momentum_updated_at, created_at, updated_at
		FROM pulse.communities
		WHERE slug = $1
	`

	return r.scanCommunity(ctx, query, slug.String())
}

// Save persists a community (insert or update).
func (r *CommunityRepository) Save(ctx context.Context, community *domain.Community) error {
	const query = `
		INSERT INTO pulse.communities (id, slug, name, description, creator_id, avatar_url, is_active,
		                               current_momentum, momentum_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			avatar_url = EXCLUDED.avatar_url,
			is_active = EXCLUDED.is_active,
			current_momentum = EXCLUDED.current_momentum,
			momentum_updated_at = EXCLUDED.momentum_updated_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.pool.Exec(ctx, query,
		community.ID().UUID(),
		community.Slug().String(),
		community.Name(),
		nullableString(community.Description()),
		community.CreatorID().UUID(),
		nullableString(community.AvatarURL()),
		community.IsActive(),
		community.CurrentMomentum().Value(),
		community.MomentumUpdatedAt(),
		community.CreatedAt(),
		community.UpdatedAt(),
	)

	if err != nil {
		return fmt.Errorf("saving community: %w", err)
	}
	return nil
}

// Exists checks if a community with the given ID exists.
func (r *CommunityRepository) Exists(ctx context.Context, id domain.CommunityID) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM pulse.communities WHERE id = $1)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, id.UUID()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking community existence: %w", err)
	}
	return exists, nil
}

// FindByIDs retrieves multiple communities by their IDs.
// maintains the order of the input IDs.
func (r *CommunityRepository) FindByIDs(ctx context.Context, ids []domain.CommunityID) ([]*domain.Community, error) {
	if len(ids) == 0 {
		return []*domain.Community{}, nil
	}

	// convert to UUIDs for query
	uuids := make([]string, len(ids))
	for i, id := range ids {
		uuids[i] = id.String()
	}

	// query using ANY with array
	const query = `
		SELECT id, slug, name, description, creator_id, avatar_url, is_active,
		       current_momentum, momentum_updated_at, created_at, updated_at
		FROM pulse.communities
		WHERE id = ANY($1)
	`

	rows, err := r.pool.Query(ctx, query, uuids)
	if err != nil {
		return nil, fmt.Errorf("finding communities by ids: %w", err)
	}
	defer rows.Close()

	// collect results in a map for reordering
	communityMap := make(map[string]*domain.Community)
	for rows.Next() {
		community, err := r.scanCommunityFromRows(rows)
		if err != nil {
			return nil, err
		}
		communityMap[community.ID().String()] = community
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating communities: %w", err)
	}

	// reorder results to match input order
	communities := make([]*domain.Community, 0, len(ids))
	for _, id := range ids {
		if community, ok := communityMap[id.String()]; ok {
			communities = append(communities, community)
		}
		// silently skip missing communities (could be deactivated/deleted)
	}

	return communities, nil
}

// ListByMomentum returns active communities ordered by momentum.
func (r *CommunityRepository) ListByMomentum(ctx context.Context, limit, offset int) ([]*domain.Community, error) {
	const query = `
		SELECT id, slug, name, description, creator_id, avatar_url, is_active,
		       current_momentum, momentum_updated_at, created_at, updated_at
		FROM pulse.communities
		WHERE is_active = true
		ORDER BY current_momentum DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing communities: %w", err)
	}
	defer rows.Close()

	var communities []*domain.Community
	for rows.Next() {
		community, err := r.scanCommunityFromRows(rows)
		if err != nil {
			return nil, err
		}
		communities = append(communities, community)
	}

	return communities, rows.Err()
}

// UpdateMomentum updates just the momentum fields for a community.
func (r *CommunityRepository) UpdateMomentum(ctx context.Context, id domain.CommunityID, momentum domain.Momentum) error {
	const query = `
		UPDATE pulse.communities
		SET current_momentum = $2, momentum_updated_at = $3, updated_at = $3
		WHERE id = $1
	`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id.UUID(), momentum.Value(), now)
	if err != nil {
		return fmt.Errorf("updating momentum: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CommunityRepository) scanCommunity(ctx context.Context, query string, args ...any) (*domain.Community, error) {
	row := r.pool.QueryRow(ctx, query, args...)

	var (
		id                string
		slug              string
		name              string
		description       *string
		creatorID         string
		avatarURL         *string
		isActive          bool
		currentMomentum   float64
		momentumUpdatedAt *time.Time
		createdAt         time.Time
		updatedAt         time.Time
	)

	err := row.Scan(
		&id, &slug, &name, &description, &creatorID, &avatarURL, &isActive,
		&currentMomentum, &momentumUpdatedAt, &createdAt, &updatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning community: %w", err)
	}

	// database stores trusted data, but we still validate for safety
	communityID, err := domain.ParseCommunityID(id)
	if err != nil {
		return nil, fmt.Errorf("corrupted community id in database: %w", err)
	}

	creatorIDParsed, err := domain.ParseUserID(creatorID)
	if err != nil {
		return nil, fmt.Errorf("corrupted creator id in database: %w", err)
	}

	return domain.ReconstructCommunity(
		communityID,
		domain.SlugFromTrusted(slug),
		name,
		derefString(description),
		creatorIDParsed,
		derefString(avatarURL),
		isActive,
		domain.NewMomentum(currentMomentum),
		momentumUpdatedAt,
		createdAt,
		updatedAt,
	), nil
}

func (r *CommunityRepository) scanCommunityFromRows(rows pgx.Rows) (*domain.Community, error) {
	var (
		id                string
		slug              string
		name              string
		description       *string
		creatorID         string
		avatarURL         *string
		isActive          bool
		currentMomentum   float64
		momentumUpdatedAt *time.Time
		createdAt         time.Time
		updatedAt         time.Time
	)

	err := rows.Scan(
		&id, &slug, &name, &description, &creatorID, &avatarURL, &isActive,
		&currentMomentum, &momentumUpdatedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning community row: %w", err)
	}

	// database stores trusted data, but we still validate for safety
	communityID, err := domain.ParseCommunityID(id)
	if err != nil {
		return nil, fmt.Errorf("corrupted community id in database: %w", err)
	}

	creatorIDParsed, err := domain.ParseUserID(creatorID)
	if err != nil {
		return nil, fmt.Errorf("corrupted creator id in database: %w", err)
	}

	return domain.ReconstructCommunity(
		communityID,
		domain.SlugFromTrusted(slug),
		name,
		derefString(description),
		creatorIDParsed,
		derefString(avatarURL),
		isActive,
		domain.NewMomentum(currentMomentum),
		momentumUpdatedAt,
		createdAt,
		updatedAt,
	), nil
}

// ActivityEventRepository implements domain.ActivityEventRepository using Postgres.
type ActivityEventRepository struct {
	pool *pgxpool.Pool
}

// NewActivityEventRepository creates a new ActivityEventRepository.
func NewActivityEventRepository(pool *pgxpool.Pool) *ActivityEventRepository {
	return &ActivityEventRepository{pool: pool}
}

// Save persists a new activity event.
func (r *ActivityEventRepository) Save(ctx context.Context, event *domain.ActivityEvent) error {
	const query = `
        INSERT INTO pulse.activity_events (id, community_id, user_id, event_type, weight, metadata, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

	var userID any
	if event.UserID() != nil {
		userID = event.UserID().UUID()
	}

	metadataJSON, err := event.MetadataJSON()
	if err != nil {
		return fmt.Errorf("serializing metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		event.ID().UUID(),
		event.CommunityID().UUID(),
		userID,
		event.EventType().String(),
		event.Weight().Value(),
		string(metadataJSON),
		event.CreatedAt(),
	)

	if err != nil {
		return fmt.Errorf("saving activity event: %w", err)
	}
	return nil
}

// SaveBatch persists multiple activity events in a single transaction.
// uses a multi-row INSERT for efficiency.
func (r *ActivityEventRepository) SaveBatch(ctx context.Context, events []*domain.ActivityEvent) error {
	if len(events) == 0 {
		return nil
	}

	// use a transaction for atomicity
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// batch insert using CopyFrom for maximum efficiency
	rows := make([][]any, len(events))
	for i, event := range events {
		var userID any
		if event.UserID() != nil {
			userID = event.UserID().UUID()
		}

		metadataJSON, err := event.MetadataJSON()
		if err != nil {
			return fmt.Errorf("serializing metadata for event %s: %w", event.ID().String(), err)
		}

		rows[i] = []any{
			event.ID().UUID(),
			event.CommunityID().UUID(),
			userID,
			event.EventType().String(),
			event.Weight().Value(),
			string(metadataJSON),
			event.CreatedAt(),
		}
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"pulse", "activity_events"},
		[]string{"id", "community_id", "user_id", "event_type", "weight", "metadata", "created_at"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("batch inserting events: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// FindByCommunity retrieves events for a community within a time window.
func (r *ActivityEventRepository) FindByCommunity(ctx context.Context, communityID domain.CommunityID, since time.Time, limit int) ([]*domain.ActivityEvent, error) {
	const query = `
		SELECT id, community_id, user_id, event_type, weight, metadata, created_at
		FROM pulse.activity_events
		WHERE community_id = $1 AND created_at >= $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, communityID.UUID(), since, limit)
	if err != nil {
		return nil, fmt.Errorf("querying activity events: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// FindByUser retrieves events generated by a user.
func (r *ActivityEventRepository) FindByUser(ctx context.Context, userID domain.UserID, limit int) ([]*domain.ActivityEvent, error) {
	const query = `
		SELECT id, community_id, user_id, event_type, weight, metadata, created_at
		FROM pulse.activity_events
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, userID.UUID(), limit)
	if err != nil {
		return nil, fmt.Errorf("querying user events: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

// CountByCommunity counts events for a community within a time window.
func (r *ActivityEventRepository) CountByCommunity(ctx context.Context, communityID domain.CommunityID, since time.Time) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM pulse.activity_events
		WHERE community_id = $1 AND created_at >= $2
	`

	var count int64
	err := r.pool.QueryRow(ctx, query, communityID.UUID(), since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting events: %w", err)
	}
	return count, nil
}

// SumWeightsByCommunity calculates the total weighted momentum contribution.
func (r *ActivityEventRepository) SumWeightsByCommunity(ctx context.Context, communityID domain.CommunityID, since time.Time) (float64, error) {
	// weights are multiplied by sign based on event type
	// leave events subtract, others add
	const query = `
		SELECT COALESCE(SUM(
			CASE WHEN event_type = 'leave' THEN -weight ELSE weight END
		), 0)
		FROM pulse.activity_events
		WHERE community_id = $1 AND created_at >= $2
	`

	var sum float64
	err := r.pool.QueryRow(ctx, query, communityID.UUID(), since).Scan(&sum)
	if err != nil {
		return 0, fmt.Errorf("summing weights: %w", err)
	}
	return sum, nil
}

func (r *ActivityEventRepository) scanEvents(rows pgx.Rows) ([]*domain.ActivityEvent, error) {
	var events []*domain.ActivityEvent

	for rows.Next() {
		var (
			id          string
			communityID string
			userID      *string
			eventType   string
			weight      float64
			metadata    []byte
			createdAt   time.Time
		)

		err := rows.Scan(&id, &communityID, &userID, &eventType, &weight, &metadata, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		// database stores trusted data, but we still validate for safety
		eventIDParsed, err := domain.ParseEventID(id)
		if err != nil {
			return nil, fmt.Errorf("corrupted event id in database: %w", err)
		}

		communityIDParsed, err := domain.ParseCommunityID(communityID)
		if err != nil {
			return nil, fmt.Errorf("corrupted community id in database: %w", err)
		}

		eventTypeParsed, err := domain.ParseEventType(eventType)
		if err != nil {
			return nil, fmt.Errorf("corrupted event type in database: %w", err)
		}

		weightParsed, err := domain.NewWeight(weight)
		if err != nil {
			return nil, fmt.Errorf("corrupted weight in database: %w", err)
		}

		var userIDParsed *domain.UserID
		if userID != nil {
			parsed, err := domain.ParseUserID(*userID)
			if err != nil {
				return nil, fmt.Errorf("corrupted user id in database: %w", err)
			}
			userIDParsed = &parsed
		}

		var metadataMap map[string]any
		if len(metadata) > 0 && string(metadata) != "null" {
			if err := json.Unmarshal(metadata, &metadataMap); err != nil {
				return nil, fmt.Errorf("corrupted metadata json in database: %w", err)
			}
		}

		event := domain.ReconstructActivityEvent(
			eventIDParsed,
			communityIDParsed,
			userIDParsed,
			eventTypeParsed,
			weightParsed,
			metadataMap,
			createdAt,
		)
		events = append(events, event)
	}

	return events, rows.Err()
}

// helper functions

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
