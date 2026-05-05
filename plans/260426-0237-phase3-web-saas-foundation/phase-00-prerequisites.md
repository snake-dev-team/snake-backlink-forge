---
name: "Phase 0 — Prerequisites Checklist"
phase: 0
priority: P0
effort: ~15 minutes click-ops (PIVOT 2026-04-26 — domain + DNS deferred to Phase 10)
status: in_progress
created: 2026-04-26
updated: 2026-04-26
---

# Phase 0 — Prerequisites Checklist (Cost-Optimized)

## Overview

Verify external account access BEFORE Phase 1 starts. NO code, just signups + secret generation. Total time: ~15 minutes. Total cost: **$0**.

## PIVOT 2026-04-26 — Domain Deferred to Phase 10

Custom domain (`snakebacklink.com`) and Cloudflare DNS setup DEFERRED from Phase 0 to **Phase 11 (custom domain trigger when tester referral active justifies $9/yr)**. Phase 3-10 (incl. V1 launch) use Vercel subdomain `snake-backlink-forge.vercel.app`. Rationale:
- Phase 3-9 dev work uses Vercel auto-assigned subdomain (e.g., `snake-backlink-forge.vercel.app`) — sufficient for testing + beta sharing
- Saves ~$9-15 cost + 24-48h DNS propagation wait during dev iteration
- Reduces tonight's wall-clock from "1-3 days waiting" → "15 min signup"
- Custom domain only matters at V1 launch when user-facing brand finalized

## Active Checklist Tonight (3 items, ~15 min)

- [ ] **WP_ENC_KEY generated** — `openssl rand -hex 32` → 64 hex chars / 32 bytes entropy. Saved to Bitwarden secrets section ("SBF Production Secrets" → "WP_ENC_KEY"). Critical: HEX format (NOT base64 — RT-R2: F1).
- [ ] **Vercel account created** — FREE TIER (Hobby plan). Signup via GitHub OAuth (`danhng876`). Free tier limits: 100GB bandwidth/mo, 100K edge function invocations/mo, 6000 build min/mo. Defer Pro ($20/mo) until launch traffic justifies. Project NOT created tonight — created Phase 7.
- [ ] **Sentry account created** — FREE TIER (Developer plan). Signup via GitHub OAuth. Free tier: 5K errors/mo, 1 project, 1 user. Create project: name=`snake-backlink-forge`, platform=`Next.js`. Capture: org slug, `SENTRY_AUTH_TOKEN`, `NEXT_PUBLIC_SENTRY_DSN`. All 3 saved to Bitwarden.

## Deferred to Phase 11 (~$9-15 + 24-48h DNS wait — trigger: tester referral active OR custom-brand revenue justification)

- [ ] **Domain registration** — `snakebacklink.com` at Porkbun ($9.13 Y1, $9.73 flat renewal) OR alternative (Namecheap, Cloudflare Registrar)
- [ ] **Cloudflare account + nameservers active** — Free tier, add site, get 2 NS values
- [ ] **Update Porkbun NS to Cloudflare**
- [ ] **DNS propagation verified** — `dig snakebacklink.com NS` returns Cloudflare globally
- [ ] **Custom domain wired to Vercel** — Project Settings → Domains → Add `snakebacklink.com` + `www.snakebacklink.com` (canonical: non-www)
- [ ] **Plan files find/replace** — Vercel subdomain references → `snakebacklink.com` across phase-02, phase-06, phase-07, phase-08, .env.example
- [ ] **TLS verification** — SSL Labs A grade on `https://snakebacklink.com`

## Cost Breakdown (Lifetime)

| Phase | Item | Cost |
|---|---|---|
| **0 (tonight)** | WP_ENC_KEY + Vercel Free + Sentry Free | **$0** |
| 11 (custom domain swap) | Domain Y1 (Porkbun .com) | ~$9.13 |
| 11 (custom domain swap) | Cloudflare DNS | $0 (free) |
| Y2+ | Domain renewal (Porkbun flat) | ~$9.73/yr |
| Optional post-launch | Vercel Pro upgrade if traffic | $20/mo |
| Optional post-launch | Sentry Team upgrade if scale | $26/mo |
| Optional post-launch | Plausible cloud (Phase 7 deploy) | $9/mo (10K events) |

## Bitwarden Secret Storage Format

Save 3 secrets in Bitwarden vault. Recommend secure note format:

**Note 1: SBF WP_ENC_KEY**
```
=== SBF WP_ENC_KEY ===
Generated: 2026-04-26
Format: hex (NOT base64 — RT-R2: F1)
Length: 64 chars / 32 bytes entropy
Value: <64 hex chars from ~/.tmp/wp_enc_key.txt>
Used in: services/api/internal/util/aesgcm.go (Phase 5)
Rotation: Phase 11 ticket (single-key model, re-encrypt all rows on rotate)
```

**Note 2: SBF Vercel**
```
=== SBF Vercel ===
Email: <github-oauth-email>
Account: GitHub OAuth (danhng876)
Plan: Hobby (Free)
Project: TBD (created Phase 7)
URL pattern (post-Phase 7): snake-backlink-forge.vercel.app
2FA: ENABLE TOTP via GitHub
Pro upgrade: deferred to Phase 10+ if needed
```

**Note 3: SBF Sentry**
```
=== SBF Sentry ===
Email: <github-oauth-email>
Plan: Developer (Free, 5K errors/mo)
Org slug: <your-org-slug>
Project: snake-backlink-forge (Next.js)
SENTRY_AUTH_TOKEN: <token-from-org-settings>
NEXT_PUBLIC_SENTRY_DSN: <dsn-from-project-settings>
2FA: ENABLE TOTP
```

## Success Criteria

- [ ] WP_ENC_KEY: 64 hex chars in Bitwarden, temp file `~/.tmp/wp_enc_key.txt` deleted after save
- [ ] Vercel: account confirmed at `https://vercel.com/dashboard`, plan = Hobby
- [ ] Sentry: project `snake-backlink-forge` visible at `https://sentry.io/organizations/<slug>/projects/`, all 3 secrets in Bitwarden

## Calendar

~15 minutes total click-ops tonight. NO blocking wait period. Phase 1 (~3h code) starts immediately after Phase 0 done.

## Phase 11 Re-activation Checklist

When ready to swap to custom domain (Phase 11 trigger — tester referral active OR brand revenue justifies):
1. Register `snakebacklink.com` at Porkbun (~$9.13)
2. Sign up Cloudflare, add site
3. Update Porkbun NS to Cloudflare
4. Wait 24-48h DNS propagation
5. Vercel Settings → Domains → Add `snakebacklink.com` (gray cloud CNAME)
6. Find/replace `snake-backlink-forge.vercel.app` → `snakebacklink.com` across plan files + apps/web env vars
7. Update Fly secret `CORS_ORIGINS=https://snakebacklink.com,https://www.snakebacklink.com`
8. SSL Labs verify A grade
