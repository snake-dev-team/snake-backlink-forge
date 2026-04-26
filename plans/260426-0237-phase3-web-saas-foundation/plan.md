---
title: "Phase 3 Web App SaaS Foundation"
description: "Pivot from Chrome extension to Next.js web SaaS — login via API key paste, dashboard, WP sites connect, landing revamp"
status: pending
priority: P0
effort: 30h + Phase 0 prereq waiting
branch: dev
tags: [phase3, web, nextjs, saas, foundation]
created: 2026-04-26
updated: 2026-04-26
blockedBy: []
blocks: [phase-4-ai-content, phase-5-wp-publish]
---

## Scope

Bootstrap web SaaS foundation: rename `apps/landing/` → `apps/web/`, install Next.js 15.5 + Tailwind v4 + shadcn/ui (Button, Input, Card, Label, Sheet, DropdownMenu — RT-R2: revert-R1-F-shadcn-slim) + Hey API codegen, extend Phase 2 Go Fiber with `/api/v1/*` namespace + bearer-key middleware (NO Redis cache) + rate-limited `/auth/verify` + strict CORS allowlist. Ship login (paste sbf_live_* key with `__Host-` cookie + Origin check incl. VERCEL_URL preview fallback — RT-R2: F5 + sanitizeNext consumer + 1MB proxy body cap — RT-R2: F8), dashboard (pure RSC, React.cache() dedup `/me` between layout+dashboard — RT-R2: F-bundled-cache-dedup, no TanStack Query), WP sites connect (SSRF + DNS rebinding TOCTOU-fix — RT-R2: F2 + diagnostic + AES-GCM single master key — RT-R2: revert-R1-F7), VN-only landing (next-intl deferred to Phase 9), deploy on Vercel Pro + Cloudflare, Sentry FE only with hardening (BE deferred Phase 4) + Plausible + Playwright tests (no Vitest, no testcontainers, no Lighthouse CI — RT-R2: F4) + CI grep guard against TanStack Query — RT-R2: F3. NO email auth, NO Telegram OAuth widget — Telegram-only identity stays. NO `/settings` page (RT-R2: F11 — UserMenu absorbs).

## Architecture decisions (locked, post-RT-R2)

- **Auth UX:** Paste `sbf_live_*` API key in `/login` → `__Host-sbf_key` cookie. No deep-link round-trip, no Telegram WebApp embed.
- **Identity:** Telegram-only. `user_id = tg_id` source of truth. No `email` column added.
- **App path:** `apps/landing/` → `apps/web/`. Package name `@sbf/web`.
- **Stack FE:** Next.js 15.5 + React 19 + Tailwind v4 (CSS-first OKLCH) + shadcn/ui 6 components (Button/Input/Card/Label/Sheet/DropdownMenu — RT-R2: revert-R1-F-shadcn-slim). Sheet + DropdownMenu chosen over native `<details>` for proper a11y + CSP-friendly (no inline scripts). No next-intl in Phase 3 (RT-R1: F8 — deferred Phase 9). No TanStack Query (RT-R1: F13 — pure RSC; RT-R2: F3 enforced via CI grep guard). React.cache() dedupes `/me` between layout+dashboard (RT-R2: F-bundled-cache-dedup).
- **Type contract:** Hand-written OpenAPI 3.1 at `packages/shared-types/openapi.yaml` + Hey API codegen → TS in `packages/shared-types/src/generated/`. NO Go contract test (RT-R2: F12 — kin-openapi dropped; Playwright E2E + Hey API codegen catches drift).
- **Backend:** New `services/api/internal/middleware/auth_apikey.go` (SHA-256 hash, NO Redis cache — RT-R1: F5). `/api/v1/*` Fiber sub-router. Public routes outside group. CORS env-driven strict allowlist (RT-R1: F1 — no wildcards). Trusted proxies tightened to Fly CIDR with `EnableTrustedProxyCheck: true` (RT-R1: F-X-F-F). Rate limit `/auth/verify` 5/min/IP (RT-R1: F2). WP_ENC_KEY hex format validated at boot (RT-R2: F1).
- **FE auth flow:** `middleware.ts` redirect gate (auth-only, no intl) + Route Handler proxy `/api/proxy/[...path]/route.ts` forwards cookie key as `Authorization: Bearer ...` + 1MB body cap (RT-R2: F8). Verify endpoint Origin check + VERCEL_URL preview fallback (RT-R2: F5) + SameSite=Strict cookie (RT-R1: F10). `sanitizeNext()` deep checks at consumer (RT-R1: F9). Env loader `lib/env.ts` Zod-validated (RT-R2: F-bundled-env-loader).
- **WP sites:** AES-256-GCM with single master key from `WP_ENC_KEY` (32 bytes hex) — random 12-byte nonce per encrypt, NO HKDF, NO version, NO rotation infra (RT-R2: revert-R1-F7 — rotation = Phase 11 ticket). SSRF-blocked custom transport rejects private/loopback/link-local IPs after DNS resolution AND rewrites addr to validated IP literal to eliminate DNS rebinding TOCTOU (RT-R2: F2). `/wp-json/` root probe diagnostic (RT-R1: F12). Single attempt per HTTP call, NO retry @ 2s — specific 5xx codes (RT-R2: F-bundled-retry-drop). Sites-table uses `useTransition` + `router.refresh()` (RT-R2: F3 — drop TanStack Query).
- **Deploy:** Vercel **FREE TIER** for `apps/web` (project + auto-assigned subdomain `snake-backlink-forge.vercel.app` for Phase 3-10 incl. V1 launch; custom domain `snakebacklink.com` **DEFERRED to Phase 11** — pivot 2026-04-26/27 saves $9-15 + 1-3 day DNS wait until tester referral active justifies). Backend stays on `snake-backlink-api.fly.dev`. Phase 0 prerequisites checklist (3 items only — Vercel/Sentry signups + WP_ENC_KEY hex; domain+Cloudflare DNS deferred Phase 11) runs FIRST in ~15 min.
- **Observability:** Sentry FE only with hardened beforeSend/beforeBreadcrumb redaction + sourcemap secure delete (RT-R1: F11). BE Sentry deferred to Phase 4 (RT-R1: F-sentry-defer). Plausible (cookieless).
- **Testing:** Playwright only (5 specs: login-flow, login-redirect, dashboard-load, wp-connect-mock, proxy-body-limit). NO Vitest, NO testcontainers, NO Lighthouse CI (RT-R2: F4 — manual Lighthouse before launch). Fixture cookie `__Host-sbf_key` + secure:true via mkcert HTTPS dev (RT-R2: F7). CI grep guard against TanStack Query (RT-R2: F3).
- **No /settings page (RT-R2: F11):** UserMenu DropdownMenu absorbs masked key + regen-key link + logout.

## Phases

| # | File | Owner files | Effort | Status |
|---|------|-------------|--------|--------|
| 00 | [phase-00-prerequisites.md](phase-00-prerequisites.md) | (none — manual procurement + waiting) | ~2h click-ops + 24-48h DNS wait | pending |
| 01 | [phase-01-setup-and-deps.md](phase-01-setup-and-deps.md) | `apps/web/*`, `packages/shared-types/openapi.yaml`, `biome.json`, root `package.json` | 3h | pending |
| 02 | [phase-02-backend-auth-and-cors.md](phase-02-backend-auth-and-cors.md) | `services/api/internal/middleware/auth_apikey.go`, `rate_limit_auth.go`, `internal/api/router.go`, `handlers/v1_*.go`, `service/key_service.go`, `cmd/api/main.go` (WP_ENC_KEY hex boot validation) | 4.5h | pending |
| 03 | [phase-03-frontend-auth-flow.md](phase-03-frontend-auth-flow.md) | `apps/web/src/app/login/`, `apps/web/src/app/api/auth/`, `apps/web/src/app/api/proxy/` (1MB cap), `apps/web/src/lib/auth/`, `apps/web/middleware.ts` | 5h | pending |
| 04 | [phase-04-app-shell-and-dashboard.md](phase-04-app-shell-and-dashboard.md) | `apps/web/src/app/(app)/**` (NO settings/), `apps/web/src/components/**` (Sheet+DropdownMenu+UserMenu absorbs settings) | 4.5h | pending |
| 05 | [phase-05-wp-sites-connect.md](phase-05-wp-sites-connect.md) | `services/api/internal/migrations/20260427001_phase3_wp_sites.sql` (no enc_key_version), `service/wp_site_service.go`, `integration/wp/*` (TOCTOU-safe), `apps/web/src/app/(app)/sites/**` (useTransition) | 5.5h | pending |
| 06 | [phase-06-landing-revamp.md](phase-06-landing-revamp.md) | `apps/web/src/app/page.tsx`, `apps/web/src/components/landing/**` (VN-only, no i18n) | 2h | pending |
| 07 | [phase-07-deploy-and-observability.md](phase-07-deploy-and-observability.md) | `apps/web/sentry.*.config.ts`, `apps/web/instrumentation.ts`, ops docs | 2.5h | pending |
| 08 | [phase-08-testing-and-ci.md](phase-08-testing-and-ci.md) | `apps/web/e2e/**` (5 specs), `.github/workflows/ci.yml` (grep guard) | 3h | pending |

**Total effort:** ~30h code work + Phase 0 prereq waiting. Calendar: **8-10 working days realistic for solo founder** (RT-R2: F15 honest estimate). Floor 7 days (no blockers, all happy path), ceiling 12 days (Windows pnpm issues + WP host quirks during testing).

## Reuse map (Phase 2 services consumed)

`KeyService` (lookup via `GetKeyByHash`) | `UserService` (`GetByID`) | `WalletService` (balance) | `TransactionService` (history) | `AuditService` (best-effort goroutine; new `LogWebLoginMeta` with structured `meta JSONB` for both fwd-IP + conn-IP per RT-R1: F-X-F-F) | `db.Pool` (pgx) | `migrator` embed | sqlc codegen pipeline | `bot.IsAdmin` (admin allowlist) | rate-limit Redis pattern (`middleware/rate_limit_webhook.go`) — reused for new `/auth/verify` rate limiter (RT-R1: F2). NEW deps: shadcn `Sheet` + `DropdownMenu` (RT-R2: revert-R1-F-shadcn-slim — 6 components total). NO `golang.org/x/crypto` (RT-R2: revert-R1-F7 — HKDF dropped). NO `kin-openapi` (RT-R2: F12 — contract test dropped).

## Out of scope (DO NOT bleed into Phase 3)

- AI article generation (Phase 4)
- WordPress publish endpoint (Phase 5)
- DataForSEO + keyword research (Phase 6)
- Campaign scheduler + cron jobs (Phase 7)
- Analytics dashboards (Phase 8)
- Subscription tier gating + recurring billing (Phase 9)
- Email/password auth (NO email column ever)
- Telegram OAuth widget / WebApp embed
- Multi-tenant team seats / contractor invites
- next-intl + EN locale + sitemap + OG metadata (RT-R1: F8 — Phase 9)
- TanStack Query / client-side query cache (RT-R1: F13 — install scoped phase if needed; RT-R2: F3 CI guard prevents reintroduction)
- Backend Sentry (RT-R1: F-sentry-defer — Phase 4)
- Redis auth cache (RT-R1: F5 — defer until proven needed)
- `/settings` standalone page (RT-R2: F11 — UserMenu absorbs)
- WP_ENC_KEY rotation tooling (RT-R2: revert-R1-F7 — Phase 11 ticket)
- HKDF + per-row key derivation (RT-R2: revert-R1-F7 — single master key sufficient for v1)
- Vitest unit tests (RT-R2: F4 — Playwright covers via real flows)
- testcontainers-go integration tests (RT-R2: F4 — existing backend tests cover)
- Lighthouse CI (RT-R2: F4 — manual run before launch)
- kin-openapi Go contract test (RT-R2: F12)

## Top 3 risks (post-RT-R2)

1. **WordPress host quirks (Wordfence, mod_security, host firewalls) cause connect to fail in confusing ways (HIGH)** — mitigated by `/wp-json/` root probe diagnostic returning specific VN error messages naming the likely culprit (RT-R1: F12); SSRF + DNS rebinding TOCTOU defense (RT-R1: F3 + RT-R2: F2) prevents attacker probing internal services via WP base_url. Phase 05.
2. **DNS propagation delay (1-3 days) blocks deploy day (HIGH)** — mitigated by Phase 0 prereq checklist running FIRST + Phase 1 code work in parallel during DNS propagation (RT-R2: F10). Phase 7 deploy day is then a confirmation, not a discovery.
3. **Solo founder context-switch fatigue across 9 phases (MED)** — mitigated by Phase 0 prereqs done first (cuts surprise risk on deploy day), 30h estimate vs R1's optimistic 25h (RT-R2: F15), explicit done-criteria per phase, Playwright-only test pyramid (RT-R2: F4 cuts CI complexity).

## Red Team Review

### Session — 2026-04-26 (Round 1)
**Findings:** 15 (all accepted) + 7 bundled minor fixes + 3 modifications to rejected
**Severity breakdown:** 8 Critical, 7 High
**Reviewers:** Security Adversary, Failure Mode Analyst, Assumption Destroyer, Scope & Complexity Critic

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | CORS wildcard *.vercel.app auth bypass | Critical | Accept | Phase 7 |
| 2 | No rate limit on /api/v1/auth/verify | Critical | Accept | Phase 2 |
| 3 | SSRF in WP Validate | Critical | Accept | Phase 5 |
| 4 | Combined middleware default-allow regression | Critical | Accept | Phase 6 |
| 5 | Redis cache + ban/revoke 60s bypass | Critical | Accept (drop cache) | Phase 2 |
| 6 | Domain/Vercel/Cloudflare prereqs unverified | Critical | Accept (new Phase 0) | Phase 0 |
| 7 | AES key rotation procedure missing | Critical | Accept | Phase 5 |
| 8 | Combined middleware breaks /login (intl) | Critical | Accept (drop intl Phase 3) | Phase 6 |
| 9 | Open redirect at login consumer | High | Accept | Phase 3 |
| 10 | Cookie hardening (__Host-, SameSite=Strict) | High | Accept | Phase 3 |
| 11 | Sentry sourcemaps + breadcrumbs leak | High | Accept (FE-only) | Phase 7 |
| 12 | WP App Password real-world failures | High | Accept (diagnostic) | Phase 5 |
| 13 | TanStack Query claim mismatch | High | Accept (drop claim) | Phase 4 |
| 14 | shadcn init non-interactive failure | High | Accept (pre-canned) | Phase 1 |
| 15 | Hey API ↔ Go drift no contract test | High | Accept (kin-openapi) | Phase 1, 8 |

**Bundled minor fixes:** trusted proxy check (`EnableTrustedProxyCheck: true` + audit log connection IP), BalancePill drop (top bar), per-card skeleton file cleanup, shadcn slim (4 components Phase 1, drop Sheet/Dialog/DropdownMenu), `pnpm dlx` version pinning, Lighthouse warn-level perf + mobile-only + runs:5, Vercel native GitHub integration (drop `amondnet/vercel-action`).

**Rejected (modified instead):** Phase 6 full defer → simplified VN-only ~2h. Sentry full drop → FE-only Phase 7, BE deferred to Phase 4. Hey API drop → kept with kin-openapi contract test.

### Session — 2026-04-26 (Round 2)
**Findings:** 15 (all accepted) + 5 bundled minor fixes + 1 rejected
**Severity breakdown:** 4 Critical, 8 High, 3 Medium
**Reviewers:** Security Adversary R2, Failure Mode Analyst R2, Assumption Destroyer R2, Scope & Complexity Critic R2

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | WP_ENC_KEY base64 vs hex format mismatch | Critical | Accept | Phase 0, 2 |
| 2 | DNS rebinding TOCTOU SSRF in WP DialContext | Critical | Accept | Phase 5 |
| 3 | Phase 5 sites-table imports TanStack Query | Critical | Accept | Phase 5, 8 |
| 4 | Testing pyramid 5 layers — drop all but Playwright | Critical | Accept | Phase 8 |
| 5 | Vercel preview can't login (Origin allowlist) | High | Accept | Phase 3, 7 |
| 6 | CSP blocks inline mobile-nav handler | High | Accept (Sheet+DropdownMenu) | Phase 4 |
| 7 | Playwright fixture cookie name mismatch | High | Accept | Phase 8 |
| 8 | Proxy unbounded body OOM | High | Accept | Phase 3 |
| 9 | Phase 0 missing Sentry slug + token + DSN | High | Accept | Phase 0 |
| 10 | Phase 0 checklist bloat | High | Accept | Phase 0 |
| 11 | Drop /settings page (UserMenu duplicates) | High | Accept | Phase 4 |
| 12 | Drop kin-openapi contract test | High | Accept | Phase 1, 2, 5, 8 |
| 13 | REVERSE R1 partial — re-add shadcn Sheet+DropdownMenu | Medium | Accept | Phase 1, 4 |
| 14 | REVERSE R1 partial — drop HKDF + enc_key_version + rotation | Medium | Accept | Phase 5 |
| 15 | Effort honesty: 5-7 days → 8-10 working days | Medium | Accept | plan.md |

**Bundled minor fixes:** React.cache() dedup (Phase 4 fetchMeServer), drop retry @ 2s on 5xx (Phase 5 single attempt), lib/env.ts Zod loader (Phase 1), .env.example completeness for all NEXT_PUBLIC_* (Phase 1), shadcn devDep + pnpm exec (Phase 1).

**Rejected (1):** Drop Sentry FE + Plausible entirely → conflicts §5.5 mandate; kept R1 disposition (FE-only Sentry, Plausible cloud).

**R2 effort delta:** ~34.5h → ~30h. Calendar: 8-10 working days (honest estimate vs R1's optimistic 5-7).

Cook handoff after this round.
