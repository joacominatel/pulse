package worker

import (
	"context"
	"sync"
	"time"

	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// MetricsRecorder abstracts prometheus metrics for the ingestion worker.
// keeps worker decoupled from metrics package.
type MetricsRecorder interface {
	RecordEventIngested(communityID, eventType string)
	SetBufferSize(size int)
}

// EventIngestionWorkerConfig holds configuration for the ingestion worker.
type EventIngestionWorkerConfig struct {
	// BufferSize is the size of the event channel buffer.
	// larger buffer = more events can queue before blocking
	BufferSize int

	// BatchSize is the number of events to accumulate before flushing.
	BatchSize int

	// FlushInterval is the maximum time to wait before flushing a partial batch.
	FlushInterval time.Duration

	// WorkerCount is the number of concurrent workers processing events.
	WorkerCount int
}

// DefaultEventIngestionConfig returns sensible defaults for the worker.
func DefaultEventIngestionConfig() EventIngestionWorkerConfig {
	return EventIngestionWorkerConfig{
		BufferSize:    10000, // support high burst traffic
		BatchSize:     100,   // larger batches for efficiency
		FlushInterval: 500 * time.Millisecond,
		WorkerCount:   4, // more workers for parallel DB writes
	}
}

// EventIngestionWorker processes activity events from a buffered channel.
// implements batch saving to reduce database roundtrips.
type EventIngestionWorker struct {
	eventChan chan *domain.ActivityEvent
	repo      domain.ActivityEventRepository
	config    EventIngestionWorkerConfig
	logger    *logging.Logger
	metrics   MetricsRecorder

	wg       sync.WaitGroup
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewEventIngestionWorker creates a new event ingestion worker.
func NewEventIngestionWorker(
	repo domain.ActivityEventRepository,
	config EventIngestionWorkerConfig,
	logger *logging.Logger,
) *EventIngestionWorker {
	return &EventIngestionWorker{
		eventChan: make(chan *domain.ActivityEvent, config.BufferSize),
		repo:      repo,
		config:    config,
		logger:    logger.WithComponent("event_ingestion_worker"),
		stopped:   make(chan struct{}),
	}
}

// WithMetrics sets the metrics recorder for observability.
func (w *EventIngestionWorker) WithMetrics(m MetricsRecorder) *EventIngestionWorker {
	w.metrics = m
	return w
}

// EventChannel returns the channel for submitting events.
// use this to push events from the use case.
func (w *EventIngestionWorker) EventChannel() chan<- *domain.ActivityEvent {
	return w.eventChan
}

// Start begins the worker goroutines.
// call this before accepting events.
func (w *EventIngestionWorker) Start(ctx context.Context) {
	w.logger.Info("event ingestion worker starting",
		"buffer_size", w.config.BufferSize,
		"batch_size", w.config.BatchSize,
		"flush_interval", w.config.FlushInterval.String(),
		"worker_count", w.config.WorkerCount,
	)

	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.runWorker(ctx, i)
	}
}

// Stop gracefully shuts down the worker, draining remaining events.
func (w *EventIngestionWorker) Stop() {
	w.stopOnce.Do(func() {
		w.logger.Info("event ingestion worker stopping, draining buffer...")

		// close the channel to signal workers to drain and exit
		close(w.eventChan)

		// wait for all workers to finish
		w.wg.Wait()

		close(w.stopped)
		w.logger.Info("event ingestion worker stopped")
	})
}

// Stopped returns a channel that closes when the worker has fully stopped.
func (w *EventIngestionWorker) Stopped() <-chan struct{} {
	return w.stopped
}

// QueueSize returns the current number of events waiting in the buffer.
func (w *EventIngestionWorker) QueueSize() int {
	return len(w.eventChan)
}

// runWorker is the main worker loop.
func (w *EventIngestionWorker) runWorker(ctx context.Context, workerID int) {
	defer w.wg.Done()

	batch := make([]*domain.ActivityEvent, 0, w.config.BatchSize)
	ticker := time.NewTicker(w.config.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		w.flushBatch(ctx, batch, workerID)
		batch = batch[:0] // reset slice, keep capacity
	}

	for {
		select {
		case event, ok := <-w.eventChan:
			if !ok {
				// channel closed, flush remaining and exit
				flush()
				w.logger.Debug("worker exiting after drain", "worker_id", workerID)
				return
			}

			batch = append(batch, event)

			// flush if batch is full
			if len(batch) >= w.config.BatchSize {
				flush()
			}

		case <-ticker.C:
			// flush partial batch on timeout
			flush()

		case <-ctx.Done():
			// context cancelled, flush and exit
			flush()
			w.logger.Debug("worker exiting on context cancel", "worker_id", workerID)
			return
		}
	}
}

// flushBatch persists a batch of events to the database.
func (w *EventIngestionWorker) flushBatch(ctx context.Context, batch []*domain.ActivityEvent, workerID int) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()

	// use bulk insert for efficiency
	err := w.repo.SaveBatch(ctx, batch)
	duration := time.Since(start)

	if err != nil {
		w.logger.Error("batch save failed",
			"worker_id", workerID,
			"batch_size", len(batch),
			"error", err.Error(),
			"duration_ms", duration.Milliseconds(),
		)
		return
	}

	// record metrics for successfully saved events
	if w.metrics != nil {
		for _, event := range batch {
			w.metrics.RecordEventIngested(event.CommunityID().String(), string(event.EventType()))
		}
		// update buffer size after flush
		w.metrics.SetBufferSize(len(w.eventChan))
	}

	w.logger.Debug("batch flushed",
		"worker_id", workerID,
		"batch_size", len(batch),
		"duration_ms", duration.Milliseconds(),
	)
}

// Stats returns current worker statistics.
type IngestionStats struct {
	QueueSize   int
	BufferSize  int
	WorkerCount int
}

// Stats returns current worker statistics.
func (w *EventIngestionWorker) Stats() IngestionStats {
	return IngestionStats{
		QueueSize:   len(w.eventChan),
		BufferSize:  w.config.BufferSize,
		WorkerCount: w.config.WorkerCount,
	}
}
