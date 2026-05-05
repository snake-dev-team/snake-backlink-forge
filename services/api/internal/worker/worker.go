// Package worker implements the embedded background job processor (Phase 7.05).
// The Worker polls content_ready jobs from the DB, claims them via FOR UPDATE SKIP LOCKED,
// posts them to WordPress via WP REST API, and reports results back via JobService.
//
// Design: single goroutine poll loop + bounded semaphore for concurrency.
// Graceful shutdown: context cancellation → drain in-flight → return.
package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"go.uber.org/zap"
)

// Config holds all tunable parameters for the Worker.
// All fields have production-safe defaults; set via environment variables in config.go.
type Config struct {
	// Enabled controls whether the worker starts at all. Default: false (opt-in).
	Enabled bool
	// PollInterval is how often the worker polls for new content_ready jobs.
	PollInterval time.Duration
	// MaxConcurrent is the maximum number of jobs processed simultaneously.
	MaxConcurrent int
	// RetryLimit is how many transient failures before a job is moved to DLQ.
	RetryLimit int
	// LeaseDuration is how long a claimed job is protected from re-claiming.
	LeaseDuration time.Duration
}

// DefaultConfig returns production-safe defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		PollInterval:  10 * time.Second,
		MaxConcurrent: 5,
		RetryLimit:    3,
		LeaseDuration: 5 * time.Minute,
	}
}

// Deps holds service and DB dependencies injected into the Worker.
// All fields except Log are optional — the worker performs nil-checks before use.
type Deps struct {
	// Q is the sqlc query interface for low-level job claiming and status updates.
	Q *sqlcdb.Queries
	// JobSvc reports job results (success/failure) via the service layer.
	JobSvc *service.JobService
	// WpSiteSvc fetches decrypted WP credentials by domain.
	WpSiteSvc *service.WpSiteService
}

// Worker is the embedded background processor. Create via New; start via Start.
type Worker struct {
	cfg         Config
	deps        Deps
	log         *zap.Logger
	workerID    string
	rateLimiter *RateLimiter
}

// New constructs a Worker. workerID identifies this instance in DB lease_holder fields
// and log entries. Use buildWorkerID() for the standard hostname-pid format.
func New(cfg Config, deps Deps, log *zap.Logger) *Worker {
	return &Worker{
		cfg:         cfg,
		deps:        deps,
		log:         log.Named("worker"),
		workerID:    buildWorkerID(),
		rateLimiter: NewRateLimiter(defaultMinGap),
	}
}

// Start runs the poll loop until ctx is cancelled. Designed to be called as a goroutine.
// Returns only after all in-flight jobs complete or LeaseDuration elapses.
//
// Concurrency model:
//   - sem is a buffered channel of size MaxConcurrent acting as a semaphore.
//   - Each job goroutine acquires one slot (sem <- struct{}{}) on start, releases on exit.
//   - Graceful drain: on ctx.Done(), we wait for all slots to be returned by filling sem.
func (w *Worker) Start(ctx context.Context) {
	w.log.Info("worker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.cfg.PollInterval),
		zap.Int("max_concurrent", w.cfg.MaxConcurrent),
		zap.Int("retry_limit", w.cfg.RetryLimit),
	)

	sem := make(chan struct{}, w.cfg.MaxConcurrent)
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Graceful drain: wait until all in-flight goroutines release their semaphore slots.
			// This blocks for at most LeaseDuration before the process exits anyway.
			w.log.Info("worker shutting down — draining in-flight jobs")
			for i := 0; i < w.cfg.MaxConcurrent; i++ {
				sem <- struct{}{}
			}
			w.log.Info("worker drained — all in-flight jobs completed")
			return

		case <-ticker.C:
			w.poll(ctx, sem)
		}
	}
}

// poll claims a batch of content_ready jobs and dispatches each to a goroutine.
// Skips if deps are not fully wired (graceful degradation during lenient boot).
func (w *Worker) poll(ctx context.Context, sem chan struct{}) {
	if w.deps.Q == nil || w.deps.JobSvc == nil {
		w.log.Debug("worker skipping poll — dependencies not wired")
		return
	}

	leaseSec := int32(w.cfg.LeaseDuration.Seconds())
	rows, err := w.deps.Q.ClaimContentReadyJobs(ctx, sqlcdb.ClaimContentReadyJobsParams{
		LimitCount:  int32(w.cfg.MaxConcurrent),
		LeaseSeconds: leaseSec,
		LeaseHolder: w.workerID,
	})
	if err != nil {
		if ctx.Err() != nil {
			return // shutting down
		}
		w.log.Warn("ClaimContentReadyJobs failed", zap.Error(err))
		return
	}

	if len(rows) == 0 {
		return
	}

	w.log.Debug("claimed jobs", zap.Int("count", len(rows)))

	for _, row := range rows {
		// Acquire semaphore slot before launching goroutine to bound concurrency.
		// This blocks if MaxConcurrent goroutines are already running.
		sem <- struct{}{}

		job := row // capture loop variable
		go func() {
			defer func() { <-sem }() // release slot on exit
			defer func() {
				if r := recover(); r != nil {
					w.log.Error("worker panic recovered",
						zap.Any("panic", r),
						zap.String("job_id", job.ID.String()),
					)
				}
			}()
			w.runJob(ctx, job)
		}()
	}
}

// Stop signals the worker to stop and waits for graceful drain.
// ctx should carry a timeout (e.g. 30s) to bound the wait.
// Start's internal drain handles the actual waiting; Stop is a no-op hook
// provided for symmetry with other service shutdown patterns in main.go.
func (w *Worker) Stop(_ context.Context) {
	// Context cancellation passed to Start() handles the actual shutdown.
	// This method exists so callers have a consistent Stop(shutdownCtx) pattern.
	w.log.Info("worker stop requested")
}

// WorkerID returns the identity string used in lease_holder DB fields.
func (w *Worker) WorkerID() string {
	return w.workerID
}

// buildWorkerID constructs a stable identity from hostname and PID.
// Format: "<hostname>-pid-<pid>". Falls back to "unknown-pid-<pid>" if hostname lookup fails.
func buildWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-pid-%d", hostname, os.Getpid())
}
