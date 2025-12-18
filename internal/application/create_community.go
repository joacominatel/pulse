package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// CreateCommunityUseCase handles the creation of new communities.
type CreateCommunityUseCase struct {
	communityRepo domain.CommunityRepository
	userRepo      domain.UserRepository
	logger        *logging.Logger
}

// NewCreateCommunityUseCase creates a new CreateCommunityUseCase.
func NewCreateCommunityUseCase(
	communityRepo domain.CommunityRepository,
	userRepo domain.UserRepository,
	logger *logging.Logger,
) *CreateCommunityUseCase {
	return &CreateCommunityUseCase{
		communityRepo: communityRepo,
		userRepo:      userRepo,
		logger:        logger,
	}
}

// CreateCommunityInput contains the data needed to create a community.
type CreateCommunityInput struct {
	// Slug is the URL-friendly identifier (3-100 chars, lowercase alphanumeric with hyphens)
	Slug string

	// Name is the display name (1-255 chars)
	Name string

	// Description is optional, can be empty
	Description string

	// CreatorExternalID is the authenticated user's external ID from JWT (sub claim)
	// this comes from the validated JWT, NOT from the request body
	CreatorExternalID string
}

// CreateCommunityOutput contains the result of community creation.
type CreateCommunityOutput struct {
	CommunityID string
	Slug        string
	Name        string
	CreatorID   string
}

// use case specific errors
var (
	ErrCreatorNotFound   = errors.New("creator user not found")
	ErrSlugAlreadyExists = errors.New("community with this slug already exists")
)

// Execute creates a new community.
// validates input, looks up the creator, and persists the community.
func (uc *CreateCommunityUseCase) Execute(ctx context.Context, input CreateCommunityInput) (*CreateCommunityOutput, error) {
	// validate creator external id is provided
	if input.CreatorExternalID == "" {
		uc.logger.Error("create community failed: missing creator external id")
		return nil, fmt.Errorf("creator external id is required")
	}

	// validate slug format
	slug, err := domain.NewSlug(input.Slug)
	if err != nil {
		uc.logger.Info("create community failed: invalid slug",
			"slug", input.Slug,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	// validate name
	if input.Name == "" {
		return nil, domain.ErrCommunityNameEmpty
	}
	if len(input.Name) > 255 {
		return nil, domain.ErrCommunityNameTooLong
	}

	// look up the creator by external id
	creator, err := uc.userRepo.FindByExternalID(ctx, input.CreatorExternalID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			uc.logger.Info("create community failed: creator not found",
				"external_id", input.CreatorExternalID,
			)
			return nil, ErrCreatorNotFound
		}
		uc.logger.Error("create community failed: error looking up creator",
			"external_id", input.CreatorExternalID,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("looking up creator: %w", err)
	}

	// check if slug already exists
	existingCommunity, err := uc.communityRepo.FindBySlug(ctx, slug)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		uc.logger.Error("create community failed: error checking slug",
			"slug", input.Slug,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("checking slug availability: %w", err)
	}
	if existingCommunity != nil {
		uc.logger.Info("create community failed: slug already exists",
			"slug", input.Slug,
		)
		return nil, ErrSlugAlreadyExists
	}

	// create the community
	community, err := domain.NewCommunity(slug, input.Name, creator.ID())
	if err != nil {
		uc.logger.Error("create community failed: domain error",
			"error", err.Error(),
		)
		return nil, fmt.Errorf("creating community: %w", err)
	}

	// set description if provided (uses UpdateDetails to preserve name)
	if input.Description != "" {
		_ = community.UpdateDetails(input.Name, input.Description, "")
	}

	// persist
	if err := uc.communityRepo.Save(ctx, community); err != nil {
		uc.logger.Error("create community failed: save error",
			"slug", input.Slug,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("saving community: %w", err)
	}

	uc.logger.Info("community created",
		"community_id", community.ID().String(),
		"slug", community.Slug().String(),
		"creator_id", creator.ID().String(),
	)

	return &CreateCommunityOutput{
		CommunityID: community.ID().String(),
		Slug:        community.Slug().String(),
		Name:        community.Name(),
		CreatorID:   creator.ID().String(),
	}, nil
}
