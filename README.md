# Snake Backlink Forge

> Professional SEO backlink automation platform — SaaS model, distributed via Chrome/Edge browser extension + Telegram bot for key management.

**Status:** Phase 1 Foundation complete (2026-04-24) · Phase 2+ per roadmap

## What is this?

Snake Backlink Forge is a backlink automation tool for SEO professionals. Users buy credits via
Telegram bot (@SnakeBacklinkBot), install a browser extension, and configure campaigns that
drip-feed backlinks to their money sites with anti-abuse safeguards built in.

- **Standard pool** (DR 0-39): mass volume, template rewrites, ~1,500-2,000đ/credit
- **Premium pool** (DR 40+): contextual, AI rewrite via Sonnet 4.6, ~8,000-10,000đ/credit
- **Ethical mode ON by default**: daily limit 10, Premium pool only, skip spam-flagged domains

## Architecture

See `docs/architecture.md` for the condensed architecture. Full spec in `docs/MASTER_PROMPT.md`.

```
  Next.js 15 landing (Vercel) ─┐
                               ├─> Fly.io Go API (Fiber + Postgres + Redis)
  Chrome/Edge MV3 extension ────┤     ↓
  (Svelte 5 + WASM sign)        │   Telegram bot (@SnakeBacklinkBot)
                                │     ↓ SePay webhook
                                └─> Cloudflare R2 (CRX + installer)
```

## Tech Stack

| Layer | Stack |
|---|---|
| API + bot | Go 1.26.2 + Fiber v2.52.6 + pgx v5.9.1 + go-redis v9 + zap (Fly.io) |
| Landing | Next.js 15.5 + React 19 + Tailwind 4 (Phase 9) |
| Extension | Svelte 5 + Vite 5 + @crxjs/vite-plugin + Rust WASM HMAC (Phase 3) |
| DB | PostgreSQL 16 + Redis 7 |
| Quality | Biome 2.3 + golangci-lint v1.68+ + gitleaks + husky v9 |

## Local Development

**Prerequisites:** Node >= 20, Go 1.26.2, Docker Desktop, pnpm 9.15.9, Git Bash (Windows).

> **Windows note:** Use Git Bash or WSL2. PowerShell / cmd.exe do not support `make` targets.

```bash
# One-time setup
pnpm install
make dev-setup   # copies .env templates

# Start full stack (Postgres + Redis + API)
make dev

# Verify
curl localhost:8080/health  # 200 {"status":"ok"}
curl localhost:8080/ready   # 200 with DB+Redis up, 503 if down
```

## Project Status

Tracked in `docs/progress.md`. Current phase: **Phase 1 Foundation** (scaffold complete).

## Documentation

| Doc | Purpose |
|---|---|
| `docs/MASTER_PROMPT.md` | Full implementation spec (source of truth, 2947 lines) |
| `docs/architecture.md` | Condensed architecture: diagram + ADR + safeguards |
| `docs/progress.md` | Phase-by-phase progress log |
| `docs/glossary.md` | Terms and abbreviations |
| `CONTRIBUTING.md` | Branch strategy, commit convention, pre-commit/PR checklists |
| `SECURITY.md` | Vulnerability reporting, threat model reference |

## License

Proprietary — Copyright (c) 2026 Snake Premium Hub. Unauthorized use prohibited.
