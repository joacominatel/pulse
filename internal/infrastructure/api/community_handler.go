package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/domain"
)

// CommunityHandler handles community-related HTTP endpoints.
type CommunityHandler struct {
	repo                   domain.CommunityRepository
	createCommunityUseCase *application.CreateCommunityUseCase
}

// NewCommunityHandler creates a new CommunityHandler.
func NewCommunityHandler(
	repo domain.CommunityRepository,
	createCommunityUseCase *application.CreateCommunityUseCase,
) *CommunityHandler {
	return &CommunityHandler{
		repo:                   repo,
		createCommunityUseCase: createCommunityUseCase,
	}
}

// RegisterRoutes registers community routes on the given group.
func (h *CommunityHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/communities", h.ListByMomentum)
	g.POST("/communities", h.Create)
}

// communityResponse is the API representation of a community.
type communityResponse struct {
	ID                string    `json:"id"`
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	Description       string    `json:"description,omitempty"`
	CreatorID         string    `json:"creator_id"`
	AvatarURL         string    `json:"avatar_url,omitempty"`
	IsActive          bool      `json:"is_active"`
	CurrentMomentum   float64   `json:"current_momentum"`
	MomentumUpdatedAt *string   `json:"momentum_updated_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// listCommunitiesResponse is the API response for listing communities.
type listCommunitiesResponse struct {
	Communities []communityResponse `json:"communities"`
	Limit       int                 `json:"limit"`
	Offset      int                 `json:"offset"`
}

// createCommunityRequest is the API request for creating a community.
type createCommunityRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// createCommunityResponse is the API response for creating a community.
type createCommunityResponse struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatorID string `json:"creator_id"`
}

// Create creates a new community.
// POST /api/v1/communities
// requires authentication - creator is taken from JWT claims, NOT request body
func (h *CommunityHandler) Create(c echo.Context) error {
	// get authenticated user from context (set by auth middleware)
	creatorExternalID := GetUserExternalID(c)
	if creatorExternalID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// parse request body
	var req createCommunityRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// validate required fields
	if req.Slug == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "slug is required")
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	// execute use case
	output, err := h.createCommunityUseCase.Execute(c.Request().Context(), application.CreateCommunityInput{
		Slug:              req.Slug,
		Name:              req.Name,
		Description:       req.Description,
		CreatorExternalID: creatorExternalID,
	})

	if err != nil {
		return mapCreateCommunityError(err)
	}

	return c.JSON(http.StatusCreated, createCommunityResponse{
		ID:        output.CommunityID,
		Slug:      output.Slug,
		Name:      output.Name,
		CreatorID: output.CreatorID,
	})
}

// mapCreateCommunityError converts use case errors to HTTP errors
func mapCreateCommunityError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, application.ErrCreatorNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "user profile not found - please complete signup first")
	case errors.Is(err, application.ErrSlugAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, "community with this slug already exists")
	case errors.Is(err, domain.ErrSlugEmpty):
		return echo.NewHTTPError(http.StatusBadRequest, "slug cannot be empty")
	case errors.Is(err, domain.ErrSlugTooShort):
		return echo.NewHTTPError(http.StatusBadRequest, "slug must be at least 3 characters")
	case errors.Is(err, domain.ErrSlugTooLong):
		return echo.NewHTTPError(http.StatusBadRequest, "slug must be at most 100 characters")
	case errors.Is(err, domain.ErrSlugInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, "slug must contain only lowercase letters, numbers, and hyphens")
	case errors.Is(err, domain.ErrCommunityNameEmpty):
		return echo.NewHTTPError(http.StatusBadRequest, "name cannot be empty")
	case errors.Is(err, domain.ErrCommunityNameTooLong):
		return echo.NewHTTPError(http.StatusBadRequest, "name must be at most 255 characters")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create community")
	}
}

// ListByMomentum returns communities ranked by current momentum.
// GET /api/v1/communities?limit=20&offset=0
func (h *CommunityHandler) ListByMomentum(c echo.Context) error {
	// parse pagination params with defaults
	limit := 20
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	communities, err := h.repo.ListByMomentum(c.Request().Context(), limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch communities")
	}

	response := listCommunitiesResponse{
		Communities: make([]communityResponse, 0, len(communities)),
		Limit:       limit,
		Offset:      offset,
	}

	for _, comm := range communities {
		response.Communities = append(response.Communities, toCommunityResponse(comm))
	}

	return c.JSON(http.StatusOK, response)
}

// toCommunityResponse converts a domain community to API response.
func toCommunityResponse(c *domain.Community) communityResponse {
	resp := communityResponse{
		ID:              c.ID().String(),
		Slug:            c.Slug().String(),
		Name:            c.Name(),
		Description:     c.Description(),
		CreatorID:       c.CreatorID().String(),
		AvatarURL:       c.AvatarURL(),
		IsActive:        c.IsActive(),
		CurrentMomentum: c.CurrentMomentum().Value(),
		CreatedAt:       c.CreatedAt(),
	}

	if t := c.MomentumUpdatedAt(); t != nil {
		formatted := t.Format(time.RFC3339)
		resp.MomentumUpdatedAt = &formatted
	}

	return resp
}
