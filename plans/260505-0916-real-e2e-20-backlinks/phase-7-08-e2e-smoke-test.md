# Phase 7.08 — E2E Smoke Test 20 Backlinks

**Priority:** P0 (final ship gate)
**Effort:** 2-3h
**Owner:** qa-engineer

## Overview

Run real end-to-end test with 20 connected WP sites. Capture metrics, write runbook, identify any missed integration issues from prior phases.

## Prerequisites (user-provided)

- 20 WordPress sites with App Passwords ready
- Application Password generated for each
- Site URLs + credentials ready to paste into `/sites/connect`
- ~50 standard credits in wallet (covers 20 jobs @ 1 credit + 30 buffer)
- ANTHROPIC_API_KEY set on Fly secrets (production) or `.env` (local dev)

If user can't provide 20 real sites, fallback: spin up 20 Docker WordPress instances locally (docker-compose.test-wp.yml). See "Docker fallback" section.

## Test scenario

1. **Setup phase** (~10min user effort):
   - User logs in to landing app
   - Navigates to /sites
   - Adds 20 WP sites via Connect form (paste URL + App username + App password)
   - Verifies all 20 show status=connected (revalidate if any fail)

2. **Campaign create** (~2min):
   - Navigates to /campaigns
   - Click "New Campaign"
   - Fill form:
     - Topic: "forex broker review vietnam"
     - Money URL: https://test-affiliate.example.com
     - Anchor texts (3 variants): "broker uy tín", "sàn forex tốt nhất", "review broker"
     - Pool: standard
     - Source mode: custom
     - Sites: select all 20
     - Quantity: 20
     - Tone: storytelling
     - Start now: ✅
   - Submit → expect campaign created + 20 jobs queued

3. **AI content generation** (~3-5min):
   - Click "Generate Content"
   - Watch progress: "5/20 generated"... "20/20 generated"
   - Verify each job.content_body is 800+ words

4. **Worker publishing** (~2-5min):
   - Worker auto-claims content_ready jobs
   - Watch /campaigns/:id polling: jobs transition `content_ready → in_progress → success`
   - If any fail: inspect error_message, retry via `/jobs/:id/retry` if recoverable

5. **Verification** (~1-5min):
   - Verifier auto-checks each result_url
   - Watch verified flags flip to ✅
   - Manual spot-check 3 random URLs in browser

6. **Capture metrics**:
   - Total wall-clock time start→last verified
   - AI cost (sum from Claude API logs)
   - Failure rate
   - Avg time per job (worker)
   - Avg time to verify
   - Any flaky tests?

## Files to create

- `docs/runbooks/e2e-20-backlinks-test.md` — full runbook with screenshots
- `plans/260505-0916-real-e2e-20-backlinks/test-results/run-1.json` — captured metrics
- `docker-compose.test-wp.yml` — fallback 20-site local WP cluster (only if needed)

## Pass criteria

- ≥18/20 jobs reach status=success (allow 2 site-specific failures)
- ≥18/20 jobs have verified=true within 5min
- Total AI cost <$1.00 for 20 articles
- Total wall-clock <30min for full E2E
- No data corruption (no duplicate jobs, no orphan content_body)
- Worker handles graceful shutdown mid-run cleanly

## Failure handling

If <18 jobs succeed:
1. Inspect logs from Fly (or local docker logs)
2. Group failures by error_code
3. Top 1 failure mode → spawn debugger agent → fix → retry
4. Document failure modes in runbook

## Bug fix loop

```
fail rate >2/20:
  /ck:debug
  /ck:fix --review
  re-run test
  3 attempts max → halt + escalate
```

## Test data cleanup

After successful run:
- DO NOT delete user's WP sites (they may want to reuse)
- DO NOT delete campaign / jobs (record for audit trail)
- DO archive the test campaign so it doesn't keep enqueueing

## Memory journal entry

`docs/journals/2026-05-05-real-e2e-20-backlinks.md`:
- What worked first try
- What broke + how fixed
- Top performance bottleneck
- Cost actuals vs estimate
- User experience note (UX painpoints)
- Next iteration improvements

## Constraints

- Live test only — no mocks at this phase
- User must approve test campaign URL choice (not random spam target)
- Capture screenshots at each phase for runbook
- Audit log every action (already wired via existing audit_log table)
