# Phase 1 Cook Resume Prompt

**Created:** 2026-04-27 (post Phase 0 complete + secret rotation + GitHub public flip)
**Plan locked:** `plans/260426-0237-phase3-web-saas-foundation/`
**Pre-cook state:** Phase 0 done. R1+R2 red-team done. 30 findings accepted.

---

## How to Use

1. Open Claude Code in `E:/tool_backlink/` or `E:\tool_backlink`
2. Run `/clear` to reset context (cook gets clean window)
3. Paste the **Resume Prompt** block below verbatim
4. Claude reads memory + plan + cook skill, asks confirmation, then begins Phase 1

---

## Resume Prompt (paste this)

```
Resume Phase 1 Web App SaaS cook — Setup + deps (~3h).

Plan: E:\tool_backlink\plans\260426-0237-phase3-web-saas-foundation\plan.md
Phase 1 file: phase-01-setup-and-deps.md
Status: Phase 0 ✅ COMPLETE 2026-04-27. Phase 1 unblocked.

Pre-flight smoke (run first, parallel):
1. curl https://snake-backlink-api.fly.dev/health → expect 200 {"status":"ok"}
2. curl https://snake-backlink-api.fly.dev/ready → expect 200 {"db":"ok","redis":"ok"}
3. flyctl status --app snake-backlink-api → expect 1 machine started
4. git log --oneline -5 → confirm tip clean

Phase 0 final state (verified):
- Production: snake-backlink-api.fly.dev (Phase 2 backend live)
- Repo: github.com/snake-dev-team/snake-backlink-forge (PUBLIC)
- Secret Scanning + Push Protection + Dependabot ENABLED
- Vercel project: snake-backlink-forge linked Hobby Free (deploy fail expected — needs apps/landing setup)
- Vercel URL: snake-backlink-forge.vercel.app
- Sentry: snake-backlink-forge org + project (Free Developer)
- Secrets rotated v3: Telegram bot, SePay webhook, JWT_SECRET — all flyctl applied + Fly machine healthy
- Custom domain snakebacklink.com: DEFERRED to Phase 11 (when tester referral active)
- Production URL Phase 3-10: snake-backlink-forge.vercel.app

Local secrets storage:
- Encrypted file: C:\Users\ACER\.secrets\sbf-secrets.gpg (AES-256, GPG symmetric)
- Master password: paper backup
- Decrypt: gpg --decrypt sbf-secrets.gpg
- Plaintext temp files: ALL DELETED

🔒 OVERRIDE FROM PLAN-01 (Option A locked Phase 0 decision):
KEEP `apps/landing` — DROP rename to `apps/web` per phase-01 spec.
- Saves 30min refactor + Vercel re-config
- Phase-01 has many `apps/web` + `@sbf/web` references — INTERPRET AS `apps/landing` + `@sbf/landing`
- Skip phase-01 Step 1 (rename) entirely
- Update phase-01 plan file inline at start of cook (find/replace `apps/web` → `apps/landing`, `@sbf/web` → `@sbf/landing`)

Phase 1 scope (~3h):
1. Skip apps/landing rename (Option A)
2. Tailwind v4 CSS-first config in apps/landing
3. shadcn devDependency 2.1.6 + pnpm exec → install Button, Input, Card, Label, Sheet, DropdownMenu (6 components)
4. Pre-canned components.json + lib/utils.ts + globals.css (no shadcn init wizard — non-interactive friendly)
5. Hey API codegen (@hey-api/openapi-ts) + scaffold packages/shared-types/openapi.yaml skeleton
6. lib/env.ts Zod-validated env loader (NEXT_PUBLIC_* fail-loud at boot)
7. apps/landing/.env.example with all NEXT_PUBLIC_* vars (TELEGRAM_BOT_USERNAME, API_BASE_URL, APP_URL, SENTRY_DSN, PLAUSIBLE_DOMAIN)
8. Pin pnpm dlx versions (no @latest) for supply chain
9. Compile baseline: pnpm -r typecheck && pnpm -r build all green

Vercel post-Phase-1 checklist (after Phase 1 push):
1. Vercel dashboard → snake-backlink-forge → Settings → General → Root Directory: apps/landing (already set from Phase 0 import)
2. Settings → General → Node.js Version: 20.x
3. Settings → Git → Production Branch: main (after merge dev → main)
4. May need: Settings → Build & Development Settings → Install Command override: `pnpm install --frozen-lockfile`
5. May need: package.json packageManager bump to pnpm@10.x if Vercel CI complains
6. Verify next push triggers green deploy
7. Visit snake-backlink-forge.vercel.app → expect placeholder page renders

Engineering bar (per §5.5 + R1+R2):
- TypeScript strict
- Conventional commits, lowercase subject
- Playwright E2E only (NO Vitest/testcontainers/Lighthouse — RT-R2: F4)
- Sentry FE hardened (beforeSend + beforeBreadcrumb redact)

KHÔNG động Phase 2 production trừ khi bug critical.
KHÔNG re-debate scope — R1+R2 dispositions locked.
KHÔNG renaming apps/landing → apps/web (Option A locked).
KHÔNG buy custom domain (Phase 11 trigger only).

Begin với pre-flight smoke + confirmation prompt + read phase-01 with apps/landing override mentality.

Spawn fullstack-developer agent (model:opus) for Phase 1 implementation per orchestration protocol.
```

---

## Expected Cook Behavior

Sau paste, Claude should:

1. Run 4 smoke check commands parallel
2. Confirm prod healthy (else STOP, debug Phase 2 first)
3. Read `plan.md` + `phase-01-setup-and-deps.md`
4. **Apply Option A override:** find/replace `apps/web` → `apps/landing` and `@sbf/web` → `@sbf/landing` in phase-01 (or interpret on-the-fly during cook)
5. Skip phase-01 Step 1 (rename) entirely
6. Spawn fullstack-developer agent for code work
7. Sub-agent: scaffold Tailwind + shadcn (6 components) + Hey API + Zod env loader + .env.example
8. Verify `pnpm -r typecheck && pnpm -r build` green
9. Conventional commit + push
10. Verify Vercel auto-redeploy → green or report build error

---

## Recovery / Override Phrases

If session goes off-rails:

- **"Stop. Re-read phase-01 with apps/landing override."** — anti-rename
- **"Don't re-debate scope. R1+R2 locked. Custom domain Phase 11."** — anti-re-litigation
- **"Use model:opus for subagents per memory."** — enforce subagent quality
- **"Test build locally before push: pnpm --filter @sbf/landing build"** — pre-deploy validation
- **"Don't auto-rotate secrets. Phase 0 done."** — anti-redundant rotation

---

## Files at Resume

```
E:\tool_backlink\
├── plans\260426-0237-phase3-web-saas-foundation\
│   ├── plan.md
│   ├── phase-00-prerequisites.md  ← Phase 0 COMPLETE notes
│   ├── phase-01-setup-and-deps.md ← cook target (apply apps/landing override)
│   ├── phase-02-backend-auth-and-cors.md
│   ├── phase-03-frontend-auth-flow.md
│   ├── phase-04-app-shell-and-dashboard.md
│   ├── phase-05-wp-sites-connect.md
│   ├── phase-06-landing-revamp.md
│   ├── phase-07-deploy-and-observability.md
│   └── phase-08-testing-and-ci.md
├── docs\
│   ├── MASTER_PROMPT.md §5.5 (scope)
│   ├── journals\2026-04-26-phase3-planning-r1-r2-redteam.md
│   └── resume-prompts\
│       ├── phase-3-cook-resume.md  (older — Phase 0 not done yet)
│       └── phase-1-cook-resume.md  ← THIS FILE
└── apps\
    ├── landing\  ← KEEP (Option A locked)
    └── extension\  ← deprecated, leave untouched (delete Phase 11+)
```

---

## Cost State

- Phase 3-10: $3/mo (Fly.io backend only)
- Phase 11 (custom domain trigger): ~$13/mo (+ domain $1/mo + Plausible $9)
- Phase 12 (5K user scale): ~$55/mo (+ Vercel Pro $20 + Sentry Team $26)

## Memory Snapshot (auto-loaded by Claude)

- User profile: solo VN founder
- Phase 2 shipped 2026-04-25
- Phase 3 PIVOT (web SaaS over extension)
- Phase 0 COMPLETE 2026-04-27 — secrets rotated, public repo, Vercel linked
- Subagent rule: always model:opus
- Communication: VN tao/mày concise, lettered options for approvals
