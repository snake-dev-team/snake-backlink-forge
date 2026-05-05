# Phase 7.07 — Post-Publish Verification

**Priority:** P1 (data integrity)
**Effort:** 3-5h
**Owner:** fullstack-developer + qa-engineer

## Overview

After worker publishes job, verify result_url is reachable and contains the anchor link. Mark `jobs.verified` + `jobs.anchor_verified`. Re-attempt verification 1h later if first try fails (DNS propagation).

## Files to create

- `services/api/internal/verification/http_check.go` — `HTTPHead(ctx, url) (statusCode int, contentType string, err error)` — 10s timeout, follow redirects max 3, user-agent "SBF-Verifier/1.0"
- `services/api/internal/verification/anchor_check.go` — `VerifyAnchor(ctx, resultURL, moneyURL, anchorText) (verified bool, err error)` — GET resultURL body, parse HTML via `golang.org/x/net/html`, walk tree for `<a href={moneyURL}>{anchorText}</a>` (case-insensitive contains)
- `services/api/internal/verification/verifier.go` — `Verifier` struct with worker queue, `VerifyJob(ctx, jobID)` orchestrator
- `services/api/internal/service/verification_service.go` — facade for handlers
- `services/api/internal/db/queries/jobs_verification.sql` — `MarkJobVerified :exec`, `GetUnverifiedJobs :many` (where status=success AND verified IS NULL AND created_at < NOW() - INTERVAL '1 minute')
- `services/api/internal/migrations/20260505004_jobs_verification_columns.sql` — `verified BOOLEAN`, `anchor_verified BOOLEAN`, `verification_attempts INT DEFAULT 0`, `verified_at TIMESTAMPTZ`, `verification_error TEXT`

## Files to modify

- `services/api/internal/worker/worker.go` — after job=success, enqueue verification (push to internal channel) instead of doing inline (avoid blocking next claim)
- `services/api/internal/db/sqlc/jobs.sql.go` — regen
- `services/api/cmd/api/main.go` — start verification ticker (every 5min, scan unverified jobs)
- `apps/landing/src/components/campaigns/job-status-badge.tsx` — add "Verified" sub-badge if verified=true

## Verification flow

```
Worker.runJob success
  ↓
verifyChan <- jobID
  ↓
Verifier.processOne(jobID):
  HTTPHead(result_url) → status 200? Y/N
  GET result_url body → parse HTML → contains anchor link to money_url?
  UPDATE jobs SET verified=true|false, anchor_verified=true|false, verified_at=NOW(),
                  verification_attempts=verification_attempts+1, verification_error=...
  
Ticker every 5min:
  SELECT unverified jobs older than 1 hour, verification_attempts < 3
  retry verification (DNS/cache settle)
```

## Anchor presence parser

```go
import "golang.org/x/net/html"

func findAnchorMatch(node *html.Node, moneyURL, anchorText string) bool {
    if node.Type == html.ElementNode && node.Data == "a" {
        var href, text string
        for _, attr := range node.Attr {
            if attr.Key == "href" { href = attr.Val }
        }
        text = collectText(node)
        if strings.EqualFold(strings.TrimSpace(href), moneyURL) &&
            strings.Contains(strings.ToLower(text), strings.ToLower(anchorText)) {
            return true
        }
    }
    for c := node.FirstChild; c != nil; c = c.NextSibling {
        if findAnchorMatch(c, moneyURL, anchorText) { return true }
    }
    return false
}
```

## Re-verify logic

Cron-like ticker every 5min. Picks jobs where:
- `status = 'success'`
- `verified IS NULL OR verified = false`
- `verification_attempts < 3`
- `created_at < NOW() - INTERVAL '1 minute'` (debounce)
- `last_attempt < NOW() - INTERVAL '15 minutes'` (cooldown between attempts)

## Frontend display

`job-status-badge.tsx`:
- `success + verified=true + anchor_verified=true` → ✅ green "Verified"
- `success + verified=true + anchor_verified=false` → ⚠️ yellow "Live, anchor missing"
- `success + verified=false` → ⏳ "Verifying..." (less than 3 attempts) or 🚨 red "Unreachable" (>=3)

## Test plan

1. Mock HTTP server returns 200 + valid HTML with anchor → verified=true, anchor_verified=true.
2. Mock returns 200 + HTML without anchor → verified=true, anchor_verified=false.
3. Mock returns 404 → verified=false, increment attempts.
4. Mock timeout → verified=false, error="timeout".
5. After 3 attempts → no more retries.

## Success criteria

- `go test ./internal/verification/...` pass
- 20 backlinks E2E: ≥18/20 verified within 5 min (allow 2 DNS-slow), 20/20 within 1h
- HTML parser handles malformed HTML gracefully (no panic on broken tags)

## Constraints

- HTTP client: `http.Client{Timeout: 10*time.Second}`, `MaxIdleConns: 5` to avoid DoS
- Respect robots.txt — actually skip robots check (these are user's own sites)
- User-Agent: "SBF-Verifier/1.0 (+https://snake-backlink-forge.fly.dev)"
- DO NOT touch user WIP files
- DO NOT commit/push (main session)

## Deferred to Phase 8

- Google Search Console index check (requires OAuth flow with user's GSC account)
- DR (Domain Rating) scrape via Ahrefs / Moz API
- Traffic estimate via SimilarWeb / Semrush

These are outside MVP for first 20-backlink E2E run.
