---
name: Quality gates (CI + husky + Biome + gitleaks)
phase: 06
status: pending
priority: P1
estimated_effort: 1.5h
blockedBy: [phase-02-go-api-skeleton, phase-05-frontend-stubs]
blocks: [phase-07-docs-housekeeping]
agents: [fullstack-developer, tester]
---

# Phase 06 — Quality Gates

## Context Links

- MASTER_PROMPT §15 Quality Gates (lines 2624–2667)
- MASTER_PROMPT §12.4 CI/CD base (lines 2221–2272)
- JS research §6 Biome, §9 gitleaks, §10 commitlint (`plans/reports/researcher-260424-0247-js-stack-versions.md`)
- Prior phases: 02 (provides Go module + golangci config), 05 (provides TS packages)

## Overview

**Priority:** P1 · **Status:** pending · **Effort:** 1.5h
Wire pre-commit (husky) + CI (GitHub Actions) that actually run on real code from phases 02+05. Biome formats/lints TS/Svelte. golangci-lint gates Go. gitleaks scans secrets. commitlint already present — verify scoped rules. NO deploy workflows Phase 1 (Phase 10).

## Key Insights

- **Husky 9** uses `.husky/<hook>` files executed directly (no `husky.sh` sourcing as in v8). Existing `.husky/commit-msg` is v8-style — upgrade simultaneously.
- **Biome 2.3** config at root; auto-detects `.ts/.tsx/.svelte/.json`. 10k files <1s (research §6). Replaces ESLint+Prettier for Phase 1.
- **gitleaks 8.18.2** — use `gitleaks protect --staged -v --redact` in pre-commit (scans staged diff only, fast).
- **lint-staged 15** — runs Biome only on changed files; Go files trigger `go vet` scoped to affected packages.
- **Existing `.github/workflows/*.yml`** (release-beta, release, sync-dev, etc.) are ClaudeKit — keep untouched Phase 1; add fresh `ci.yml` that coexists.

## Requirements

**Functional:**
- `.github/workflows/ci.yml` runs on PR + push to `dev`/`main` — jobs `go` + `web` run in parallel.
- `go` job: Go 1.26, `go vet ./...`, `golangci-lint run`, `go test ./... -race -cover`.
- `web` job: Node 20, pnpm 9.15.9, `pnpm install --frozen-lockfile`, `pnpm -r typecheck`, `pnpm -r build`, `pnpm biome check .`.
- `.husky/pre-commit` runs: gitleaks staged → `pnpm lint-staged` (Biome on changed .ts/.svelte/.json) → `cd services/api && go vet ./... && go fmt -l ./...` (fail if any file needs formatting).
- `.husky/commit-msg` retains commitlint (existing).
- `biome.json` root config — formatter (2-space, LF), linter recommended, ignores `dist`, `.next`, `node_modules`, `services/api/internal/db/sqlc` (generated).
- `.gitleaks.toml` with allowlist for `*.example`, `plans/**/*.md` (plan file hypotheticals), `docs/**/*.md` (prompt references).

**Non-functional:**
- CI total duration < 5 min on cold cache.
- Pre-commit total < 15s on typical change (< 20 files).

## Architecture

```
.github/workflows/
  ci.yml                                (NEW)
    jobs:
      go:                               (services/api)
        steps: checkout → setup-go 1.26 → cache → vet → golangci-lint → test
      web:                              (apps/* + packages/*)
        steps: checkout → setup-node 20 → pnpm 9.15.9 → install → typecheck → build → biome check

.husky/
  commit-msg        (KEEP — existing commitlint)
  pre-commit        (NEW — gitleaks + lint-staged + go vet)

biome.json          (NEW)
.gitleaks.toml      (NEW)
.lintstagedrc.json  (NEW)

package.json (root, from phase-01 — add scripts)
  scripts:
    lint: "biome check ."
    format: "biome format --write ."
    prepare: "husky"           (note: v9 syntax, not `husky install`)
```

## Related Code Files

**CREATE:**
- `C:\Users\Hanna\Desktop\tool_backlink\.github\workflows\ci.yml`
- `C:\Users\Hanna\Desktop\tool_backlink\biome.json`
- `C:\Users\Hanna\Desktop\tool_backlink\.gitleaks.toml`
- `C:\Users\Hanna\Desktop\tool_backlink\.lintstagedrc.json`
- `C:\Users\Hanna\Desktop\tool_backlink\.husky\pre-commit`

**MODIFY:**
- `C:\Users\Hanna\Desktop\tool_backlink\package.json` — add `lint`, `format`, update `prepare` to `husky` (v9), add devDeps `@biomejs/biome@2.3.0`, `lint-staged@^15.0.0` (already from phase-01).
- `C:\Users\Hanna\Desktop\tool_backlink\.husky\commit-msg` — upgrade to husky 9 shape (remove v8 `husky.sh` sourcing if present).
- `C:\Users\Hanna\Desktop\tool_backlink\.commitlintrc.json` if present — verify rules match research §10 (type-enum, scope-case kebab).

**DELETE:** none (keep existing ClaudeKit workflows).

## Implementation Steps

1. Create `biome.json` — `$schema` 2.3.0, `organizeImports.enabled`, `formatter` (space/2/lf/lineWidth 100), `linter.rules.recommended`, `files.ignore`: `node_modules`, `dist`, `.next`, `pkg`, `target`, `services/api/internal/db/sqlc/**`, `plans/**`, `docs/MASTER_PROMPT.md`; override `**/*.svelte` formatter enabled.
2. **[H4]** Create `.lintstagedrc.json` — scope TS/Svelte/JSON globs to project source directories only (not docs/plans which Biome may process unexpectedly when given explicit file args):
   - `apps/**/*.{ts,tsx,svelte,json}` → `biome format --write` + `biome lint`
   - `packages/**/*.{ts,tsx,svelte,json}` → `biome format --write` + `biome lint`
   - `services/api/**/*.go` → `gofmt -w` (Go files use separate golangci-lint runner, not Biome)
   - `docs/**` and `plans/**` explicitly excluded (not matched by above globs)
3. Create `.gitleaks.toml` — `[extend] useDefault=true`; allowlist paths: `\.example$`, `docs/MASTER_PROMPT\.md$`, `plans/.*\.md$`, `README\.md$`.
4. Create `.husky/pre-commit` (husky v9 plain shebang `#!/usr/bin/env sh`): sequential `gitleaks protect --staged --verbose --redact` → `pnpm lint-staged` → conditional `services/api` block that runs `go vet ./...` + `gofmt -l . | (! grep .)` when any `*.go` staged. `chmod +x`.
5. **[H9]** Upgrade `.husky/commit-msg` to v9 shape. PRESERVE the existing CI skip guard — only drop the v8 `_/husky.sh` sourcing line. Final content must be:
   ```sh
   #!/usr/bin/env sh
   if [ "$CI" = "true" ]; then exit 0; fi
   pnpm commitlint --edit "$1"
   ```
   The `$CI` guard prevents bot commits in GitHub Actions from being blocked by commitlint. Do NOT remove it.
6. Update root `package.json` scripts: `lint: "biome check ."`, `format: "biome format --write ."`, `prepare: "husky"`, `lint-staged: "lint-staged"`.
7. Create `.github/workflows/ci.yml` — `on: pull_request + push[main,dev]`; 3 jobs:
   - `go`: `working-directory: services/api` · checkout@v4 · setup-go@v5 (1.26) · `go mod download` · `go vet ./...` · `golangci/golangci-lint-action` at **latest stable version supporting Go 1.26** (use latest golangci-lint-action; as of Apr 2026 likely v7 or v8 — implementer pins after checking current release; golangci-lint itself must be v1.68+ for Go 1.26 support) **[N6, Controller #4]** · timeout 5m · `go test ./... -race -cover`.
   - `web`: checkout@v4 · pnpm/action-setup@v4 (9.15.9) · setup-node@v4 (20, cache pnpm) · `pnpm install --frozen-lockfile` · `pnpm -r typecheck` · `pnpm -r build` · `pnpm lint`.
   - `secrets`: checkout@v4 `fetch-depth: 0` · `gitleaks/gitleaks-action@v2` with `GITLEAKS_CONFIG=.gitleaks.toml`.
8. Local validation: `pnpm lint` exit 0 on empty staging · plant fake `AKIA...` secret in temp file + stage → pre-commit blocks · violate commitlint (`wrongtype: x`) → commit-msg blocks · `actionlint .github/workflows/ci.yml` clean.
9. On first remote push (Phase 10), verify GitHub Actions all 3 jobs green.

## Todo List

- [ ] Write `biome.json` (formatter + linter + ignores)
- [ ] Write `.lintstagedrc.json`
- [ ] Write `.gitleaks.toml` (default + allowlist for docs/plans/.example)
- [ ] Write `.husky/pre-commit` (gitleaks + lint-staged + go vet)
- [ ] Upgrade `.husky/commit-msg` to v9 if needed
- [ ] Update root `package.json` scripts (lint/format/prepare/lint-staged)
- [ ] Write `.github/workflows/ci.yml` (3 jobs: go, web, secrets)
- [ ] Verify: pre-commit blocks secret + bad format; CI yaml valid via `actionlint`

## Success Criteria

- `pnpm biome check .` exit 0 on empty staging
- `echo "const x: any = 1" > apps/extension/src/test.ts && pnpm biome check apps/extension/src/test.ts` → exit 1 (linter catches `any`)
- `pre-commit` hook triggers on `git commit` locally (verify by `git commit --dry-run`)
- `gitleaks protect --staged` on staged file with `PRIVATE_KEY_BASE64=AKIA...` → exit 1
- `act -j go` (nektos/act) or GitHub web run — both jobs pass on clean code
- `actionlint .github/workflows/ci.yml` clean (syntax valid)
- **[H7]** Note: Svelte `.svelte` lint path in `biome.json` is untested in Phase 1 (no `.svelte` files exist yet). First real validation occurs in Phase 3 extension UI. Config is structurally correct — runtime validation deferred.

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| golangci-lint-action timeout on cold cache | Med | Low | Latest golangci-lint-action caches by default; set `args: --timeout=5m`. **[N6/Controller #4]** Pin to v1.68+ (Go 1.26 support); use latest golangci-lint-action (v7+ as of Apr 2026) |
| Biome 2.3 linter rule strictness breaks on stub files | Med | Med | `rules.recommended` only; document override path in README; skip Phase 1 UI files that will be Phase 3+ |
| Husky 9 path mismatch on Windows Git Bash | Low | Med | Use `#!/usr/bin/env sh` shebang; scripts are POSIX-only; test `wsl` + Git Bash |
| gitleaks false positive on `docs/MASTER_PROMPT.md` (example keys) | High | Med | Explicit allowlist entry for that path in `.gitleaks.toml` |
| Existing ClaudeKit `release.yml` collides with new `ci.yml` triggers | Low | Low | Different `on:` paths (release triggers on tags); no overlap |

## Security Considerations

- `gitleaks` runs BEFORE commit lands — prevents secret exposure (§15.1 rule).
- `.gitleaks.toml` allowlist scoped to markdown only — **do not** allowlist `.go`/`.ts` files.
- CI `secrets` job runs full-history scan (`fetch-depth: 0`) on push — catches historical leaks too (before remote exists; noop in Phase 1 but shape ready).
- `go vet` catches common insecure patterns (printf misuse, lock copy); `gosec` (via golangci) adds security lints.

## Next Steps

Unblocks Phase 07 (docs can reference `pnpm lint` / `pre-commit` setup; CONTRIBUTING.md documents rules). Enables Phase 02 developers to commit safely. Phase 10 extends CI with deploy-api + build-extension workflows (see §12.4 skeletons).
