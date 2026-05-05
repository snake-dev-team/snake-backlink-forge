# Phase 7.04 — AI Content Generation

**Priority:** P0 (blocker for E2E)
**Effort:** 5-8h
**Owner:** fullstack-developer

## Overview

Replace hardcoded 1-line HTML in extension with full AI-generated article (800-1500 words VN tone). Multi-provider routing: Claude primary, OpenAI fallback.

## Files to create

- `services/api/internal/ai/claude_client.go` — Anthropic API client
- `services/api/internal/ai/openai_client.go` — OpenAI fallback
- `services/api/internal/ai/types.go` — ContentRequest, ContentResponse, AnchorVariant
- `services/api/internal/ai/router.go` — primary→fallback routing
- `services/api/internal/service/content_service.go` — orchestrator
- `services/api/internal/service/content_service_test.go` — unit test with mock AI
- `services/api/internal/api/handlers/v1_campaigns_content.go` — POST /campaigns/:id/generate-content
- `services/api/internal/migrations/20260505001_jobs_content_columns.sql` — content_body, content_title, content_meta TEXT NULL
- `services/api/internal/db/queries/jobs_content.sql` — UpdateJobContent + GetCampaignContentReadyJobs

## Files to modify

- `services/api/internal/db/sqlc/jobs.sql.go` — regen after migration + new query
- `services/api/internal/db/sqlc/querier.go` — interface update
- `services/api/internal/api/router.go` — wire content endpoint
- `services/api/internal/config/config.go` — `AnthropicAPIKey`, `OpenAIAPIKey`, `AIModelPrimary` env vars
- `services/api/cmd/api/main.go` — wire ContentService into deps
- `apps/extension/src/background/index.ts` — replace hardcoded content with `job.content_body || job.content_html` from API; skip with `no_content` if empty

## Job state extension

Add new status `content_ready` to JobStatus enum: `queued → content_ready → dispatched → in_progress → success | failed | skipped`.

After Enqueue, jobs are `queued`. Generate-content endpoint populates content_body + transitions to `content_ready`. Extension polls `content_ready` jobs (not `queued`) for claim.

## Prompt template (VN)

System: `Bạn là chuyên gia content writer SEO Việt Nam. Viết bài 800-1500 từ tone tự nhiên, qua AI detection 95%, structure: intro hook + 3-5 sections với h2/h3 + conclusion CTA. Embed anchor link tự nhiên không gượng ép. KHÔNG dùng cụm từ AI điển hình ("As an AI", "I cannot", "However, it's important to note").`

User: `Topic: {topic}. Anchor text: {anchor_text}. Money URL: {money_url}. Tone: {tone_preference}. Trả về JSON: { "title": "...", "slug": "...", "meta_description": "...", "body_html": "<h2>...</h2>...", "anchor_position": "intro|section_2|conclusion" }`

## Quality guards (content_service.go)

- `word_count >= 800 && word_count <= 1500` → else regen once
- `body_html contains anchor_text` (case-insensitive)
- `body_html contains money_url`
- No AI phrases regex: `/(as an? ai|i cannot|i can't|i'm unable|it's important to note|however, it's worth)/i`
- Cost cap: $0.05/article (Sonnet 4.6 ~$3/M input, ~$15/M output → ~3K tokens at $0.045 typical)

## Endpoint contract

**POST /api/v1/campaigns/:id/generate-content**
- Auth: Bearer token
- Body: `{ "tone_preference": "professional" }` (optional)
- Returns: `202 Accepted { "job_count": 20, "estimated_cost_usd": 0.95 }`
- Background goroutine batch-generates content; updates `jobs.content_body` + status=content_ready as it goes
- Idempotent: skip jobs with `content_body IS NOT NULL`

## Anchor rotation

Campaign exposes `anchor_texts` JSON array (already in schema). Per-job: random pick weighted by `weight` field. Already implemented in `Enqueue` via `anchors[i%len(anchors)]`. Reuse in content gen.

## Extension change

`apps/extension/src/background/index.ts:242-244`:

```ts
// BEFORE (hardcoded):
const postResult = await postBacklink(creds, {
  title: `${anchor} — sponsored review`,
  content_html: `<p>Featured: <a href="${job.money_url}" rel="nofollow sponsored">${anchor}</a></p>`,
});

// AFTER:
if (!job.content_body || !job.content_title) {
  await reportResult(settings, job.id, {
    status: "skipped",
    error_code: "no_content",
    error_message: "AI content not generated for this job",
  });
  return;
}
const postResult = await postBacklink(creds, {
  title: job.content_title,
  content_html: job.content_body,
  meta_description: job.content_meta,
});
```

`CampaignJob` type in `index.ts:8-19` adds:
```ts
content_body?: string | null;
content_title?: string | null;
content_meta?: string | null;
```

`claimedJobMap` in `v1_campaigns.go` already returns sqlc row fields — add content_body/title/meta to the map.

## Frontend (minimal — full UI in 7.06)

`apps/landing/src/app/(app)/campaigns/page.tsx`: add "Generate Content" button per campaign with `content_ready_count / total` badge.

## Test plan

1. Unit `content_service_test.go`: mock AI client returns valid JSON → verify quality guards.
2. Unit: AI returns AI-phrase content → quality guard rejects + retries once.
3. Unit: cost cap exceeded → returns ErrAIQuotaExceeded.
4. Integration (live AI, gated by `ANTHROPIC_API_KEY` env): 1 article gen → assert word count, anchor presence, JSON parse.
5. Migration test: `content_body` column nullable, no breaking change.

## Success criteria

- `go build ./services/api/...` clean
- `go test ./services/api/internal/service/... -count=1 -timeout=120s` pass
- Migration applied locally via goose up
- Extension typecheck pass with new content_body fields
- Endpoint returns 202 with job_count for valid campaign
- AI client gracefully errors if `ANTHROPIC_API_KEY` unset (returns ErrAIUnavailable; doesn't panic)

## Constraints

- DO NOT touch user WIP: `apps/landing/src/app/(app)/sites/`, `apps/landing/src/app/api/proxy/`, `apps/landing/src/lib/auth/origin.ts`, `services/api/internal/service/user_service_test.go`
- DO NOT modify existing tests
- DO NOT commit/stage/push (main session handles git)
- Follow conventional commits naming: `feat(ai): ...`
- Fly secrets for API keys must be added separately (note in report); local dev uses `.env`
