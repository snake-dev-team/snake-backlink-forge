# Phase 7.05 — Embedded Go Worker

**Priority:** P0 (auto-execution; extension fallback)
**Effort:** 10-15h
**Owner:** fullstack-developer

## Overview

Add server-side goroutine that polls `content_ready` jobs, claims via `FOR UPDATE SKIP LOCKED`, executes WP REST publish, reports result. Replaces extension-only Chrome alarm path (extension remains as fallback).

## Architecture

Embedded goroutine in `services/api/cmd/api/main.go` — NOT a separate Fly machine for MVP. Spawn-on-startup, graceful shutdown via context cancellation.

```
WorkerPool (max 5 concurrent)
  ├── pollLoop ticker (10s) → ClaimNextBatch(ctx, limit=5)
  ├── per-job goroutine: WP REST publish → Report
  └── retry queue: failed jobs with retry_count<3 → re-enqueue with backoff
```

## Files to create

- `services/api/internal/worker/worker.go` — `Worker` struct, `Start(ctx)`, `Stop(ctx)`, pool loop
- `services/api/internal/worker/job_runner.go` — `runJob(ctx, job)` — fetch wp_site creds → WP REST POST → Report
- `services/api/internal/worker/wp_client.go` — `PostArticle(ctx, creds, ContentRequest) (PublishResult, error)` — reuses logic from `apps/extension/src/wp/poster.ts` ported to Go
- `services/api/internal/worker/retry.go` — exponential backoff: `2^retry_count * 30s` capped at 30min
- `services/api/internal/worker/rate_limiter.go` — per-site rate limit (max 2 posts/min/site) + global (100/min)
- `services/api/internal/worker/worker_test.go` — table-driven, mock WP server via httptest
- `services/api/internal/db/queries/worker_jobs.sql` — `ClaimContentReadyJobs :many` (status='content_ready' FOR UPDATE SKIP LOCKED LIMIT $1), `MarkJobInProgress :exec`, `IncrementJobRetry :exec`, `MoveJobToDLQ :exec`
- `services/api/internal/migrations/20260505002_jobs_worker_columns.sql` — `lease_until TIMESTAMPTZ`, `lease_holder TEXT`, `retry_count INT DEFAULT 0`, `last_error_at TIMESTAMPTZ`, add 'dlq' to job_status enum
- `services/api/internal/api/handlers/v1_worker_health.go` — GET /health/worker → queue depth, in-flight, last_completed

## Files to modify

- `services/api/cmd/api/main.go` — start Worker on boot if `WORKER_ENABLED=true`, defer Stop on shutdown
- `services/api/internal/config/config.go` — `WorkerEnabled bool`, `WorkerPollInterval time.Duration` (default 10s), `WorkerMaxConcurrent int` (default 5), `WorkerRetryLimit int` (default 3)
- `services/api/internal/db/sqlc/jobs.sql.go` — regen
- `services/api/internal/api/router.go` — wire /health/worker

## Worker poll loop pseudo

```go
ticker := time.NewTicker(s.cfg.WorkerPollInterval)
sem := make(chan struct{}, s.cfg.WorkerMaxConcurrent)
for {
  select {
  case <-ctx.Done(): return
  case <-ticker.C:
    rows, err := s.q.ClaimContentReadyJobs(ctx, ClaimContentReadyJobsParams{
      Limit: int32(s.cfg.WorkerMaxConcurrent),
      LeaseHolder: s.workerID, // hostname-pid
    })
    for _, row := range rows {
      sem <- struct{}{}
      go func(j Job) {
        defer func() { <-sem }()
        s.runJob(ctx, j)
      }(row)
    }
  }
}
```

## ClaimContentReadyJobs SQL

```sql
-- name: ClaimContentReadyJobs :many
WITH claimed AS (
  SELECT id FROM jobs
  WHERE status = 'content_ready'
    AND (lease_until IS NULL OR lease_until < NOW())
  ORDER BY created_at ASC
  LIMIT sqlc.arg(limit)
  FOR UPDATE SKIP LOCKED
)
UPDATE jobs
SET status = 'in_progress',
    lease_until = NOW() + INTERVAL '5 minutes',
    lease_holder = sqlc.arg(lease_holder)::text,
    updated_at = NOW()
FROM claimed
WHERE jobs.id = claimed.id
RETURNING jobs.*;
```

## Per-site rate limit

In-memory token bucket: `map[siteDomain]*time.Time` last-post timestamp. Min 30s gap between posts to same site.

## Retry logic

On `runJob` failure:
- HTTP 4xx (auth, validation) → no retry, mark `failed` immediately
- HTTP 5xx, network timeout, parse error → retry_count++. If <3, re-set status='content_ready' + lease_until=NOW()+backoff. If >=3, status='dlq', send Telegram alert.

## DLQ alert

`services/api/internal/notification/dlq_alert.go` — call existing TelegramService to message campaign owner: "Job {id} failed after 3 retries: {error_reason}".

## Health endpoint

```json
GET /health/worker
{
  "enabled": true,
  "queue_depth": 47,
  "in_flight": 3,
  "completed_last_5min": 12,
  "failed_last_5min": 1,
  "last_completed_at": "2026-05-05T09:33:12Z",
  "worker_id": "ip-10-0-0-1-pid-1234"
}
```

## Test plan

1. Unit `worker_test.go`: mock WP server returns 201 → assert job marked success, result_url populated.
2. Unit: mock WP returns 401 → assert no retry, job=failed.
3. Unit: mock WP returns 502 → retry once (retry_count=1, backoff verified).
4. Unit: 3 failures → DLQ status, alert sent.
5. Concurrency: 10 goroutines claim from 5 jobs → exactly 5 succeed (FOR UPDATE SKIP LOCKED works).
6. Rate limiter: 5 jobs same site → first 2 immediate, 3rd waits 30s.

## Success criteria

- `go test ./internal/worker/...` pass with race detector
- 20-job smoke test in <5 min wall-clock
- `/health/worker` returns 200 with valid JSON
- Graceful shutdown: SIGTERM → drain in-flight → exit clean within 30s

## Constraints

- Reuse existing `wp_site_service.GetByDomainPlain` for creds lookup
- Reuse existing `job_service.Report` for result persistence (don't duplicate)
- Worker must not block API server startup (spawn detached goroutine)
- DO NOT touch user WIP files (same list as 7.04)
- DO NOT commit/push (main session)
