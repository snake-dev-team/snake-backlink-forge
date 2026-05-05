# Snake Backlink Forge — Architecture

> Condensed from MASTER_PROMPT.md §1-2. For business logic details see §3-17.

## 1. High-level diagram

```
                        ┌─────────────────────────────────┐
                        │  VERCEL                         │
                        │  snakebacklink.com              │
                        │  Next.js 15 + shadcn + MagicUI  │
                        │  + Aceternity + Motion + R3F    │
                        │  Landing, Pricing, Blog, Docs   │
                        └────────────┬────────────────────┘
                                     │ CTA: t.me/SnakeBacklinkBot
                                     ▼
┌──────────────────────────┐  ┌─────────────────────────────────┐
│  BROWSER EXTENSION       │  │  TELEGRAM BOT                   │
│  Chrome/Edge MV3         │  │  @SnakeBacklinkBot              │
│  Svelte 5 + Vite + WASM  │  │  - /start, /key, /buy,          │
│  Service Worker +        │  │    /balance, /topup, /history,  │
│  Offscreen Document      │  │    /support, /download          │
│  (Client UI only —       │  │  - SePay webhook auto top-up    │
│   no business logic)     │  │  - Trial: 5 credit/user lifetime│
└────────────┬─────────────┘  └────────────┬────────────────────┘
             │ X-API-Key + X-Signature      │
             │ (HMAC-WASM)                  │
             └──────────────┬───────────────┘
                            ▼
                ┌──────────────────────────────────┐
                │  FLY.IO — snake-api (Go + Fiber) │
                │  sin region + iad failover       │
                │  - /v1/campaign/next             │
                │  - /v1/campaign/result           │
                │  - /v1/captcha/solve             │
                │  - /v1/finder/search             │
                │  - /v1/me                        │
                │  - Middleware: auth, HMAC, RL    │
                └────┬──────────────┬──────────────┘
                     │              │
           ┌─────────▼────────┐  ┌──▼──────────────┐
           │ PostgreSQL 16    │  │ Redis 7         │
           │ Fly Postgres     │  │ Queue + cache   │
           │ - users, keys    │  │ - per-key queue │
           │ - wallets        │  │ - rate limits   │
           │ - campaigns      │  │ - SSE pubsub    │
           │ - targets        │  │                 │
           │ - jobs, ledger   │  │                 │
           └──────────────────┘  └─────────────────┘
                            │
          ┌─────────────────┼──────────────────┬───────────────┐
          ▼                 ▼                  ▼               ▼
   ┌───────────┐    ┌──────────────┐   ┌────────────┐  ┌─────────────┐
   │ 9Router   │    │ SerpAPI /    │   │ 2captcha / │  │ Ahrefs /    │
   │ Claude    │    │ ScrapingBee  │   │ CapSolver  │  │ Moz API     │
   │ Sonnet 4.6│    │ (dork)       │   │ (BYOK+     │  │ (DR/DA)     │
   │           │    │              │   │  Credits)  │  │             │
   └───────────┘    └──────────────┘   └────────────┘  └─────────────┘

          ┌──────────────────────────────────────┐
          │  CLOUDFLARE R2                       │
          │  cdn.snakebacklink.com               │
          │  - ext/ext.crx                       │
          │  - ext/updates.xml                   │
          │  - installer/SnakeBacklinkSetup.exe  │
          │  - blog/images/**                    │
          └──────────────────────────────────────┘
```

## 2. Architecture Decision Record (ADR)

### Locked decisions (§1.2)

| Layer | Tech | Host |
|---|---|---|
| Landing | Next.js 15.5 App Router + shadcn/ui + MagicUI + Aceternity UI + Motion v11 + Tailwind v4 + R3F + Lenis | Vercel |
| Blog | MDX + velite + next-seo | Vercel |
| Extension | Manifest V3, Svelte 5 (runes), Vite 5, Tailwind 4, TS 6 | Browser |
| WASM | Rust 2021 + wasm-bindgen + wasm-pack | Embedded in ext |
| Backend | Go 1.26.2 + Fiber v2 + sqlc + goose + zap | Fly.io sin region |
| Bot | Go (same binary), go-telegram-bot-api/v5 | Fly.io same app |
| DB | PostgreSQL 16 | Fly Postgres |
| Cache/Queue | Redis 7 | Fly Redis |
| CDN | Cloudflare R2 | cdn.snakebacklink.com |
| Code signing | Sectigo OV cert | Self-host |
| Installer | NSIS 3.x + signtool | GitHub Actions CI |
| Payment VN | SePay webhook | Fly bot |
| AI | Claude Sonnet 4.6 via 9Router | `cc/claude-sonnet-4-6` |

### Phase 1 deviations

| Original §1.2 | Applied | Reason |
|---|---|---|
| Go 1.23 | Go 1.26.2 | Go 1.23 EOL April 2026 |
| TypeScript 5.5+ | TypeScript 6.0.3 | Biome 2 Svelte support requires TS 6 |
| Fiber built-in logger | fiberzap/v2 contrib | Structured JSON output via zap |

## 3. Credit & Key model

- API key format: `sbf_live_<32-char-base58>`, SHA256 hash in DB, shown once
- Two credit pools:
  - **Standard** (DR 0-39): template + light rewrite, ~1,500-2,000d/credit
  - **Premium** (DR 40+): AI rewrite Sonnet 4.6, ~8,000-10,000d/credit
- 1 successful backlink = 1 credit consumed (atomic); failed = 0 credit
- Captcha: BYOK = 0 credit; Snake-pool = +1 credit per captcha
- Trial: 5 Standard credits per Telegram user (lifetime, phone verify required)

## 4. Anti-abuse safeguards (§1.5) — default ON, no exceptions

| Safeguard | Rule | Override? |
|---|---|---|
| Per-domain rate limit | Max 1 backlink/30 days/key per domain | No |
| Per-key daily cap | Max 300 backlinks/day/key | No |
| Per-campaign drip | Default 20/day; user config 5-50 | Yes (range) |
| Anchor diversity | 60% branded / 20% naked / 15% generic / 5% exact | No |
| Content uniqueness | Cosine similarity < 80% vs last 7 days same key | No |
| Niche relevance | Min 0.6 cosine match vs campaign niche | Yes (toggle) |
| Ethical mode | Premium only + 10/day + skip DR<30 + skip spam domains | Yes (default ON) |
| Target blocklist | .gov, .edu, major brands always skipped | No |
| Anomaly detection | >3 country codes/24h -> auto-suspend 1h + Telegram notify | No |
| Payment fraud | Same SePay txn ref > 2 retries -> manual review flag | No |

## 5. Security model (§1.6)

- **Request signing**: HMAC-SHA256(payload + timestamp, Argon2id-derived secret) in WASM
- **Timestamp skew**: +-60s window; outside = reject
- **Nonce cache**: Redis 5-minute window; duplicate = reject (replay guard)
- **Rate limits**: 250 req/min/key, 5000 req/hour/key, 100 req/min/IP
- **Key rotation**: `/regenkey` invalidates old key; Redis blocklist 24h for pending clears
- **Audit log**: immutable PostgreSQL table for all credit create/consume events
- **DB roles**: `api_user` has SELECT/INSERT/UPDATE only; no DDL

## 6. Repository layout (§2)

```
snake-backlink-forge/
├── apps/
│   ├── landing/          # Next.js 15 stub (Phase 9 full impl)
│   └── extension/        # Svelte 5 MV3 stub (Phase 3 full impl)
├── services/
│   └── api/              # Go API + Telegram bot (same binary)
├── packages/
│   └── shared-types/     # TypeScript types shared across apps
├── installer/            # NSIS Windows installer (Phase 7)
├── tools/                # crx-packager, db-seed (Phase 7+)
├── ops/
│   └── docker-compose.yml
├── .github/
│   └── workflows/ci.yml  # Matrix CI: node + go + gitleaks
├── docs/                 # Architecture, progress, glossary
├── plans/                # Implementation plans (committed for audit trail)
├── scripts/
│   └── _claudekit/       # ClaudeKit helper scripts (namespaced)
└── Makefile              # dev-setup, dev-up, dev-down, migrate-up/down
```

## 7. See also

- `docs/MASTER_PROMPT.md` — full implementation spec (§3-20 + Appendices)
- `docs/progress.md` — phase-by-phase progress log
- `docs/glossary.md` — terms and abbreviations
- `plans/260424-0247-phase-1-foundation/` — Phase 1 plan artifacts
