# Scout Report — Phase 1 Foundation state

**Date:** 2026-04-24
**Scope:** Verify current state of `tool_backlink` repo vs Phase 1 Foundation spec (MASTER_PROMPT.md §13, §2)
**Context:** Repo bootstrapped từ ClaudeKit Engineer template v2.14.0 (Apr 3). Chưa có bất kỳ code nào của Snake Backlink Forge. Repo KHÔNG phải git repo (no .git). CWD = `C:\Users\Hanna\Desktop\tool_backlink`.

---

## Gap Analysis

| # | Item | Status | Path | Note |
|---|------|--------|------|------|
| 1 | Root `package.json` | **PARTIAL** | `/package.json` | Là ClaudeKit Engineer (v2.14.0), không phải Snake Backlink Forge. Thiếu pnpm workspaces config. Cần rewrite (giữ husky/commitlint) |
| 1 | `pnpm-workspace.yaml` | **ABSENT** | — | Phải tạo, list `apps/*`, `packages/*`, `tools/*` |
| 1 | `turbo.json` | **ABSENT** | — | Phải tạo, pipeline: build/lint/test/typecheck |
| 1 | pnpm lockfile | **ABSENT** | — | Sẽ gen sau `pnpm install` |
| 2 | `apps/landing/` | **ABSENT** | — | Scaffold Next.js 15 (chỉ skeleton Phase 1, chi tiết Phase 9) |
| 2 | `apps/extension/` | **ABSENT** | — | Scaffold Vite+CRXJS+Svelte 5 (skeleton, WASM Phase 3) |
| 3 | `services/api/go.mod` | **ABSENT** | — | Go 1.23 module, Fiber v2, goose, sqlc, zap, redis, pgx |
| 3 | `services/api/cmd/api/main.go` | **ABSENT** | — | Fiber app boot + /health + /ready |
| 3 | `services/api/internal/**` | **ABSENT** | — | config, db, redis, middleware, api/server.go, util |
| 4 | Migrations folder | **ABSENT** | `services/api/internal/migrations/` | `20260424_001_init.up.sql` (full DDL §3.1) + `.down.sql` + `20260424_002_seed_dorks.up.sql` |
| 4 | `goose` config | **ABSENT** | — | CLI via `go install` + Makefile targets `migrate-up/down/new` |
| 5 | `sqlc.yaml` | **ABSENT** | `services/api/sqlc.yaml` | queries dir = `internal/db/queries/`, out = `internal/db/sqlc/` |
| 6 | `ops/docker-compose.yml` | **ABSENT** | — | pg16 + redis7 services, named volumes, env via .env |
| 6 | `ops/` dir | **ABSENT** | — | Sẽ chứa runbooks/ (Phase 10) |
| 7 | `.editorconfig` | **ABSENT** | — | Cần — tab/space, EOL LF, final newline |
| 7 | `golangci-lint` config | **ABSENT** | — | `.golangci.yml` với govet, gosec, errcheck, staticcheck |
| 7 | Biome/ESLint config | **ABSENT** | — | Biome v1 (recommend) ở root, hoặc per-app ESLint |
| 7 | Stylelint config | **ABSENT** | — | Optional Phase 1, chủ yếu Phase 9 khi có Tailwind |
| 7 | `.gitignore` Go/Rust/WASM | **PARTIAL** | `/.gitignore` | Có Node+Flutter; THIẾU: `*.exe`, `*.test`, `vendor/`, `target/`, `pkg/` (WASM), `/apps/extension/crates/*/target/`, `*-debug.log`, `.idea/`, `Thumbs.db`, `private-key.pem` |
| 8 | `.github/workflows/ci.yml` | **ABSENT** | — | Lint + test Go + TS per PR |
| 8 | `.github/workflows/deploy-api.yml` | **ABSENT** | — | Fly deploy (Phase 12, skeleton ok Phase 1) |
| 8 | Existing workflows | **PARTIAL** | `/.github/workflows/` | Chỉ có release-beta, release, sync-dev-after-release, sync-main-to-dev, branch-protection (ClaudeKit boilerplate — giữ hoặc refactor) |
| 9 | `packages/shared-types/` | **ABSENT** | — | Skeleton `package.json` + `tsconfig.json` + `src/index.ts` (stub) |
| 9 | `installer/` | **ABSENT** | — | Phase 7 chi tiết, Phase 1 chỉ placeholder README |
| 9 | `tools/crx-packager/` | **ABSENT** | — | Phase 7, Phase 1 skip |
| 9 | `tools/db-seed/` | **ABSENT** | — | Phase 1 optional |
| 10 | `scripts/` | **PARTIAL** | `/scripts/` | Toàn script của ClaudeKit (generate-opencode, release-manifest, discord...) — KHÔNG liên quan Snake Backlink Forge. Giữ nguyên (không conflict) hoặc move sang `scripts/claudekit/` |
| 10 | `.husky/commit-msg` | **PRESENT** | `/.husky/commit-msg` | Commitlint hook — giữ |
| 10 | `.husky/pre-commit` | **ABSENT** | — | Cần thêm: `go vet`, `golangci-lint`, `pnpm lint`, `pnpm typecheck`, gitleaks |
| 11 | `docs/MASTER_PROMPT.md` | **PRESENT** | `/docs/MASTER_PROMPT.md` | 2947 dòng — source of truth |
| 11 | `docs/architecture.md` | **ABSENT** | — | Cần tạo (condensed version của MASTER_PROMPT §1-2) |
| 11 | `docs/api-contract.md` | **ABSENT** | — | Detail từ §4 |
| 11 | `docs/action-protocol.md` | **ABSENT** | — | server↔ext message spec |
| 11 | `docs/threat-model.md` | **ABSENT** | — | từ §16 |
| 11 | `docs/glossary.md` | **ABSENT** | — | Appendix A |
| 11 | `docs/progress.md` | **ABSENT** | — | Phase tracking (§20.5) |
| 11 | ClaudeKit docs | **PRESENT** | `/docs/*.md` | project-overview-pdr, code-standards, codebase-summary, system-architecture, project-roadmap, agent-teams-guide, skill-native-task, skills-interconnection-map, references/, research/, journals/, infographics/, assets/ — giữ nhưng KHÔNG phải cho Snake Backlink Forge |
| 12 | `.env.example` root | **PARTIAL** | `/.env.example` | Chỉ Discord/Telegram webhook (cho ClaudeKit hook notifications). Cần tạo `services/api/.env.example` với 16 vars từ §12.1 |
| — | Git repo init | **ABSENT** | — | `git init` + `main`/`dev` branches, remote (Phase 1 deliverable phụ) |
| — | Tailwind v4, shadcn configs | **ABSENT** | — | Phase 9 scope, không Phase 1 |
| — | Rust crate `sbf-wasm` | **ABSENT** | `apps/extension/crates/sbf-wasm/` | Phase 3 scope |

---

## What Phase 1 MUST Create

**Priority 1 — Monorepo backbone**
1. Rewrite root `package.json` (Snake Backlink Forge root; keep husky, commitlint, semantic-release — remove ClaudeKit-specific devDeps)
2. `pnpm-workspace.yaml` listing `apps/*`, `packages/*`, `tools/*`
3. `turbo.json` pipeline (build, lint, test, typecheck, clean)
4. `.editorconfig`
5. Expand `.gitignore` (Go, Rust target/, WASM pkg/, Windows, *.pem already ok, private-key.pem explicit)

**Priority 2 — Go API skeleton**
6. `services/api/go.mod` + `go.sum` (deps: fiber v2, pgx/v5, go-redis/v9, zap, joho/godotenv, uuid, pressly/goose, hmac stdlib)
7. `services/api/cmd/api/main.go` (Fiber + health/ready + graceful shutdown)
8. `services/api/internal/config/config.go` (env parse)
9. `services/api/internal/db/db.go` (pgx pool init)
10. `services/api/internal/redis/redis.go` (client init)
11. `services/api/internal/api/server.go` + `router.go` + `handlers/health.go`
12. `services/api/internal/middleware/{logger,recover}.go` (auth/hmac/ratelimit/nonce là Phase 2-3)
13. `services/api/Makefile` (build, run, lint, test, migrate-up/down/new, sqlc-gen)
14. `services/api/Dockerfile` (multi-stage từ §12.3)
15. `services/api/.env.example` (16 vars từ §12.1)
16. `services/api/.golangci.yml`

**Priority 3 — Database**
17. `services/api/internal/migrations/20260424_001_init.up.sql` + `.down.sql` (full DDL §3.1 — users, api_keys, wallets, ledger, transactions, campaigns+trigger, targets, jobs, domain_cooldown, dork_patterns, audit_log, safeguard_hits + consume/grant stored procs + v_user_stats view)
18. `services/api/internal/migrations/20260424_002_seed_dorks.up.sql` + `.down.sql` (30 dork patterns từ §3.2)
19. `services/api/sqlc.yaml` + query stubs: `users.sql`, `keys.sql`, `wallets.sql`, `campaigns.sql`, `targets.sql`, `jobs.sql`, `ledger.sql`, `audit.sql`
20. `services/api/db/goose/` scripts (hoặc dùng goose CLI)

**Priority 4 — Local dev infra**
21. `ops/docker-compose.yml` (postgres:16-alpine + redis:7-alpine, volumes, healthchecks)
22. Root `Makefile` (orchestrate compose up/down + make migrate + make run)

**Priority 5 — Frontend stubs (minimal, không full impl)**
23. `apps/landing/package.json` (Next.js 15, React 19, Tailwind 4 — stub, chưa cần content)
24. `apps/landing/next.config.mjs` + `tsconfig.json` + `src/app/layout.tsx` + `page.tsx` (Hello)
25. `apps/extension/package.json` (Vite, @crxjs/vite-plugin, Svelte 5, TypeScript 5.5)
26. `apps/extension/vite.config.ts` (basic, chưa obfuscate) + `src/manifest.config.ts`
27. `packages/shared-types/package.json` + `src/index.ts` (export `{}`)

**Priority 6 — CI**
28. `.github/workflows/ci.yml` (lint + test + build trên PR) — chạy `pnpm install`, `pnpm -r build`, `cd services/api && go vet && go test ./... -race`
29. `.husky/pre-commit` (gitleaks, lint, typecheck) — extend bên cạnh existing commit-msg

**Priority 7 — Docs + housekeeping**
30. `docs/architecture.md` (condensed MASTER §1-2)
31. `docs/progress.md` (template Phase N)
32. `CONTRIBUTING.md`, `SECURITY.md`
33. Rename/keep `README.md` để reflect Snake Backlink Forge
34. Move `scripts/*` sang `scripts/_claudekit/` (optional, tránh conflict Phase tương lai)

---

## Deliverable Success Criteria (Phase 1)

- [ ] `pnpm install` chạy clean ở root
- [ ] `docker compose up -d` bring up postgres + redis healthy
- [ ] `cd services/api && make migrate-up` apply cleanly (full §3.1 schema)
- [ ] `make migrate-down` revert cleanly (zero residue)
- [ ] `cd services/api && go vet ./... && go test ./...` pass (dù test mới chỉ smoke)
- [ ] `cd services/api && go run ./cmd/api` boot, `curl localhost:8080/health` → 200 JSON `{"status":"ok"}`, `/ready` → 200 sau khi DB+Redis reachable, 503 nếu không
- [ ] `turbo run lint` clean (landing + extension typecheck ok, dù empty)
- [ ] All Phase 1 files committed với conventional commit: `feat(repo): scaffold monorepo + foundation`

---

## Unresolved Questions

1. **Repo phải `git init` trong Phase 1 này?** Hiện chưa phải git repo. Spec nói "NEVER commit directly to dev/main" — ngụ ý cần git init + main/dev branches. Có cần tạo remote (GitHub) trong Phase 1 không, hay để phase sau?
2. **ClaudeKit boilerplate `scripts/`, ClaudeKit docs, existing workflows** — giữ nguyên, di dời sang `_claudekit/`, hay xóa? Đề xuất: giữ `.claude/` (hooks+agents+skills) nhưng di dời `scripts/*` và ClaudeKit-specific workflows sang namespace riêng để tránh đụng CI của Snake Backlink Forge.
3. **Root `package.json` name** — rename thành `snake-backlink-forge` hay giữ `claudekit-engineer`? Đề xuất rename + bump về `0.1.0`.
4. **`.env.example` Discord/Telegram webhook** (ClaudeKit hook notifications) — giữ để hook dev vẫn chạy? Đề xuất đổi tên thành `.env.claudekit.example` hoặc merge vào `services/api/.env.example` (không lý tưởng vì scope khác).
5. **Phase 1 có scaffold `apps/landing` và `apps/extension` tối thiểu không?** Spec Phase 1 chủ yếu focus Go + infra, nhưng scope có nói "Create apps/* stubs". Đề xuất: tạo `package.json` + config files, KHÔNG implement UI (để Phase 3+9).
6. **`sbf_live_` key prefix, SePay bank account, Telegram bot username** — cần trước khi code bot? Phase 1 chưa cần vì chỉ scaffold; Phase 2 sẽ block nếu thiếu. Note để user chuẩn bị.
