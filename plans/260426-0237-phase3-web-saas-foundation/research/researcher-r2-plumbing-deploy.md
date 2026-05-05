# R2 — Plumbing & Deploy Research (Phase 3 Web App)

**Date:** 2026-04-26
**Scope:** Topics 3, 5, 6, 7 — TS gen, auth, Vercel deploy, WP REST API
**Backend:** Go Fiber @ snake-backlink-api.fly.dev (no OpenAPI spec yet — confirmed via Glob)

---

## TL;DR

- **Topic 3:** **Hey API (`@hey-api/openapi-ts`)** — TS-first, plugin arch, native TanStack Query / Zod plugins, beats orval+openapi-ts old guard. Pair with `swaggo/swag` on Go side.
- **Topic 5:** **Route Handlers (proxy) + `cookies()` + lightweight `proxy.ts`** — NO next-auth (overkill for single-key bearer). httpOnly cookie holds raw `sbf_*` key, proxy injects `Authorization` header.
- **Topic 6:** **Vercel Pro $20/mo mandatory** — Hobby forbids commercial use (TOS violation). Cloudflare DNS-only (gray cloud OFF) → CNAME to `cname.vercel-dns.com`. ~$20/mo at 10k MAU.
- **Topic 7:** **Application Passwords (WP 5.6+) + Basic Auth** — `Authorization: Basic base64(user:app_pwd)`. Validate via `GET /wp-json/wp/v2/users/me` before save. Defer OAuth2 to v2.

---

## Topic 3 — OpenAPI TS Gen: **Hey API wins**

### Decision matrix

| Tool | Type safety | Runtime validation | Tree-shaking | TanStack Query | Maturity | Verdict |
|------|-------------|--------------------|--------------|----------------|----------|---------|
| **openapi-typescript** | Types only | None (manual) | N/A (types only) | N/A | High (3M dl/wk) | Too thin — no client gen |
| **orval** | Good | Zod plugin | OK | Yes (custom hooks) | High | Default = custom hooks (heavy) |
| **Stainless** | Excellent | Built-in | Excellent | Yes | Paid | Overkill, $$ — REJECT |
| **Hey API** | Excellent | Zod/Valibot plugin | Best (options-pattern) | Native plugin | Newer but stable | **WINNER** |

### Why Hey API

- Plugin architecture: `@hey-api/client-fetch` + `@hey-api/schemas` (Zod) + `@tanstack/react-query` plugin all opt-in.
- Options-based output (no custom hooks bloat) → smaller bundle, idiomatic with TanStack Query v5.
- Spiritual successor to abandoned `openapi-typescript-codegen` — same DNA, modern.
- Free, open source (Apache 2.0).
- Active 2026 dev — doesn't carry orval's React-hooks-by-default legacy.

### Risks

- Younger than orval — fewer SO answers, but docs are good.
- Plugin churn possible. Pin versions in `package.json`.

### Go side: spec generation

We have **no OpenAPI spec** in `E:\tool_backlink` yet (confirmed via Glob). Two options:

1. **`swaggo/swag`** (recommended) — annotate Go handlers with `// @Summary`, `// @Param`, etc. Run `swag init` → emits `docs/swagger.json`. Convert to OpenAPI 3.0 via `swagger2openapi`.
2. **Hand-written `openapi.yaml`** in `internal/api/openapi.yaml` — lower friction for ~15 endpoints, full control. **Pick this if SBF API surface stays small.**

**Recommendation:** Hand-write `openapi.yaml` for Phase 3. Phase 2 has ~10-15 routes — annotation overhead > benefit. Switch to `swag` only when surface > 30 endpoints.

### Install snippet (frontend)

```bash
# In Next.js project
pnpm add -D @hey-api/openapi-ts
pnpm add @hey-api/client-fetch @tanstack/react-query

# hey-api.config.ts
export default {
  input: '../backend/openapi.yaml',  // path to Go spec
  output: 'src/lib/api/generated',
  plugins: [
    '@hey-api/client-fetch',
    '@hey-api/schemas',                  // emits Zod schemas
    '@tanstack/react-query',
  ],
}

# package.json script
"gen:api": "openapi-ts"
```

Sources: [Hey API docs](https://heyapi.dev/openapi-ts/integrations), [Saschb2b 2026 codegen blog](https://www.saschb2b.com/blog/typesafe-api-codegen-2026), [orval](https://orval.dev/), [npm trends](https://npmtrends.com/openapi-typescript-vs-orval-vs-swagger-typescript-api-generator).

---

## Topic 5 — Auth: **Route Handler proxy + httpOnly cookie**

### Verdict

**REJECT next-auth/auth.js v5.** Reasons:
- Designed for OAuth/credentials/magic-link flows. We have a **single bearer key paste** flow.
- Adds JWT signing, session table, providers config — none needed.
- Runtime cost + learning curve > value.

**REJECT pure middleware-based auth.** Edge runtime can't decrypt session if we ever add encryption. Use it ONLY for redirect gating.

**ACCEPT:**
- `proxy.ts` (Next 15 renamed `middleware.ts`) → redirect-only gate.
- Route Handlers (`app/api/proxy/[...path]/route.ts`) → forward to Go backend with `Authorization: Bearer <key>`.
- Server Actions for **only** `/login` (key paste) and `/logout` (cookie clear).
- `cookies()` API for httpOnly cookie set/read.

> **Note:** Next.js 15 renamed `middleware.ts` → `proxy.ts`. Keep both names in mind during migration; `middleware.ts` still works in 15.x but is deprecated.

### Flow

```
[user] paste sbf_xxx → POST /api/auth/login (Server Action)
       ↓ (Go: GET /api/me to validate key)
       ↓ on 200, set httpOnly cookie `sbf_key`
       ↓
[user] navigates / → proxy.ts checks cookie exists → if not, redirect /login
       ↓
[user] data fetch → fetch('/api/proxy/wordpress/sites') (Route Handler)
       → reads cookie → adds Authorization: Bearer sbf_xxx → forwards to Fly.io
```

### CSRF, expiry, rotation

- **CSRF:** `SameSite=Lax` cookie + state-changing routes use Server Actions (Next.js 15 has built-in CSRF for Server Actions via origin check).
- **Expiry:** SBF API keys are long-lived (issued via Telegram). Cookie `Max-Age = 30 days`, refresh on activity.
- **Key rotation:** When user regens key in Telegram bot, old key 401s → catch in proxy → redirect `/login` with toast "session expired".
- **Server-side fetch:** All proxy routes run on Node runtime (NOT edge — edge can't access full `cookies()` ergonomics + Node fetch is fine for our scale).

### `proxy.ts` skeleton (~25 lines)

```typescript
// proxy.ts (root)
import { NextRequest, NextResponse } from 'next/server'

const PUBLIC = ['/login', '/api/auth/login', '/_next', '/favicon.ico']

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl
  if (PUBLIC.some(p => pathname.startsWith(p))) return NextResponse.next()

  const key = req.cookies.get('sbf_key')?.value
  if (!key) {
    const url = req.nextUrl.clone()
    url.pathname = '/login'
    url.searchParams.set('next', pathname)
    return NextResponse.redirect(url)
  }
  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|.*\\..*).*)'],
}
```

### Login Server Action

```typescript
// app/login/actions.ts
'use server'
import { cookies } from 'next/headers'

export async function login(formData: FormData) {
  const key = formData.get('apiKey')?.toString().trim()
  if (!key?.startsWith('sbf_')) return { error: 'Invalid key format' }

  // Validate against Go backend
  const r = await fetch(`${process.env.SBF_API_URL}/api/me`, {
    headers: { Authorization: `Bearer ${key}` },
    cache: 'no-store',
  })
  if (!r.ok) return { error: 'Invalid or expired key' }

  ;(await cookies()).set('sbf_key', key, {
    httpOnly: true,
    secure: true,
    sameSite: 'lax',
    maxAge: 60 * 60 * 24 * 30,
    path: '/',
  })
  return { ok: true }
}
```

### Proxy Route Handler

```typescript
// app/api/proxy/[...path]/route.ts
import { cookies } from 'next/headers'

const BACKEND = process.env.SBF_API_URL!

async function forward(req: Request, params: { path: string[] }) {
  const key = (await cookies()).get('sbf_key')?.value
  if (!key) return new Response('Unauthorized', { status: 401 })

  const url = `${BACKEND}/${params.path.join('/')}${new URL(req.url).search}`
  const r = await fetch(url, {
    method: req.method,
    headers: {
      Authorization: `Bearer ${key}`,
      'Content-Type': req.headers.get('content-type') ?? 'application/json',
    },
    body: ['GET', 'HEAD'].includes(req.method) ? undefined : await req.text(),
    cache: 'no-store',
  })
  return new Response(r.body, { status: r.status, headers: r.headers })
}

export const GET = forward
export const POST = forward
export const PATCH = forward
export const DELETE = forward
```

Sources: [Next.js auth guide](https://nextjs.org/docs/app/guides/authentication), [Next.js cookies()](https://nextjs.org/docs/app/api-reference/functions/cookies), [Next.js proxy.js](https://nextjs.org/docs/app/api-reference/file-conventions/proxy), [Authgear JWT Next.js](https://www.authgear.com/post/nextjs-jwt-authentication), [LogRocket auth libs 2026](https://blog.logrocket.com/best-auth-library-nextjs-2026/).

---

## Topic 6 — Vercel + Cloudflare DNS

### Tier choice: **Pro mandatory ($20/mo)**

| Limit | Hobby | Pro | SBF need (10k MAU) |
|-------|-------|-----|--------------------|
| Bandwidth | 100 GB | 1 TB | ~50-200 GB → fits both |
| Function invocations | 1M | unlimited (metered) | ~3M → **Hobby breaks** |
| Edge requests | included | 10M | ~5M → fits Pro |
| Commercial use | **FORBIDDEN** | yes | **paid SaaS = must Pro** |
| Cost overage | hard stop | $0.15/GB | manageable |

**Hobby = TOS violation for SaaS.** Vercel suspends accounts. Pro from day 1.

### Cost @ 10k MAU

- $20 base (1 seat).
- ~3-5M function calls bundled.
- ~150 GB bandwidth bundled.
- Estimated: **$20-30/mo** (rare overage). Add Fly.io ($5-15) + Postgres ($0-15) = total infra ~$40-60/mo at 10k MAU. Healthy margin if pricing tier ≥ $5/mo per user.

### Env vars

- `NEXT_PUBLIC_*` → bundled into client JS, public. Use ONLY for: site URL, Sentry DSN (public DSN), feature flags.
- Server-only (no prefix) → only available in Server Components, Server Actions, Route Handlers. Use for: `SBF_API_URL`, any secrets, telemetry keys.
- **Never** put `sbf_*` keys in env vars — those are per-user, live in cookies.
- Set per env: Production / Preview / Development separately in Vercel dashboard.

### ISR + edge cache for landing

- Marketing pages (`/`, `/pricing`, `/blog/*`) → `export const revalidate = 3600` (1h ISR) → served from Vercel edge.
- App pages (`/dashboard`, `/sites`) → `dynamic = 'force-dynamic'` (auth-gated, no cache).

### DNS chain (Cloudflare → Vercel)

We own `snakebacklink.com` on Cloudflare.

**Vercel side:**
1. Project → Settings → Domains → Add `snakebacklink.com` and `www.snakebacklink.com`.
2. Click Edit → "Manual setup". Vercel shows: CNAME target `cname.vercel-dns.com` (or per-project `xxx.vercel-dns-xxx.com`).

**Cloudflare side:**
1. DNS → Records.
2. Add `CNAME` record:
   - Name: `@` (apex — works via Cloudflare CNAME flattening)
   - Target: `cname.vercel-dns.com`
   - **Proxy: DNS only (GRAY CLOUD).** Orange cloud = Cloudflare proxy intercepts TLS → Vercel SSL issuance fails.
3. Add `CNAME` record for `www` → same target, gray cloud.
4. Wait ~5-15 min for Vercel SSL cert provisioning.

**Why gray cloud:** Vercel issues Let's Encrypt certs via HTTP-01 challenge. Cloudflare orange-cloud terminates TLS at Cloudflare, breaking the chain. Trade-off: lose Cloudflare DDoS/WAF for the apex. Mitigation: enable Vercel firewall (Pro included).

### Vercel ↔ Fly.io CORS

Browser does NOT call Fly directly — all fetches go through Next.js Route Handlers (same origin). **Zero CORS issues.**

If we ever expose Fly directly to browser (e.g., for direct WP webhook callbacks):
- Go Fiber: `app.Use(cors.New(cors.Config{ AllowOrigins: "https://snakebacklink.com", AllowCredentials: true }))`
- Match origin exactly, not `*`.

### Preview deployments

- Auto on every PR + push to non-main branches.
- Each gets unique URL: `sbf-pr-42-org.vercel.app`.
- Preview env vars: set `SBF_API_URL=https://snake-backlink-api.fly.dev` (point to prod backend) OR spin staging Fly app.
- **Recommendation:** Use prod backend for previews — read-only routes safe, mutations gated by per-user API key. Saves staging cost.

Sources: [Vercel pricing](https://vercel.com/pricing), [Vercel pricing docs](https://vercel.com/docs/pricing), [DeployHandbook breakdown](https://deployhandbook.com/pricing/vercel), [Cloudflare+Vercel 2026 setup](https://dev.to/getcraftly/custom-domains-for-nextjs-the-cloudflare-vercel-setup-that-works-in-2026-2a3i), [Vercel custom domains](https://vercel.com/docs/domains/working-with-domains/add-a-domain).

---

## Topic 7 — WordPress REST API + Application Password

### Auth method: **Application Passwords (WP 5.6+, built-in)**

- Header: `Authorization: Basic <base64(username:app_password)>`
- App password format: `xxxx xxxx xxxx xxxx xxxx xxxx` (24 chars, 4-char groups, spaces optional).
- HTTPS REQUIRED — WP refuses Basic Auth over HTTP.
- Defer OAuth2 to v2 — plugin-based, not core, complexity unjustified.

### Endpoints (Phase 3 minimum)

| Method | Path | Use case |
|--------|------|----------|
| `GET` | `/wp-json/wp/v2/users/me` | **Validation ping** — auth check before save credentials |
| `POST` | `/wp-json/wp/v2/posts` | Publish article (status: `publish` / `draft` / `future`) |
| `POST` | `/wp-json/wp/v2/media` | Upload featured image (multipart/form-data) |
| `GET` | `/wp-json/wp/v2/categories` | Fetch taxonomy for dropdown |
| `GET` | `/wp-json/wp/v2/tags` | Same for tags |

### Header format

```http
POST /wp-json/wp/v2/posts HTTP/1.1
Host: example.com
Authorization: Basic YWRtaW46eHh4eCB4eHh4IHh4eHggeHh4eCB4eHh4IHh4eHg=
Content-Type: application/json

{
  "title": "Auto-published article",
  "content": "<p>...</p>",
  "status": "publish",
  "categories": [12],
  "tags": [5, 8],
  "featured_media": 421
}
```

Base64 of `username:xxxx xxxx xxxx xxxx xxxx xxxx`. Most HTTP libs handle this — Go: `req.SetBasicAuth(user, pwd)`.

### Validation flow (before save)

```go
// Go pseudocode in our backend
func validateWordPressCreds(siteURL, user, appPwd string) error {
    req, _ := http.NewRequest("GET", siteURL+"/wp-json/wp/v2/users/me?context=edit", nil)
    req.SetBasicAuth(user, appPwd)
    r, err := httpClient.Do(req)
    if err != nil { return fmt.Errorf("network: %w", err) }
    defer r.Body.Close()

    switch r.StatusCode {
    case 200:
        var u struct{ Capabilities map[string]bool `json:"capabilities"` }
        json.NewDecoder(r.Body).Decode(&u)
        if !u.Capabilities["publish_posts"] {
            return errors.New("user lacks publish_posts capability")
        }
        return nil
    case 401: return errors.New("invalid credentials")
    case 403: return errors.New("user has no permission")
    case 404: return errors.New("REST API disabled or wrong URL")
    case 429: return errors.New("rate limited — try later")
    default:  return fmt.Errorf("unexpected %d", r.StatusCode)
    }
}
```

`?context=edit` forces capability detection; without it, `capabilities` field omitted.

### Error matrix

| Status | Cause | User message |
|--------|-------|--------------|
| 401 | Bad password / username typo | "Username or App Password incorrect" |
| 403 | User lacks `publish_posts` (Subscriber/Contributor) | "User must be Editor or Author" |
| 404 | REST API disabled / wrong domain | "REST API not reachable — check URL or security plugin" |
| 429 | Rate limit (rare on WP core, common with Wordfence) | "Too many requests — try in 1 min" |
| 500 | Plugin conflict / PHP error | "Server error — check WP error log" |
| Connection refused | DNS, firewall | "Cannot reach site" |

### Common gotchas (HARD-WON)

1. **Authorization header stripped** by Apache/.htaccess. Add to `.htaccess`:
   ```apache
   SetEnvIf Authorization "(.*)" HTTP_AUTHORIZATION=$1
   ```
   Or: `RewriteRule .* - [E=HTTP_AUTHORIZATION:%{HTTP:Authorization}]`
2. **Security plugins block REST API**: Wordfence "Disable XML-RPC" sometimes also kills `/wp-json`. iThemes Security has explicit REST API toggle.
3. **Reverse proxy strips headers**: Nginx in front of Apache often drops `Authorization`. User must set `proxy_set_header Authorization $http_authorization;`.
4. **Site URL mismatch**: WP `home_url()` vs `site_url()` differ → REST endpoint may live at `/blog/wp-json/...` not `/wp-json/...`. Always parse `link` header on first call.
5. **Multisite subdir**: REST API path includes site path (`/site2/wp-json/...`).
6. **WAF / Cloudflare**: Some WAF rules flag Basic Auth as suspicious. Tell users to whitelist our Fly egress IP if 403 persists.

### Rate limiting strategy (our side)

- WP core: NO rate limit by default. Plugins (Wordfence, Sucuri) impose ~5-10 req/min for non-admin paths.
- Our publisher: queue jobs, max 1 publish/site/30s, exponential backoff on 429.
- Use Go `golang.org/x/time/rate` per-site limiter.

### Test harness

```bash
# Quick CLI verification (run from dev machine)
curl -i -X GET "https://example.com/wp-json/wp/v2/users/me?context=edit" \
  -u "admin:abcd 1234 efgh 5678 ijkl 9012"

# Expected 200 + JSON body with `capabilities.publish_posts: true`
```

Sources: [WP REST API auth handbook](https://developer.wordpress.org/rest-api/using-the-rest-api/authentication/), [WP App Passwords admin handbook](https://developer.wordpress.org/advanced-administration/security/application-passwords/), [Cloudways guide](https://www.cloudways.com/blog/setup-basic-authentication-in-wordpress-rest-api/), [Notipo App Passwords guide](https://notipo.com/blog/wordpress-application-passwords).

---

## Unresolved questions

1. **OpenAPI spec authority** — Should we hand-write `openapi.yaml` in `backend/` repo, OR generate from Go via `swag`? Recommendation hand-write but needs founder OK on maintenance overhead.
2. **Edge runtime for proxy routes?** — We default to Node runtime for cookie ergonomics. Edge would be faster (~50ms savings) but `cookies()` async API is fine. Defer to perf testing.
3. **Cookie encryption** — Raw `sbf_*` key in httpOnly cookie. Acceptable since httpOnly + secure + same-site. Optional: AES-encrypt with `iron-session` if compliance ever needed (defer).
4. **WP credential storage** — Where does Go backend store user's `username:app_pwd` pairs? Encrypted column in Postgres? KMS? Out of R2 scope but R3/R4 must address.
5. **Vercel Pro seat count** — Solo founder = 1 seat. If we ever invite a contractor, it's $20 more. Confirm solo-only for Phase 3.
6. **Image upload pipeline** — Frontend uploads → Next.js Route Handler → Go backend → WP `/media`? Or direct frontend → Go → WP? Affects Vercel function exec time limits (Pro: 60s, generous).
7. **Backend OpenAPI spec writing effort** — Phase 3 plan needs to budget 4-8h for hand-writing 15 endpoints × paths/responses/schemas. Confirm with planner.

---

**Status:** DONE
**Summary:** Confirmed 4 stack choices for Phase 3 web SaaS — Hey API for TS gen, Route Handler proxy + httpOnly cookie for auth (no next-auth), Vercel Pro + Cloudflare gray-cloud DNS, WP App Passwords with Basic Auth. Brutal opinions, ranked, install snippets and skeletons included.
**Concerns/Blockers:** None — 7 unresolved questions listed for planner/founder decision.
