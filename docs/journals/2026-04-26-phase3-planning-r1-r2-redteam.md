# Phase 3 Planning Session — R1+R2 Red Team Verdict

**Date:** 2026-04-26
**Duration:** ~3.5 hours wall clock (smoke check → plan locked)
**Branch:** dev
**Plan dir:** `plans/260426-0237-phase3-web-saas-foundation/`
**Status:** Plan locked. Cook deferred to next session (founder fatigue management).

## Session Overview

Day after Phase 2 production ship (2026-04-25, tag v0.1.0-beta-phase2-complete). Pivoted Phase 3 from Chrome Extension MV3 + Rust WASM to Web App SEO SaaS Service per `docs/MASTER_PROMPT.md §5.5` (commit `3ff2205`). This session consumed the pivot scope and produced a hardened, security-vetted implementation plan.

**Workflow executed:**
1. Smoke check 4 prod endpoints — all green (health, ready, fly machine, git tip)
2. `/ck:plan --hard` with explicit research topics
3. Spawned 2 parallel researchers (frontend foundation + plumbing/deploy) + 1 Explore scout
4. AskUserQuestion gated 3 architecture decisions (login UX, identity model, app path) before planner spawn
5. Planner authored 9 files (plan.md + 8 phase files) — 39h initial estimate
6. Red Team Round 1 — 4 hostile reviewers in parallel (Security, Failure, Assumption, Scope critic)
7. R1 adjudication + apply 15 findings + 7 bundled fixes
8. Red Team Round 2 — 4 fresh reviewers on UPDATED plan with explicit "do not re-raise R1" instruction
9. R2 adjudication + apply 15 findings + 5 bundled fixes (incl. 2 partial R1 reversals)

## Findings Summary

**Total: 30 findings accepted** (12 Critical, 15 High, 3 Medium)

**Round 1: 15 findings (8 Critical, 7 High)**
- 8 Critical: CORS wildcard auth bypass, no rate limit on /auth/verify, SSRF in WP Validate, combined middleware default-allow regression, Redis cache + ban 60s bypass, domain prereqs unverified, AES key rotation procedure missing, combined middleware breaks /login (intl conflict)
- 7 High: open redirect at consumer, cookie hardening (`__Host-` prefix + SameSite=Strict), Sentry sourcemaps + breadcrumbs leak, WP App Password real-world failures, TanStack Query claim mismatch, shadcn init non-interactive failure, Hey API ↔ Go drift contract test

**Round 2: 15 findings (4 Critical, 8 High, 3 Medium)**
- 4 Critical: WP_ENC_KEY format mismatch (base64 vs hex), DNS rebinding TOCTOU SSRF, Phase 5 sites-table TanStack import contradicts R1, testing pyramid 5 layers (drop all but Playwright)
- 8 High: Vercel preview Origin allowlist gap, CSP blocks inline mobile-nav, Playwright fixture cookie name mismatch, proxy unbounded body OOM, Phase 0 Sentry slug/token/DSN gap, Phase 0 checklist bloat, drop /settings page, drop kin-openapi contract test
- 3 Medium: REVERSE R1 partial (re-add shadcn Sheet+DropdownMenu), REVERSE R1 partial (drop HKDF + enc_key_version + rotation), effort honesty (5-7d → 8-10 working days)

## R1 → R2 Reversals (2 partial)

R2 caught 2 cases where R1 over-corrected:

1. **shadcn Sheet + DropdownMenu** — R1 dropped them in favor of native `<details>` for "scope simplicity". R2 found custom replacements created CSP issues (inline JS sprinkle approach blocked under `script-src 'self'`) + accessibility gaps (no Escape key, no focus trap, no auto-close on route change). Net: re-adding the shadcn primitives is LESS code AND better UX. R1 KISS instinct misread "less dependencies" as "less work" — was actually more work + less correct.

2. **HKDF + enc_key_version + rotation infra** — R1 added per-row HKDF derivation + `enc_key_version` column + CLI scaffold + 30-line runbook to support future key rotation. R2 caught: (a) v1 has zero stored creds at launch — rotation is meaningless until users exist, (b) CLI is `os.Exit(1)` stub so runbook procedure cannot execute, (c) `EncryptAESGCM` hardcodes `version := int16(1)` so even if CLI implemented, function signature blocks it. R2 verdict: drop HKDF, single AES-256-GCM key + random nonce (correct usage at v1 scale), rotation = Phase 11 ticket.

**Lesson:** Red Team Round 2 is not redundant. R1 introduces second-order complexity that R2 catches. The 2 reversals saved ~1.5h dev + future maintenance burden.

## Critical Fixes Prevented Pre-Deploy

R1+R2 caught issues that would have shipped to production without red-team:

1. **WP_ENC_KEY format mismatch** — Phase 0 said `openssl rand -base64 32`, code expected hex. First user click on "Connect WP site" weeks later → 500 with cryptic `hex.DecodeString` error. Caught by R2-Sec-F1. Fix: align Phase 0 to hex + boot-time hex validation.

2. **DNS rebinding TOCTOU SSRF in WP Validate** — Custom `DialContext` resolved DNS twice (validation + actual dial). Attacker uses split-horizon DNS to bypass IP allowlist, hits Fly internal services / AWS metadata. Caught by R2-Asn-F6. Fix: rewrite addr to validated IP literal before `baseDial`.

3. **CORS wildcard `*.vercel.app` auth bypass** — Fiber doesn't support glob origins; wildcard would either fail silently or open subdomain takeover. Attacker deploys `evil-snake.vercel.app` → fetch with credentials → bypass. Caught by R1-Sec-F1. Fix: exact match only + env-var validator rejects globs.

4. **No rate limit on `/api/v1/auth/verify`** — Original plan deferred to Phase 7. Attacker could hammer 10K RPS to brute force keys. Caught by R1-Sec-F6. Fix: 5/min/IP + 20/hr/IP + 200/day/IP via existing webhook rate limit pattern, NOT deferred.

5. **Cookie hardening gaps** — Plain `sbf_key` name, dev/prod secure mismatch, SameSite=Lax on verify endpoint allowed session fixation via CSRF. Caught by R1-Sec-F5. Fix: `__Host-sbf_key` prefix + SameSite=Strict for verify + Origin header check + secure unconditional outside dev.

6. **Phase 5 sites-table TanStack Query import** — File imported `useMutation`, `useQueryClient` despite R1 architecturally dropping TanStack Query for pure RSC. Build would have failed. Caught by R2-Sec-F4. Fix: rewrite to `useTransition` + `router.refresh()` + CI grep guard.

## Plan Structure

**9 files** (plan.md + 8 phase files + 1 prereq):

- `plan.md` — overview, phases table, reuse map, top 3 risks, Red Team Review sections
- `phase-00-prerequisites.md` — 5-item checklist (domain, Cloudflare DNS, Vercel Pro, hex WP_ENC_KEY, Sentry slug+token+DSN); 1-3 days wait absorbed by Phase 1-2 code work
- `phase-01-setup-and-deps.md` — apps/landing→apps/web rename, Tailwind v4 + shadcn (Button, Input, Card, Label, Sheet, DropdownMenu) + Hey API codegen + Zod env loader (~3h)
- `phase-02-backend-auth-and-cors.md` — `/api/v1/*` sub-router, `auth_apikey` middleware (DB-only, no Redis cache), rate limit /auth/verify, EnableTrustedProxyCheck, hex boot validation (~4.5h)
- `phase-03-frontend-auth-flow.md` — `__Host-sbf_key` cookie + SameSite=Strict for verify, sanitizeNext at consumer, VERCEL_URL preview Origin fallback, 1MB proxy body cap (~5h)
- `phase-04-app-shell-and-dashboard.md` — pure RSC + React.cache() dedupe, shadcn Sheet (mobile nav) + DropdownMenu (UserMenu w/ key+logout absorbed from dropped /settings), 3 dashboard cards (~4.5h)
- `phase-05-wp-sites-connect.md` — single AES-256-GCM key (no HKDF), TOCTOU SSRF fix (addr rewrite to IP literal), `/wp-json/` diagnostic with 5 specific error codes, useTransition (no TanStack), no retry (~5.5h)
- `phase-06-landing-revamp.md` — VN-only single page, no `[locale]/`, no next-intl (R1 simplification preserved) (~2h)
- `phase-07-deploy-and-observability.md` — strict CORS exact match, Sentry FE-only hardened (beforeBreadcrumb + maskAllInputs + sourcemap delete), Plausible cloud (~2.5h)
- `phase-08-testing-and-ci.md` — Playwright-only 5 specs (login-flow, dashboard-load, wp-connect-mock, login-redirect, proxy-body-limit), mkcert HTTPS fixture, CI grep guard for TanStack (~3h)

## Effort + Calendar

**Code work:** ~30h across 8 phases + Phase 0 prereqs (~2h click-ops)
**Calendar:** 8-10 working days realistic for solo founder (Windows + iteration time)
- Floor: 7 days (no blockers, all happy path)
- Ceiling: 12 days (Windows pnpm slowness + WP host quirks during testing)
- Phase 0 DNS propagation (24-48h) absorbed by Phase 1-2 code work in parallel

**Initial planner estimate:** 39h. After R1 cuts: 34.5h. After R2 cuts: 30h. Honest calendar: 8-10 days (was advertised 5-7).

## Lessons / Observations

1. **Red Team Round 2 caught real bugs.** 4 Critical findings emerged in R2 that R1 missed entirely — primarily code-level details surfaced by R1's simplifications (TanStack import contradiction, hex vs base64 format, fixture cookie name). Two-round red-team is not theatre.

2. **Two partial R1 reversals justify the cost.** Re-adding shadcn Sheet+DropdownMenu and dropping HKDF complexity emerged as net wins after second-order analysis. R1's KISS heuristic is good signal but not always right.

3. **Architecture decisions gated via AskUserQuestion before planner spawn worked well.** 3 decisions (login UX, email column, app path) shaped the entire plan. Asking upfront prevented expensive rework.

4. **Effort estimates drift optimistic.** 39h → 34.5h → 30h after cuts, but calendar honesty (5-7 → 8-10 days) is the more important admission. Solo founder + Windows + iteration = 1.5-2x the ideal estimate.

5. **Phase 2 production untouched.** Smoke check at session start confirmed prod healthy. No regression risk from planning session.

## Next Steps

- Founder fatigue management: cook deferred. Resume in fresh session.
- Resume prompt saved at `docs/resume-prompts/phase-3-cook-resume.md`.
- Phase 0 prereqs are mostly click-ops + waiting — can start Day 1 morning, then Phase 1 code work parallel during DNS propagation.

## References

- Plan dir: `plans/260426-0237-phase3-web-saas-foundation/`
- Master prompt §5.5: `docs/MASTER_PROMPT.md` lines 1317-1394
- Phase 2 ship state: tag `v0.1.0-beta-phase2-complete`, commit `bdd9c70`
- Phase 3 pivot decision: commit `3ff2205`
