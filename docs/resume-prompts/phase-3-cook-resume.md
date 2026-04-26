# Phase 3 Cook Resume Prompt

**Created:** 2026-04-26
**Plan locked:** `plans/260426-0237-phase3-web-saas-foundation/`
**Status pre-cook:** R1+R2 red-team done, 30 findings accepted, plan finalized.

---

## How to Use

1. Open Claude Code in `E:/tool_backlink/`
2. Run `/clear` to start fresh session (drops planning context, cook gets clean window)
3. Paste the **Resume Prompt** block below verbatim
4. Claude reads memory + plan + cook skill, asks for confirmation, then begins Phase 0

---

## Resume Prompt (paste this)

```
Resume Phase 3 Web App SaaS Foundation cook.

Plan: E:\tool_backlink\plans\260426-0237-phase3-web-saas-foundation\plan.md
Status: Locked. R1+R2 red-team done 2026-04-26 — 30 findings accepted (12 Critical, 15 High, 3 Medium).

Pre-flight smoke (run first):
1. curl https://snake-backlink-api.fly.dev/health → expect 200 {"status":"ok"}
2. curl https://snake-backlink-api.fly.dev/ready → expect 200 {"db":"ok","redis":"ok"}
3. flyctl status --app snake-backlink-api → expect 1 machine started
4. git log --oneline -3 → confirm tip = 3ff2205 + any new commits

Memory pre-loaded:
- Phase 2 shipped 2026-04-25 (tag v0.1.0-beta-phase2-complete)
- Phase 3 pivot to Web SaaS per §5.5
- Subagent rule: always model:opus
- Communication: VN tao/mày concise, lettered options for approvals

Execute /ck:cook E:\tool_backlink\plans\260426-0237-phase3-web-saas-foundation\plan.md

Cook order — sequential, NO parallel:
- Phase 0 first (1-3 days wait absorbed by Phase 1 code in parallel)
- Phase 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8

Phase 0 = mostly click-ops:
- domain snakebacklink.com register
- Cloudflare nameservers
- Vercel Pro signup ($20/mo)
- WP_ENC_KEY = openssl rand -hex 32 (HEX not base64 — RT-R2: F1)
- Sentry org slug + SENTRY_AUTH_TOKEN + NEXT_PUBLIC_SENTRY_DSN

Engineering bar (per §5.5 + R1+R2):
- Type-safe end-to-end (TS strict + Go)
- Playwright E2E only (no Vitest/testcontainers/Lighthouse — RT-R2: F4)
- Sentry FE hardened (beforeSend + beforeBreadcrumb redact)
- Conventional commits, lowercase subject

KHÔNG động Phase 2 production trừ khi bug critical.
KHÔNG re-debate scope — R1+R2 dispositions locked, accept the plan.
KHÔNG skip Phase 0 — DNS propagation 24-48h is real wall-clock.

Begin với confirmation prompt + Phase 0 checklist hand-off.
```

---

## Expected Cook Behavior

After paste, Claude should:

1. Run 4 smoke check commands in parallel
2. Confirm prod healthy (else STOP, debug Phase 2 first)
3. Read `plan.md` + frontmatter status
4. Read `phase-00-prerequisites.md` (the active starting phase)
5. Present Phase 0 checklist as actionable items (5 items: domain, Cloudflare, Vercel, hex key, Sentry IDs)
6. For each Phase 0 item: ask if mày want guidance link OR confirm done
7. After Phase 0 checked → spawn fullstack-developer agent (model:opus) for Phase 1 setup
8. Resume sequential phase execution per plan

If Claude tries to:
- Skip Phase 0 → stop and remind 1-3 day DNS wait is real
- Re-run red-team → not needed, plan locked
- Question scope → not needed, R1+R2 dispositions final
- Run phases in parallel → reject, sequential per plan

---

## Recovery / Override Phrases

If session goes off-rails, paste these to course-correct:

- **"Stop. Re-read plan.md first."** — forces re-orientation
- **"Phase X is not yet done — resume current phase."** — anti-skip
- **"Don't re-debate scope. Plan is locked."** — anti-re-litigation
- **"Use model:opus for subagents per memory."** — enforce subagent quality

---

## Files at Resume

```
E:\tool_backlink\
├── plans\260426-0237-phase3-web-saas-foundation\
│   ├── plan.md                                    ← cook entry point
│   ├── phase-00-prerequisites.md                  ← START HERE
│   ├── phase-01-setup-and-deps.md
│   ├── phase-02-backend-auth-and-cors.md
│   ├── phase-03-frontend-auth-flow.md
│   ├── phase-04-app-shell-and-dashboard.md
│   ├── phase-05-wp-sites-connect.md
│   ├── phase-06-landing-revamp.md
│   ├── phase-07-deploy-and-observability.md
│   ├── phase-08-testing-and-ci.md
│   ├── research\researcher-r1-frontend-foundation.md
│   ├── research\researcher-r2-plumbing-deploy.md
│   └── reports\scout-codebase-state.md
└── docs\
    ├── MASTER_PROMPT.md §5.5                      ← scope reference
    └── journals\2026-04-26-phase3-planning-r1-r2-redteam.md
```

---

## Memory Snapshot (auto-loaded by Claude)

- User profile: solo VN founder, fintech-adjacent SBF codebase
- Phase 2 shipped state (production refs)
- Phase 3 PIVOT (web SaaS over extension)
- Production debug cheatsheet (flyctl + psql)
- Communication style (VN tao/mày, lettered options)
- Subagent rule (always opus model)
