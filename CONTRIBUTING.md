# Contributing

## Branch strategy

- `main` — production (protected; merges via PR only)
- `dev` — integration (default development target)
- `feat/*` — feature branches (from dev)
- `fix/*`, `chore/*`, `docs/*`, `refactor/*`, `test/*` — topic branches

**NEVER** commit directly to `dev` or `main`. Always PR from a topic branch.

## Commit message convention

Conventional Commits with module scope:

```
feat(bot): add /regenkey command
fix(api): prevent double credit consume on race
chore(ci): upgrade Go to 1.26.2
docs(architecture): document safeguard rules
test(wallet): add concurrent consume test
refactor(adapter): extract humanType helper to base class
```

Allowed types: `feat | fix | docs | test | refactor | chore | perf`
Scope: kebab-case module name (`api`, `bot`, `ext`, `landing`, `db`, `ci`, `infra`)

## Pre-commit checklist

- [ ] `go vet ./...` clean (services/api)
- [ ] `golangci-lint run` clean
- [ ] `go test ./... -race` pass
- [ ] `pnpm lint` clean (Biome)
- [ ] `pnpm typecheck` clean
- [ ] `pnpm -r build` pass
- [ ] No `console.log` or `fmt.Println` debug left behind
- [ ] No hardcoded secrets (gitleaks pre-commit scan pass)
- [ ] Migration `down` reverses `up` cleanly (when DB changes included)

## Pre-PR checklist

- [ ] Code review agent run -> 0 critical, 0 high findings
- [ ] Coverage >= 80% Go, >= 70% TS (new code only)
- [ ] E2E test covering the new feature path
- [ ] Docs updated (`docs/architecture.md`, API contract if endpoints changed)
- [ ] `docs/progress.md` entry if phase-boundary change
- [ ] CHANGELOG entry for user-visible changes

## Local development

See `README.md` Local Development section.

**Windows primary shell: Git Bash.** WSL2 is optional fallback.
Docker Desktop on Win11 must have WSL2 integration enabled for the default distro.
If `make dev-up` fails with volume errors: Docker Desktop -> Settings -> Resources ->
WSL Integration -> enable default distro.

## Testing

```bash
# Go tests (with race detector)
cd services/api && make test

# TypeScript (Phase 2+ adds Vitest suites)
pnpm -r test

# Integration (requires Docker Desktop running)
make dev-up          # starts PG + Redis
make migrate-up      # applies schema
make run &           # start API
curl localhost:8080/ready
make dev-down        # teardown
```

## Agent workflow

Claude Code subagents follow `.claude/rules/` conventions. See `.claude/rules/primary-workflow.md`
for the planning -> implementation -> testing -> review chain.

## Code of conduct

Respectful collaboration. No harassment, no dark-pattern tooling, no selling user data.
