package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// WebhookWorkerConfig holds configuration for the webhook dispatcher.
type WebhookWorkerConfig struct {
	// BufferSize is the size of the notification channel buffer.
	BufferSize int

	// WorkerCount is the number of concurrent workers dispatching webhooks.
	WorkerCount int

	// RequestTimeout is the max time to wait for each outgoing HTTP request.
	RequestTimeout time.Duration

	// Thresholds define when momentum changes are considered spikes.
	Thresholds domain.MomentumSpikeThresholds
}

// DefaultWebhookWorkerConfig returns sensible defaults.
func DefaultWebhookWorkerConfig() WebhookWorkerConfig {
	return WebhookWorkerConfig{
		BufferSize:     1000,
		WorkerCount:    2,
		RequestTimeout: 5 * time.Second,
		Thresholds:     domain.DefaultSpikeThresholds(),
	}
}

// WebhookWorker dispatches webhook notifications for momentum spikes.
// implements domain.NotificationService.
type WebhookWorker struct {
	spikeChan  chan domain.MomentumSpike
	subRepo    domain.WebhookSubscriptionRepository
	httpClient *http.Client
	config     WebhookWorkerConfig
	logger     *logging.Logger

	wg       sync.WaitGroup
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewWebhookWorker creates a new webhook worker.
func NewWebhookWorker(
	subRepo domain.WebhookSubscriptionRepository,
	config WebhookWorkerConfig,
	logger *logging.Logger,
) *WebhookWorker {
	return &WebhookWorker{
		spikeChan: make(chan domain.MomentumSpike, config.BufferSize),
		subRepo:   subRepo,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		config:  config,
		logger:  logger.WithComponent("webhook_worker"),
		stopped: make(chan struct{}),
	}
}

// Start begins the worker goroutines.
func (w *WebhookWorker) Start(ctx context.Context) {
	w.logger.Info("webhook worker starting",
		"buffer_size", w.config.BufferSize,
		"worker_count", w.config.WorkerCount,
		"request_timeout", w.config.RequestTimeout.String(),
	)

	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.runWorker(ctx, i)
	}
}

// Stop gracefully shuts down the worker.
func (w *WebhookWorker) Stop() {
	w.stopOnce.Do(func() {
		w.logger.Info("webhook worker stopping, draining buffer...")
		close(w.spikeChan)
		w.wg.Wait()
		close(w.stopped)
		w.logger.Info("webhook worker stopped")
	})
}

// Stopped returns a channel that closes when the worker has fully stopped.
func (w *WebhookWorker) Stopped() <-chan struct{} {
	return w.stopped
}

// NotifyMomentumSpike queues a momentum spike for notification.
// implements domain.NotificationService.
func (w *WebhookWorker) NotifyMomentumSpike(ctx context.Context, spike domain.MomentumSpike) (int, error) {
	select {
	case w.spikeChan <- spike:
		w.logger.Debug("spike queued for notification",
			"community_id", spike.CommunityID.String(),
			"new_momentum", spike.NewMomentum,
		)
		// actual count will be determined during dispatch
		// return 0 here as it's async
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		// buffer full, log and drop
		w.logger.Warn("webhook buffer full, spike dropped",
			"community_id", spike.CommunityID.String(),
		)
		return 0, nil
	}
}

// Thresholds returns the configured spike thresholds.
func (w *WebhookWorker) Thresholds() domain.MomentumSpikeThresholds {
	return w.config.Thresholds
}

// runWorker is the main worker loop.
func (w *WebhookWorker) runWorker(ctx context.Context, workerID int) {
	defer w.wg.Done()

	for {
		select {
		case spike, ok := <-w.spikeChan:
			if !ok {
				w.logger.Debug("worker exiting after drain", "worker_id", workerID)
				return
			}
			w.dispatchSpike(ctx, spike, workerID)

		case <-ctx.Done():
			w.logger.Debug("worker exiting on context cancel", "worker_id", workerID)
			return
		}
	}
}

// dispatchSpike sends webhook notifications for a spike.
func (w *WebhookWorker) dispatchSpike(ctx context.Context, spike domain.MomentumSpike, workerID int) {
	// get subscriptions for this community
	subs, err := w.subRepo.FindByCommunity(ctx, spike.CommunityID)
	if err != nil {
		w.logger.Error("failed to fetch subscriptions",
			"worker_id", workerID,
			"community_id", spike.CommunityID.String(),
			"error", err.Error(),
		)
		return
	}

	if len(subs) == 0 {
		w.logger.Debug("no subscriptions for community",
			"community_id", spike.CommunityID.String(),
		)
		return
	}

	// prepare payload
	payload := WebhookPayload{
		Event:         "momentum_spike",
		CommunityID:   spike.CommunityID.String(),
		CommunityName: spike.CommunityName,
		OldMomentum:   spike.OldMomentum,
		NewMomentum:   spike.NewMomentum,
		PercentChange: spike.PercentChange,
		Timestamp:     spike.Timestamp.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		w.logger.Error("failed to marshal payload",
			"worker_id", workerID,
			"error", err.Error(),
		)
		return
	}

	// dispatch to each subscriber
	var sent, failed int
	for _, sub := range subs {
		if w.sendWebhook(ctx, sub, payloadBytes, workerID) {
			sent++
		} else {
			failed++
		}
	}

	w.logger.Info("spike notifications dispatched",
		"worker_id", workerID,
		"community_id", spike.CommunityID.String(),
		"sent", sent,
		"failed", failed,
	)
}

// sendWebhook sends a single webhook notification.
func (w *WebhookWorker) sendWebhook(ctx context.Context, sub *domain.WebhookSubscription, payload []byte, workerID int) bool {
	// compute HMAC signature
	signature := w.computeSignature(payload, sub.Secret())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL(), bytes.NewReader(payload))
	if err != nil {
		w.logger.Error("failed to create request",
			"worker_id", workerID,
			"target_url", sub.TargetURL(),
			"error", err.Error(),
		)
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pulse-Signature", signature)
	req.Header.Set("X-Pulse-Event", "momentum_spike")
	req.Header.Set("User-Agent", "Pulse-Webhook/1.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Warn("webhook request failed",
			"worker_id", workerID,
			"target_url", sub.TargetURL(),
			"error", err.Error(),
		)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.logger.Debug("webhook delivered",
			"target_url", sub.TargetURL(),
			"status", resp.StatusCode,
		)
		return true
	}

	w.logger.Warn("webhook returned non-success status",
		"worker_id", workerID,
		"target_url", sub.TargetURL(),
		"status", resp.StatusCode,
	)
	return false
}

// computeSignature generates HMAC-SHA256 signature.
func (w *WebhookWorker) computeSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

// WebhookPayload is the JSON structure sent to webhook endpoints.
type WebhookPayload struct {
	Event         string  `json:"event"`
	CommunityID   string  `json:"community_id"`
	CommunityName string  `json:"community_name"`
	OldMomentum   float64 `json:"old_momentum"`
	NewMomentum   float64 `json:"new_momentum"`
	PercentChange float64 `json:"percent_change"`
	Timestamp     string  `json:"timestamp"`
}
