---
name: Monorepo backbone
phase: 01
status: completed
priority: P0
estimated_effort: 1.5h
blockedBy: []
blocks: [phase-02-go-api-skeleton, phase-04-local-infra, phase-05-frontend-stubs]
agents: [fullstack-developer]
---

# Phase 01 — Monorepo Backbone

## Context Links

- MASTER_PROMPT §2 Repository Structure (lines 206–521)
- MASTER_PROMPT §13 Phase 1 scope (lines 2286–2331)
- Scout report: `plans/reports/scout-260424-0247-phase-1-foundation.md` (Priority 1 §7)
- JS stack research: `plans/reports/researcher-260424-0247-js-stack-versions.md` (§1, §2)

## Overview

**Priority:** P0 · **Status:** pending · **Effort:** 1.5h
Gate phase for all other phases. Rewrites root `package.json` (Snake Backlink Forge root), adds pnpm workspace + Turborepo pipeline + `.editorconfig`, expands `.gitignore` for Go/Rust/WASM/Windows. No app/service code yet.

## Key Insights

- ClaudeKit-Engineer boilerplate already present with husky+commitlint — preserve, don't recreate.
- pnpm 9.15.9 monorepo features fully mature; use `packageManager` field for corepack pin.
- Turborepo v2 schema uses `tasks.*` (not `pipeline.*`); strict env inheritance.
- Root `.env.example` (ClaudeKit hook webhooks) renamed to `.env.claudekit.example` to free the canonical name for Snake Backlink Forge usage later.

## Requirements

**Functional:**
- Root `package.json` = Snake Backlink Forge monorepo root, `name: snake-backlink-forge`, `version: 0.1.0`, `private: true`.
- Workspace packages: `apps/*`, `packages/*`, `services/*`, `tools/*`.
- Turborepo pipeline tasks: `build`, `lint`, `typecheck`, `test`, `dev`, `clean`.
- `.editorconfig` enforces LF, UTF-8, final newline, 2-space indent (TS/JSON/YAML), tab (Go/Makefile).
- `.gitignore` covers Go (`vendor/`, `*.exe`, `*.test`), Rust (`target/`, `Cargo.lock` kept for bins), WASM (`pkg/`), Windows (`Thumbs.db`, `desktop.ini`), IDE (`.idea/`, `.vscode/*`), secrets (`private-key.pem`, `*.p12`, `*.pfx`).
- **LICENSE = proprietary from day 1** (user decision). Overwrite existing MIT LICENSE with Snake Premium Hub proprietary notice.

**Non-functional:**
- `pnpm install` clean on fresh clone (post lockfile commit).
- `pnpm -r build` and `turbo run build` resolve (may be no-op if no apps yet).

## Architecture

```
tool_backlink/                       (git root)
├── package.json          ← rewrite (workspaces + scripts + devDeps)
├── pnpm-workspace.yaml   ← NEW
├── turbo.json            ← NEW
├── .editorconfig         ← NEW
├── .gitignore            ← EXPAND
├── .env.claudekit.example ← RENAME from .env.example
├── .claude/              (preserve untouched)
├── .husky/commit-msg     (preserve)
└── scripts/              (moved in Phase 07)
```

## Related Code Files

**CREATE:**
- `C:\Users\Hanna\Desktop\tool_backlink\pnpm-workspace.yaml`
- `C:\Users\Hanna\Desktop\tool_backlink\turbo.json`
- `C:\Users\Hanna\Desktop\tool_backlink\.editorconfig`

**MODIFY:**
- `C:\Users\Hanna\Desktop\tool_backlink\package.json` (full rewrite; keep husky/commitlint only)
- `C:\Users\Hanna\Desktop\tool_backlink\.gitignore` (append Go/Rust/WASM/Windows blocks)
- `C:\Users\Hanna\Desktop\tool_backlink\LICENSE` (overwrite MIT → proprietary Snake Premium Hub notice)

**RENAME:**
- `.env.example` → `.env.claudekit.example`

**DELETE:** none.

## Implementation Steps

0. **[C8] Initialize git FIRST**: Run `git init -b main` in project root before `pnpm install`. Husky v9 `prepare: "husky"` script requires `.git/` to exist when `pnpm install` runs — otherwise husky errors silently and hooks are not wired.
1. Rewrite `package.json`: name `snake-backlink-forge`, version `0.1.0`, private true, `packageManager: "pnpm@9.15.9"`, `engines.node >=20`, scripts `{build,lint,typecheck,test,dev,clean}` → `turbo run <task>`, devDeps pin `{turbo: 2.9.6, typescript: 6.0.3, @biomejs/biome: 2.3.0, husky: ^9.0.0, @commitlint/cli: ^19.5.0, @commitlint/config-conventional: ^19.5.0, lint-staged: ^15.0.0}`. **[N1] Note:** commitlint v19.5.0 is latest stable as of Apr 2026; v20 not yet released.
2. Create `pnpm-workspace.yaml` with packages `apps/*`, `packages/*`, `services/*`, `tools/*` (exclude `services/api` from TS globs since it's Go).
3. Create `turbo.json` v2 schema with `$schema: https://turborepo.dev/schema.json`, tasks per researcher §2 (build cache outputs `dist/**`, `build/**`; lint/typecheck/test uncached; dev persistent). **[H3] Note:** `build` task with `dependsOn: ["^build"]` — turbo v2 skips workspaces that have no `build` script silently; this is expected for `packages/shared-types` in Phase 1.
4. Create `.editorconfig` per researcher conventions (LF, UTF-8, 2-space default, tab for `{Makefile,*.go}`, no trailing whitespace, insert final newline).
5. Expand `.gitignore` — add sections: "Go", "Rust / WASM", "Windows", "IDE", "Extension / installer secrets" (explicit `private-key.pem`, `*.p12`, `*.pfx`, `*.snk`). Add `.env*` block with exceptions:
   ```
   .env*
   !.env.example
   !.env.claudekit.example
   !services/api/.env.example
   ```
   **[C7] Plans commit policy — COMMIT plans:** Add exceptions for plans directory:
   ```
   # Plans committed for history/audit
   !plans/260424-0247-*/**
   !plans/reports/**
   ```
6. Rename `.env.example` → `.env.claudekit.example` preserving content.
7. **Overwrite `LICENSE` file** with proprietary notice (replacing ClaudeKit MIT):
   ```
   Copyright (c) 2026 Snake Premium Hub. All rights reserved.
   Proprietary and confidential. Unauthorized copying, modification,
   distribution, or use is strictly prohibited without prior written
   consent from Snake Premium Hub.
   ```
8. Sanity: `pnpm install` → expect lockfile generated; commit `pnpm-lock.yaml`. **[C8] Note:** With git initialized in step 0, husky v9 hooks are wired correctly during `pnpm install`.

## Todo List

- [x] **[C8]** Run `git init -b main` (BEFORE pnpm install)
- [x] Rewrite root `package.json` (commitlint pinned to v19.5.0 per N1)
- [x] Create `pnpm-workspace.yaml`
- [x] Create `turbo.json` v2 (with H3 note on build skip behavior)
- [x] Create `.editorconfig`
- [x] Expand `.gitignore` (Go/Rust/WASM/Windows/IDE/secrets + C3 .env exceptions + C7 plans exceptions)
- [x] Rename `.env.example` → `.env.claudekit.example`
- [x] Overwrite `LICENSE` with Snake Premium Hub proprietary notice
- [x] Run `pnpm install` → commit lockfile

## Success Criteria

- `cat package.json | jq -r .name` returns `snake-backlink-forge`
- `pnpm install` exits 0; `pnpm-lock.yaml` present
- `turbo run build` exits 0 (no workspaces yet → no-op ok)
- `grep -q "^vendor/$" .gitignore && grep -q "^target/$" .gitignore && grep -q "^pkg/$" .gitignore`
- `.editorconfig` present; `editorconfig-checker` (optional) clean

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Removing semantic-release breaks release workflows | Med | Low | Keep release workflows inert (no tag push) until Phase 10; re-add dep then |
| pnpm version mismatch between dev + CI | Med | Med | Pin `packageManager` field; CI uses `corepack enable` |
| `.gitignore` accidentally excludes `services/api/sqlc/` generated code | Low | Med | Explicit `!services/api/internal/db/sqlc/` whitelist |

## Security Considerations

- `.env.claudekit.example` contains webhook URL placeholders only — never real tokens; verify with gitleaks dry-run after rename.
- `private-key.pem`, `*.p12`, `*.pfx` added to `.gitignore` before any cert work begins.
- **LICENSE = proprietary from Phase 01 step 7** — all source code created after this commit falls under Snake Premium Hub proprietary license. MIT from ClaudeKit-Engineer template is superseded.

## Next Steps

Unblocks Phase 02 (needs pnpm workspace root + `.gitignore` Go coverage), Phase 04 (needs root for compose path), Phase 05 (needs workspace globs), Phase 06 (needs Biome+commitlint from root).
