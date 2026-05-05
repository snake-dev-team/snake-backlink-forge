// v1_worker_health.go: GET /health/worker — returns worker queue depth, in-flight count,
// and last completed timestamp. Public endpoint (no auth); safe to expose to load balancers.
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type workerHealthResponse struct {
	Enabled           bool       `json:"enabled"`
	WorkerID          string     `json:"worker_id,omitempty"`
	QueueDepth        int32      `json:"queue_depth"`
	InFlight          int32      `json:"in_flight"`
	CompletedLast5Min int32      `json:"completed_last_5min"`
	FailedLast5Min    int32      `json:"failed_last_5min"`
	LastCompletedAt   *time.Time `json:"last_completed_at,omitempty"` // nil when no jobs completed yet
}

// V1WorkerHealth returns current worker health metrics from the DB.
// Does not require authentication — intended for load balancer health checks and ops dashboards.
// Returns 200 with enabled:false when the worker is not configured (WorkerEnabled=false).
func V1WorkerHealth(deps *ApiHandlerDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// When worker is disabled or DB unavailable, return minimal enabled:false response.
		if deps.Queries == nil {
			return c.JSON(workerHealthResponse{Enabled: false})
		}

		if !deps.WorkerEnabled {
			return c.JSON(workerHealthResponse{Enabled: false})
		}

		ctx := c.Context()
		row, err := deps.Queries.GetWorkerHealth(ctx)
		if err != nil {
			deps.Log.Warn("GetWorkerHealth query failed", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "worker health unavailable",
			})
		}

		resp := workerHealthResponse{
			Enabled:           true,
			WorkerID:          deps.WorkerID,
			QueueDepth:        row.QueueDepth,
			InFlight:          row.InFlight,
			CompletedLast5Min: row.CompletedLast5min,
			FailedLast5Min:    row.FailedLast5min,
		}

		// LastCompletedAt is interface{} because sqlc cannot infer MAX(timestamptz) type.
		// Driver returns time.Time when non-NULL, nil when no rows. Type-assert defensively.
		if t, ok := row.LastCompletedAt.(time.Time); ok && !t.IsZero() {
			resp.LastCompletedAt = &t
		}

		return c.JSON(resp)
	}
}
