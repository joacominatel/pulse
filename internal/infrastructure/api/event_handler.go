package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/domain"
)

// EventHandler handles activity event related HTTP requests.
type EventHandler struct {
	ingestUseCase *application.IngestEventUseCase
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(ingestUseCase *application.IngestEventUseCase) *EventHandler {
	return &EventHandler{
		ingestUseCase: ingestUseCase,
	}
}

// RegisterRoutes registers the event routes on the given group.
func (h *EventHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/events", h.IngestEvent)
}

// IngestEventRequest is the request body for ingesting an activity event.
type IngestEventRequest struct {
	CommunityID string         `json:"community_id" validate:"required"`
	EventType   string         `json:"event_type" validate:"required"`
	Weight      *float64       `json:"weight,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// IngestEventResponse is the response for a successfully ingested event.
type IngestEventResponse struct {
	EventID     string  `json:"event_id"`
	CommunityID string  `json:"community_id"`
	EventType   string  `json:"event_type"`
	Weight      float64 `json:"weight"`
	Accepted    bool    `json:"accepted"`
}

// IngestEvent handles POST /api/v1/events
// ingests a new activity event into the system.
//
// @Summary Ingest activity event
// @Description Records a new activity event for a community
// @Tags events
// @Accept json
// @Produce json
// @Param body body IngestEventRequest true "Event data"
// @Success 201 {object} IngestEventResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/events [post]
func (h *EventHandler) IngestEvent(c echo.Context) error {
	var req IngestEventRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// validate required fields
	if req.CommunityID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "community_id is required")
	}
	if req.EventType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "event_type is required")
	}

	// get user from context (optional for events - some can be anonymous)
	userID := GetUserExternalID(c)
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	// execute use case
	output, err := h.ingestUseCase.Execute(c.Request().Context(), application.IngestEventInput{
		CommunityID: req.CommunityID,
		UserID:      userIDPtr,
		EventType:   req.EventType,
		Weight:      req.Weight,
		Metadata:    req.Metadata,
	})

	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusCreated, IngestEventResponse{
		EventID:     output.EventID,
		CommunityID: output.CommunityID,
		EventType:   output.EventType,
		Weight:      output.Weight,
		Accepted:    output.Accepted,
	})
}

// mapDomainError maps domain/application errors to HTTP errors.
func mapDomainError(err error) error {
	switch {
	case isNotFoundError(err):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case isValidationError(err):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case isOverloadError(err):
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}

// isNotFoundError checks if the error indicates a not found condition.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// check for domain not found error
	if err == domain.ErrNotFound {
		return true
	}
	// check error message for common patterns
	errMsg := err.Error()
	return contains(errMsg, "not found") || contains(errMsg, "not active")
}

// isValidationError checks if the error indicates a validation failure.
func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "invalid") || contains(errMsg, "required")
}

// isOverloadError checks if the error indicates the system is overloaded.
func isOverloadError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "buffer full") || contains(errMsg, "try again later")
}

// contains checks if s contains substr (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
