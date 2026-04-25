# SNAKE BACKLINK FORGE — MASTER IMPLEMENTATION PROMPT v1.0

> **Dành cho Claude Code + ClaudeKit.**
> Đây là prompt master. Đừng implement toàn bộ trong 1 session. Chia theo PHASE ở cuối tài liệu, mỗi phase chạy đủ loop `/ck:scout → /watzup → /ck:plan --hard → review plan → /clear → /ck:cook → /ck:test → code-review loop → /ck:git cm`.

---

## 0. IDENTITY & MISSION BRIEF

Mày là lead engineer của Snake Premium Hub, được thuê để xây dựng **Snake Backlink Forge** — một sản phẩm SaaS tool backlink automation cho thị trường SEO Việt Nam + quốc tế. Mô hình bán credit qua Telegram bot, distribution qua browser extension force-installed bằng NSIS installer.

Sản phẩm phải đạt các tiêu chí sau:
1. **Production-grade**: không phải MVP prototype. Mọi module phải có tests, error handling, logging, metrics.
2. **Source code protection**: extension phải obfuscated heavy + WASM critical path. Backend logic core phải server-side.
3. **Anti-abuse built-in**: tool có thể bị dùng spam phá Google → MUST có rate limits, diversity enforcement, ethical mode default-on.
4. **UI đẹp tier Linear/Stripe**: landing không được kiểu "tool SEO rác 2015".
5. **Deploy-ready**: Fly.io + Vercel + R2, CI/CD qua GitHub Actions.
6. **Cost < $80/tháng** fixed infra cho 100 active users đầu tiên.

Khách hàng:
- Người dùng SEO Việt Nam chạy nhiều site (primary)
- SEO agency quốc tế (secondary)
- Khách crypto/affiliate cần build link đa dạng (tertiary)

---

## 1. ARCHITECTURE DECISION RECORD (ADR)

### 1.1 High-level diagram

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
│  (Khách thấy UI, không   │  │  - SePay webhook auto top-up    │
│   thấy business logic)   │  │  - Trial: 5 credit/user lifetime│
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

### 1.2 Decision table (locked, KHÔNG thay đổi trong quá trình implement)

| Layer | Tech | Host | Lock Reason |
|---|---|---|---|
| Landing | Next.js 15 App Router + shadcn/ui + MagicUI + Aceternity UI + Motion v11 + Tailwind CSS v4 + React Three Fiber + Lenis | Vercel | UI top-tier |
| Blog | MDX + Contentlayer/velite + next-seo | Vercel | SEO + DX |
| Extension | Manifest V3, Svelte 5 (runes), Vite 5, TailwindCSS 4, TypeScript 5.5+ strict | Browser khách | Modern, performant |
| WASM critical | Rust 2021 + wasm-bindgen + wasm-pack | Embedded in ext | HMAC signing secret |
| Backend API | Go 1.23 + Fiber v2 + sqlc + goose migrations + Zap logger | Fly.io sin region | Performance + familiar |
| Telegram bot | Go 1.23 + go-telegram-bot-api/v5 (long poll) cùng module với API | Fly.io cùng app | Share DB, đơn giản |
| DB | PostgreSQL 16 | Fly Postgres (initial shared-cpu-1x, 1GB) | Relational + ACID |
| Queue/Cache | Redis 7 | Fly Redis (256MB) | Queue + rate limit |
| Storage CDN | Cloudflare R2 | cdn.snakebacklink.com (CNAME → R2 public bucket + CF caching) | Free egress |
| Code signing | Sectigo OV cert (~$100/năm) | Self-host | Defender không flag |
| Installer | NSIS 3.x + signtool | GitHub Actions CI | Standard Windows |
| Payment VN | SePay webhook (MoMo, VietinBank, Vietcombank, TPBank) | Fly bot | Auto match |
| Payment global (Phase 2) | Lemon Squeezy | Merchant of record | Handle tax |
| Email | Resend | Transactional (license delivery, receipt) | Free tier đủ |
| AI | Claude Sonnet 4.6 qua 9Router | `cc/claude-sonnet-4-6` | Owner setup sẵn |
| Target finder | SerpAPI (primary) + fallback ScrapingBee | Call từ Fly | Rate limit-safe |
| DA/DR | Moz API (free 10K/month) → Ahrefs (Phase 2) | Call từ Fly | Cost-effective initial |
| Monitoring | Fly.io native metrics + Better Stack (logtail) + Sentry | SaaS free tier | |
| CI/CD | GitHub Actions + Fly deploy + Vercel auto-deploy + R2 upload | GitHub | |

### 1.3 Credit & Key Model (CỰC QUAN TRỌNG — CORE BUSINESS LOGIC)

**Key:**
- Format: `sbf_live_<32-char-base58>` (41 chars total, ~187 bits base58 entropy)
  - Breakdown: `sbf_live_` prefix (9) + 32 base58 chars = 41 chars
  - Entropy: 32 × log2(58) ≈ 187 bits (exceeds UUID v4 122 bits; collision-resistant far beyond SBF scale)
  - Base58 alphabet excludes `0OIl` to prevent eye-confusion
  - [Locked 2026-04-25 — phase-03 code review; supersedes earlier 46-char draft]
- Store: SHA256 hash in DB, plaintext chỉ hiện 1 lần cho user khi `/start` hoặc `/regenkey`
- 1 Telegram user = 1 active key (regenerate invalidate cũ)
- Không lock HWID, không lock IP — khách dùng đâu cũng được
- Chống abuse qua credit consumption, không qua device binding

**2 Credit pools, tách riêng:**

| Pool | Target | Content quality | Giá/credit (VND) | Use case |
|---|---|---|---|---|
| **Premium** 🔥 | Site DR ≥ 40, traffic thật, dofollow % cao, contextual | AI rewrite full Sonnet 4.6, unique 100%, anchor diversity enforce | 7,997 - 9,980 | Money site power link |
| **Standard** ⚡ | Site DR 0-39, mass volume, blog comment, forum sig | Template + light rewrite | 1,530 - 1,980 | Tier 2/3, index boost |

**Packages (initial, dễ chỉnh):**

```
Standard:
- Starter 50cr  → 99,000đ
- Basic 100cr   → 179,000đ  
- Pro 200cr     → 329,000đ  ⭐ featured (Decoy middle)
- Max 300cr     → 459,000đ

Premium:
- Starter 50cr  → 499,000đ
- Basic 100cr   → 899,000đ
- Pro 200cr     → 1,699,000đ ⭐ featured
- Max 300cr     → 2,399,000đ

Combo (boost AOV):
- Premium 100 + Standard 50   → 999,000đ (tiết kiệm 79K)
- Premium 200 + Standard 100  → 1,799,000đ (tiết kiệm 109K)
```

**Credit consumption rules:**
- 1 successful backlink = 1 credit consumed (atomic, sau khi job success)
- Failed backlink (captcha fail, target down, form validation fail) = 0 credit
- Captcha solve cost: nếu dùng BYOK → 0 credit; nếu dùng Snake credits pool → +1 credit per captcha
- Campaign creation: 0 credit
- Auto-find query: 3 credit/batch (tối đa 50 URL kết quả)

**Trial:**
- 5 credit Standard pool, 0 credit Premium
- 1 lần duy nhất / Telegram user ID (lifetime)
- Yêu cầu verify phone qua `request_contact`
- Account Telegram < 30 ngày tuổi → không trial (anti-bot)

### 1.4 Target Backlink Types (4 loại, mỗi loại 1 adapter module)

| Type | Mô tả | Credit pool chủ yếu | Complexity |
|---|---|---|---|
| `blog_comment` | Comment WordPress/Disqus/Blogger | Standard | Low |
| `forum_profile` | Register + profile bio link (vBulletin, phpBB, Discourse, Flarum) | Mixed | Medium |
| `web2_post` | Post bài lên Medium/Blogger/WordPress.com/Tumblr | Premium | High (need email temp + login) |
| `directory_listing` | Submit URL vào directory / business listing | Standard | Medium |

### 1.5 Anti-abuse Safeguards (MANDATORY — DEFAULT ON)

**MỌI safeguard dưới đây phải được implement như first-class feature, KHÔNG được là "nice to have".**

| Safeguard | Rule | Override? |
|---|---|---|
| Per-domain rate limit | 1 target domain nhận max 1 backlink/30 ngày/key | No |
| Per-key daily limit (floor) | Max 300 backlink/ngày/key | No |
| Per-campaign daily drip | Default 20/ngày, user config 5-50 | Yes (5-50 range) |
| Anchor diversity enforce | 60% branded / 20% naked URL / 15% generic / 5% exact match (weighted random) | No |
| Content uniqueness | Cosine similarity < 80% giữa các comment sinh ra trong 7 ngày gần nhất cùng key | No |
| Niche relevance filter | Site language + topic detection phải match campaign niche (min 0.6 cosine) | Yes (user toggle off) |
| Ethical mode | Premium pool only + daily limit 10 + skip site DR < 30 + skip spam-flagged domains | Yes (default ON) |
| Target blocklist | .gov, .edu, major brand domains (apple.com, google.com...) → skip always | No |
| Anomaly detection | Key request từ > 3 country codes trong 24h → auto-suspend 1h, notify Telegram | No |
| Payment fraud | Same SePay transaction ref retry > 2 times → flag manual review | No |

### 1.6 Security Model

1. **Request signing**: Mỗi request từ extension → server phải có header `X-Signature: hex(HMAC_SHA256(payload + timestamp, WASM_derived_secret))`. Secret được derive từ API key qua Rust WASM module (Argon2id với salt công khai embedded in WASM). Clone extension nhưng bypass WASM = server reject.
2. **Timestamp skew**: Request timestamp (header `X-Timestamp`) phải trong ±60s so với server time. Chống replay attack.
3. **Nonce cache**: Redis lưu nonce đã dùng trong 5 phút, trùng = reject. Chống replay trong window.
4. **Rate limit middleware**: Redis-backed, `250 req/min/key`, `5000 req/hour/key`, `100 req/min/IP` (chống DDoS).
5. **Key rotation**: User có thể `/regenkey` qua bot. Cũ invalidated ngay (Redis blocklist 24h để clear pending).
6. **Audit log**: Mọi action tạo/consume credit ghi vào `audit_log` immutable table (PostgreSQL).
7. **Secrets management**: Dùng Fly secrets cho prod, `.env.local` cho dev. KHÔNG commit secret.
8. **Database access**: API service dùng role `api_user` với privilege giới hạn (SELECT/INSERT/UPDATE trên tables cụ thể, KHÔNG DDL).

---

## 2. REPOSITORY STRUCTURE

Monorepo với pnpm workspaces cho JS packages + Go modules riêng. Sử dụng **Turborepo** cho task orchestration (build, lint, test song song).

```
snake-backlink-forge/
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
├── .gitignore
├── .editorconfig
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                  # Lint + test mỗi PR
│   │   ├── deploy-api.yml          # Fly deploy khi merge main
│   │   ├── deploy-landing.yml      # Vercel auto (via Vercel integration)
│   │   ├── build-extension.yml     # Build + sign .crx + upload R2
│   │   └── build-installer.yml     # Build + sign NSIS .exe + upload R2
│   └── PULL_REQUEST_TEMPLATE.md
├── pnpm-workspace.yaml
├── turbo.json
├── package.json                    # Root: workspace + scripts
│
├── apps/
│   ├── landing/                    # Next.js 15 landing + blog + docs
│   │   ├── src/
│   │   │   ├── app/
│   │   │   │   ├── (marketing)/
│   │   │   │   │   ├── layout.tsx
│   │   │   │   │   ├── page.tsx              # Landing
│   │   │   │   │   ├── pricing/page.tsx
│   │   │   │   │   ├── features/page.tsx
│   │   │   │   │   └── changelog/page.tsx
│   │   │   │   ├── blog/
│   │   │   │   │   ├── page.tsx              # List
│   │   │   │   │   └── [slug]/page.tsx       # Post
│   │   │   │   ├── docs/
│   │   │   │   │   ├── page.tsx
│   │   │   │   │   └── [...slug]/page.tsx
│   │   │   │   ├── legal/
│   │   │   │   │   ├── terms/page.tsx
│   │   │   │   │   ├── privacy/page.tsx
│   │   │   │   │   └── refund/page.tsx
│   │   │   │   ├── layout.tsx                # Root layout, fonts, theme
│   │   │   │   ├── globals.css
│   │   │   │   ├── not-found.tsx
│   │   │   │   └── api/
│   │   │   │       └── og/route.tsx          # Dynamic OG image
│   │   │   ├── components/
│   │   │   │   ├── ui/                       # shadcn registry
│   │   │   │   ├── magic/                    # MagicUI components
│   │   │   │   ├── aceternity/               # Aceternity components
│   │   │   │   ├── landing/
│   │   │   │   │   ├── hero-3d-globe.tsx
│   │   │   │   │   ├── hero-text-reveal.tsx
│   │   │   │   │   ├── social-proof.tsx
│   │   │   │   │   ├── how-it-works.tsx
│   │   │   │   │   ├── feature-bento.tsx
│   │   │   │   │   ├── pricing-table.tsx
│   │   │   │   │   ├── testimonials-marquee.tsx
│   │   │   │   │   ├── faq-accordion.tsx
│   │   │   │   │   └── cta-final.tsx
│   │   │   │   ├── nav/
│   │   │   │   ├── footer/
│   │   │   │   └── theme-toggle.tsx
│   │   │   ├── content/
│   │   │   │   ├── blog/
│   │   │   │   │   └── *.mdx                 # Seed 5 posts
│   │   │   │   └── docs/
│   │   │   │       └── *.mdx
│   │   │   ├── lib/
│   │   │   │   ├── fonts.ts
│   │   │   │   ├── seo.ts
│   │   │   │   ├── utils.ts
│   │   │   │   ├── analytics.ts
│   │   │   │   └── content.ts
│   │   │   └── styles/
│   │   │       └── mdx.css
│   │   ├── public/
│   │   │   ├── og-default.png
│   │   │   ├── robots.txt
│   │   │   └── sitemap.xml
│   │   ├── next.config.mjs
│   │   ├── tailwind.config.ts
│   │   ├── postcss.config.mjs
│   │   ├── tsconfig.json
│   │   ├── velite.config.ts                  # Or contentlayer
│   │   ├── components.json                   # shadcn
│   │   └── package.json
│   │
│   └── extension/                  # Chrome/Edge MV3
│       ├── src/
│       │   ├── manifest.config.ts            # Generate manifest.json via CRXJS plugin
│       │   ├── background/
│       │   │   ├── index.ts                  # SW entry
│       │   │   ├── router.ts                 # chrome.runtime.onMessage dispatcher
│       │   │   ├── api-client.ts             # Fetch + WASM sign
│       │   │   ├── campaign-runner.ts        # Poll next action, orchestrate
│       │   │   ├── offscreen-manager.ts      # Create/destroy offscreen doc
│       │   │   ├── state-manager.ts          # chrome.storage wrapper
│       │   │   ├── heartbeat.ts              # keep SW alive via alarms
│       │   │   └── logger.ts
│       │   ├── offscreen/
│       │   │   ├── offscreen.html
│       │   │   ├── offscreen.ts              # Hidden tab, DOM manipulation
│       │   │   ├── adapters/
│       │   │   │   ├── base.ts
│       │   │   │   ├── blog-comment.ts
│       │   │   │   ├── forum-profile.ts
│       │   │   │   ├── web2-post.ts
│       │   │   │   └── directory-listing.ts
│       │   │   └── captcha-hooks.ts
│       │   ├── content/
│       │   │   └── executor.ts               # Inject on demand for DOM ops
│       │   ├── popup/
│       │   │   ├── index.html
│       │   │   ├── main.ts
│       │   │   └── App.svelte                # Quick status + stop
│       │   ├── options/
│       │   │   ├── index.html
│       │   │   ├── main.ts
│       │   │   ├── App.svelte
│       │   │   └── components/
│       │   │       ├── KeySection.svelte
│       │   │       ├── CaptchaConfig.svelte
│       │   │       ├── ProxyConfig.svelte
│       │   │       ├── CampaignList.svelte
│       │   │       ├── CampaignEditor.svelte
│       │   │       ├── BacklinkHistory.svelte
│       │   │       ├── EthicalModeToggle.svelte
│       │   │       └── Stats.svelte
│       │   ├── wasm/
│       │   │   └── pkg/                      # Generated by wasm-pack (gitignored, build step)
│       │   ├── lib/
│       │   │   ├── types.ts
│       │   │   ├── constants.ts
│       │   │   ├── storage-schema.ts
│       │   │   └── telemetry.ts
│       │   └── assets/
│       │       ├── icon-16.png
│       │       ├── icon-48.png
│       │       └── icon-128.png
│       ├── crates/
│       │   └── sbf-wasm/                     # Rust crate
│       │       ├── Cargo.toml
│       │       ├── src/
│       │       │   ├── lib.rs
│       │       │   ├── hmac.rs
│       │       │   ├── derive.rs             # Argon2id key derive
│       │       │   └── fingerprint.rs
│       │       └── build.rs
│       ├── vite.config.ts                    # CRXJS + obfuscator plugin
│       ├── obfuscator.config.cjs
│       ├── tsconfig.json
│       ├── tailwind.config.ts
│       └── package.json
│
├── services/
│   ├── api/                        # Go Fiber API + Telegram bot (same binary)
│   │   ├── cmd/
│   │   │   └── api/
│   │   │       └── main.go                   # Entry: start fiber + bot goroutines
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   │   └── config.go                 # Env parse, validation
│   │   │   ├── db/
│   │   │   │   ├── db.go                     # Pool init
│   │   │   │   ├── sqlc/                     # sqlc generated
│   │   │   │   └── queries/                  # .sql files
│   │   │   │       ├── users.sql
│   │   │   │       ├── keys.sql
│   │   │   │       ├── wallets.sql
│   │   │   │       ├── campaigns.sql
│   │   │   │       ├── targets.sql
│   │   │   │       ├── jobs.sql
│   │   │   │       ├── ledger.sql
│   │   │   │       └── audit.sql
│   │   │   ├── migrations/                   # goose .sql
│   │   │   │   ├── 20260424001_init.sql       # single-file goose (+goose Up/Down sections)
│   │   │   │   └── ...
│   │   │   ├── redis/
│   │   │   │   └── redis.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go                   # X-API-Key → user
│   │   │   │   ├── hmac.go                   # X-Signature verify
│   │   │   │   ├── ratelimit.go              # Redis sliding window
│   │   │   │   ├── nonce.go                  # Replay guard
│   │   │   │   ├── logger.go                 # Structured log
│   │   │   │   └── recover.go                # Panic recovery
│   │   │   ├── api/
│   │   │   │   ├── server.go                 # Fiber app setup
│   │   │   │   ├── router.go                 # Route mount
│   │   │   │   └── handlers/
│   │   │   │       ├── me.go                 # GET /v1/me
│   │   │   │       ├── campaign.go           # CRUD + next/result
│   │   │   │       ├── target.go             # Finder
│   │   │   │       ├── captcha.go            # Proxy
│   │   │   │       ├── webhook.go            # SePay
│   │   │   │       └── health.go
│   │   │   ├── bot/
│   │   │   │   ├── bot.go                    # Init + long poll
│   │   │   │   ├── router.go                 # Command dispatcher
│   │   │   │   ├── state.go                  # FSM conversation state (Redis)
│   │   │   │   ├── commands/
│   │   │   │   │   ├── start.go
│   │   │   │   │   ├── key.go
│   │   │   │   │   ├── balance.go
│   │   │   │   │   ├── buy.go
│   │   │   │   │   ├── topup.go
│   │   │   │   │   ├── history.go
│   │   │   │   │   ├── download.go
│   │   │   │   │   ├── support.go
│   │   │   │   │   └── regenkey.go
│   │   │   │   ├── keyboards/
│   │   │   │   │   └── inline.go
│   │   │   │   └── templates/
│   │   │   │       └── messages.go           # Message templates i18n VN
│   │   │   ├── service/
│   │   │   │   ├── user_service.go
│   │   │   │   ├── key_service.go
│   │   │   │   ├── wallet_service.go         # Credit add/consume atomic
│   │   │   │   ├── campaign_service.go
│   │   │   │   ├── job_service.go
│   │   │   │   ├── target_service.go
│   │   │   │   ├── action_planner.go         # Core: quyết định action cho extension
│   │   │   │   ├── content_generator.go      # Claude Sonnet rewrite
│   │   │   │   ├── captcha_service.go
│   │   │   │   ├── finder_service.go         # SerpAPI
│   │   │   │   ├── scorer_service.go         # DR/DA via Moz
│   │   │   │   ├── safeguard_service.go      # Anti-abuse gates
│   │   │   │   └── audit_service.go
│   │   │   ├── integration/
│   │   │   │   ├── claude/
│   │   │   │   │   └── client.go             # 9Router client
│   │   │   │   ├── sepay/
│   │   │   │   │   └── webhook.go
│   │   │   │   ├── serpapi/
│   │   │   │   │   └── client.go
│   │   │   │   ├── moz/
│   │   │   │   │   └── client.go
│   │   │   │   ├── twocaptcha/
│   │   │   │   │   └── client.go
│   │   │   │   └── capsolver/
│   │   │   │       └── client.go
│   │   │   ├── worker/                       # Background goroutines
│   │   │   │   ├── prebuilt_crawler.go       # Weekly cron
│   │   │   │   ├── dr_refresher.go           # Monthly cron
│   │   │   │   ├── job_janitor.go            # Clean stale jobs
│   │   │   │   └── anomaly_detector.go       # Periodic scan
│   │   │   └── util/
│   │   │       ├── random.go
│   │   │       ├── hash.go
│   │   │       ├── hmac.go
│   │   │       ├── token.go                  # API key gen
│   │   │       └── tldextract.go
│   │   ├── sqlc.yaml
│   │   ├── Dockerfile
│   │   ├── fly.toml
│   │   ├── Makefile
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   └── edge/                       # Cloudflare Workers (Phase 2, optional)
│       └── ... (skip initial)
│
├── packages/
│   ├── shared-types/               # Shared TS types (ext ↔ landing, generated from Go struct via quicktype or handwritten)
│   │   ├── src/
│   │   │   ├── api.ts
│   │   │   ├── action.ts
│   │   │   └── index.ts
│   │   ├── tsconfig.json
│   │   └── package.json
│   │
│   └── ui/                         # Shared shadcn components (nếu cần cross-app)
│       └── ...
│
├── installer/                      # NSIS installer (Windows only, build trong GH Actions)
│   ├── setup.nsi                   # Main script
│   ├── registry.nsh                # Macros force-install
│   ├── assets/
│   │   ├── icon.ico
│   │   ├── banner.bmp              # 150x57 NSIS installer banner
│   │   └── welcome.bmp             # 164x314
│   ├── scripts/
│   │   ├── build.ps1               # Build + sign
│   │   └── sign.ps1                # signtool wrapper
│   └── README.md
│
├── tools/
│   ├── crx-packager/               # Node script: build ext → crx3 + updates.xml
│   │   ├── package.xml
│   │   ├── index.mjs
│   │   └── private-key.pem         # gitignored!
│   └── db-seed/                    # Seed prebuilt targets for dev
│       └── main.go
│
├── ops/
│   ├── docker-compose.yml          # Local dev: Postgres + Redis
│   ├── grafana/                    # Phase 2
│   ├── prometheus/                 # Phase 2
│   └── runbooks/
│       ├── incident-response.md
│       ├── deploy-checklist.md
│       └── key-rotation.md
│
└── docs/
    ├── architecture.md             # This prompt condensed
    ├── api-contract.md             # Detail từ Section 6
    ├── bot-protocol.md             # Telegram commands
    ├── action-protocol.md          # Server ↔ extension message spec
    ├── threat-model.md             # Security threats + mitigations
    ├── cost-model.xlsx             # Cost per user calc
    └── glossary.md
```

---

## 3. DATABASE SCHEMA (PostgreSQL 16)

### 3.1 Full DDL

```sql
-- migrations/20260424001_init.sql (Up section)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ─────────── USERS ────────────────────────────────────────────
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    telegram_id     BIGINT UNIQUE NOT NULL,
    telegram_username VARCHAR(64),
    phone_e164      VARCHAR(20),
    is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    is_banned       BOOLEAN NOT NULL DEFAULT FALSE,
    trial_used      BOOLEAN NOT NULL DEFAULT FALSE,
    language        VARCHAR(8) NOT NULL DEFAULT 'vi',
    referred_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ
);
CREATE INDEX idx_users_telegram_id ON users(telegram_id);

-- ─────────── API KEYS ────────────────────────────────────────
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash        BYTEA NOT NULL UNIQUE,          -- SHA256(plaintext)
    key_prefix      VARCHAR(12) NOT NULL,           -- First 8 chars for display "sbf_live_Zk3p..."
    name            VARCHAR(64),                    -- User label
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);
CREATE INDEX idx_keys_user ON api_keys(user_id) WHERE is_active = TRUE;
CREATE INDEX idx_keys_hash ON api_keys(key_hash) WHERE is_active = TRUE;

-- ─────────── WALLETS ────────────────────────────────────────
CREATE TABLE wallets (
    user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    premium_credits      INT NOT NULL DEFAULT 0 CHECK (premium_credits >= 0),
    standard_credits     INT NOT NULL DEFAULT 0 CHECK (standard_credits >= 0),
    total_premium_spent  INT NOT NULL DEFAULT 0,
    total_standard_spent INT NOT NULL DEFAULT 0,
    total_vnd_spent      BIGINT NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── LEDGER (immutable audit) ────────────────────────
CREATE TYPE ledger_event_type AS ENUM (
    'trial_grant',
    'topup',
    'consume_backlink',
    'consume_captcha',
    'consume_finder',
    'refund',
    'admin_adjust'
);

CREATE TABLE ledger (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),
    event_type      ledger_event_type NOT NULL,
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    delta_credits   INT NOT NULL,                   -- + for add, - for consume
    balance_after   INT NOT NULL,
    ref_entity_type VARCHAR(32),                    -- 'transaction', 'job', 'campaign'
    ref_entity_id   UUID,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ledger_user_time ON ledger(user_id, created_at DESC);
CREATE INDEX idx_ledger_ref ON ledger(ref_entity_type, ref_entity_id);

-- ─────────── TRANSACTIONS (top-ups) ──────────────────────────
CREATE TYPE transaction_status AS ENUM (
    'pending',
    'paid',
    'failed',
    'refunded',
    'manual_review'
);

CREATE TYPE transaction_provider AS ENUM (
    'sepay',
    'lemonsqueezy',
    'manual'
);

CREATE TABLE transactions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id),
    provider          transaction_provider NOT NULL,
    provider_ref      VARCHAR(255) UNIQUE,          -- SePay transaction ID, LS order ID
    package_code      VARCHAR(64) NOT NULL,         -- 'standard_pro_200', 'premium_max_300', 'combo_p100_s50'
    amount_vnd        BIGINT NOT NULL CHECK (amount_vnd > 0),
    premium_granted   INT NOT NULL DEFAULT 0,
    standard_granted  INT NOT NULL DEFAULT 0,
    status            transaction_status NOT NULL DEFAULT 'pending',
    paid_at           TIMESTAMPTZ,
    metadata          JSONB NOT NULL DEFAULT '{}',  -- raw webhook payload
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tx_user ON transactions(user_id, created_at DESC);
CREATE INDEX idx_tx_provider_ref ON transactions(provider, provider_ref);
CREATE INDEX idx_tx_status ON transactions(status) WHERE status IN ('pending', 'manual_review');

-- ─────────── CAMPAIGNS ──────────────────────────────────────
CREATE TYPE campaign_status AS ENUM (
    'draft', 'running', 'paused', 'completed', 'archived'
);

CREATE TABLE campaigns (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(128) NOT NULL,
    money_site_url  TEXT NOT NULL,
    niche_keywords  TEXT[] NOT NULL DEFAULT '{}',   -- ['crypto wallet', 'airdrop']
    anchor_texts    JSONB NOT NULL DEFAULT '[]',    -- [{text, weight, type}]
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    source_mode     VARCHAR(32) NOT NULL CHECK (source_mode IN ('prebuilt', 'autofind', 'custom', 'mixed')),
    daily_limit     INT NOT NULL DEFAULT 20 CHECK (daily_limit BETWEEN 5 AND 50),
    status          campaign_status NOT NULL DEFAULT 'draft',
    ethical_mode    BOOLEAN NOT NULL DEFAULT TRUE,
    niche_filter    BOOLEAN NOT NULL DEFAULT TRUE,
    credits_allocated INT NOT NULL DEFAULT 0,
    credits_consumed  INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_campaigns_user ON campaigns(user_id, status);

-- Max 3 active campaigns per user enforced at app layer + trigger below
CREATE OR REPLACE FUNCTION check_active_campaign_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'running' THEN
        IF (SELECT COUNT(*) FROM campaigns
            WHERE user_id = NEW.user_id
              AND status = 'running'
              AND id != NEW.id) >= 3 THEN
            RAISE EXCEPTION 'Max 3 active campaigns per user';
        END IF;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_campaign_limit BEFORE INSERT OR UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION check_active_campaign_limit();

-- ─────────── TARGETS (prebuilt pool + per-campaign custom) ─────────
CREATE TYPE target_type AS ENUM (
    'blog_comment', 'forum_profile', 'web2_post', 'directory_listing'
);

CREATE TYPE target_source AS ENUM ('prebuilt', 'autofind', 'custom');

CREATE TABLE targets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url             TEXT NOT NULL,
    domain          TEXT NOT NULL,
    tld             VARCHAR(32),
    type            target_type NOT NULL,
    source          target_source NOT NULL,
    owner_user_id   UUID REFERENCES users(id) ON DELETE CASCADE, -- NULL if global prebuilt
    pool            VARCHAR(16) NOT NULL CHECK (pool IN ('premium', 'standard')),
    dr              INT,                           -- Domain Rating 0-100
    da              INT,                           -- Domain Authority 0-100
    traffic_est     INT,
    language        VARCHAR(8),
    niche_tags      TEXT[] NOT NULL DEFAULT '{}',
    platform        VARCHAR(32),                   -- 'wordpress', 'disqus', 'discourse', 'phpbb'
    form_selectors  JSONB,                         -- cached DOM selectors
    captcha_type    VARCHAR(32),                   -- 'none', 'recaptcha_v2', 'hcaptcha', 'cf_turnstile'
    captcha_sitekey TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_blocklisted  BOOLEAN NOT NULL DEFAULT FALSE,
    success_rate    NUMERIC(4,3) NOT NULL DEFAULT 0.500,
    last_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(url, owner_user_id)
);
CREATE INDEX idx_targets_pool_type ON targets(pool, type) WHERE is_active AND NOT is_blocklisted;
CREATE INDEX idx_targets_owner ON targets(owner_user_id) WHERE owner_user_id IS NOT NULL;
CREATE INDEX idx_targets_domain ON targets(domain);
CREATE INDEX idx_targets_niche ON targets USING GIN(niche_tags);

-- ─────────── JOBS (each backlink attempt) ──────────────────────
CREATE TYPE job_status AS ENUM (
    'queued', 'dispatched', 'in_progress', 'success', 'failed', 'skipped'
);

CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    target_id       UUID NOT NULL REFERENCES targets(id),
    target_url_snapshot TEXT NOT NULL,
    anchor_text     TEXT NOT NULL,
    anchor_type     VARCHAR(16) NOT NULL,          -- branded/naked/generic/exact
    content_body    TEXT,                          -- AI generated
    status          job_status NOT NULL DEFAULT 'queued',
    pool            VARCHAR(16) NOT NULL,
    credits_cost    INT NOT NULL DEFAULT 1,
    captcha_cost    INT NOT NULL DEFAULT 0,
    error_code      VARCHAR(64),
    error_message   TEXT,
    result_url      TEXT,                          -- final posted URL
    dispatched_at   TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_jobs_user_status ON jobs(user_id, status, created_at DESC);
CREATE INDEX idx_jobs_campaign ON jobs(campaign_id);
CREATE INDEX idx_jobs_target ON jobs(target_id);
CREATE INDEX idx_jobs_pending ON jobs(status) WHERE status IN ('queued', 'dispatched', 'in_progress');

-- ─────────── DOMAIN COOLDOWN (prevent spam same domain) ─────
CREATE TABLE domain_cooldown (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain      TEXT NOT NULL,
    last_used   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, domain)
);
CREATE INDEX idx_cooldown_time ON domain_cooldown(last_used);

-- ─────────── DORK PATTERNS (prebuilt seed) ──────────────────
CREATE TABLE dork_patterns (
    id           SERIAL PRIMARY KEY,
    pattern      TEXT NOT NULL,                    -- 'site:wordpress.com "{niche}" "leave a comment"'
    target_type  target_type NOT NULL,
    expected_platform VARCHAR(32),
    success_weight NUMERIC(4,3) DEFAULT 0.500,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── AUDIT LOG (security events) ────────────────────
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users(id),
    key_id      UUID REFERENCES api_keys(id),
    event       VARCHAR(64) NOT NULL,              -- 'key_generated', 'key_revoked', 'anomaly_detected'
    ip_hash     BYTEA,                             -- sha256(ip), never store raw IP
    country     VARCHAR(2),
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_user ON audit_log(user_id, created_at DESC);

-- ─────────── SAFEGUARD VIOLATIONS ──────────────────────────
CREATE TABLE safeguard_hits (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    rule        VARCHAR(64) NOT NULL,              -- 'per_domain_limit', 'anchor_diversity'
    severity    VARCHAR(16) NOT NULL,              -- 'info', 'warn', 'block'
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────── FUNCTIONS: atomic credit consume ──────────────
CREATE OR REPLACE FUNCTION consume_credits(
    p_user_id UUID,
    p_pool VARCHAR,
    p_amount INT,
    p_event_type ledger_event_type,
    p_ref_type VARCHAR,
    p_ref_id UUID
) RETURNS INT AS $$
DECLARE
    new_balance INT;
BEGIN
    IF p_pool = 'premium' THEN
        UPDATE wallets
        SET premium_credits = premium_credits - p_amount,
            total_premium_spent = total_premium_spent + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id AND premium_credits >= p_amount
        RETURNING premium_credits INTO new_balance;
    ELSE
        UPDATE wallets
        SET standard_credits = standard_credits - p_amount,
            total_standard_spent = total_standard_spent + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id AND standard_credits >= p_amount
        RETURNING standard_credits INTO new_balance;
    END IF;

    IF new_balance IS NULL THEN
        RAISE EXCEPTION 'INSUFFICIENT_CREDITS' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO ledger (user_id, event_type, pool, delta_credits, balance_after, ref_entity_type, ref_entity_id)
    VALUES (p_user_id, p_event_type, p_pool, -p_amount, new_balance, p_ref_type, p_ref_id);

    RETURN new_balance;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION grant_credits(
    p_user_id UUID,
    p_pool VARCHAR,
    p_amount INT,
    p_event_type ledger_event_type,
    p_ref_type VARCHAR,
    p_ref_id UUID
) RETURNS INT AS $$
DECLARE
    new_balance INT;
BEGIN
    IF p_pool = 'premium' THEN
        UPDATE wallets
        SET premium_credits = premium_credits + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id
        RETURNING premium_credits INTO new_balance;
    ELSE
        UPDATE wallets
        SET standard_credits = standard_credits + p_amount,
            updated_at = NOW()
        WHERE user_id = p_user_id
        RETURNING standard_credits INTO new_balance;
    END IF;

    INSERT INTO ledger (user_id, event_type, pool, delta_credits, balance_after, ref_entity_type, ref_entity_id)
    VALUES (p_user_id, p_event_type, p_pool, p_amount, new_balance, p_ref_type, p_ref_id);

    RETURN new_balance;
END $$ LANGUAGE plpgsql;

-- ─────────── VIEWS ──────────────────────────────────────────
CREATE VIEW v_user_stats AS
SELECT
    u.id,
    u.telegram_id,
    w.premium_credits,
    w.standard_credits,
    COUNT(DISTINCT c.id) FILTER (WHERE c.status = 'running') AS active_campaigns,
    COUNT(DISTINCT j.id) FILTER (WHERE j.status = 'success') AS total_backlinks,
    COUNT(DISTINCT j.id) FILTER (WHERE j.status = 'success' AND j.created_at > NOW() - INTERVAL '24h') AS backlinks_24h
FROM users u
LEFT JOIN wallets w ON w.user_id = u.id
LEFT JOIN campaigns c ON c.user_id = u.id
LEFT JOIN jobs j ON j.user_id = u.id
GROUP BY u.id, w.premium_credits, w.standard_credits;

-- ─────────── SEED ──────────────────────────────────────────
-- (seed dork patterns, tiers sẽ có trong file migration tiếp theo)
```

### 3.2 Seed dork patterns file (`20260424002_seed_dorks.sql`)

Viết file seed 30 dork patterns VN + EN cho mỗi `target_type`. Ví dụ:

```sql
INSERT INTO dork_patterns (pattern, target_type, expected_platform) VALUES
('site:wordpress.com "{niche}" "leave a reply"', 'blog_comment', 'wordpress'),
('"{niche}" "powered by wordpress" inurl:/comments/', 'blog_comment', 'wordpress'),
('site:medium.com "{niche}" -inurl:signin', 'web2_post', 'medium'),
('"{niche}" "discourse" inurl:/t/', 'forum_profile', 'discourse'),
('site:*.blogspot.com "{niche}"', 'web2_post', 'blogger'),
-- ... 25 more patterns
;
```

---

## 4. API CONTRACT (Fiber REST)

### 4.1 Base URL & headers

- Base: `https://api.snakebacklink.com/v1`
- All requests require:
  - `X-API-Key: sbf_live_...`
  - `X-Timestamp: <unix_seconds>`
  - `X-Nonce: <uuid_v4>`
  - `X-Signature: hex(hmac_sha256(body + timestamp + nonce, derived_secret))` (derived from API key via Rust WASM Argon2id)

### 4.2 Endpoints

#### `GET /v1/me`
Returns user profile + wallet balance.

**Response 200:**
```json
{
  "user_id": "uuid",
  "telegram_id": 123456789,
  "credits": { "premium": 87, "standard": 142 },
  "stats": {
    "total_backlinks": 523,
    "backlinks_24h": 18,
    "active_campaigns": 2
  },
  "trial_used": true,
  "features": {
    "max_campaigns_parallel": 3,
    "max_daily_per_campaign": 50,
    "ethical_mode_default": true
  }
}
```

#### `POST /v1/campaigns`
Create campaign.

**Request:**
```json
{
  "name": "SnakeAI Homepage Boost",
  "money_site_url": "https://snakepremiumhub.com/snakeai",
  "niche_keywords": ["ai model", "deepseek", "vietnamese ai"],
  "anchor_texts": [
    {"text": "SnakeAI", "weight": 60, "type": "branded"},
    {"text": "https://snakepremiumhub.com/snakeai", "weight": 20, "type": "naked"},
    {"text": "vietnamese AI model", "weight": 15, "type": "generic"},
    {"text": "best ai model vn", "weight": 5, "type": "exact"}
  ],
  "pool": "premium",
  "source_mode": "mixed",
  "daily_limit": 15,
  "ethical_mode": true,
  "niche_filter": true
}
```

**Response 201:** Campaign object.

**Validation:**
- `anchor_texts` weights phải tổng = 100
- `money_site_url` phải là HTTPS URL valid
- `niche_keywords` 1-10 items
- `daily_limit` 5-50
- `pool` enum check

**Error codes:**
- `429 TOO_MANY_CAMPAIGNS` — user đã có 3 active
- `400 INVALID_ANCHOR_WEIGHTS` — weights không tổng 100
- `402 INSUFFICIENT_CREDITS` — không đủ credit để start (check min 5 cr pool này)

#### `GET /v1/campaigns`
List campaigns của user.

**Query params:** `?status=running&limit=20&offset=0`

#### `PATCH /v1/campaigns/:id`
Update (only `status`, `daily_limit`, `ethical_mode`, `niche_filter`).

#### `DELETE /v1/campaigns/:id`
Archive (soft delete).

#### `POST /v1/campaign/next` ⭐ CORE
Extension gọi endpoint này để nhận **action plan** tiếp theo.

**Request:**
```json
{
  "campaign_id": "uuid",
  "client_info": {
    "ext_version": "1.0.3",
    "browser": "chrome",
    "browser_version": "131.0",
    "os": "win"
  }
}
```

**Response 200** (action = build_backlink):
```json
{
  "action": "build_backlink",
  "job_id": "uuid",
  "target": {
    "url": "https://example-blog.com/post/123",
    "type": "blog_comment",
    "platform": "wordpress"
  },
  "form_plan": {
    "selectors": {
      "name_input": "input#author",
      "email_input": "input#email",
      "website_input": "input#url",
      "comment_textarea": "textarea#comment",
      "submit_button": "input#submit"
    },
    "values": {
      "name": "John Smith",
      "email": "randomuser+abc@gmail.com",
      "website": "https://snakepremiumhub.com/snakeai",
      "comment": "Great insights on vietnamese AI landscape! SnakeAI looks promising especially for local language use cases. Bookmarked for later read."
    },
    "human_delays_ms": {
      "before_start": 2400,
      "between_fields": [900, 1800],
      "before_submit": 3200
    }
  },
  "captcha": null,
  "expected_signals": {
    "success_selectors": [".comment-awaiting-moderation", ".comment-posted"],
    "error_selectors": [".wp-die-message", ".error"]
  },
  "expires_at": 1745506800
}
```

**Response 200** (action = wait):
```json
{
  "action": "wait",
  "reason": "daily_limit_reached",
  "retry_after_seconds": 3600
}
```

**Response 200** (action = campaign_completed):
```json
{
  "action": "campaign_completed",
  "campaign_id": "uuid"
}
```

**Logic bên server:**
1. Auth check (key → user → wallet balance > 0 for chosen pool)
2. Rate limit check (Redis sliding window)
3. Load campaign, verify status=running, verify owned by user
4. Safeguard gates (sequential):
   - Campaign daily limit (count jobs today) → nếu đạt → `action=wait`
   - Per-key daily floor (300/day) → nếu đạt → `action=wait` reason=`key_daily_cap`
5. Target selection:
   - Query `targets` WHERE pool match, type match, niche overlap, NOT in `domain_cooldown` với user này trong 30d, is_active
   - Order: weighted random by `success_rate DESC, DR DESC`
   - Lock target cho user (Redis SET NX 5min TTL) để tránh 2 campaign của cùng user pick cùng target
6. Anchor pick: weighted random theo `anchor_texts`
7. Content generation: gọi `claude-sonnet-4-6` với system prompt tùy target type + target snapshot
8. Safeguard content check: cosine similarity với 50 comment gần nhất của user → nếu > 0.8 → regen (max 2 retries)
9. Pre-consume 1 credit (hold, chưa commit) → create `job` row status=queued
10. Return action plan với job_id

**Error codes:**
- `402 INSUFFICIENT_CREDITS`
- `404 CAMPAIGN_NOT_FOUND`
- `409 CAMPAIGN_NOT_RUNNING`
- `503 NO_TARGETS_AVAILABLE` — pool cạn

#### `POST /v1/campaign/result` ⭐ CORE
Extension báo kết quả job.

**Request:**
```json
{
  "job_id": "uuid",
  "status": "success",
  "result_url": "https://example-blog.com/post/123#comment-789",
  "posted_at_ms": 1745506500123,
  "error_code": null,
  "error_message": null,
  "evidence": {
    "final_dom_hash": "a1b2c3...",
    "success_selector_matched": ".comment-awaiting-moderation"
  }
}
```

**Response 204.**

**Logic server:**
1. Lookup job, verify owned by user, status=dispatched/in_progress
2. If `status=success`:
   - Commit credit consume (via `consume_credits()` stored proc)
   - Update target `success_rate` (exponential moving avg)
   - Insert into `domain_cooldown`
   - Update campaign `credits_consumed++`
3. If `status=failed`:
   - Release credit hold
   - Decrement target `success_rate` slightly
   - Insert into `domain_cooldown` với TTL ngắn (24h) để thử lại sau
4. Broadcast SSE event tới extension (qua Redis pubsub) để popup update realtime

#### `POST /v1/captcha/solve`
Proxy captcha to 2captcha/CapSolver.

**Request:**
```json
{
  "job_id": "uuid",
  "type": "recaptcha_v2",
  "sitekey": "6Ld...",
  "pageurl": "https://example.com/post"
}
```

**Response 200:**
```json
{
  "token": "03AGdBq25...",
  "provider": "2captcha",
  "cost_credits": 1,
  "solve_duration_ms": 18400
}
```

**Logic server:**
- Check user config BYOK hay Snake Credits (stored in encrypted user_settings)
- Nếu BYOK: call trực tiếp với user's key, 0 credit charged
- Nếu Credits: check credit pool (captcha dùng standard_credits chung với backlink), consume 1, call 2captcha với master key của mày
- Timeout 180s, poll 2captcha every 5s
- Retry 1 lần nếu fail

#### `POST /v1/finder/search`
Auto-find targets từ niche keywords.

**Request:**
```json
{
  "niche_keywords": ["ai coding"],
  "target_types": ["blog_comment", "forum_profile"],
  "pool": "standard",
  "max_results": 30
}
```

**Logic:**
- Consume 3 credit pool this
- Pick random dork patterns từ `dork_patterns` matching target_types
- Substitute `{niche}` → first keyword
- Call SerpAPI với query, get URLs
- For each URL: HEAD check, language detect, find form structure (optional scrape)
- Dedup vs existing `targets` (owner=user)
- Insert rows vào `targets` (source=autofind, owner_user_id=user)
- Return list of added targets

**Response 200:** Array of targets.

#### `POST /v1/targets/import`
Custom import (multipart form-data CSV).

#### `GET /v1/jobs?campaign_id=&limit=&offset=`
History.

#### `POST /webhooks/sepay`
SePay bank webhook. Bearer token verify. Match `provider_ref` vào `transactions` và grant credits.

#### `POST /webhooks/lemonsqueezy` (Phase 2)

#### `GET /health`, `GET /ready` (k8s/Fly health probes).

### 4.3 HMAC signature algorithm

```
secret = argon2id(
    password = api_key_plaintext,
    salt = "sbf.v1.hmac-secret" (16 bytes fixed in WASM),
    time_cost = 2,
    memory_cost = 19456, // 19 MiB
    parallelism = 1,
    hash_len = 32
)

sig_input = body_bytes || "\n" || timestamp || "\n" || nonce
signature = hex( hmac_sha256(secret, sig_input) )
```

Server verify:
1. Parse headers
2. Check timestamp within ±60s of server time
3. Check nonce not in Redis `nonce:<nonce>` (if exists → reject; else SET with TTL 5min)
4. Load user by API key hash (SHA256 of plaintext in header)
5. Derive secret using Argon2id with same params
6. Compute expected signature, compare constant-time
7. If pass → continue handler

---

## 5. TELEGRAM BOT SPEC

### 5.1 Commands & flow

| Command | Description | State |
|---|---|---|
| `/start` | Welcome + force `request_contact` nếu chưa verified → generate key + grant 5 standard trial credit | Always |
| `/key` | Show current API key (masked: `sbf_live_Zk3p•••••dVo5`), with inline button "Copy" + "Regenerate" | Authenticated |
| `/balance` | Premium + Standard credits + total spent | Authenticated |
| `/buy` | Menu chọn package với inline keyboard | Authenticated |
| `/topup` | After picking package → hiển thị QR MoMo/Bank từ SePay + hướng dẫn | Auth + has_pending_tx |
| `/history` | Last 10 transactions + last 10 backlinks (với pagination inline) | Authenticated |
| `/campaigns` | (Phase 2) List campaigns đang chạy | Authenticated |
| `/download` | Link tải installer + instructions | Authenticated |
| `/support` | Menu FAQ inline + ticket submission | Always |
| `/regenkey` | Confirm + regenerate | Authenticated |
| `/language` | Chuyển VN/EN | Always |
| `/ref` | Referral code của user + stats | Authenticated |
| `/admin *` | Admin only (mày) commands | Role check |

### 5.2 State machine (Redis-backed FSM)

```
// Key: tg:state:<telegram_id>
// Value: JSON { state: string, data: {...}, expires_at: unix }

States:
- idle
- awaiting_contact (during /start)
- buy_selecting_package (inline keyboard chọn Standard/Premium, size)
- buy_confirming (show summary, confirm)
- topup_waiting (QR hiện, poll SePay webhook)
- support_describing (user typing issue)
```

### 5.3 Message templates (VN default, EN secondary)

Tất cả messages phải trong file `bot/templates/messages.go` với map `lang → key → template`. Dùng `text/template` cho interpolation.

Example:
```go
var Messages = map[string]map[string]string{
    "vi": {
        "start_welcome": "🐍 *Chào mừng đến Snake Backlink Forge!*\n\nTool automation xây backlink chuyên nghiệp, dùng qua Chrome/Edge extension.\n\nĐể bắt đầu, hãy xác thực số điện thoại của bạn bằng nút bên dưới (chỉ dùng để chống lạm dụng, KHÔNG lưu trữ).",
        "start_verified": "✅ Xác thực thành công!\n\n🎁 Bạn được tặng *5 credit Standard* để thử nghiệm.\n\n🔑 API Key của bạn:\n`{{.Key}}`\n\n⚠️ Lưu lại key này, bot sẽ không hiện lại. Dùng /key để xem lại phần hash mask.",
        "balance": "💰 *Số dư của bạn:*\n\n🔥 Premium: *{{.Premium}}* credit\n⚡ Standard: *{{.Standard}}* credit\n\n💸 Đã chi: {{.TotalVND}} đ",
        // ... etc
    },
    "en": { ... },
}
```

### 5.4 SePay webhook flow

SePay sẽ POST vào `/webhooks/sepay` khi có giao dịch tới tài khoản đã bind:

```json
{
  "id": 92345,
  "gateway": "MBBank",
  "transactionDate": "2026-04-24 15:22:41",
  "accountNumber": "1234567890",
  "subAccount": null,
  "code": null,
  "content": "SBF TOPUP 7a3c9",  // Extracted order ID
  "transferType": "in",
  "description": "...",
  "transferAmount": 329000,
  "referenceCode": "FT26115...",
  "accumulated": 0
}
```

Logic:
1. Verify bearer token (Fly secret `SEPAY_WEBHOOK_TOKEN`)
2. Parse `content` → extract order code (ví dụ `7a3c9`)
3. Find matching transaction với `status=pending` và `provider_ref=7a3c9`
4. Compare `transferAmount` >= expected `amount_vnd`
5. If match:
   - Atomic update `transactions.status=paid, paid_at=NOW()`
   - Call `grant_credits()` stored proc for premium & standard granted
   - Send Telegram message to user: "✅ Nạp thành công {N} credit Premium + {M} Standard"
6. Return `200 OK` always (SePay expects 200 or will retry)

### 5.5 Anti-abuse trial gate

Before granting 5 trial credits in `/start`:
```go
func canGrantTrial(ctx, tgUser) error {
    if tgUser.CreatedDate.After(time.Now().AddDate(0, 0, -30)) {
        return ErrAccountTooNew
    }
    existingUser, _ := db.GetUserByTelegramID(ctx, tgUser.ID)
    if existingUser != nil && existingUser.TrialUsed {
        return ErrTrialAlreadyUsed
    }
    // Phone verify required
    if tgUser.Phone == "" {
        return ErrPhoneVerifyRequired
    }
    return nil
}
```

---

## 6. EXTENSION ARCHITECTURE (MV3)

### 6.1 manifest.json (generated by CRXJS from TS config)

```typescript
// src/manifest.config.ts
import { defineManifest } from '@crxjs/vite-plugin'

export default defineManifest({
  manifest_version: 3,
  name: 'Snake Backlink Forge',
  version: '1.0.0',
  description: 'Professional backlink automation with AI content',
  icons: {
    16: 'src/assets/icon-16.png',
    48: 'src/assets/icon-48.png',
    128: 'src/assets/icon-128.png',
  },
  action: {
    default_popup: 'src/popup/index.html',
    default_title: 'Snake Backlink Forge',
  },
  options_page: 'src/options/index.html',
  background: {
    service_worker: 'src/background/index.ts',
    type: 'module',
  },
  permissions: [
    'storage',
    'alarms',
    'offscreen',
    'scripting',
    'tabs',
    'cookies',
    'declarativeNetRequest',
  ],
  host_permissions: ['<all_urls>'], // required for content script inject on arbitrary target
  content_scripts: [
    {
      matches: ['<all_urls>'],
      js: ['src/content/executor.ts'],
      run_at: 'document_idle',
      world: 'ISOLATED',
    },
  ],
  web_accessible_resources: [
    { resources: ['src/wasm/pkg/*'], matches: ['<all_urls>'] },
  ],
  content_security_policy: {
    extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self';",
  },
})
```

### 6.2 Service Worker — `background/index.ts`

Responsibilities:
- Maintain `chrome.alarms` heartbeat (every 30s) để keep SW alive (MV3 SW idle after 30s)
- Listen `chrome.runtime.onInstalled` → open options page
- Listen `chrome.runtime.onMessage` → route to handlers
- Manage offscreen document lifecycle
- Run `campaign-runner.ts` loop when any campaign active

**Loop logic:**
```typescript
async function campaignLoop(campaignId: string) {
  while (await isRunning(campaignId)) {
    try {
      const plan = await api.post('/v1/campaign/next', { campaign_id: campaignId })
      if (plan.action === 'wait') {
        await sleep(plan.retry_after_seconds * 1000)
        continue
      }
      if (plan.action === 'campaign_completed') {
        await markCampaignCompleted(campaignId)
        break
      }
      // plan.action === 'build_backlink'
      const result = await executeViaOffscreen(plan)
      await api.post('/v1/campaign/result', { job_id: plan.job_id, ...result })
      await sleep(randomBetween(20_000, 45_000)) // human-like cooldown between links
    } catch (err) {
      logger.error('loop_error', err)
      await sleep(60_000)
    }
  }
}
```

### 6.3 Offscreen Document — `offscreen/offscreen.ts`

Chrome MV3 offscreen document: invisible DOM context, no UI, runs iframe/scripts without user seeing tab change.

```typescript
import { BlogCommentAdapter } from './adapters/blog-comment'
import { ForumProfileAdapter } from './adapters/forum-profile'
import { Web2PostAdapter } from './adapters/web2-post'
import { DirectoryAdapter } from './adapters/directory-listing'

const ADAPTERS = {
  blog_comment: BlogCommentAdapter,
  forum_profile: ForumProfileAdapter,
  web2_post: Web2PostAdapter,
  directory_listing: DirectoryAdapter,
}

chrome.runtime.onMessage.addListener(async (msg, sender, sendResponse) => {
  if (msg.type !== 'execute_plan') return
  const { plan } = msg
  const adapter = new ADAPTERS[plan.target.type](plan)
  try {
    const result = await adapter.run()
    sendResponse({ success: true, result })
  } catch (err) {
    sendResponse({ success: false, error: err.message })
  }
  return true
})
```

### 6.4 Adapter base class

```typescript
abstract class BaseAdapter {
  constructor(protected plan: ActionPlan) {}
  abstract run(): Promise<AdapterResult>

  protected async humanDelay(rangeMs: [number, number]): Promise<void> {
    const ms = rangeMs[0] + Math.random() * (rangeMs[1] - rangeMs[0])
    await new Promise(r => setTimeout(r, ms))
  }

  protected async humanType(el: HTMLInputElement | HTMLTextAreaElement, text: string) {
    el.focus()
    for (const ch of text) {
      el.value += ch
      el.dispatchEvent(new InputEvent('input', { bubbles: true, data: ch }))
      await this.humanDelay([40, 130]) // varied typing speed
    }
    el.dispatchEvent(new Event('change', { bubbles: true }))
  }

  protected async solveCaptcha(sitekey: string, pageurl: string, type: string) {
    return chrome.runtime.sendMessage({
      type: 'solve_captcha',
      job_id: this.plan.job_id,
      sitekey, pageurl, captchaType: type,
    })
  }
}
```

### 6.5 Blog comment adapter (example)

```typescript
class BlogCommentAdapter extends BaseAdapter {
  async run(): Promise<AdapterResult> {
    const iframe = document.createElement('iframe')
    iframe.src = this.plan.target.url
    iframe.style.cssText = 'position:absolute;left:-9999px;width:1280px;height:800px'
    document.body.appendChild(iframe)

    await this.waitForLoad(iframe)
    await this.humanDelay([this.plan.form_plan.human_delays_ms.before_start, this.plan.form_plan.human_delays_ms.before_start + 1000])

    const doc = iframe.contentDocument!
    const sel = this.plan.form_plan.selectors
    const val = this.plan.form_plan.values

    await this.humanType(doc.querySelector(sel.name_input)!, val.name)
    await this.humanDelay(this.plan.form_plan.human_delays_ms.between_fields)

    await this.humanType(doc.querySelector(sel.email_input)!, val.email)
    await this.humanDelay(this.plan.form_plan.human_delays_ms.between_fields)

    if (sel.website_input && val.website) {
      await this.humanType(doc.querySelector(sel.website_input)!, val.website)
      await this.humanDelay(this.plan.form_plan.human_delays_ms.between_fields)
    }

    await this.humanType(doc.querySelector(sel.comment_textarea)!, val.comment)
    await this.humanDelay([this.plan.form_plan.human_delays_ms.before_submit, this.plan.form_plan.human_delays_ms.before_submit + 1500])

    if (this.plan.captcha) {
      const token = await this.solveCaptcha(this.plan.captcha.sitekey, this.plan.target.url, this.plan.captcha.type)
      const cap = doc.querySelector('#g-recaptcha-response') as HTMLTextAreaElement
      if (cap) cap.value = token
    }

    doc.querySelector<HTMLButtonElement>(sel.submit_button)!.click()

    const outcome = await this.waitForOutcome(doc, this.plan.expected_signals)
    iframe.remove()
    return outcome
  }

  // waitForLoad, waitForOutcome... detailed implementations
}
```

### 6.6 Popup UI (Svelte 5, runes)

Minimal: status light, 3 latest backlinks, Stop all button.

### 6.7 Options page (Svelte 5)

Major sections:
1. **Key Section**: Input + verify button → test call `/v1/me`, shows balance
2. **Captcha Config**: Toggle BYOK vs Credits; if BYOK, inputs for 2captcha/CapSolver keys
3. **Proxy Config**: Optional — note giải thích rõ proxy KHÔNG bắt buộc (default IP khách là đủ)
4. **Campaigns**: CRUD list. Tạo mới → wizard 4 bước (info → anchors → source → safeguards)
5. **Ethical Mode Toggle**: Prominent, default ON, tooltip explain
6. **Backlink History**: Table with filters
7. **Stats**: 24h chart using lightweight-charts or recharts
8. **About**: Version, link landing + telegram bot

### 6.8 WASM module (`crates/sbf-wasm/`)

```rust
use argon2::{Argon2, Algorithm, Params, Version};
use hmac::{Hmac, Mac};
use sha2::Sha256;
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub struct Signer {
    secret: [u8; 32],
}

#[wasm_bindgen]
impl Signer {
    #[wasm_bindgen(constructor)]
    pub fn new(api_key: &str) -> Result<Signer, JsValue> {
        let mut out = [0u8; 32];
        let params = Params::new(19456, 2, 1, Some(32))
            .map_err(|e| JsValue::from_str(&format!("argon2 params: {e}")))?;
        let argon = Argon2::new(Algorithm::Argon2id, Version::V0x13, params);
        argon.hash_password_into(api_key.as_bytes(), b"sbf.v1.hmac-secr", &mut out)
            .map_err(|e| JsValue::from_str(&format!("argon2 derive: {e}")))?;
        Ok(Signer { secret: out })
    }

    #[wasm_bindgen]
    pub fn sign(&self, body: &[u8], timestamp: &str, nonce: &str) -> String {
        type HmacSha256 = Hmac<Sha256>;
        let mut mac = HmacSha256::new_from_slice(&self.secret).unwrap();
        mac.update(body);
        mac.update(b"\n");
        mac.update(timestamp.as_bytes());
        mac.update(b"\n");
        mac.update(nonce.as_bytes());
        hex::encode(mac.finalize().into_bytes())
    }
}
```

Build: `wasm-pack build --target web --release` → output vào `pkg/`.

Từ TS:
```typescript
import init, { Signer } from '../wasm/pkg/sbf_wasm.js'

let signer: Signer | null = null
export async function initSigner(apiKey: string) {
  await init()
  signer = new Signer(apiKey)
}
export function signRequest(body: Uint8Array, ts: string, nonce: string): string {
  return signer!.sign(body, ts, nonce)
}
```

### 6.9 Vite config with obfuscation

```typescript
// vite.config.ts
import { defineConfig } from 'vite'
import { crx } from '@crxjs/vite-plugin'
import manifest from './src/manifest.config'
import svelte from '@sveltejs/vite-plugin-svelte'
import { javascriptObfuscator } from 'vite-plugin-javascript-obfuscator'
import wasmPack from 'vite-plugin-wasm-pack'

export default defineConfig({
  plugins: [
    svelte(),
    wasmPack('./crates/sbf-wasm'),
    crx({ manifest }),
    javascriptObfuscator({
      include: ['src/background/**', 'src/offscreen/**', 'src/content/**'],
      exclude: ['src/popup/**', 'src/options/**'], // UI needs to be debuggable in Svelte devtools
      options: {
        compact: true,
        controlFlowFlattening: true,
        controlFlowFlatteningThreshold: 0.75,
        deadCodeInjection: true,
        deadCodeInjectionThreshold: 0.4,
        debugProtection: true,
        debugProtectionInterval: 2000,
        disableConsoleOutput: true,
        identifierNamesGenerator: 'mangled-shuffled',
        renameGlobals: false,
        selfDefending: true,
        splitStrings: true,
        splitStringsChunkLength: 8,
        stringArray: true,
        stringArrayEncoding: ['rc4'],
        stringArrayIndexShift: true,
        stringArrayRotate: true,
        stringArrayShuffle: true,
        stringArrayWrappersCount: 3,
        stringArrayWrappersChainedCalls: true,
        stringArrayWrappersType: 'function',
        stringArrayThreshold: 0.85,
        transformObjectKeys: true,
        unicodeEscapeSequence: false,
      },
    }),
  ],
  build: {
    target: 'es2022',
    minify: 'terser',
    terserOptions: {
      mangle: { toplevel: true },
      compress: { passes: 3, drop_console: true, drop_debugger: true },
    },
    sourcemap: false,
  },
})
```

---

## 7. CORE SERVICE LOGIC DETAIL

### 7.1 Action Planner (`service/action_planner.go`)

```go
type ActionPlanner struct {
    db *sqlc.Queries
    redis *redis.Client
    finder *FinderService
    content *ContentGenerator
    scorer *ScorerService
    safeguard *SafeguardService
}

func (p *ActionPlanner) PlanNext(ctx context.Context, userID, campaignID uuid.UUID) (*ActionPlan, error) {
    // 1. Load campaign, verify
    camp, err := p.db.GetCampaign(ctx, campaignID)
    if err != nil || camp.UserID != userID || camp.Status != "running" {
        return nil, ErrCampaignNotActive
    }

    // 2. Safeguard gate
    if decision, err := p.safeguard.CheckAll(ctx, userID, camp); err != nil {
        return nil, err
    } else if decision.Wait != nil {
        return &ActionPlan{Action: "wait", RetryAfter: decision.Wait.Seconds, Reason: decision.Reason}, nil
    } else if decision.Completed {
        return &ActionPlan{Action: "campaign_completed", CampaignID: campaignID}, nil
    }

    // 3. Pick target
    target, err := p.pickTarget(ctx, userID, camp)
    if err != nil {
        return nil, err
    }

    // 4. Weighted random anchor
    anchor := pickAnchor(camp.AnchorTexts)

    // 5. Generate content
    content, err := p.content.Generate(ctx, ContentRequest{
        TargetType: target.Type,
        TargetURL: target.URL,
        MoneySiteURL: camp.MoneySiteURL,
        AnchorText: anchor.Text,
        Niche: camp.NicheKeywords,
    })
    if err != nil {
        return nil, err
    }

    // 6. Similarity check
    if sim, _ := p.safeguard.ContentSimilarityCheck(ctx, userID, content.Body); sim > 0.8 {
        // regen once
        content, err = p.content.Generate(ctx, ContentRequest{...})
    }

    // 7. Create job (with hold on credit)
    job, err := p.createJobWithHold(ctx, userID, camp, target, anchor, content)
    if err != nil {
        return nil, err
    }

    // 8. Build plan
    plan := buildPlan(target, content, anchor, job)
    return plan, nil
}
```

### 7.2 Content Generator (Claude Sonnet 4.6 via 9Router)

```go
type ContentGenerator struct {
    claude *claude.Client // wraps 9Router endpoint
}

const BLOG_COMMENT_SYSTEM_PROMPT = `You are a professional SEO content writer. Generate a thoughtful, natural-sounding comment for a blog post.

Rules:
- 40-120 words
- Match the blog's tone (casual/professional) based on URL
- Include specific engagement with the topic (quote a point, ask a question)
- NEVER mention the anchor URL explicitly — it will be placed in the author website field, not comment body
- Vietnamese comments if target language=vi, else English
- Avoid generic phrases like "great post", "thanks for sharing"
- No emojis unless matching site style

Output JSON only: {"name": "...", "email": "...@gmail.com", "comment": "..."}`

func (c *ContentGenerator) Generate(ctx context.Context, req ContentRequest) (*Content, error) {
    systemPrompt := selectPrompt(req.TargetType)
    userPrompt := fmt.Sprintf("Target URL: %s\nNiche: %s\nAnchor text to use as display name on website field: %s\nGenerate the form values in JSON.",
        req.TargetURL, strings.Join(req.Niche, ", "), req.AnchorText)

    resp, err := c.claude.Messages(ctx, &claude.Request{
        Model: "cc/claude-sonnet-4-6",
        MaxTokens: 500,
        System: systemPrompt,
        Messages: []claude.Message{{Role: "user", Content: userPrompt}},
        Temperature: 0.8,
    })
    if err != nil {
        return nil, err
    }
    var content Content
    if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &content); err != nil {
        return nil, err
    }
    return &content, nil
}
```

### 7.3 Safeguard Service (implements all rules từ 1.5)

```go
type SafeguardService struct { ... }

func (s *SafeguardService) CheckAll(ctx, userID, camp) (*Decision, error) {
    // 1. Per-campaign daily drip
    todayCount, _ := s.db.CountJobsToday(ctx, camp.ID)
    if todayCount >= camp.DailyLimit {
        return &Decision{Wait: &WaitHint{Seconds: secondsUntilMidnightVN()}, Reason: "daily_limit_reached"}, nil
    }
    // 2. Per-key daily cap
    keyTotal, _ := s.db.CountJobsToday_User(ctx, userID)
    if keyTotal >= 300 {
        return &Decision{Wait: &WaitHint{Seconds: 3600}, Reason: "key_daily_cap"}, nil
    }
    // 3. Campaign allocation exhausted
    if camp.CreditsConsumed >= camp.CreditsAllocated && camp.CreditsAllocated > 0 {
        return &Decision{Completed: true}, nil
    }
    // 4. Check wallet has credit for pool
    wallet, _ := s.db.GetWallet(ctx, userID)
    poolBal := pickPoolBalance(wallet, camp.Pool)
    if poolBal < 1 {
        return &Decision{Wait: &WaitHint{Seconds: 1800}, Reason: "insufficient_credits"}, nil
    }
    return &Decision{Proceed: true}, nil
}

func (s *SafeguardService) ContentSimilarityCheck(ctx, userID, body string) (float64, error) {
    // Last 50 comments
    recent, _ := s.db.GetRecentComments(ctx, userID, 50)
    maxSim := 0.0
    for _, r := range recent {
        sim := cosineSimilarityTFIDF(body, r.ContentBody)
        if sim > maxSim { maxSim = sim }
    }
    return maxSim, nil
}
```

---

## 8. LANDING PAGE SPEC (Next.js 15)

### 8.1 Design system

**Colors (Tailwind v4 `@theme`):**
```css
@theme {
  --color-bg: oklch(0.14 0 0);
  --color-surface: oklch(0.18 0 0);
  --color-border: oklch(0.28 0 0);
  --color-primary: oklch(0.72 0.17 160);     /* Emerald */
  --color-primary-foreground: oklch(0.14 0 0);
  --color-accent-cyan: oklch(0.82 0.12 200);
  --color-accent-purple: oklch(0.68 0.18 300);
  --color-text: oklch(0.95 0 0);
  --color-muted: oklch(0.65 0 0);
}
```

**Fonts:**
- `Cal Sans` (headlines) — local file
- `Inter Variable` (body)
- `Geist Mono` (code)

**Motion tokens:**
- Enter: `fade-up-blur` (duration 600ms, easing cubic-bezier(0.16, 1, 0.3, 1))
- Hover: transform + glow
- Scroll: Lenis smooth + Framer `useInView`

### 8.2 Hero section spec

```tsx
<section className="relative min-h-screen overflow-hidden">
  {/* Aceternity Spotlight */}
  <Spotlight className="-top-40 left-0" fill="rgba(16, 185, 129, 0.3)" />

  {/* R3F animated globe with backlink beams */}
  <div className="absolute inset-0 z-0 opacity-80">
    <Canvas>
      <Suspense fallback={null}>
        <BacklinkGlobe /> {/* animated arcs từ nhiều điểm về 1 điểm trung tâm */}
      </Suspense>
    </Canvas>
  </div>

  {/* Content */}
  <div className="relative z-10 container mx-auto px-4 pt-32 pb-20">
    <TextReveal>
      <h1 className="text-6xl md:text-8xl font-cal bg-gradient-to-r from-emerald-400 via-cyan-400 to-purple-500 bg-clip-text text-transparent">
        Build backlinks
        <br /> like a ghost in the machine
      </h1>
    </TextReveal>
    <p className="mt-8 text-xl md:text-2xl text-muted-foreground max-w-2xl">
      AI-powered backlink automation qua Chrome extension.
      Pay per link, scale on demand.
    </p>
    <div className="mt-12 flex gap-4">
      <ShimmerButton href="https://t.me/SnakeBacklinkBot">
        Dùng thử 5 credit miễn phí
      </ShimmerButton>
      <Button variant="ghost" href="#how-it-works">
        Xem demo 90s
      </Button>
    </div>
    <SocialProofLogos className="mt-20" />
  </div>

  <BorderBeam />
</section>
```

### 8.3 Sections list (in order)

1. **Hero** (R3F globe + spotlight + shimmer CTA)
2. **Social proof** (logo customers + stats "10K+ backlinks built" animated counter)
3. **How it works** (3 step với `<Timeline>` từ Aceternity)
4. **Feature Bento grid** (6 tiles: AI rewrite, Premium pool, Ethical mode, Auto-find, Anti-detect, Credit system)
5. **Demo video** (Mux player autoplay loop, 90s screencast)
6. **Pricing** (tabs Standard/Premium, cards với `<BorderBeam>` highlight tier Pro)
7. **Compare vs competitors** (table: GSA, Money Robot, RankerX, Snake Backlink Forge)
8. **Testimonials** (3D marquee với 20+ fake reviews — **Note:** khi launch dùng reviews thật, ban đầu có thể "coming soon")
9. **FAQ** (Accordion, 12 câu phổ biến)
10. **Changelog preview** (last 3 updates)
11. **CTA final** (Spotlight background + large shimmer button)
12. **Footer** (multi-column: Product / Resources / Legal / Social)

### 8.4 Blog setup

Dùng `velite` (modern hơn Contentlayer) cho MDX:

```ts
// velite.config.ts
import { defineConfig, defineCollection, s } from 'velite'

const posts = defineCollection({
  name: 'Post',
  pattern: 'blog/**/*.mdx',
  schema: s.object({
    title: s.string().max(99),
    slug: s.slug('global', ['docs', 'tags']),
    date: s.isodate(),
    cover: s.image().optional(),
    description: s.string().max(999).optional(),
    draft: s.boolean().default(false),
    featured: s.boolean().default(false),
    tags: s.array(s.string()).default([]),
    toc: s.toc(),
    metadata: s.metadata(),
    excerpt: s.excerpt(),
    content: s.mdx(),
  }),
})

export default defineConfig({
  root: 'src/content',
  output: { data: '.velite', assets: 'public/static', base: '/static/' },
  collections: { posts },
  mdx: {
    rehypePlugins: [rehypeSlug, rehypeAutolinkHeadings, rehypePrettyCode],
    remarkPlugins: [remarkGfm],
  },
})
```

Seed 5 blog posts:
1. "Tool automation xây backlink 2026: hướng dẫn toàn tập"
2. "Hiểu đúng về backlink chất lượng — DR, DA, trust flow"
3. "10 Google dork tìm site guest post trong niche của bạn"
4. "Ethical SEO vs Black-hat: ranh giới ở đâu?"
5. "Snake Backlink Forge changelog v1.0 — những gì có trong bản đầu tiên"

### 8.5 SEO setup

- `next-seo` hoặc Metadata API native
- Schema.org `Product`, `SoftwareApplication`, `FAQPage`, `Article` cho blog
- `sitemap.xml` generated via `next-sitemap`
- `robots.txt` allow all
- OG images dynamic via `/api/og`

---

## 9. INSTALLER (NSIS, Windows)

### 9.1 `installer/setup.nsi`

```nsis
!include "MUI2.nsh"
!include "x64.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"

Name "Snake Backlink Forge"
OutFile "SnakeBacklinkSetup.exe"
Unicode True
RequestExecutionLevel admin

InstallDir "$PROGRAMFILES64\SnakeBacklinkForge"

!define EXTENSION_ID "aabbccddeeff112233445566778899aa"  ; Will be set from build script
!define UPDATE_XML_URL "https://cdn.snakebacklink.com/ext/updates.xml"

!define MUI_ICON "assets\icon.ico"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_BITMAP "assets\banner.bmp"
!define MUI_WELCOMEFINISHPAGE_BITMAP "assets\welcome.bmp"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENSE.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Vietnamese"
!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetOutPath $INSTDIR
    File "LICENSE.txt"
    File "README.txt"

    ; Write Chrome ExtensionInstallForcelist
    WriteRegStr HKLM "Software\Policies\Google\Chrome\ExtensionInstallForcelist" "1" "${EXTENSION_ID};${UPDATE_XML_URL}"
    ; Write Edge ExtensionInstallForcelist
    WriteRegStr HKLM "Software\Policies\Microsoft\Edge\ExtensionInstallForcelist" "1" "${EXTENSION_ID};${UPDATE_XML_URL}"

    ; Write uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"

    ; Add to Add/Remove Programs
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SnakeBacklinkForge" \
        "DisplayName" "Snake Backlink Forge"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SnakeBacklinkForge" \
        "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SnakeBacklinkForge" \
        "DisplayVersion" "1.0.0"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SnakeBacklinkForge" \
        "Publisher" "Snake Premium Hub"

    ; Prompt user to restart Chrome/Edge
    MessageBox MB_ICONINFORMATION \
        "Cài đặt thành công!$\n$\nHãy đóng và mở lại Chrome hoặc Edge để extension được kích hoạt.$\n$\nSau đó, inbox @SnakeBacklinkBot trên Telegram để nhận API key."
SectionEnd

Section "Uninstall"
    DeleteRegValue HKLM "Software\Policies\Google\Chrome\ExtensionInstallForcelist" "1"
    DeleteRegValue HKLM "Software\Policies\Microsoft\Edge\ExtensionInstallForcelist" "1"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SnakeBacklinkForge"

    Delete "$INSTDIR\uninstall.exe"
    Delete "$INSTDIR\LICENSE.txt"
    Delete "$INSTDIR\README.txt"
    RMDir "$INSTDIR"
SectionEnd
```

### 9.2 Build + sign script

```powershell
# installer/scripts/build.ps1
param(
    [string]$Version = "1.0.0",
    [string]$ExtensionId,
    [string]$CertPath,
    [string]$CertPassword
)

# Generate setup.nsi with EXTENSION_ID from param
(Get-Content setup.nsi) -replace 'aabbccddeeff112233445566778899aa', $ExtensionId | Set-Content setup.generated.nsi

# Build with NSIS
& "C:\Program Files (x86)\NSIS\makensis.exe" /V2 setup.generated.nsi

# Sign
& signtool.exe sign `
    /f $CertPath /p $CertPassword `
    /tr http://timestamp.sectigo.com /td sha256 `
    /fd sha256 `
    /d "Snake Backlink Forge" `
    SnakeBacklinkSetup.exe

# Verify signature
& signtool.exe verify /pa /v SnakeBacklinkSetup.exe

Write-Host "Built SnakeBacklinkSetup.exe version $Version"
```

---

## 10. CRX PACKAGING & UPDATE MANIFEST

### 10.1 `tools/crx-packager/index.mjs`

```javascript
import { ChromeExtension } from 'crx3'
import fs from 'node:fs/promises'

const args = Object.fromEntries(process.argv.slice(2).map(a => a.split('=')))
const { version = '1.0.0', src = '../../apps/extension/dist', out = '../../dist' } = args

const privateKeyPath = './private-key.pem'
const crx = new ChromeExtension({ privateKey: await fs.readFile(privateKeyPath) })

const fileBuffer = await crx.loadContents(src)
const crxBuffer = await crx.pack(fileBuffer)

await fs.writeFile(`${out}/ext-${version}.crx`, crxBuffer)

// Generate updates.xml
const extId = crx.appId
const updatesXml = `<?xml version='1.0' encoding='UTF-8'?>
<gupdate xmlns='http://www.google.com/update2/response' protocol='2.0'>
  <app appid='${extId}'>
    <updatecheck codebase='https://cdn.snakebacklink.com/ext/ext-${version}.crx' version='${version}' />
  </app>
</gupdate>`

await fs.writeFile(`${out}/updates.xml`, updatesXml)
console.log(`Packed ext-${version}.crx with appid=${extId}`)
```

### 10.2 Upload script to R2 via rclone

```bash
# scripts/publish-extension.sh
VERSION=$1
rclone copy ./dist/ext-${VERSION}.crx r2:sbf-cdn/ext/
rclone copy ./dist/updates.xml r2:sbf-cdn/ext/
# Cloudflare purge cache
curl -X POST "https://api.cloudflare.com/client/v4/zones/${CF_ZONE}/purge_cache" \
  -H "Authorization: Bearer ${CF_TOKEN}" \
  -d '{"files":["https://cdn.snakebacklink.com/ext/updates.xml"]}'
```

---

## 11. TESTING STRATEGY

### 11.1 Backend (Go)

- **Unit tests**: mỗi service có `*_test.go`, target ≥ 80% coverage
- **Integration tests**: test containers (testcontainers-go) spin up Postgres + Redis, run full HTTP flow
- **Migration smoke test**: MANDATORY in any phase modifying DB schema. Use testcontainers-go for real Postgres in CI — don't defer runtime test. Lesson: Phase 1 Block C bug layer 1 (duplicate version from `YYYYMMDD_NNN_` prefix collision) + layer 2 (split `.up.sql`/`.down.sql` files incompatible with goose single-file parser) both exposed by deferred runtime test. Single-file goose format is the only supported convention: `YYYYMMDDNNN_name.sql` with `-- +goose Up` and `-- +goose Down` sections separated by directive lines.
- **Load tests**: `k6` scripts cho `/v1/campaign/next` và `/webhooks/sepay` (target p95 < 300ms @ 50 VU)
- **Security tests**:
  - HMAC bypass attempt → must 401
  - Replay attack (same nonce) → must 401
  - Timestamp skew > 60s → must 401
  - Credit race condition (1000 concurrent consume) → consistent balance

### 11.2 Extension (TS)

- **Unit tests**: Vitest cho adapters, signer, state manager
- **E2E tests**: Playwright với extension loaded, test flow:
  1. Install extension
  2. Paste key in options → verify
  3. Create campaign
  4. Start → observe network calls mock server
  5. Check backlink history updated

### 11.3 Landing (Next.js)

- **Unit**: Vitest cho components isolated
- **E2E**: Playwright check all critical paths (landing → pricing → CTA → blog post render)
- **Lighthouse CI**: ≥ 95 Performance, 100 SEO, 100 Accessibility, 100 Best Practices
- **Visual regression**: Percy hoặc Chromatic (optional)

### 11.4 Installer

- **Manual QA matrix**: Windows 10/11 × Chrome/Edge × Admin/Non-admin → document results
- **AV scan**: VirusTotal upload post-sign, expect 0 detections

### 11.5 ClaudeKit test commands

Sau mỗi /ck:cook, luôn chạy `/ck:test` với agent `tester` → đảm bảo:
- `go test ./... -race -cover` cho services/api
- `pnpm test` cho apps/extension, apps/landing
- Code review với agent `code-reviewer` → 0 critical issues mới commit

### 11.6 Lesson: SePay payload field discrepancy

Discovered Phase 10 production E2E test that SePay actual webhook payload uses
gateway field with FULL bank name ('MBBank'), not short code ('MB'). Phase 06
spec initially assumed short code, requiring hotfix 34e52a9.

For future expansion to other Vietnamese banks (Vietcombank, ACB, TPBank, etc.):
- DO NOT trust docs.sepay.vn/banks.html short code mapping for gateway field
- DO test with real webhook payload BEFORE deploying strict matcher
- DO log full payload at INFO level in dev for first 100 transactions per bank
- DO keep gateway match flexible: try fuzzy match before strict equals

Action items adding new bank:
1. Generate fresh test transaction
2. Capture full webhook payload from production logs
3. Verify gateway field exact value (case-sensitive)
4. Update gateway whitelist with verified value
5. Add unit test with captured payload as fixture

This is a 'docs vs reality' bug class. Apply same skepticism to all 3rd-party
webhook integrations.

---

## 12. DEPLOYMENT

### 12.1 Environment variables

**services/api/.env (Fly secrets):**
```
DATABASE_URL=postgres://...
REDIS_URL=redis://...
CLAUDE_BASE_URL=https://r7yyfje.9router.com
CLAUDE_MODEL=cc/claude-sonnet-4-6
TELEGRAM_BOT_TOKEN=...
SEPAY_WEBHOOK_TOKEN=...
SERPAPI_KEY=...
MOZ_ACCESS_ID=...
MOZ_SECRET=...
TWOCAPTCHA_MASTER_KEY=...
CAPSOLVER_MASTER_KEY=...
RESEND_KEY=...
JWT_SECRET=... (admin dashboard phase 2)
ADMIN_TELEGRAM_IDS=123456789,987654321
LOG_LEVEL=info
PORT=8080
ENV=production
```

### 12.2 fly.toml

```toml
app = "snake-backlink-api"
primary_region = "sin"

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "suspend"
  auto_start_machines = true
  min_machines_running = 1
  processes = ["app"]

  [[http_service.checks]]
    interval = "15s"
    timeout = "3s"
    grace_period = "10s"
    method = "GET"
    path = "/health"

[[vm]]
  size = "shared-cpu-2x"
  memory = "1gb"

[mounts]
  source = "sbf_logs"
  destination = "/var/log/sbf"
```

### 12.3 Dockerfile (multi-stage, minimal)

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

### 12.4 CI/CD (GitHub Actions)

`.github/workflows/deploy-api.yml`:
```yaml
name: Deploy API
on:
  push:
    branches: [main]
    paths: ['services/api/**']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: superfly/flyctl-actions/setup-flyctl@master
      - working-directory: services/api
        run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

`.github/workflows/build-extension.yml`:
```yaml
name: Build Extension
on:
  push:
    tags: ['ext-v*']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
        with: { targets: wasm32-unknown-unknown }
      - run: cargo install wasm-pack
      - uses: pnpm/action-setup@v4
      - run: pnpm install
      - working-directory: apps/extension
        run: pnpm build
      - working-directory: tools/crx-packager
        env:
          PRIVATE_KEY_B64: ${{ secrets.CRX_PRIVATE_KEY_B64 }}
        run: |
          echo "$PRIVATE_KEY_B64" | base64 -d > private-key.pem
          node index.mjs version=${GITHUB_REF_NAME#ext-v}
      - name: Upload to R2
        env:
          RCLONE_CONFIG: ${{ secrets.RCLONE_CONFIG }}
        run: |
          echo "$RCLONE_CONFIG" > /tmp/rclone.conf
          rclone --config=/tmp/rclone.conf copy dist/ r2:sbf-cdn/ext/
```

---

## 13. CLAUDEKIT EXECUTION PLAN — 10 PHASES

Mỗi phase đi đủ flow: `/ck:scout → /watzup → /ck:plan --hard → review plan/ → /clear → /ck:cook → /ck:test → code review loop → /ck:git cm`.

**Trước khi bắt đầu phase nào cũng PHẢI:**
1. Đọc lại master prompt (file này)
2. Check `docs/architecture.md` đã cập nhật chưa
3. Run `/ck:scout` để grep codebase hiện có
4. Dùng agent `researcher` nếu phase có dependency bên ngoài cần tra (ví dụ crxjs latest API, wasm-pack config)

### Phase 1 — Foundation (Week 1)

**Scope:**
- Init monorepo: pnpm workspaces + Turborepo
- Create apps/* và services/* stubs
- Set up .editorconfig, .gitignore, linters (golangci-lint, biome hoặc eslint+prettier, stylelint)
- Init Go module, Postgres migrations (goose), sqlc, Fiber skeleton
- Docker Compose cho Postgres + Redis local
- Health endpoints working

**ClaudeKit flow:**
```
/ck:scout           # empty repo
/watzup             # init state
/ck:plan --hard
  → review plan/phase-1.md
/clear
/ck:cook            # agent: fullstack-developer + docs-manager
  - scaffold repo
  - write Makefile targets
  - init go module + fiber
  - write first migration (schema từ section 3)
  - docker-compose.yml
/ck:test            # agent: tester
  - docker compose up → migration run clean
  - curl /health → 200
  - go test runs
  Code review loop until 0 issues
/ck:git cm          # agent: git-manager
  - conventional commit: "feat(repo): scaffold monorepo + foundation"
```

**Deliverables:**
- [ ] Repo skeleton committed
- [ ] `make dev` brings up full local stack
- [ ] Migrations apply cleanly
- [ ] /health, /ready endpoints live

**Agent recommendation:**
- planner (plan)
- researcher (verify latest Go/Fiber/sqlc versions)
- fullstack-developer (implement)
- tester (verify)
- code-reviewer (quality gate)
- docs-manager (update README)
- git-manager (commit)

---

### Phase 2 — Telegram Bot + Wallet + SePay

**Scope:**
- Implement bot package (bot.go, router, commands: start/key/balance/buy/topup/history/download/support/regenkey/ref)
- User service, key service, wallet service
- Ledger atomic operations
- SePay webhook handler
- Trial grant logic
- Message templates VN + EN
- Anti-abuse trial gates
- Admin commands

**ClaudeKit flow:**
```
/ck:scout
  grep: "bot", "telegram"
/watzup
/ck:plan --hard
  → plan/phase-2.md including:
    - FSM state table
    - Message template keys
    - SePay webhook signature verify
    - Race condition analysis for /buy + concurrent webhook
/clear
/ck:cook
  - spawn multiple agents parallel:
    - fullstack-developer-1: bot/ package (commands, keyboards)
    - fullstack-developer-2: service/ (user, key, wallet, ledger)
    - fullstack-developer-3: integration/sepay
    - copywriter: messages.go templates VN bắt tai, EN chuyên nghiệp
/ck:test
  - tester: integration tests cho full flow
    1. New TG user → /start → get key
    2. /balance → 5 standard, 0 premium
    3. Simulate SePay webhook → /buy completed
    4. /history shows transaction
  - security-auditor agent: review webhook signature, race conditions
  Loop until 0 issues
/ck:git cm
```

**Deliverables:**
- [ ] Bot running locally, responding to commands
- [ ] SePay webhook test green
- [ ] Ledger atomic correctness test green
- [ ] VN + EN messages complete

---

### Phase 3 — Extension Skeleton + WASM + Auth

**Scope:**
- Vite + CRXJS + Svelte 5 + TS strict
- Rust crate sbf-wasm, wasm-pack build
- Service worker skeleton + router
- Offscreen doc setup (empty handler)
- Popup + Options Svelte shells
- API client with HMAC signer
- Options page: Key section (paste key → call /v1/me → show balance)
- Load unpacked in Chrome canary → manual QA

**ClaudeKit flow:**
```
/ck:scout
/watzup
/ck:plan --hard
  → consider:
    - MV3 SW lifecycle (idle after 30s, alarms keep-alive)
    - WASM bundling in extension (CSP wasm-unsafe-eval)
    - chrome.storage.local vs sync
/clear
/ck:cook
  agents:
    - fullstack-developer (TS/Svelte)
    - researcher (Rust wasm-bindgen API latest)
/ck:test
  - Vitest unit: signer correctness (compare với Go HMAC)
  - Playwright E2E: load ext → paste dummy key → /v1/me call signed correctly
/ck:git cm
```

**Deliverables:**
- [ ] Extension loads as unpacked, passes Chrome warnings
- [ ] Paste key in options → sees wallet balance
- [ ] WASM signing matches Go verify (crypto test)
- [ ] Service worker survives 5 min idle (via alarm heartbeat)

---

### Phase 4 — Core Campaign Engine (Blog Comment MVP)

**Scope:**
- Campaign CRUD API endpoints + bot commands
- Target schema seed (1000 prebuilt WP blogs for testing)
- Action Planner service
- Content Generator (Claude Sonnet 4.6 integration)
- `/v1/campaign/next` + `/v1/campaign/result` endpoints
- Blog comment adapter in extension (offscreen iframe)
- Safeguard service (daily drip, key daily cap, insufficient credits)
- Domain cooldown enforcement
- Anchor weighted selection
- Full E2E: create campaign → start → build 1 comment on test WP blog → verify DB row

**ClaudeKit flow:**
```
/ck:scout
/watzup
/ck:plan --hard
  → this is the biggest phase, plan must break into sub-tasks:
    - Task A: Campaign CRUD
    - Task B: Target seeding + picker
    - Task C: Content generator
    - Task D: Action planner orchestration
    - Task E: Extension adapter (blog comment)
    - Task F: Result handler + ledger commit
    - Task G: Safeguards
    - Task H: E2E test with local WP instance
  Each sub-task gets its own /ck:cook cycle if needed
/clear
/ck:cook (multiple cycles)
  - Per sub-task, spawn fullstack-developer + tester pair
  - Run debug/fix loops
/ck:test
  - E2E with local docker WP blog
  - Stress test: 20 concurrent jobs same user → ledger consistent
/ck:git cm (per sub-task commit)
```

**Deliverables:**
- [ ] 1 backlink comment successfully posted on local WP via extension
- [ ] Ledger entry created, credit consumed
- [ ] Safeguard prevents 2nd comment same domain in 30d
- [ ] /v1/campaign/next returns action_plan with human delays

---

### Phase 5 — Multi-type Backlink + Captcha + Email Temp

**Scope:**
- Forum profile adapter (vBulletin, phpBB, Discourse)
- Web 2.0 post adapter (Medium API + Blogger via UI automation)
- Directory listing adapter
- Captcha proxy endpoint + 2captcha/CapSolver client
- Email temp integration (mail.tm API) for web 2.0 account registration
- BYOK vs Credits captcha logic
- Platform auto-detection in content script (pre-scan target page)

**ClaudeKit flow:**
```
/ck:plan --hard (include all 4 platforms)
/ck:cook
  - Agent researcher: check latest Medium API deprecation, Blogger access
  - fullstack-developer-extension: 4 adapter implementations
  - fullstack-developer-backend: action planner branches per type
/ck:test: each type run E2E with test sandbox
```

**Deliverables:**
- [ ] All 4 backlink types working end-to-end
- [ ] Captcha solve integrated
- [ ] Email temp flow for web 2.0 account create

---

### Phase 6 — Target Finder System

**Scope:**
- Prebuilt crawler worker (cron weekly)
- SerpAPI integration
- Moz DA/DR client
- Auto-find endpoint `/v1/finder/search`
- Custom import endpoint (CSV parse + validation)
- Platform detection (request URL, parse HTML signature)
- Niche classification (embedding via Sonnet) — store niche_tags
- Blocklist enforcement (gov/edu + brand list)

**Deliverables:**
- [ ] 5000+ targets seeded after 1 crawler run
- [ ] Targets distributed premium vs standard based on DR
- [ ] Auto-find returns results consuming 3 credit

---

### Phase 7 — Obfuscation + WASM Hardening + Installer

**Scope:**
- Finalize obfuscator.config with all flags
- Harden WASM: mangle names, strip debug symbols, enable `wasm-opt -Oz`
- Build reproducible CRX (fixed version salt)
- NSIS installer with signed EV cert
- CI pipeline builds + signs + uploads R2
- Manual QA matrix Windows

**Agent recommendation:** `security-auditor` reviews obfuscation actually hides meaningful logic (run `webpack-bundle-analyzer` on final bundle, confirm no readable strings).

---

### Phase 8 — Anti-abuse + Quality + Anomaly Detection

**Scope:**
- Content similarity (TF-IDF cosine, can use simple implementation — no need embeddings API initially)
- Niche relevance filter (compare target niche_tags vs campaign niche_keywords)
- Ethical mode (default ON) rules applied
- Anomaly detector worker (scan audit_log + detect geo anomaly)
- Alert channel: send Telegram notification to user + admin

**Deliverables:**
- [ ] All safeguards from ADR 1.5 active
- [ ] Ethical mode toggle respected
- [ ] Anomaly test: simulate 3-country requests in 1h → auto-suspend

---

### Phase 9 — Landing Page + Blog + Docs

**Scope:**
- Next.js 15 project setup with design system
- All 12 landing sections
- Pricing page with calculator
- Features page
- Blog with 5 seed MDX posts
- Docs with 8 pages (install, first campaign, credits explained, troubleshooting, API for advanced, ethical guide, changelog, FAQ)
- Legal pages (ToS, Privacy, Refund)
- OG image generator
- Vercel deploy + custom domain
- Lighthouse 95+

**Agent recommendation:**
- `copywriter`: viết toàn bộ content VN + EN, tone chuyên nghiệp nhưng approachable
- `fullstack-developer`: implement components
- Use Figma MCP đầu tiên cho design mockup nếu mày có time, else AI-generated via v0.dev + tweak

---

### Phase 10 — Launch Readiness + Monitoring

**Scope:**
- Sentry integration (API + extension + landing)
- Better Stack log aggregation
- Grafana Cloud dashboard (Fly metrics)
- Runbooks: incident response, key rotation, DB restore, deploy rollback
- Admin dashboard MVP (Telegram commands cho: user list, suspend user, grant credit manual, view metrics)
- Load test production-like with k6 (100 concurrent users)
- Beta test với 3-5 khách thân quen, collect feedback, iterate
- Launch announcement
- Referral program activation

**Deliverables:**
- [ ] Production hardened
- [ ] First 3 paying beta users
- [ ] Monitoring alerting working
- [ ] Public launch on Telegram/Facebook/Zalo groups

---

## 14. AGENT ASSIGNMENT MATRIX

| Task type | Primary agent | Supporting agents |
|---|---|---|
| Architecture plan | planner | researcher |
| External API integration (SerpAPI, Moz, SePay) | researcher | fullstack-developer |
| Go backend services | fullstack-developer | tester, code-reviewer |
| Extension TypeScript | fullstack-developer | tester |
| Rust WASM | fullstack-developer (Rust specialist) | researcher, tester |
| Frontend Svelte/Next.js | fullstack-developer (UI specialist) | copywriter |
| Copy VN/EN | copywriter | - |
| Security review | security-auditor (custom role) | code-reviewer |
| Obfuscation review | security-auditor | researcher |
| DB schema + migrations | fullstack-developer | code-reviewer |
| Test writing | tester | debugger (if failing) |
| Code review loop | code-reviewer | debugger |
| Commits | git-manager | - |
| Docs | docs-manager | copywriter |

**Parallel agent usage (spam multi-agent khi cần):**

Ví dụ Phase 4 Task A-H có thể spam parallel:
```
spawn fullstack-developer-1 on Task A (Campaign CRUD backend)
spawn fullstack-developer-2 on Task B (Target seed + picker)
spawn fullstack-developer-3 on Task C (Content generator)
spawn fullstack-developer-4 on Task E (Extension adapter) — parallel vì ext repo riêng
each with their own tester before merging
```

Claude Code pool tasks trong `TodoWrite` per phase, gate parallel/sequential đúng cách.

---

## 15. QUALITY GATES & COMMIT DISCIPLINE

Đây là rule cứng — **KHÔNG commit nếu vi phạm:**

### 15.1 Pre-commit

- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean
- [ ] `go test ./... -race` pass
- [ ] `pnpm lint` clean
- [ ] `pnpm typecheck` clean
- [ ] `pnpm test` pass
- [ ] No `console.log`, `fmt.Println` for debug left behind
- [ ] No hardcoded secrets (gitleaks scan pass)
- [ ] Migration down reverses up cleanly

### 15.2 Pre-PR

- [ ] Code review agent run → 0 critical, 0 high issues
- [ ] Coverage report ≥ 80% Go, ≥ 70% TS
- [ ] E2E test covering new feature added
- [ ] docs updated (architecture.md, api-contract.md)
- [ ] CHANGELOG.md entry added

### 15.3 Commit message convention

Conventional Commits. Prefix với module:
```
feat(bot): add /regenkey command
fix(api): prevent double credit consume on race
chore(ci): upgrade Go to 1.23.2
docs(architecture): document safeguard rules
test(wallet): add concurrent consume test
refactor(adapter): extract humanType helper to base class
```

### 15.4 Branch strategy

- `main` — production
- `dev` — integration
- `feat/*` — features (per task from plan)
- `fix/*`, `chore/*`, `docs/*`

**NEVER commit directly to `dev` or `main`.** Always branch từ `dev`, PR merge.

---

## 16. SECURITY & COMPLIANCE CHECKLIST

### 16.1 Threat model

| Threat | Mitigation |
|---|---|
| API key leak (Telegram chat screenshot) | Hash in DB, display masked, easy /regenkey |
| Replay attack | Nonce cache + timestamp window |
| Key bruteforce | Rate limit + IP fingerprint |
| WASM reverse engineering | Best-effort obfuscation, critical secret derive from ephemeral input |
| Extension decompile | Server-heavy architecture; ext alone is useless without server |
| Server credential leak | Fly secrets, role-separated DB users, no secret in repo |
| SQL injection | sqlc parameterized queries, never raw SQL interpolation |
| XSS in options/popup | Svelte auto-escapes, CSP restrictive |
| CSRF on webhook | Bearer token verify |
| Malicious target injection (user imports bad URL) | URL validation, domain blocklist |
| Anomaly (account takeover) | Geo/country anomaly auto-suspend |
| Credit double-spend | Atomic `consume_credits()` stored proc with CHECK constraint |
| SePay webhook spoofing | Bearer token + IP allowlist |
| Telegram bot spoofing | `secret_token` on bot webhook (if later switch from long-poll) |

### 16.2 Data retention

- Audit log: retain 365 days, archive to S3 Glacier-like
- Jobs: retain 180 days, older soft-deleted
- User can request data export/deletion via `/support` → manual (Phase 1) / automated (Phase 2)

### 16.3 Compliance

- **Vietnam**: comply với Nghị định 13/2023 về bảo vệ dữ liệu cá nhân. Privacy Policy phải disclose data collection.
- **GDPR** (international users): right to delete, right to export. Phase 2.
- **Payment**: SePay compliant với quy định ngân hàng VN. Không tự xử lý thẻ credit.

### 16.4 Ethical disclaimer (MUST appear in ToS + Docs)

> **Snake Backlink Forge là công cụ automation dành cho các chuyên gia SEO hiểu rõ rủi ro của link building grey-hat. Người dùng chịu trách nhiệm về chất lượng backlink xây dựng và rủi ro Google penalty. Ethical Mode được bật mặc định để giảm rủi ro, không được tắt trên site chính/money site trừ khi bạn hiểu rõ hậu quả. Snake Premium Hub không chịu trách nhiệm cho bất kỳ tổn thất ranking, deindex, hay penalty nào phát sinh từ việc sử dụng tool này.**

---

## 17. COST MODEL (Fixed infra per month)

| Item | Est. | Note |
|---|---|---|
| Domain snakebacklink.com | $1/tháng | Annual $10 |
| Vercel Hobby | $0 | Hobby đủ initial |
| Fly.io (2x shared-cpu-2x + Postgres + Redis) | $15-25 | Scale up khi cần |
| Cloudflare R2 | $0 | Free tier |
| Code signing cert | $9/tháng | Sectigo OV annual $100 |
| SerpAPI (5K searches/mo) | $50 | Finder calls |
| Moz Pro API | $79 | DR/DA checks. Alternative: free 10K/mo tier |
| Resend email | $0 | 3K free tier đủ |
| Sentry (Dev plan) | $0 | 5K errors/mo |
| Better Stack Logs | $0 | 1GB free |
| **Total fixed** | **~$75-85/tháng** | For up to 100 active users |

Variable costs (per-user):
- Claude API (AI rewrite): ~$0.003/backlink (Sonnet input 500t + output 150t @ 9Router pass-through)
- 2captcha (if serving from Snake pool): ~$0.003/captcha solve
- SerpAPI overflow: $0.005/query after quota

**Unit economics:**
- Standard Pro package 200cr @ 329K VND = $13.16
- Cost to fulfill 200 backlinks: ~$0.60 AI + $0.30 captcha (avg 50% require) = $0.90
- Gross margin: ($13.16 - $0.90) / $13.16 = **93%** 🔥

---

## 18. SUCCESS CRITERIA (for go-live)

**Before public launch, ALL must be green:**

- [ ] Full E2E test green cho cả 4 backlink types trên real sandbox targets
- [ ] 100 concurrent users k6 load test: p95 `/v1/campaign/next` < 500ms, 0 errors
- [ ] 24h soak test: no memory leak, no goroutine leak, no connection pool exhaustion
- [ ] Security audit: no HIGH/CRITICAL from agent `security-auditor`
- [ ] Installer signs + installs cleanly on Windows 10/11 with Chrome + Edge
- [ ] Extension passes Chrome "Manifest v3 compliance" (no remote code, no eval)
- [ ] Landing page Lighthouse ≥ 95 all categories
- [ ] Docs complete: install guide, first campaign guide, credits explained, troubleshooting
- [ ] Legal pages: ToS, Privacy, Refund Policy published
- [ ] Telegram bot response time p95 < 2s for all commands
- [ ] SePay webhook integration tested with real bank transfer
- [ ] Monitoring: Sentry receives errors, Better Stack aggregates logs, Fly metrics visible
- [ ] Runbook: incident response, key rotation, DB restore documented
- [ ] Beta: 3-5 paying users successful full flow (mua credit → cài ext → chạy campaign → nhận backlink)
- [ ] Refund process tested (manual grant reverse via admin command)
- [ ] Referral tracking working end-to-end

---

## 19. POST-LAUNCH ROADMAP (Phase 11+)

Phase 11: i18n EN complete + LemonSqueezy + global launch
Phase 12: Web dashboard (optional, user analytics deep dive)
Phase 13: Tier 2 / Tier 3 scheduler (backlink to backlinks, pyramids)
Phase 14: API public (headless mode for agencies)
Phase 15: Chrome Web Store unlisted listing (optional, for UX improvement)
Phase 16: Mobile companion app (view campaign progress)
Phase 17: Affiliate partner program
Phase 18: Enterprise plan (custom quotas, SLA)

---

## 20. FINAL INSTRUCTIONS FOR CLAUDE CODE

### 20.1 Session kick-off ritual

Bắt đầu mỗi session mới:
1. `cat SNAKE_BACKLINK_FORGE_MASTER_PROMPT.md | head -300` để re-prime context
2. `/watzup` để check state current
3. `git status` + `git log --oneline -10`
4. Đọc `plan/` folder nếu có plan đang dở
5. Hỏi: phase nào đang làm? sub-task nào next?

### 20.2 Quality principles

- **Prefer boring tech.** Go + Postgres + Redis là boring và chính xác là cái mày cần. Không add new tech without strong justification.
- **Explicit > implicit.** Function params đầy đủ, không global state. Config qua env, không magic default hardcoded in code.
- **Fail loud, fail fast.** Errors phải return early với context. Không silent catch.
- **Tests are specs.** Write test first cho bất kỳ service method nào. Not TDD religious, but tests-before-commit.
- **Delete > comment out.** Git history còn code cũ, không cần comment out block 50 lines.
- **Security by default.** Mọi endpoint DEFAULT authenticated. Phải explicit mark `.Public()` để bypass.

### 20.3 Anti-patterns to reject

- ❌ Storing plaintext API key in DB
- ❌ Allowing direct SQL string interpolation
- ❌ Using `any` in TypeScript (except controlled boundaries)
- ❌ Ignoring Go errors (`_ = err`)
- ❌ Blocking service worker with sync operations
- ❌ Hardcoded VN text — must go through i18n map
- ❌ Adding CPU-heavy operations to SePay webhook handler (respond 200 fast, queue work)
- ❌ Calling Claude API in request path without timeout + fallback
- ❌ Trusting client-supplied credit delta

### 20.4 When in doubt

- Re-read this master prompt
- Check ADR table (Section 1.2) — locked decisions
- Ask in plan/ file for clarification before implementing
- Use `researcher` agent to verify external API behavior
- Write failing test first to lock the spec

### 20.5 Status reporting

End of each phase, update `docs/progress.md`:
```
## Phase N — <name>
- Started: YYYY-MM-DD
- Completed: YYYY-MM-DD
- Commits: <list>
- Tests added: <count>
- Coverage delta: +X%
- Notable decisions: ...
- Known issues deferred: ...
```

---

## APPENDIX A — Glossary

| Term | Meaning |
|---|---|
| **Backlink** | Link từ site khác trỏ về money site của user |
| **Money site** | Site user muốn boost SEO |
| **Pool** | Premium (DR 40+) hoặc Standard (DR 0-39) |
| **Credit** | 1 successful backlink consumes 1 credit |
| **Campaign** | Nhóm backlink build cho 1 money site cụ thể |
| **Target** | Site đích để xây link (blog, forum, web 2.0, directory) |
| **Anchor** | Text hiển thị của link |
| **Ethical Mode** | Default safeguard profile, Premium + low daily limit |
| **BYOK** | Bring Your Own Key — user dùng 2captcha key của họ |
| **Drip feed** | Build link tần suất thấp đều đặn, tránh spike |
| **ADR** | Architecture Decision Record |
| **HWID** | Hardware ID (không dùng trong design này) |
| **MV3** | Chrome Manifest V3 |
| **CRX** | Chrome Extension binary package |
| **NSIS** | Nullsoft Scriptable Install System (Windows installer) |
| **SePay** | VN payment webhook service matching bank transfers |

---

## APPENDIX B — References

- Chrome MV3 docs: https://developer.chrome.com/docs/extensions/mv3/
- Offscreen API: https://developer.chrome.com/docs/extensions/reference/offscreen/
- ExtensionInstallForcelist policy: https://chromeenterprise.google/policies/#ExtensionInstallForcelist
- Rust wasm-bindgen: https://rustwasm.github.io/wasm-bindgen/
- Fiber v2: https://docs.gofiber.io/
- sqlc: https://docs.sqlc.dev/
- Fly.io: https://fly.io/docs/
- SePay webhook: https://sepay.vn/docs
- MagicUI: https://magicui.design/docs
- Aceternity: https://ui.aceternity.com/
- shadcn/ui: https://ui.shadcn.com/
- Velite: https://velite.js.org/

---

## APPENDIX C — PROMPT LIBRARY (for Claude Sonnet calls in backend)

### C.1 Blog comment content generator

```
SYSTEM:
You are a professional SEO content writer generating blog comments that look authentic.

CONSTRAINTS (strict):
- 40-120 words
- Match blog's tone inferred from URL/niche
- Engage with post topic specifically (quote point, ask question)
- NEVER include anchor URL in comment body
- Language: {{.Language}}
- Output JSON: {"name": "...", "email": "firstname.lastname@gmail.com", "comment": "..."}
- Names: plausible {{.Language}}-sounding, not celebrity or brand
- NO clichés: "great post", "thanks for sharing", "very informative"
- Include 1 specific detail showing you "read" the post

USER:
Target URL: {{.URL}}
Inferred niche: {{.Niche}}
Anchor-equivalent display name (website field only, not comment): {{.AnchorText}}
```

### C.2 Forum profile bio

```
SYSTEM:
Generate a forum profile bio for a new user signing up to participate in {{.Niche}} community.

CONSTRAINTS:
- Bio: 2-4 sentences
- Signature: 1 line with website link placeholder {{.AnchorText}}
- Plausible interest overlap with {{.Niche}}
- Avoid mentioning job titles or companies (reduce spam signal)
- Language: {{.Language}}
- Output JSON: {"username": "...", "bio": "...", "signature": "..."}
```

### C.3 Web 2.0 post draft

```
SYSTEM:
Generate a blog post draft for web 2.0 platform (Medium/Blogger/Tumblr).

CONSTRAINTS:
- Title: 40-70 characters, clickable but not clickbait
- Body: 400-700 words, 3-4 paragraphs with H2 subheadings
- Include 1 contextual mention of {{.AnchorText}} with hyperlink to {{.MoneySiteURL}}
- Anchor placement: 60% of time in body paragraph 2-3, 40% at end CTA
- Tone: informative, slightly casual
- Output JSON: {"title": "...", "body_markdown": "...", "tags": ["...", "..."]}
```

### C.4 Niche classification

```
SYSTEM:
Classify given URL content into niche tags.

TASK:
Given URL + scraped HTML snippet, output 3-7 lowercase tags describing topic.

Examples:
- crypto news site → ["crypto", "blockchain", "finance", "trading"]
- vegan recipes blog → ["food", "vegan", "recipes", "health", "cooking"]

Output JSON array only.
```

---

**END OF MASTER PROMPT v1.0**

Khi Claude Code bắt đầu implement, ship file này như root-level doc của repo tại `docs/MASTER_PROMPT.md`, sau đó commit ban đầu. Mọi phase tương lai reference back đến doc này cho quyết định đã lock.

Good luck — let's ship it. 🐍
