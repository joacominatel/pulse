package api

import (
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/domain"
)

// SubscriptionHandler handles webhook subscription HTTP endpoints.
type SubscriptionHandler struct {
	repo domain.WebhookSubscriptionRepository
}

// NewSubscriptionHandler creates a new SubscriptionHandler.
func NewSubscriptionHandler(repo domain.WebhookSubscriptionRepository) *SubscriptionHandler {
	return &SubscriptionHandler{repo: repo}
}

// RegisterRoutes registers subscription routes on the given group.
// all routes require authentication.
func (h *SubscriptionHandler) RegisterRoutes(g *echo.Group) {
	subs := g.Group("/subscriptions")
	subs.POST("", h.Create)
	subs.GET("", h.List)
	subs.DELETE("/:id", h.Delete)
}

// --- Request/Response DTOs ---

// createSubscriptionRequest is the request body for creating a subscription.
// @Description Request body for creating a webhook subscription.
type createSubscriptionRequest struct {
	// CommunityID is the UUID of the community to subscribe to.
	CommunityID string `json:"community_id"`
	// TargetURL is the webhook endpoint that will receive notifications.
	TargetURL string `json:"target_url"`
	// Secret is used for HMAC-SHA256 signature verification.
	Secret string `json:"secret"`
}

// subscriptionResponse is the API representation of a webhook subscription.
// @Description Webhook subscription details.
type subscriptionResponse struct {
	ID          string    `json:"id"`
	CommunityID string    `json:"community_id"`
	TargetURL   string    `json:"target_url"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// listSubscriptionsResponse is the response for listing subscriptions.
// @Description List of webhook subscriptions for the authenticated user.
type listSubscriptionsResponse struct {
	Subscriptions []subscriptionResponse `json:"subscriptions"`
	Count         int                    `json:"count"`
}

// --- Handlers ---

// Create creates a new webhook subscription.
// @Summary Create a webhook subscription
// @Description Subscribe to momentum spike notifications for a community.
// @Tags subscriptions
// @Accept json
// @Produce json
// @Param request body createSubscriptionRequest true "Subscription details"
// @Success 201 {object} subscriptionResponse
// @Failure 400 {object} echo.HTTPError "Invalid request"
// @Failure 401 {object} echo.HTTPError "Unauthorized"
// @Failure 409 {object} echo.HTTPError "Subscription already exists"
// @Router /api/v1/subscriptions [post]
// @Security BearerAuth
func (h *SubscriptionHandler) Create(c echo.Context) error {
	// require authentication - user id comes from JWT, not body
	userExternalID := GetUserExternalID(c)
	if userExternalID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// parse request body
	var req createSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// validate required fields
	if req.CommunityID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "community_id is required")
	}
	if req.TargetURL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_url is required")
	}
	if req.Secret == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "secret is required")
	}

	// validate target_url is a valid URL with http/https scheme
	parsedURL, err := url.Parse(req.TargetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_url must be a valid HTTP or HTTPS URL")
	}

	// parse domain IDs
	userID, err := domain.ParseUserID(userExternalID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id format")
	}

	communityID, err := domain.ParseCommunityID(req.CommunityID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid community_id format")
	}

	// generate subscription ID
	subID, err := domain.NewWebhookSubscriptionID(uuid.New().String())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate subscription id")
	}

	// create domain entity
	subscription, err := domain.NewWebhookSubscription(subID, userID, communityID, req.TargetURL, req.Secret)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid subscription data")
	}

	// persist
	if err := h.repo.Save(c.Request().Context(), subscription); err != nil {
		// check for duplicate (upsert behavior means this rarely fails)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save subscription")
	}

	return c.JSON(http.StatusCreated, subscriptionResponse{
		ID:          subscription.ID().String(),
		CommunityID: subscription.CommunityID().String(),
		TargetURL:   subscription.TargetURL(),
		IsActive:    subscription.IsActive(),
		CreatedAt:   subscription.CreatedAt(),
		UpdatedAt:   subscription.UpdatedAt(),
	})
}

// List returns all subscriptions for the authenticated user.
// @Summary List webhook subscriptions
// @Description Get all webhook subscriptions for the authenticated user.
// @Tags subscriptions
// @Produce json
// @Success 200 {object} listSubscriptionsResponse
// @Failure 401 {object} echo.HTTPError "Unauthorized"
// @Router /api/v1/subscriptions [get]
// @Security BearerAuth
func (h *SubscriptionHandler) List(c echo.Context) error {
	// require authentication
	userExternalID := GetUserExternalID(c)
	if userExternalID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	userID, err := domain.ParseUserID(userExternalID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id format")
	}

	// fetch from repository
	subs, err := h.repo.FindByUser(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch subscriptions")
	}

	// map to response
	response := listSubscriptionsResponse{
		Subscriptions: make([]subscriptionResponse, 0, len(subs)),
		Count:         len(subs),
	}

	for _, sub := range subs {
		response.Subscriptions = append(response.Subscriptions, subscriptionResponse{
			ID:          sub.ID().String(),
			CommunityID: sub.CommunityID().String(),
			TargetURL:   sub.TargetURL(),
			IsActive:    sub.IsActive(),
			CreatedAt:   sub.CreatedAt(),
			UpdatedAt:   sub.UpdatedAt(),
		})
	}

	return c.JSON(http.StatusOK, response)
}

// Delete removes a subscription by ID.
// @Summary Delete a webhook subscription
// @Description Delete a webhook subscription. Only the owner can delete their subscription.
// @Tags subscriptions
// @Param id path string true "Subscription ID"
// @Success 204 "No Content"
// @Failure 401 {object} echo.HTTPError "Unauthorized"
// @Failure 403 {object} echo.HTTPError "Forbidden - not your subscription"
// @Failure 404 {object} echo.HTTPError "Subscription not found"
// @Router /api/v1/subscriptions/{id} [delete]
// @Security BearerAuth
func (h *SubscriptionHandler) Delete(c echo.Context) error {
	// require authentication
	userExternalID := GetUserExternalID(c)
	if userExternalID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// parse subscription id from path
	subIDStr := c.Param("id")
	if subIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "subscription id is required")
	}

	subID, err := domain.NewWebhookSubscriptionID(subIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid subscription id format")
	}

	userID, err := domain.ParseUserID(userExternalID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id format")
	}

	// authorization check: verify the subscription belongs to this user
	// we need to fetch all user's subscriptions and check if this id is in there
	// this is a workaround since FindByID isn't in the interface
	subs, err := h.repo.FindByUser(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify ownership")
	}

	// check if the subscription belongs to this user
	found := false
	for _, sub := range subs {
		if sub.ID().String() == subID.String() {
			found = true
			break
		}
	}

	if !found {
		// either doesn't exist or belongs to another user
		// return 404 to avoid leaking info about other users' subscriptions
		return echo.NewHTTPError(http.StatusNotFound, "subscription not found")
	}

	// delete
	if err := h.repo.Delete(c.Request().Context(), subID); err != nil {
		if err == domain.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "subscription not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete subscription")
	}

	return c.NoContent(http.StatusNoContent)
}
