# Phase 7.06 — Frontend Site Multi-Select + Auto-Enqueue + Progress Polling

**Priority:** P0 (UX gap from audit)
**Effort:** 3-5h
**Owner:** fullstack-developer + ui-ux-designer

## Overview

Add site multi-select to campaign create form, quantity slider, tone preference dropdown. Fix `start_now=true` to auto-enqueue jobs. Replace force-dynamic refresh with TanStack Query polling 5s.

## Files to create

- `apps/landing/src/components/campaigns/site-multi-select.tsx` — checkbox list of user's connected wp_sites with select-all
- `apps/landing/src/components/campaigns/quantity-slider.tsx` — slider 1-100 with cost preview
- `apps/landing/src/components/campaigns/tone-select.tsx` — dropdown professional/casual/storytelling/technical
- `apps/landing/src/components/campaigns/job-status-badge.tsx` — queued/content_ready/in_progress/success/failed pill
- `apps/landing/src/components/campaigns/campaign-progress-bar.tsx` — "12/20 done" with percentage
- `apps/landing/src/hooks/use-campaign-progress.ts` — TanStack Query hook polling /campaigns/:id every 5s, stops when status=completed/all_done

## Files to modify

- `apps/landing/src/app/(app)/campaigns/campaign-create-form.tsx` — add site-multi-select, quantity-slider, tone-select. Submit calls action with these fields.
- `apps/landing/src/app/(app)/campaigns/actions.ts` — `createCampaignAction` Zod schema add `site_ids: string[]`, `quantity: number`, `tone_preference: string`. POST /campaigns then call /campaigns/:id/enqueue with quantity if start_now=true.
- `apps/landing/src/app/(app)/campaigns/[id]/page.tsx` (create if not exists) — campaign detail with progress bar + job table polling
- `apps/landing/src/app/(app)/campaigns/page.tsx` — remove `force-dynamic`, use TanStack Query SSR-hydration pattern instead
- `services/api/internal/service/campaign_service.go` — `Create()` if `in.StartNow` → call `JobService.Enqueue(ctx, userID, c.ID, in.Quantity)` after campaign row insert. Atomic: if enqueue fails, rollback campaign too (via tx).
- `services/api/internal/api/handlers/v1_campaigns.go` — POST /campaigns request body adds `quantity int32` + `site_ids []uuid.UUID`. Pass through.
- `services/api/internal/db/queries/campaigns.sql` — `CreateCampaign` add `site_ids` jsonb column (or new `campaign_target_sites` join table — prefer join table for normalization)

## New table: campaign_target_sites

`services/api/internal/migrations/20260505003_campaign_target_sites.sql`:

```sql
CREATE TABLE campaign_target_sites (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    wp_site_id UUID NOT NULL REFERENCES wp_sites(id) ON DELETE CASCADE,
    PRIMARY KEY (campaign_id, wp_site_id)
);
CREATE INDEX idx_campaign_target_sites_campaign ON campaign_target_sites(campaign_id);
```

When `Enqueue` runs: instead of `PickTargetsForCampaign` (which uses generic targets table), join `campaign_target_sites` × `wp_sites` to get user's selected sites. Each wp_site = 1 target.

This shifts the model: targets are user's WP sites, not third-party blogs.

## Backward compat

Old campaigns without `campaign_target_sites` rows keep working with `targets` table picker. New campaigns use site_ids picker. Add SQL: `PickTargetSitesForCampaign` if site_ids exist, else fallback to `PickTargetsForCampaign`.

## Cost preview

Frontend: `quantity * 1 credit` (standard pool) or `quantity * 2 credits` (premium). Display under slider live.

Server-side estimated AI cost in /campaigns/:id/generate-content response: `quantity * 0.045 USD` (Claude Sonnet typical). Frontend shows: "Estimated AI cost: $0.95".

## TanStack Query polling

```tsx
const { data, isLoading } = useQuery({
  queryKey: ["campaign", campaignId, "progress"],
  queryFn: () => fetcher(`/api/v1/campaigns/${campaignId}`),
  refetchInterval: (query) => {
    const status = query.state.data?.status;
    if (status === "completed" || status === "archived") return false;
    return 5000;
  },
});
```

## UI/UX (delegate to ui-ux-designer)

Match existing UI B aesthetic: violet primary, glass surfaces, Geist font. Use shadcn Checkbox + Slider + Select components.

Progress bar: animated gradient fill, percentage label, "12/20 backlinks live" text.

Job table: sortable columns (status, target_url, created_at), click row → expand evidence preview.

## Test plan

1. Vitest unit: `site-multi-select` renders connected sites, calls onChange.
2. Vitest unit: `quantity-slider` cost preview updates on change.
3. Playwright E2E: full create campaign flow with 2 sites + quantity=5 → assert 5 jobs created, status=queued.
4. Vitest hook: `use-campaign-progress` mocked fetcher returns running → polls every 5s; status=completed → stops polling.

## Success criteria

- `pnpm typecheck` clean
- `pnpm test` (Vitest + Playwright) pass
- Campaign create with start_now=true creates campaign + N jobs in single API roundtrip
- Progress page updates without manual refresh

## Constraints

- DO NOT touch user WIP: `apps/landing/src/app/(app)/sites/`, `apps/landing/src/app/api/proxy/`, `apps/landing/src/lib/auth/origin.ts`
- DO NOT modify Phase 6 layout files
- Reuse existing shadcn components, don't reinvent
- Mobile responsive (max-w-md form constraint)
