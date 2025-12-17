package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/application"
)

// MomentumHandler handles momentum calculation related HTTP requests.
type MomentumHandler struct {
	calculateUseCase *application.CalculateMomentumUseCase
}

// NewMomentumHandler creates a new MomentumHandler.
func NewMomentumHandler(calculateUseCase *application.CalculateMomentumUseCase) *MomentumHandler {
	return &MomentumHandler{
		calculateUseCase: calculateUseCase,
	}
}

// RegisterRoutes registers the momentum routes on the given group.
func (h *MomentumHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/communities/:id/momentum/calculate", h.CalculateMomentum)
	g.POST("/momentum/calculate-all", h.CalculateAllMomentum)
}

// CalculateMomentumResponse is the response for momentum calculation.
type CalculateMomentumResponse struct {
	CommunityID string  `json:"community_id"`
	OldMomentum float64 `json:"old_momentum"`
	NewMomentum float64 `json:"new_momentum"`
	EventCount  int64   `json:"event_count"`
	TimeWindow  string  `json:"time_window"`
	WasUpdated  bool    `json:"was_updated"`
}

// CalculateAllMomentumRequest is the request body for batch momentum calculation.
type CalculateAllMomentumRequest struct {
	Limit int `json:"limit,omitempty"`
}

// CalculateAllMomentumResponse is the response for batch momentum calculation.
type CalculateAllMomentumResponse struct {
	Processed int `json:"processed"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// CalculateMomentum handles POST /api/v1/communities/:id/momentum/calculate
// calculates and updates momentum for a single community.
//
// @Summary Calculate community momentum
// @Description Triggers momentum recalculation for a specific community
// @Tags momentum
// @Accept json
// @Produce json
// @Param id path string true "Community ID"
// @Success 200 {object} CalculateMomentumResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/communities/{id}/momentum/calculate [post]
func (h *MomentumHandler) CalculateMomentum(c echo.Context) error {
	communityID := c.Param("id")
	if communityID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "community id is required")
	}

	output, err := h.calculateUseCase.Execute(c.Request().Context(), application.CalculateMomentumInput{
		CommunityID: communityID,
	})

	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusOK, CalculateMomentumResponse{
		CommunityID: output.CommunityID,
		OldMomentum: output.OldMomentum,
		NewMomentum: output.NewMomentum,
		EventCount:  output.EventCount,
		TimeWindow:  output.TimeWindow.String(),
		WasUpdated:  output.WasUpdated,
	})
}

// CalculateAllMomentum handles POST /api/v1/momentum/calculate-all
// calculates momentum for all active communities (batch operation).
//
// @Summary Calculate all community momentum
// @Description Triggers momentum recalculation for all active communities
// @Tags momentum
// @Accept json
// @Produce json
// @Param body body CalculateAllMomentumRequest false "Batch options"
// @Success 200 {object} CalculateAllMomentumResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/momentum/calculate-all [post]
func (h *MomentumHandler) CalculateAllMomentum(c echo.Context) error {
	var req CalculateAllMomentumRequest
	if err := c.Bind(&req); err != nil {
		// bind errors are fine here, we have defaults
		req = CalculateAllMomentumRequest{}
	}

	output, err := h.calculateUseCase.ExecuteAll(c.Request().Context(), application.CalculateAllInput{
		Limit: req.Limit,
	})

	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusOK, CalculateAllMomentumResponse{
		Processed: output.Processed,
		Succeeded: output.Succeeded,
		Failed:    output.Failed,
	})
}
