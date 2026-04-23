# Red-Team Review — Phase 1 Foundation

**Reviewer:** code-reviewer (adversarial)
**Date:** 2026-04-24
**Target:** `plans/260424-0247-phase-1-foundation/` (8 files)
**Approach:** Hostile-but-fair. Attack ordering, transitive deps, Windows compat, version pins, CI vacuity.

---

## Critical Issues (MUST fix before /ck:cook)

**C1. DAG ordering bug: Phase 03 `make migrate-up` smoke test cannot run without Phase 04.**
- `phase-03-database-layer.md` step 13 (Success Criteria line 162–166) requires `make migrate-up` / `psql -c "\dt"` / `SELECT COUNT(*) FROM dork_patterns` to run — but no Postgres container exists until Phase 04.
- DAG says 03 → 07 and 04 → 07 (parallel). If cook executes 03 immediately after 02, success gate is unsatisfiable.
- **Fix:** Either add `phase-04-local-infra` to `blockedBy` of phase-03, OR split 03 into `03a-migration-code` (no DB needed) + `03b-migration-smoke-test` (after 04). Plan also lists 03 as agent `code-reviewer` but code-reviewer doesn't run migrations — unclear handoff.

**C2. Phase 02 Success Criteria requires running binary against Postgres+Redis — no infra yet.**
- `phase-02-go-api-skeleton.md` lines 129–131: `go run ./cmd/api` + `curl /health` + kill-PG → 503. This presumes PG + Redis exist.
- DAG puts 02 before 04. If `/health` is pure, it works. But `/ready` check for the 503 case requires PG running first then killed — impossible in a pre-04 environment.
- **Fix:** Move PG+Redis-dependent success criteria from Phase 02 to Phase 04; Phase 02 success = build+boot+health (no `/ready` 503 test).

**C3. Root `.env` required to boot docker-compose — who creates it?**
- `phase-04-local-infra.md` line 83 mentions `.env` placeholder (gitignored), line 92 uses `${POSTGRES_USER:-postgres}` interpolation — compose file has defaults so `.env` is strictly optional. OK so far.
- BUT `phase-04` line 37 asserts compose reads `env_file: ../services/api/.env` for API config. `.env` is gitignored by Phase 01 (current `.gitignore` line 44 `.env*` already blocks). Fresh clone → no `services/api/.env`. If compose file declares `env_file:` on API service, compose fails boot. If it doesn't, API doesn't know creds.
- Also: existing `.gitignore` line 44 `.env*` with exception `!.env.example` means Phase 01's new `.env.claudekit.example` is ignored unless added to exception list. Phase 01 doesn't explicitly update the `.env*` exception.
- **Fix:** (a) Add an instruction step "copy `.env.example` → `.env` on first run" in README + Makefile `dev-setup` target. (b) Explicitly update `.gitignore` exception to `!.env.example` AND `!.env.claudekit.example`.

**C4. goose `CREATE FUNCTION $$ ... $$` multi-statement blocks will break parser without `StatementBegin/End` markers.**
- `phase-03-database-layer.md` §Risk Assessment row 1 acknowledges this, but Implementation Steps line 99 says "begin file with `-- +goose Up` `-- +goose StatementBegin` markers" — singular block. The init DDL has **4 function-like blocks**: `check_active_campaign_limit()`, `consume_credits()`, `grant_credits()`, trigger `trg_campaign_limit`. Each `$$ ... $$` needs its own `-- +goose StatementBegin` / `-- +goose StatementEnd` wrapper. A single statement-begin around the whole file will NOT work — goose needs to know block boundaries.
- **Fix:** Spell out explicitly: "wrap EACH `CREATE FUNCTION` + `CREATE TRIGGER` block individually with `StatementBegin`/`StatementEnd` markers (4 blocks total)."

**C5. CI `go test ./...` passes vacuously — Phase 02 writes zero tests.**
- `phase-06-quality-gates.md` line 100 mandates `go test ./... -race -cover`. But Phase 02 creates no `*_test.go` files. Success = vacuous pass. Claim of "race detector works" is unverified.
- MASTER_PROMPT §15 mandates ≥80% coverage — impossible to stage with no tests.
- **Fix:** Phase 02 add one smoke test (e.g., `handlers/health_test.go` testing Health handler returns 200 + JSON body). Otherwise Phase 06 CI test stage is ceremony.

**C6. Dockerfile Go version vs distroless glibc compat claim needs explicit CGO_ENABLED=0 — ADR deviation not propagated.**
- ADR Deviation table (plan.md) says Go 1.26.2. `phase-02` step 16 Dockerfile: `golang:1.26-alpine` + distroless/static-debian12. Distroless/static has NO libc at all — CGO_ENABLED=0 is mandatory. Step 16 has `CGO_ENABLED=0` ✓ but doesn't mention `GOOS=linux` (plan on Windows may pick up GOOS=windows default under Git Bash builds; `docker build` runs inside container so ok, but the Makefile `build` target `go build -o bin/api ./cmd/api` in step 15 doesn't set CGO_ENABLED — if user builds locally on Windows + golangci includes gosec (which may pull CGO deps), issue.
- pgx v5 is pure Go ✓, go-redis v9 pure Go ✓, zap pure Go ✓. Confirmed. But fiberzap/v2 dependency tree? Not explicitly verified in plan — depends on fiber which is pure Go, confirmed OK.
- **Fix:** Makefile `build` target add `CGO_ENABLED=0 GOOS=linux` for parity with Docker. Note in Risk Assessment.

**C7. Plans directory is gitignored — initial commit will NOT include plans/.**
- Current `.gitignore` line 61: `plans/**/*` with `!plans/templates/*` exception only.
- Phase 07 step 8: `git add -A && git commit -m "feat(repo): scaffold monorepo + phase 1 foundation"` — plans/260424-0247-phase-1-foundation/*.md will NOT be committed. Scout/researcher reports in `plans/reports/` also excluded.
- If design intent: plans are ephemeral, not committed. If intent: plan is project history, commit it. Plan is silent.
- **Fix:** Decide + document. If keep plans: add `!plans/**/*.md` or `!plans/260424-0247-*` exception in Phase 01 `.gitignore` expansion. Phase 07 success criteria should explicitly assert "plan docs committed" OR "plans intentionally excluded".

**C8. Phase 07 `git init` runs AFTER Phase 06 but Phase 06 modifies `.husky/pre-commit` which requires active git repo to trigger.**
- `phase-06` line 96: creates `.husky/pre-commit`. `phase-07` step 8: `git init`. Husky v9 install path — `prepare: "husky"` in package.json runs `husky` binary which needs `.git` to install hooks.
- `pnpm install` in Phase 01 (before git init) will run `prepare` script → husky will fail or no-op silently. Resulting `.husky` directory symlinks/config may be inconsistent.
- Current husky 8 handles via `husky install || true` (package.json line 10). Phase 06 switches to `husky` command without `|| true` (line 72 "prepare: husky").
- **Fix:** Either (a) run `git init` in Phase 01 step before `pnpm install`, or (b) keep `prepare: "husky || true"` to tolerate pre-init state, or (c) Phase 07 explicitly re-runs `pnpm install` post-init to wire hooks. Option (a) cleanest; Phase 07 then just creates branches + first commit.

---

## High Issues (SHOULD fix)

**H1. Windows Makefile compat: Git Bash vs PowerShell — user is on Windows 11.**
- `phase-04` root Makefile uses `docker compose -f ops/docker-compose.yml up -d`, `&&`, `sleep 5`. Default Windows shell for `make` is `cmd.exe` unless `SHELL := /usr/bin/bash` set. `make` itself is not a Windows builtin — requires Chocolatey `make` or Git Bash PATH.
- `phase-07` step 1 uses bash-specific `for f in scripts/*; do ... done`. Won't execute in PowerShell or cmd.
- `phase-03` Makefile target `@test -n "$(name)" && goose ...` — bash-only.
- **Fix:** Phase 01 README/docs state "requires Git Bash or WSL2". Phase 04 Makefile header add `SHELL := bash` (prevents silent cmd execution). Phase 07 bash script block note "run in Git Bash".

**H2. `sleep 5` in `make dev` is race-prone; compose `--wait` is better.**
- `phase-04-local-infra.md` line 74 `dev: dev-up && sleep 5 && migrate && run-api`. Risk Assessment row 3 acknowledges and recommends `--wait`. Why wasn't the Implementation Step updated?
- **Fix:** Replace `sleep 5` with `docker compose up -d --wait` in `dev-up` target; drop the sleep from `dev`.

**H3. `turbo run build --dry-run` success criteria in Phase 05 — shared-types has no build task but typecheck is claimed.**
- `phase-05` Success Criteria line 138: "exactly 2 build tasks (landing + extension)" — but turbo.json from Phase 01 defines `build` task globally. If `shared-types/package.json` has no `build` script, turbo may either (a) skip silently, (b) warn, (c) fail depending on turbo.json strictness. Implementation Step 12 doesn't add a `build` script to shared-types — OK. But Phase 05 also claims `pnpm -r typecheck` succeeds across all 3 packages (line 137) which requires each package has a `typecheck` script — verified for all 3 in steps 2/7/12 ✓.
- Potential issue: turbo v2 strict mode may error on missing `build` in a workspace that pnpm thinks is part of workspaces. Depends on turbo.json `dependsOn` chain.
- **Fix:** Add explicit step in Phase 01 turbo.json design: `build` task with `dependsOn: ["^build"]` and allow-empty on packages without build script (turbo handles this by default — but document in turbo.json comment).

**H4. `pnpm biome check` vs `pnpm lint` inconsistency; biome config ignores `plans/**` but pre-commit path differs.**
- `phase-06` line 39: CI runs `pnpm biome check .`. Line 93: biome.json ignores `plans/**`, `docs/MASTER_PROMPT.md`. Line 102 CI step web: `pnpm lint`. Package.json script (line 98): `lint: "biome check ."`. Consistent. ✓
- But `.lintstagedrc.json` uses `*.{ts,tsx,svelte,json,md}` (line 94) — lint-staged matches files by glob, then runs biome ON THEM. Biome's own `files.ignore` list handles subsequent filter. If a file in `plans/` is staged (unlikely but possible), biome will lint it. If biome.json ignore takes effect → OK; if lint-staged runs biome with explicit file args, biome may override its own ignore rules (biome precedence: CLI args trump config ignores).
- **Fix:** Low risk. Consider `.lintstagedrc.json` glob scope tighter: `"apps/**/*.{ts,tsx,svelte,json}"`, `"packages/**/*.{ts,tsx,svelte,json}"` — excludes docs/plans explicitly.

**H5. Seed dork patterns: 30 patterns claimed, but enumerated count in Phase 03 step 4 = 8+8+8+6 = 30 ✓ — BUT step 4 lists only ~10 example patterns inline. Full list not spelled out.**
- Risk: implementer guesses remaining patterns poorly (repetition, bad Google dork syntax). MASTER_PROMPT §3.2 shows only 5 examples.
- **Fix:** Either commit full 30-pattern list now in plan file (30 lines SQL), or acknowledge "implementer to draft from templates; reviewer validates" in Phase 03.

**H6. TS 6.0.3 across all package.json — verify devDep pins match.**
- plan.md Key Version Pins: TypeScript 6.0.3.
- Phase 01 step 1 devDeps: `typescript: 6.0.3` ✓.
- Phase 05 steps 2, 7, 12 use `typescript@workspace:*` — meaning workspace root owns exact version. Requires root to have `typescript` as devDep, which it does (Phase 01 step 1). ✓
- BUT: Next.js 15.5 ships `@types/react@^19`. TS 6 may emit type errors on React 19 prerelease types. Untested on this specific combo.
- **Fix:** Note as Known Risk in `phase-05` Risk Assessment row 3 (already partial note on Svelte 5 + TS 6 — extend to Next+React).

**H7. Biome 2.3 + Svelte 5 integration "stable per research" but `.svelte` files exist only in Phase 3+.**
- Phase 06 biome.json enables `**/*.svelte` override. No `.svelte` file in Phase 1 to validate. Risk: config is untested. Acceptable for Phase 1 scope.
- **Fix:** None; note in Phase 06 Success Criteria that svelte lint path is Phase 3 gated.

**H8. CRX-JS permission list includes `offscreen` — but Phase 1 has no offscreen doc.**
- `phase-05` step 10 manifest permissions: `["offscreen", "storage", "alarms"]`. MV3 `offscreen` permission without a reason is usually flagged by Chrome Web Store review (Phase 10 concern). For Phase 1 local build, manifest parses fine. But if `background/index.ts` logs console only, `alarms` and `offscreen` are unused — dead permissions.
- **Fix:** Drop unused permissions for Phase 1 (use `[]`); add back when consumed in Phase 3.

**H9. `.husky/commit-msg` Phase 06 upgrade to v9 — verify CI skip still works.**
- Current commit-msg (line 6): `if [ "$CI" = "true" ]; then exit 0; fi` — ClaudeKit pattern. Phase 06 step 5 says "drop v8 `_/husky.sh` sourcing" but doesn't explicitly preserve the CI skip. If implementer follows literally, CI runs commitlint on every commit in workflow — may block bot commits.
- **Fix:** Phase 06 step 5 explicit: "preserve `if [ \"$CI\" = \"true\" ]; then exit 0; fi` guard; only drop `_/husky.sh` sourcing."

**H10. Phase 07 LICENSE question flagged but not decided — blocker for git init commit with LICENSE in tree.**
- `phase-07` Security Considerations line 184: "LICENSE = MIT inherited from ClaudeKit — verify this is intended for Snake Backlink Forge; if not, swap to proprietary (user decision, flag in Unresolved Qs)." Initial commit will include LICENSE file — if license is wrong, bad provenance.
- **Fix:** Resolve with user BEFORE /ck:cook starts Phase 07. If proprietary, replace LICENSE in Phase 01 or 07 step 0.

---

## Medium Issues (nice to fix)

**M1. Goose config: SetDialect string case matters. `"postgres"` (lowercase) is correct per goose v3; doc-check passes. ✓ No fix needed.**

**M2. Phase 02 Success Criteria image size ≤25MB — claimed in Non-functional <20MB. Inconsistent.**
- Line 47 Non-functional: "Distroless image < 20MB".
- Line 132 Success Criteria: "image ≤ 25MB".
- Distroless/static-debian12:nonroot base is ~2.5MB. Go binary with zap+fiber+pgx+redis ~15-20MB. 20MB target aggressive but achievable; 25MB safer.
- **Fix:** Align both to 25MB (safety margin) or ditch the non-functional claim.

**M3. Phase 04 Makefile `migrate` target calls `cd services/api && make migrate-up`. Requires `services/api/Makefile` from Phase 02. Phase 04 blocks on Phase 01 only — not Phase 02. Cook may execute 04 before 02.**
- DAG says Phase 04 `blockedBy: [phase-01-monorepo-backbone]`. Phase 04 can run parallel with 02.
- But the `migrate` target content reads `cd services/api && make migrate-up` — the Makefile exists only after Phase 02 completes. Until then, `make migrate` at root fails.
- Phase 04 Success Criteria line 117 `make migrate applies clean (from Phase 03)` — implicit Phase 03 dependency, which requires Phase 02.
- **Fix:** Add `blockedBy: [phase-02-go-api-skeleton]` to phase-04 OR split Makefile creation (minimal targets for Phase 04, `migrate` left as TODO until 03).

**M4. golangci-lint v1.60 pinned in Phase 06 CI — is it compatible with Go 1.26?**
- golangci-lint v1.60 released Aug 2024 — supports Go up to 1.23 per compat matrix at release time. Go 1.26 (April 2026) requires golangci-lint likely v1.65+ for full analyzer coverage.
- **Fix:** Research current golangci-lint version supporting Go 1.26 (probably v1.68+). Pin accordingly. Otherwise CI warns "unsupported Go version" and some analyzers skip.

**M5. `.env.claudekit.example` rename loses Claude Code hook notification setup for Phase 1 developers.**
- Phase 01 step 6 renames `.env.example` → `.env.claudekit.example`. ClaudeKit hooks read `.env` (gitignored). After rename, fresh dev clones don't know `.env.claudekit.example` is the template — discovery pattern is `.env.example`.
- **Fix:** README or `.env.example` stub with one-line pointer: "For Claude Code hook notifications, see `.env.claudekit.example`. For API, see `services/api/.env.example`."

**M6. `sqlc.yaml` `schema` points to single file `20260424_001_init.up.sql` — seed file not tracked as schema.**
- `phase-03` step 11: `schema: "internal/migrations/20260424_001_init.up.sql"`. If Phase 2+ adds another migration (`20260501_003_*.up.sql`), sqlc won't see it — regenerated types will be stale.
- **Fix:** Use glob pattern `schema: "internal/migrations/*.up.sql"` — sqlc supports this. Document in phase-03.

**M7. `internal/db/sqlc/.gitkeep` — but sqlc generates files in this directory. `.gitignore` treatment ambiguous.**
- Phase 03 Implementation line 87 creates `.gitkeep`. Phase 01 risk row 3 mentions whitelist `!services/api/internal/db/sqlc/`. Decision: should sqlc-generated code be committed or regenerated? Both are valid; spec silent.
- **Fix:** Commit generated code (more predictable for new devs — no need to install sqlc CLI just to build). Drop `.gitkeep` idea. Phase 01 `.gitignore` section for generated code should NOT add sqlc output to ignore.

**M8. `phase-02` step 9 /ready ping uses 2s context per service. Combined worst case 4s. Fly grace period `10s`. OK.**
- But health-check timeout in compose (`phase-04` PG: `timeout 3s`) may flap if PG takes >3s to respond on cold start. Healthcheck retry 10 × 5s = 50s max wait — OK.
- **Fix:** None. Document.

**M9. Phase 07 `git mv` for script relocation — but no git repo exists yet.**
- Phase 07 Requirements line 46: "done via `git mv` (preserves history if later initialized)". Nonsense — if git init happens later (step 8), `git mv` before init is impossible, and `mv` before init has no history to preserve.
- **Fix:** Either move BEFORE `git init` (just `mv`, no git), or move AFTER `git init` with `git mv`. Current ordering in step 8 is git init AFTER move in step 1 — so step 1 `mv` is fine; remove "git mv" wording from requirements.

---

## Nits & Clarifications

**N1.** Phase 01 step 1 pins `@commitlint/cli: ^20.5.0` — commitlint current is v19. v20 may not be released (as of 2026-04). Double-check via npm.

**N2.** Phase 03 step 8 references `cmd/migrate/main.go` — not listed in Architecture diagram (only `services/api/cmd/api/main.go` shown in Phase 02). Add to Architecture for clarity.

**N3.** Phase 02 `.env.example` docs `sslmode=require` for prod, `sslmode=disable` for local. Fine, but mention Fly postgres requires TLS; user may forget when adding production secrets.

**N4.** Phase 05 Risk row 2 "Next 15.5 requires Node >= 20". Current package.json engines `node >= 18`. Phase 01 step 1 updates to `>= 20`. ✓ consistent. Nit: worth single-source-of-truth check.

**N5.** "30 dork patterns" distribution 8+8+8+6. MASTER_PROMPT §3.2 says "30 patterns VN + EN". Plan requires 6 VN patterns only (20%). Adequate for Phase 1. Not an issue.

**N6.** Phase 06 step 7 `golangci/golangci-lint-action@v6` — v6 released Mar 2024. Current per Apr 2026 likely v7+. Verify.

**N7.** Phase 06 `.gitleaks.toml` allowlist regex `\.example$` — matches `.env.example` ✓, but also allowlists any file ending `.example`. Tight enough; no fix.

**N8.** Phase 07 `docs/glossary.md` copies MASTER Appendix A lines 2830–2852 = 22 lines. Check actual bounds; may be longer.

**N9.** `phase-04` Docker Desktop on Windows WSL2 mention missing. Docker Desktop defaults to WSL2 backend in Win11 — usually fine. But PG 16 on WSL2 named volume works correctly only if Docker Desktop integration enabled for default WSL distro. Minor troubleshooting note.

**N10.** Plan total effort 13h claimed but Phase 01 1.5h + 02 3.5h + 03 2.5h + 04 1h + 05 1.5h + 06 1.5h + 07 1.5h = 13h ✓.

---

## Verdict

**APPROVE_WITH_PATCHES**

Plan is well-researched, thorough, and traceable to MASTER_PROMPT + scout + researchers. Version pins are mostly defensible. ADR deviations (Go 1.26, TS 6, fiberzap) are justified with evidence.

Issues are ordering + Windows compat + infra-dependency gates — fixable in <1h of plan patches before /ck:cook. Critical issues C1/C2/C7/C8 will cause cook to fail mid-phase if unaddressed.

**Overall:** structurally sound, execution-ready after patches. Do NOT proceed to /ck:cook until C1–C8 patched.

---

## Suggested Patches

For each Critical/High (one-liner, phase file + change):

**C1** `plans/260424-0247-phase-1-foundation/plan.md` — update DAG: `P04 --> P03` instead of parallel; update `phase-03.md` frontmatter `blockedBy: [phase-02-go-api-skeleton, phase-04-local-infra]`.

**C2** `phase-02-go-api-skeleton.md` Success Criteria — move lines 130–131 (PG-dependent tests) to `phase-04-local-infra.md` Success Criteria.

**C3** `phase-01-monorepo-backbone.md` step 5 — add `.gitignore` exception `!.env.claudekit.example`; `phase-04-local-infra.md` step 6 — add new step "copy `services/api/.env.example` to `services/api/.env` on first run; Makefile `dev-setup` target automates."

**C4** `phase-03-database-layer.md` step 2 — rewrite: "Wrap each `CREATE FUNCTION ... $$ ... $$ LANGUAGE plpgsql;` AND each `CREATE TRIGGER` with its own `-- +goose StatementBegin` / `-- +goose StatementEnd` pair. 4 blocks total in init.up.sql: check_active_campaign_limit, consume_credits, grant_credits, trg_campaign_limit trigger body."

**C5** `phase-02-go-api-skeleton.md` add Todo + file: `internal/api/handlers/health_test.go` — assert Health returns 200 JSON; Ready returns 200 with mocked pools.

**C6** `phase-02-go-api-skeleton.md` step 15 Makefile `build` target: `CGO_ENABLED=0 GOOS=linux go build -o bin/api ./cmd/api`.

**C7** `phase-01-monorepo-backbone.md` step 5 `.gitignore` expansion — add: `!plans/260424-0247-phase-1-foundation/**`, `!plans/reports/**` OR explicit decision to exclude (document in plan.md).

**C8** `phase-01-monorepo-backbone.md` — add step 0 `cd . && git init -b main` BEFORE step 7 `pnpm install`; `phase-07-docs-housekeeping.md` step 8 — remove `git init -b main`, keep branch creation + initial commit only.

**H1** `phase-04-local-infra.md` Makefile header — add `SHELL := bash`. `phase-07-docs-housekeeping.md` step 1 — note "run in Git Bash on Windows".

**H2** `phase-04-local-infra.md` step 1 / Makefile `dev` target — replace `sleep 5` with `docker compose up -d --wait`.

**H3** `phase-01-monorepo-backbone.md` step 3 turbo.json — add comment: "`build` task is optional per-workspace; absent scripts skip silently".

**H4** `phase-06-quality-gates.md` step 2 `.lintstagedrc.json` — scope `*.{ts,tsx,svelte,json}` to `{apps,packages,services}/**/` glob only.

**H5** `phase-03-database-layer.md` step 4 — commit full 30-line SQL insert block OR delegate to reviewer with explicit validation note.

**H8** `phase-05-frontend-stubs.md` step 10 manifest — `permissions: []`, note "add offscreen/alarms/storage Phase 3".

**H9** `phase-06-quality-gates.md` step 5 — explicit: "preserve existing `$CI` guard in commit-msg; only drop `_/husky.sh` sourcing line."

**H10** BEFORE cook: ask user "confirm LICENSE=MIT for Snake Backlink Forge?" — if not, replace LICENSE file in Phase 01.

---

## Unresolved Questions

1. **Plans commit policy** — plan/research/reports files: commit to repo (history) or exclude (ephemeral)? Affects `.gitignore` in Phase 01 + Phase 07 initial commit scope.
2. **LICENSE final decision** — MIT (inherited) vs proprietary for Snake Backlink Forge?
3. **sqlc generated code commit policy** — commit `internal/db/sqlc/*.go` or regenerate? Affects Phase 01 `.gitignore` whitelist.
4. **golangci-lint / golangci-lint-action current-supported versions for Go 1.26** — verify Apr 2026 release matrix before pinning.
5. **Windows primary or fallback?** — user on Win11; plan assumes Git Bash for Makefile. WSL2 recommended in README? Decision affects which smoke-test commands validate.
6. **Phase 03 reviewer handoff** — plan says `code-reviewer` agent participates in Phase 03 but its role (DDL correctness review? migration reversibility check?) is under-specified.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Plan is well-structured and execution-ready after 8 Critical + 10 High patches. Core issues are DAG ordering, .env/.gitignore transitive deps, Windows compat, and CI vacuity on missing tests.
**Concerns/Blockers:** C1–C8 must be patched before /ck:cook or cook will fail mid-phase. LICENSE + plans-commit-policy decisions block Phase 07 initial commit.
