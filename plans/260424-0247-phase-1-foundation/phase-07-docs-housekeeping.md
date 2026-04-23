---
name: Docs + housekeeping + git init
phase: 07
status: pending
priority: P2
estimated_effort: 1.5h
blockedBy: [phase-03-database-layer, phase-04-local-infra, phase-06-quality-gates]
blocks: []
agents: [docs-manager, git-manager]
---

# Phase 07 — Docs + Housekeeping + Git Init

## Context Links

- MASTER_PROMPT §1.1 High-level diagram (line 29)
- MASTER_PROMPT §1.2 ADR, §1.5 Safeguards (for architecture.md condensation)
- MASTER_PROMPT §15.1–15.4 Commit/branch rules
- MASTER_PROMPT §16.1 Threat model (for SECURITY.md)
- MASTER_PROMPT §20.5 Status reporting template (for progress.md)
- MASTER_PROMPT Appendix A Glossary
- Scout report Priority 7 + Open Q #1–#5

## Overview

**Priority:** P2 · **Status:** pending · **Effort:** 1.5h
Terminal phase. Rewrite README for Snake Backlink Forge. Author condensed `docs/architecture.md`, `docs/progress.md` template, `CONTRIBUTING.md`, `SECURITY.md`, `docs/glossary.md`. Move ClaudeKit `scripts/*` to `scripts/_claudekit/` namespace. `git init` + `main`/`dev` branches + initial commit. NO remote push (Phase 10 pre-launch).

## Key Insights

- Existing ClaudeKit `README.md`, `CLAUDE.md`, `AGENTS.md`, `HOW_TO_USE_CLAUDEKIT.md`, `docs/codebase-summary.md`, etc. = boilerplate — KEEP (useful for Claude Code orientation) but move Snake Backlink Forge docs to distinct filenames.
- `docs/MASTER_PROMPT.md` is source of truth (2947 lines) — `docs/architecture.md` is a < 400-line condensed pointer.
- Git branching: per §15.4, `main` = production, `dev` = integration, feature branches from `dev`. Phase 1 initial commit lands on `dev` then fast-forward to `main`.
- `scripts/_claudekit/` namespace isolates ClaudeKit helper scripts (generate-opencode, release-manifest, discord webhooks) from future `scripts/sbf-*` project-specific tooling. No functionality change; import paths updated only if those scripts are invoked (they are not in CI yet).

## Requirements

**Functional:**
- `README.md` = Snake Backlink Forge description + Phase 1 quickstart: clone → pnpm install → `make dev` → health check. Reference MASTER_PROMPT + architecture.md.
- `docs/architecture.md` < 400 lines: high-level diagram (ASCII or reference MASTER §1.1), ADR table summary, safeguards summary, tech stack quick-ref, link to MASTER_PROMPT for depth.
- `docs/progress.md` seeded with Phase 1 header template per §20.5.
- `docs/glossary.md` copies Appendix A.
- `CONTRIBUTING.md`: branch strategy (§15.4), commit convention (§15.3), pre-commit rules (§15.1), PR checklist (§15.2).
- `SECURITY.md`: threat model ref (§16.1), vulnerability reporting email `security@snakebacklink.com` placeholder, responsible disclosure policy.
- `scripts/` → `scripts/_claudekit/` move done via `mv` (git repo exists from Phase 01; `git mv` is optional refinement, plain `mv` is correct here). **[M9]**
- Git initialized: `main` + `dev` branches; initial commit `feat(repo): scaffold monorepo + phase 1 foundation` on `dev`; `main` fast-forwarded.

**Non-functional:**
- README < 150 lines; architecture.md < 400 lines.
- Glossary entries sorted alphabetically.
- All markdown passes Biome format (Phase 06 config).

## Architecture

```
Repo root
├── README.md                        (REWRITE — Snake Backlink Forge)
├── CONTRIBUTING.md                  (NEW)
├── SECURITY.md                      (NEW)
├── LICENSE                          (KEEP — MIT, already present)
├── docs/
│   ├── MASTER_PROMPT.md             (KEEP — source of truth)
│   ├── architecture.md              (NEW — condensed)
│   ├── progress.md                  (NEW — Phase tracker)
│   ├── glossary.md                  (NEW — Appendix A copy)
│   ├── code-standards.md            (KEEP — ClaudeKit, still useful)
│   ├── codebase-summary.md          (KEEP — ClaudeKit orientation)
│   ├── project-overview-pdr.md      (KEEP)
│   ├── development-roadmap.md       (KEEP — will extend Phase 2+)
│   └── project-changelog.md         (KEEP — will extend Phase 2+)
├── scripts/
│   └── _claudekit/                  (MOVED from scripts/*)
│       ├── generate-opencode.*
│       ├── release-manifest.*
│       └── ...
└── .git/                            (INIT)
    ├── branches: main + dev
    └── initial commit on dev
```

## Related Code Files

**CREATE:**
- `C:\Users\Hanna\Desktop\tool_backlink\docs\architecture.md`
- `C:\Users\Hanna\Desktop\tool_backlink\docs\progress.md`
- `C:\Users\Hanna\Desktop\tool_backlink\docs\glossary.md`
- `C:\Users\Hanna\Desktop\tool_backlink\CONTRIBUTING.md`
- `C:\Users\Hanna\Desktop\tool_backlink\SECURITY.md`

**MODIFY:**
- `C:\Users\Hanna\Desktop\tool_backlink\README.md` — full rewrite (replace ClaudeKit-Engineer content with Snake Backlink Forge).

**RENAME / MOVE:**
- `scripts/*` (except new `_claudekit/` dir itself) → `scripts/_claudekit/*` (shell: `mkdir scripts/_claudekit && mv scripts/!(_claudekit) scripts/_claudekit/` or enumerate).

**DELETE:** none.

## Implementation Steps

1. **[M9, H1]** Move ClaudeKit scripts using plain `mv` (git repo already exists from Phase 01 step 0; `git mv` is optional but `mv` is sufficient since history tracks from initial commit). Run in **Git Bash on Windows** (not PowerShell — bash loop syntax):
   ```bash
   cd "C:\Users\Hanna\Desktop\tool_backlink"
   mkdir -p scripts/_claudekit
   # enumerate existing files to avoid self-move
   for f in scripts/*; do
     name=$(basename "$f")
     [ "$name" = "_claudekit" ] && continue
     mv "$f" "scripts/_claudekit/$name"
   done
   ```
2. Write `README.md` with sections: "Snake Backlink Forge" one-liner, "Phase 1 Status: scaffolding" note, "Quickstart" (`make dev-setup && pnpm install && make dev`), "Prerequisites" note: **requires Git Bash or WSL2** on Windows (PowerShell / cmd.exe not supported for `make` targets), "Architecture" link to `docs/architecture.md` + `docs/MASTER_PROMPT.md`, "Contributing" link, "License" MIT. **[Controller Decision #5, C3]**
3. Write `docs/architecture.md`:
   - Section 1: System diagram (copy MASTER §1.1 ASCII OR 1-paragraph summary + link)
   - Section 2: ADR decision table (copy MASTER §1.2 verbatim with Phase 1 deviations callout: Go 1.26, TS 6.0, fiberzap)
   - Section 3: Safeguards summary (condensed MASTER §1.5 — 1-line per rule)
   - Section 4: Credit model quick-ref (§1.3 pools + rules)
   - Section 5: Repo layout diff (link MASTER §2)
   - Section 6: Links back to MASTER_PROMPT section numbers
4. Write `docs/progress.md` — H1 "Snake Backlink Forge — Progress"; H2 "Phase 1 — Foundation" with fields: Started 2026-04-24, Completed `<pending>`, Commits `<pending>`, Tests added 0 (smoke only), Coverage delta n/a (baseline), Notable decisions (Go 1.26 ADR deviation, TS 6.0.3, fiberzap/v2 contrib, plans committed to git), Known issues deferred (DB role separation `api_user` → Phase 10; extension icons → Phase 3), Pre-req user actions for Phase 2 (Telegram bot token + username, SePay merchant, 9Router Claude key).
   Also add to `docs/progress.md`:
   - **LICENSE TBD**: Currently MIT inherited from ClaudeKit. User to confirm MIT or switch to proprietary before Phase 10 public launch. **[Controller Decision #2]**
   - **Windows dev path**: Git Bash is primary dev shell. WSL2 is optional fallback. Docker Desktop on Win11 must have WSL2 integration enabled for the default distro — if `make dev-up` fails with volume errors, open Docker Desktop → Settings → Resources → WSL Integration → enable default distro. **[Controller Decision #5]**
5. Write `docs/glossary.md` — copy MASTER Appendix A sections 2830–2852.
6. Write `CONTRIBUTING.md`:
   - Branch strategy (§15.4)
   - Commit convention with examples (§15.3) + allowed types `feat|fix|docs|test|refactor|chore|perf` + kebab-case scope
   - Pre-commit checklist (§15.1): `go vet`, `golangci-lint`, `go test -race`, `pnpm lint`, `pnpm typecheck`, `pnpm test`, gitleaks, no `console.log`/`fmt.Println`, migration down reversibility
   - Pre-PR checklist (§15.2): code-review agent 0 critical/high, ≥80% Go coverage, ≥70% TS coverage, E2E added, docs updated, CHANGELOG entry
   - Agent workflow reference → `.claude/` docs
7. Write `SECURITY.md`:
   - Supported versions: main branch only (pre-1.0)
   - Reporting: email `security@snakebacklink.com` (placeholder — update Phase 10)
   - Responsible disclosure: 90-day coordinated disclosure standard
   - Scope: API, Telegram bot, extension WASM; out-of-scope: user's own targets (link-building is their risk)
   - Threat model reference: `docs/MASTER_PROMPT.md` §16.1
8. **[C8]** Create branches + initial commit (git repo already exists from Phase 01 step 0 — do NOT run `git init` again):
   ```bash
   cd "C:\Users\Hanna\Desktop\tool_backlink"
   git add -A
   # review: git status --short | head -20
   git commit -m "feat(repo): scaffold monorepo + phase 1 foundation"
   git branch dev
   git checkout dev
   ```
   *(Leave `main` + `dev` in sync; no remote added — user configures Phase 10.)*
9. Update `docs/progress.md` "Commits:" with hash after commit lands.

## Todo List

- [ ] Move `scripts/*` → `scripts/_claudekit/*`
- [ ] Rewrite `README.md` for Snake Backlink Forge
- [ ] Write `docs/architecture.md` (<400 lines, condensed MASTER §1-2)
- [ ] Write `docs/progress.md` (§20.5 template, Phase 1 seeded)
- [ ] Write `docs/glossary.md` (Appendix A copy)
- [ ] Write `CONTRIBUTING.md` (§15 rules consolidated)
- [ ] Write `SECURITY.md` (threat model ref + disclosure)
- [ ] `git add -A && git commit -m "feat(repo): scaffold monorepo + phase 1 foundation"` (git was already initialized in Phase 01 step 0 — **do NOT** re-run `git init`) **[C8]**
- [ ] `git branch dev && git checkout dev`

## Success Criteria

- `git log --oneline -1` returns initial commit on `dev`
- `git branch` lists `main` and `dev`, both at same commit
- `wc -l docs/architecture.md` ≤ 400
- `wc -l README.md` ≤ 150
- `grep -q "Snake Backlink Forge" README.md` (not "ClaudeKit")
- `ls scripts/_claudekit/` shows moved scripts; `ls scripts/` has only `_claudekit/` dir
- `docs/progress.md`, `docs/glossary.md`, `CONTRIBUTING.md`, `SECURITY.md` all exist
- `pnpm biome format --check docs/*.md README.md CONTRIBUTING.md SECURITY.md` exit 0

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `scripts/*` move breaks ClaudeKit hooks relying on hardcoded paths | Med | Low | Phase 1 doesn't invoke these scripts in CI; if `.claude/` hooks reference them, update refs at same time as move |
| `git init` on Windows path with spaces | Low | Med | Path `C:\Users\Hanna\Desktop\tool_backlink` has no spaces — safe |
| Commit size very large (first commit with lockfile + all deps in lockfile) | High | Low | Expected; single initial commit is standard for repo bootstrap |
| README quickstart fails for users on Docker Desktop Windows | Med | Med | Smoke-test `make dev` on Windows before declaring phase done; document WSL2 fallback |

## Security Considerations

- `SECURITY.md` email `security@snakebacklink.com` is PLACEHOLDER — update when domain provisioned (Phase 10).
- Do NOT commit `.env` (gitignore from Phase 01 covers); verify `git status` before initial commit.
- Initial commit runs through pre-commit hook (from Phase 06) — gitleaks scans all files; expect clean pass because no secrets, only `.example` placeholders.
- `LICENSE` = MIT inherited from ClaudeKit — verify this is intended for Snake Backlink Forge; if not, swap to proprietary (user decision, flag in Unresolved Qs).

## Next Steps

Phase 1 complete. Unblocks Phase 2 kickoff — user must provision: Telegram bot token (via @BotFather), SePay merchant account + webhook secret, confirm 9Router Claude key active, decide LICENSE (MIT vs proprietary). Phase 10 adds remote + deploy workflows.
