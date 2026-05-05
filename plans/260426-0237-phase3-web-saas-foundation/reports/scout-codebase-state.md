# Codebase Scout Report — Phase 3 Web App SaaS Foundation

**Date:** 2026-04-26
**Author:** Explore subagent (model:opus)
**Plan:** plans/260426-0237-phase3-web-saas-foundation/
**Status:** DONE_WITH_CONCERNS

---

## 1. Existing Next.js scaffold

- **Path:** `E:\tool_backlink\apps\landing\` (NOT `services/web/landing` — path mismatch with §5.5)
- **Status:** Minimal placeholder, runnable but no real content
- **Versions:** `next@15.5.0`, `react@19.1.0`, `react-dom@19.1.0`, `eslint-config-next@15.5.0`, `typescript@6.0.3`, `@types/node@^20`, `@types/react@^19`
- **Files:**
  - `apps/landing/package.json` — name `@sbf/landing`, scripts: `build|dev|start|lint|typecheck`
  - `apps/landing/next.config.mjs` — only `reactStrictMode: true` (no Turbopack flag, no rewrites/proxy, no images config)
  - `apps/landing/tsconfig.json` — extends `../../tsconfig.base.json`, `strict: true`, `jsx: preserve`, includes `next-env.d.ts` + `.next/types/**/*.ts`
  - `apps/landing/src/app/layout.tsx` — vanilla App Router root layout, no fonts/CSS imports
  - `apps/landing/src/app/page.tsx` — single `<h1>Snake Backlink Forge — landing placeholder</h1>`
- **shadcn/ui:** NOT installed. No `components.json`, no `tailwind.config`, no `globals.css`, no `lib/utils.ts`. Tailwind CSS not yet wired.
- **Gap:** App Router scaffold exists but is bare — no Tailwind/CSS, no shadcn, no API client, no auth, no env wiring.

---

## 2. Backend handler patterns (services/api)

- **Routes registered (live HTTP endpoints):**
  - `GET /health` → `handlers.Health` (`services/api/internal/api/handlers/health.go`)
  - `GET /ready` → `handlers.Ready(pool, rdb)` (DB+Redis ping)
  - `POST /webhooks/sepay` → `handlers.SePayWebhook` (rate-limited 20/sec/IP via `middleware.NewRateLimitWebhook`)
- **NO `/api/v1/*` namespace yet** — entire SaaS surface is greenfield
- **Auth middleware:**
  - SePay webhook auth: `sepay.VerifyApikey(c.Get("Authorization"), cfg.SepayWebhookToken)` — constant-time, fail-closed (`webhook.go:23`)
  - Telegram bot uses long-poll, not HTTP-auth
  - **NO bearer/API-key middleware for application API yet.** API keys are stored as `BYTEA` SHA-256 hashes in `api_keys` table with `key_hash` + `key_prefix` (12 chars, e.g. `sbf_live_Zk3`). Lookup query exists: `GetKeyByHash` in `internal/db/queries/keys.sql`. Plaintext `sbf_live_<base58>` shown once at issue.
  - **Gap:** Web app needs new `auth.go` middleware that hashes incoming bearer, calls `GetKeyByHash`, attaches `userID` to ctx.
- **User context propagation pattern (bot side, reusable model):**
  - Bot uses `context.WithValue(ctx, ctxKeyUser, BotUser{ID, Language, IsBanned, IsVerified})` set by `loadUser` middleware in `bot/middleware.go`. Retrieved via `UserFromCtx(ctx)`.
  - Same `ctxKeyUser`-style pattern can be ported to Fiber for HTTP API.
- **Existing services to reuse via dependency injection (`cmd/api/main.go:85-131`):**
  - `*service.UserService` (`user_service.go`) — EnsureStub, VerifyContactAndGrantTrial
  - `*service.KeyService` (`key_service.go`) — Issue, GetActiveMasked, **needs** new `ValidatePlaintext(ctx, plaintext)` for HTTP middleware
  - `*service.WalletService` (`wallet_service.go`) — balance fetch
  - `*service.TransactionService` (`transaction_service.go`) — top-up flow, history
  - `*service.SupportService`, `*service.ReferralService`, `*service.AuditService`, `*service.AdminService`, `*service.WebhookService`
- **DB access pattern:**
  - `pgx/v5` + `pgxpool` (no ORM)
  - sqlc-generated queries in `internal/db/sqlc/` (package `sqlcdb`); `New(pool)` or `New(tx)` for transactional
  - Tx pattern: `pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})` + `defer tx.Rollback` + `qtx := sqlcdb.New(tx)`
  - PG functions called raw: `consume_credits(...)`, `grant_credits(...)` via `tx.QueryRow`
  - Audit log inserts: best-effort goroutine post-commit with detached `context.Background()`
- **Server config (`internal/api/server.go`):** Fiber 64KB BodyLimit, 15s read/write, ProxyHeader=X-Forwarded-For, EnableTrustedProxyCheck (open `0.0.0.0/0` — TODO tighten), `jsonErrorHandler` returns `{"error": ...}`. **No CORS middleware** — must add for browser frontend.

---

## 3. Database schema state

- **Path:** `services/api/internal/migrations/`
- **Naming convention:** `YYYYMMDD<seq>_<slug>.sql` (consolidated goose single-file format with `-- +goose Up` / `-- +goose Down`, `NO TRANSACTION` markers per file). Example: `20260424001_init.sql`. NOT split `.up.sql/.down.sql`.
- **Migration files (5 total):**
  1. `20260424001_init.sql` — full Phase 2 base schema
  2. `20260424002_seed_dorks.sql` — dork patterns seed
  3. `20260424003_phase2_indexes.sql` — partial unique indexes + `support_tickets` + `referrals` tables
  4. `20260424004_phase2_schema_deltas.sql` — enum additions: `topup_excess`, `cancelled`, `recovered_by_late_payment`; provider_ref partial UNIQUE
  5. `20260425001_key_unique_active.sql` — `idx_keys_user_active_unique` (1-active-key invariant)
- **Tables present (all from §3.1 Phase 2 DDL):** `users`, `api_keys`, `wallets`, `ledger`, `transactions`, `campaigns`, `targets`, `jobs`, `domain_cooldown`, `dork_patterns`, `audit_log`, `safeguard_hits`, `support_tickets`, `referrals` (14 tables)
- **Enums:** `ledger_event_type`, `transaction_status`, `transaction_provider`, `campaign_status`, `target_type`, `target_source`, `job_status`
- **PG functions/views:** `consume_credits()`, `grant_credits()`, `check_active_campaign_limit()` trigger, `v_user_stats` view
- **Migrator:** `internal/db/migrator/` — auto-runs on boot from `cmd/api/main.go:78` via `migrator.Up(rootCtx, dbPool)`; embedded SQL via `internal/migrations/embed.go`. Hard-fails boot on migration error (Phase 10 policy).
- **sqlc config:** `services/api/sqlc.yaml` — engine postgresql, queries dir `internal/db/queries`, schema globs migrations, output `internal/db/sqlc` (package `sqlcdb`), `pgx/v5` driver, UUID overrides for `github.com/google/uuid.UUID` (pointer for nullable)
- **Existing query files (`internal/db/queries/`):** `admin_stats.sql`, `audit.sql`, `campaigns.sql`, `jobs.sql`, `keys.sql`, `ledger.sql`, `referrals.sql`, `support.sql`, `targets.sql`, `transactions.sql`, `users.sql`, `wallets.sql`
- **Gap for Phase 3 web app:** No `sessions` table, no `password_hash` column on users (Telegram-only auth so far), no `api_keys.scopes` or `expires_at` column, no `email` column.

---

## 4. Shared types package

- **Path:** `packages/shared-types/`
- **Status:** Stub only
  - `package.json` — `@sbf/shared-types`, no deps, only `typecheck` script, `main: src/index.ts`
  - `src/index.ts` — single line: `export {};`
- **No OpenAPI spec anywhere in repo.** `Glob **/openapi*` returned zero results.
- **Gap:** Empty package, ready to receive shared interfaces. No type generation pipeline (e.g. openapi-typescript, sqlc-to-ts) wired. Phase 3 will need to:
  - Either author OpenAPI spec + codegen or hand-write types matching backend Go structs
  - Or adopt tRPC/zod-shared-schemas pattern (no precedent in repo)

---

## 5. Monorepo structure

- **Workspace manager:** pnpm 9.15.9 (declared in root `package.json`)
- **`pnpm-workspace.yaml`:** packages globs `apps/*`, `packages/*`, `services/*`, `tools/*` (`tools/` empty/missing)
- **`turbo.json`** (turbo 2.9.6): tasks `build`, `lint`, `typecheck`, `test`, `dev` (persistent), `clean`. Build outputs `dist/**`, `build/**`, `.next/**`. Test depends on `^build`.
- **Apps:** `apps/landing` (Next.js 15), `apps/extension` (Svelte 5 + Vite 5 + @crxjs Chrome extension — DEPRECATED per §5.5 pivot)
- **Packages:** `packages/shared-types` (stub)
- **Services:** `services/api` (Go Fiber)
- **Root tooling:** Biome 2.3.0 (formatter+linter), `husky`, `lint-staged`, `commitlint` (conventional), `semantic-release` configs (beta/prod), `gitleaks.toml`
- **TS base:** `tsconfig.base.json` (extended by landing). Root `typescript@6.0.3`.
- **Biome ignore:** `services/api`, `scripts`, `plans`, `.claude*`, `.agents`, `docs/MASTER_PROMPT.md` — Biome only lints TS/JS in apps/packages.

---

## 6. Docker compose + dev infra

- **File:** `ops/docker-compose.yml`
- **Services:**
  - `postgres:16-alpine` — port `${POSTGRES_PORT:-5432}`, user/password/db default `sbf/sbf/sbf_dev`, healthcheck `pg_isready`, volume `sbf_pgdata`
  - `redis:7-alpine` — port `${REDIS_PORT:-6379}`, AOF appendonly, healthcheck `redis-cli ping`, volume `sbf_redisdata`
- **Env files:**
  - Root `.env.example` — only docker compose vars (POSTGRES_*, REDIS_PORT)
  - Root `.env` (gitignored), `services/api/.env.example` — full service config: `DATABASE_URL`, `REDIS_URL`, `CLAUDE_*`, `TELEGRAM_BOT_TOKEN`, `SEPAY_*`, `JWT_SECRET`, `ADMIN_TELEGRAM_IDS`, `INSTALLER_URL`, `SERPAPI_KEY`, etc.
- **No `.env.local` convention** for Next.js app yet — landing has no env file. **Gap:** Phase 3 needs `apps/landing/.env.example` with `NEXT_PUBLIC_API_BASE_URL`, etc.
- **Compose run command:** `docker compose -f ops/docker-compose.yml up -d --wait`

---

## 7. CI/CD

- **Workflows (`.github/workflows/`):**
  - `ci.yml` (push/PR to main+dev) — 3 jobs:
    - `node`: pnpm install (frozen), `biome ci .`, `pnpm -r typecheck`, `pnpm -r build`
    - `go`: `go vet`, `golangci-lint v1.68`, `go test -race -cover` in `services/api`
    - `gitleaks`: secret scan
  - `release.yml` (push to main) — semantic-release + Discord notify
  - `release-beta.yml`, `branch-protection.yml`, `sync-dev-after-release.yml`, `sync-main-to-dev.yml`
- **Linting:**
  - TS/JS: Biome 2.3.0 (`biome.json`) — double quotes, lineWidth 100, organizeImports
  - Go: golangci-lint (`services/api/.golangci.yml`)
- **Test runners:** `go test` for backend; `turbo run test` for FE (no test framework wired in landing yet)
- **Deploy:**
  - **Backend:** Fly.io (`services/api/fly.toml` — app `snake-backlink-api`, region `sin`, 512MB shared-cpu-1x, min_machines_running=1, /health check)
  - **Frontend:** **NONE configured** — no Vercel/Netlify config, no frontend deploy job in CI. **Gap.**
- **Husky:** `.husky/` exists with hooks; commitlint conventional config

---

## 8. Telegram bot integration touchpoints

- **API key issuance entry points:**
  - **`/start`** flow (`bot/cmd_start.go`) — new user verifies phone → `UserService.VerifyContactAndGrantTrial` → which calls `KeyService.Issue` (interface `KeyIssuer`) → returns plaintext shown ONCE in `KeyStartVerifiedFirst` template
  - **`/regenkey`** (`bot/cmd_regenkey.go`) — confirm-flow callback `key:regen:confirm` → `KeyService.Issue` → revokes old, issues new, shows plaintext
  - **`/key`** (`bot/cmd_key.go`) — masked display only (`<prefix>•••••<last4hash>`), NO plaintext
- **Issue mechanism:**
  - `service.KeyService.Issue(ctx, userID)` in `internal/service/key_service.go` — Serializable tx, retries on 23505, returns `(plaintext, prefix, error)`. Audit-log goroutine on success.
  - Plaintext format generated in `util.GenerateAPIKey()` (`internal/util/token.go`) — alphanumeric, prefixed `sbf_live_<base58>`
  - Storage: `key_hash BYTEA` (SHA-256), `key_prefix VARCHAR(12)`, single active enforced by partial unique index
- **Bot user → web app handoff:**
  - **Existing deep link pattern (REUSABLE):** `cmd_ref.go:54` builds `t.me/<botUsername>?start=ref_<code>` — bot listens for `/start ref_XXX` payloads. Same mechanism repurposable for `start=login_<token>` deep link.
  - **TELEGRAM_BOT_USERNAME** env var already wired (`fly.toml`, `config.go`) — `SnakeBacklinkForgeBot`
  - **Reverse handoff (web → bot):** `t.me/SnakeBacklinkForgeBot?start=login_<jwt>` works out of the box
  - **Bot → web SSO:** would need new `/api/v1/auth/telegram-link` endpoint + short-lived single-use token table. Telegram WebApp (`tgWebAppData`) with `validateInitData(botToken)` HMAC also viable — no current code.
- **Admin gating reusable pattern:** `bot.IsAdmin(cfg, tgID) bool` — slice scan `cfg.AdminTelegramIDs` (parsed from CSV env). Web admin pages can reuse same allowlist via shared service layer.

---

## Reuse Opportunities for Phase 3

**File-level handles for new web work:**

| Phase 3 need | Reuse from | Reference |
|--------------|------------|-----------|
| Bearer-key auth middleware | `internal/integration/sepay/verify.go` constant-time pattern + `db/queries/keys.sql:GetKeyByHash` | Wrap into new `internal/middleware/auth_apikey.go` |
| User context in handler | `bot/middleware.go:loadUser` + `BotUser` struct | Mirror as `ApiUser` in `internal/middleware/` |
| Tx + sqlc pattern | `service/key_service.go:issueOnce` (Serializable tx + qtx) | Same idiom for new endpoints |
| Server bootstrap + DI | `cmd/api/main.go:85-200` (services constructed once, passed via deps) | Add `ApiHandlerDeps` struct mirroring `WebhookDeps`/`bot.Deps` |
| Health/readiness pattern | `handlers/health.go` + `handlers/health_test.go` | Template for new endpoint tests |
| Error JSON shape | `server.go:jsonErrorHandler` returning `{error: ...}` | Keep contract; document for OpenAPI |
| Rate limiting | `middleware/rate_limit_webhook.go` Redis fixed-window | Reuse for `/api/v1/*` per-user/per-key buckets |
| Audit logging | `service/key_service.go:insertAuditLog` (best-effort goroutine) | Same for sensitive web actions |
| Wallet read | `service/wallet_service.go` + `db/queries/wallets.sql:GetWalletByUser` | Direct call from new `GET /api/v1/balance` handler |
| Transaction history | `db/queries/transactions.sql:GetTxByUserPage`+`CountTxByUser` | Direct call from `GET /api/v1/transactions` |
| Ledger pagination | `db/queries/ledger.sql:GetLedgerPage`+`CountLedgerByUser` | `GET /api/v1/ledger` |
| Top-up intent flow | `service/transaction_service.go:CreateTopupIntent` (idempotent on `idx_tx_user_pkg_pending`) | `POST /api/v1/topup/intent` |
| Bot deep link | `cmd_ref.go:54` (`t.me/<botUsername>?start=<payload>`) | Web→Bot SSO bootstrap |
| Admin allowlist | `bot.IsAdmin(cfg, tgID)` + `cfg.AdminTelegramIDs` | Web admin page guards |
| Migration tooling | `internal/db/migrator/` + `internal/migrations/embed.go` | New migrations follow `YYYYMMDD<seq>_<slug>.sql` goose format |
| sqlc codegen | `services/api/sqlc.yaml` + `make sqlc-gen` | Add new query files, run `make sqlc-gen` |
| CI typecheck/build | `.github/workflows/ci.yml` node job | Already covers `apps/landing` via `pnpm -r typecheck` + `pnpm -r build` |

**Identified gaps to fill in Phase 3:**
- HTTP API key auth middleware (none exists — webhook auth is shared-secret only)
- CORS middleware on Fiber (not configured)
- Tailwind + shadcn/ui in `apps/landing` (greenfield)
- API client + auth state management on FE (greenfield)
- Frontend deploy pipeline (Vercel/Cloudflare/Fly Frontend — no config in CI)
- Shared TS types: `packages/shared-types/src/index.ts` is empty — needs DTO definitions OR OpenAPI spec + codegen
- Sessions/JWT for browser auth (only `JWT_SECRET` env var declared, no JWT issue/verify code)
- Email column on `users` (currently Telegram-only identity)
- Frontend `.env.example` for `NEXT_PUBLIC_API_BASE_URL`
- Telegram WebApp `validateInitData` helper (none in codebase)

---

## Unresolved Questions

1. **Auth model** — Will Phase 3 web app use existing API key (bearer) for end users, or introduce session/JWT cookie auth? Spec hints `JWT_SECRET` env var exists but unused.
2. **Web→Bot login UX** — Telegram Login Widget (oauth-style HMAC) vs Telegram WebApp (in-bot embedded) vs deep-link round-trip with one-time token? Each has different security model and UX.
3. **Email** — No `email` column on `users` table. Does Phase 3 require email/password fallback (Resend already wired in env), or strictly Telegram-only?
4. **`services/web/landing` vs `apps/landing`** — Plan brief mentions `services/web/landing` but actual path is `apps/landing`. Naming/relocation decision needed before Phase 3 scaffolding.
5. **OpenAPI** — Adopt OpenAPI 3.1 spec + codegen (e.g. oapi-codegen Go, openapi-typescript FE) to keep `packages/shared-types` in sync, or hand-roll types?
6. **shadcn/ui** — Is the design system shadcn (Radix+Tailwind) confirmed, or open (e.g. Mantine, Chakra)?
7. **Frontend deploy target** — Vercel (zero-config Next.js) or Fly.io (uniform with backend, single ops surface) or Cloudflare Pages?
8. **Trusted proxies** — `server.go` currently `0.0.0.0/0`; should be tightened to Fly internal CIDR before exposing browser-driven API.
9. **CORS** — Origin allowlist for `localhost:3000` (dev) + production frontend domain — needs decision early.
10. **API versioning convention** — Brief specifies `/api/v1/*`; confirm prefix lives directly on Fiber app or in a sub-router group with shared middleware.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Backend Phase 2 fully shipped with reusable services (User, Key, Wallet, Tx, Webhook, Admin, Audit) and 14 PG tables; landing scaffold is bare Next.js 15 placeholder with no Tailwind/shadcn/auth. Phase 3 web app is greenfield: needs new HTTP auth middleware (no precedent), CORS, sessions, frontend deploy pipeline, and shared-types population — but rich service layer exists for direct DI. Path mismatch noted: brief says `services/web/landing` but actual is `apps/landing`.
