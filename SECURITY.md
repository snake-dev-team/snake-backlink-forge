# Security Policy

## Reporting a vulnerability

Report security issues to: **security@snakebacklink.com**
(placeholder — update when domain provisioned in Phase 10)

For faster response: `/support` in Telegram @SnakeBacklinkBot

**Do NOT** file public GitHub issues for security bugs. This exposes users before a fix is ready.

Include in your report:
- Description of the vulnerability
- Steps to reproduce
- Affected component: API / extension / bot / landing / installer
- Suggested mitigation (optional)

Response SLA: acknowledge within 48h. Fix window: critical < 7 days, high < 30 days, medium < 90 days.

## Supported versions

Phase 1 (Foundation). Security patches applied to `main` HEAD only until v1.0 release.
No backport policy pre-1.0.

## Responsible disclosure

We follow coordinated disclosure: 90-day embargo from report date before public disclosure,
unless the vulnerability is already being actively exploited. We do not pursue legal action
against good-faith security researchers.

## Scope

In scope:
- Go API (services/api) — auth, credit logic, rate limiting, HMAC verification
- Telegram bot — payment webhook, key issuance, command handlers
- Browser extension — WASM signing, content scripts, service worker
- Installer (Phase 7) — code signing, update mechanism

Out of scope:
- The link-building targets themselves (user's own SEO strategy)
- Third-party services (SePay, 2captcha, SerpAPI, Fly.io infrastructure)
- Social engineering attacks against Snake Premium Hub staff

## Threat model reference

See `docs/MASTER_PROMPT.md` §16.1 for the full threat table. Key mitigations:

| Threat | Mitigation |
|---|---|
| API key leak | SHA256 hash in DB, masked display, `/regenkey` rotation |
| Replay attack | Nonce cache (Redis 5-min) + timestamp window +-60s |
| WASM reverse engineering | Server-heavy architecture; ext alone is inert without valid key |
| Extension decompile | Critical secrets server-side only; WASM obfuscation best-effort |
| SQL injection | sqlc parameterized queries only; no raw string concatenation |
| XSS | Svelte auto-escape + restrictive CSP headers |
| Credit double-spend | Atomic `consume_credits()` stored proc with CHECK constraints |
| SePay webhook spoofing | Bearer token verification + IP allowlist |
| DDoS | Redis sliding rate limit: 250 req/min/key, 100 req/min/IP |
| Anomaly / account takeover | >3 country codes/24h -> auto-suspend + Telegram notify |

## Data handling

- Audit log retention: 365 days (immutable PostgreSQL table)
- Job history retention: 180 days (soft-delete after)
- User data export/deletion: manual via `/support` Phase 1; automated Phase 2+
- Secrets: Fly secrets for prod; `.env.local` for dev; never committed to git
- Compliance: VN Nghi dinh 13/2023; GDPR considerations deferred to Phase 2

## Contact

- Security email: security@snakebacklink.com
- Telegram: @SnakeBacklinkBot `/support`
