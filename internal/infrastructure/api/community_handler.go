package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/domain"
)

// CommunityHandler handles community-related HTTP endpoints.
type CommunityHandler struct {
	repo domain.CommunityRepository
}

// NewCommunityHandler creates a new CommunityHandler.
func NewCommunityHandler(repo domain.CommunityRepository) *CommunityHandler {
	return &CommunityHandler{
		repo: repo,
	}
}

// RegisterRoutes registers community routes on the given group.
func (h *CommunityHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/communities", h.ListByMomentum)
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
