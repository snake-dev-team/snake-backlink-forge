# R1 — Frontend Foundation Research (Phase 3 SBF Web SaaS)

**Date:** 2026-04-26
**Researcher:** R1
**Scope:** Next.js 15 RSC patterns, shadcn/ui v2 + Tailwind v4, next-intl vs next-i18next
**Constraint:** Solo VN founder, fast ship, fintech-adjacent risk awareness, Phase 2 Go Fiber backend = source of truth.

---

## TL;DR — Recommendations

- **Next.js 15.5** (latest stable Apr 2026; 16 emerging but skip for now). Server Components for landing/dashboard shell; Client Components for forms + interactive widgets; **Route Handlers (Go Fiber proxy) for mutations, NOT Server Actions** — bearer auth header simpler client-side.
- **shadcn/ui latest CLI (`shadcn@latest`)** + **Tailwind v4 CSS-first** + **OKLCH tokens** (default since Mar 2025). Install via `pnpm dlx shadcn@latest init -t next`. Use **next-themes** for dark mode.
- **next-intl v4** wins decisively. next-i18next is dead for App Router — built for Pages Router, no first-class RSC support. No contest.

---

## Topic 1 — Next.js 15 App Router + RSC

### Version
- **Stable: 15.5** (Apr 2026). Includes Turbopack builds beta, Node.js middleware stable, typed routes.
- Next.js 16 has migration guide live but: skip. 15.5 is battle-tested; 16 brings deprecation churn (next lint removed, more breaking caching). **Use 15.5 for Phase 3.**
- React 19 stable required. Node 18.18+ minimum.
- Patches as of Apr 2026: streaming fetch hang fix, CVE-2026-27979 (maxPostponedStateSize), CVE-2026-29057 (http-proxy). **Pin minor + auto-patch.**

### Server vs Client Component split — concrete rules for SBF

| Surface | Type | Why |
|---|---|---|
| Landing page (marketing) | **Server** | SEO critical, zero JS to client, fast TTFB |
| Dashboard shell + nav | **Server** | Fetch user/wallet from Go Fiber, stream skeleton |
| Wallet balance widget | **Server** (with Suspense) | Fresh on each request, no client interactivity |
| Campaign list table | **Server** initial + **Client** for sort/filter | Initial data RSC fetched, table interaction client-side via TanStack Table |
| Campaign create form | **Client** (`"use client"`) | React Hook Form + Zod + shadcn Form — needs interactivity |
| Telegram OAuth callback | **Route Handler** | Backend bearer token issuance, no UI |
| Theme toggle | **Client** | next-themes requires client |

**Heuristic:** Default to Server. Add `"use client"` only when needed (`useState`, `useEffect`, event handlers, browser APIs, Radix interactive primitives).

### Data fetching — split rule

- **RSC:** native `fetch()` with explicit `cache: 'no-store'` or `next: { revalidate: N }`. Next 15 default = uncached (breaking change from 14). For SBF: most calls hit Go Fiber → use `cache: 'no-store'` for user/wallet/campaigns; `revalidate: 3600` for static taxonomy/pricing pages.
- **Client:** **TanStack Query v5** for: wallet balance polling, campaign status (pending → publishing → done), real-time content gen progress. Don't use Query for one-shot RSC data.
- **Pattern:** RSC fetches initial state → passes as `initialData` → TanStack Query takes over client-side. Avoids waterfall + loses no SSR benefit.

### Mutations — Route Handlers, NOT Server Actions

**Recommendation: Route Handlers** (`app/api/.../route.ts`) that proxy to Go Fiber.

Why not Server Actions:
- Bearer API key auth model — Server Actions hide HTTP layer, harder to debug auth flows
- Go Fiber is the source of truth; Next.js is just the UI shell — Server Actions tempt you to put business logic in the wrong place
- Server Actions IDs are obfuscated but still public endpoints — extra attack surface for fintech-adjacent
- Multi-step mutations (top-up wallet → confirm SePay → update UI) need explicit HTTP semantics

When Route Handlers shine:
- Forward `Authorization: Bearer <key>` from cookie/session to Go Fiber
- Centralize error mapping (Fiber 401 → Next redirect to /login)
- Add Next.js middleware logging without modifying Go backend

**Exception:** Use Server Actions ONLY for trivial form submissions that revalidate a path (e.g., update profile name) — but even then, Route Handler is cleaner for this codebase.

### Streaming + Suspense

- Wrap dashboard sections in `<Suspense fallback={<Skeleton />}>`. Each fetches independently.
- `loading.tsx` per route segment for instant nav feedback.
- shadcn `Skeleton` component pairs perfectly.
- Pattern:
  ```tsx
  <Suspense fallback={<WalletSkeleton />}>
    <WalletBalance /> {/* RSC, awaits Go Fiber */}
  </Suspense>
  <Suspense fallback={<CampaignsTableSkeleton />}>
    <CampaignsTable /> {/* RSC */}
  </Suspense>
  ```

### Pitfalls 2026 (real ones, save you a day each)

1. **`"use client"` viral spread** — adding it to a layout wraps everything. Keep it leaf-level. Use composition: pass server children as props to client components.
2. **Hydration mismatch from `Date`/`Math.random()`** — never compute time/random in render without `useEffect` guard. SBF wallet balance display: format on server with fixed locale.
3. **Env var leak** — `NEXT_PUBLIC_*` ships to bundle. **NEVER** prefix `BACKEND_API_KEY` or anything secret. Audit `next build` output.
4. **Async params** — Next 15 made `params`, `searchParams`, `cookies()`, `headers()` async. Codemod available: `npx @next/codemod@canary next-async-request-api .`
5. **GET Route Handler caching** — uncached by default in 15. If you need cache, explicit `export const dynamic = 'force-static'`.
6. **Server-only imports** — wrap secret-touching code in `import 'server-only'` package to crash at build-time if mistakenly imported client-side.
7. **TanStack Query in RSC** — don't. Query is client-only. Use `HydrationBoundary` to pass server-prefetched data.
8. **Turbopack dev** stable; Turbopack build still beta in 15.5 — use webpack for prod build until 16 GA.

---

## Topic 2 — shadcn/ui v2 + Tailwind v4

### Version
- **CLI: `shadcn@latest`** (no version pin in docs — CLI auto-resolves).
- **Tailwind v4** required for new projects (CSS-first, no `tailwind.config.js`).
- Components updated for **React 19** (`forwardRef` removed, `data-slot` attrs added).
- Default theme tokens: **OKLCH** (HSL deprecated since shadcn v2 update, Mar 2025).

### Install flow (Next.js 15 + Tailwind v4)

```bash
# 1. Create Next.js 15 project (skip if existing)
pnpm create next-app@latest sbf-web --typescript --tailwind --app --src-dir --import-alias "@/*"

# 2. Init shadcn
cd sbf-web
pnpm dlx shadcn@latest init -t next

# Prompts:
# - Style: New York (default, more compact)
# - Base color: Zinc (neutral, fits VN/EN)
# - CSS variables: Yes (OKLCH)

# 3. Add components needed for Phase 3
pnpm dlx shadcn@latest add button card dialog form input label \
  select tabs table toast skeleton badge avatar dropdown-menu \
  separator sonner tooltip
```

### components.json + Tailwind v4 relationship

In Tailwind v4 there is **no `tailwind.config.js`**. Config lives in CSS:

```css
/* app/globals.css */
@import "tailwindcss";

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --radius-lg: var(--radius);
  /* ... */
}

:root {
  --background: oklch(1 0 0);
  --foreground: oklch(0.145 0 0);
  --primary: oklch(0.205 0 0);
  --radius: 0.625rem;
}

.dark {
  --background: oklch(0.145 0 0);
  --foreground: oklch(0.985 0 0);
  /* ... */
}
```

`components.json` still tells the CLI where to put components, alias paths, base color preset. Generated automatically by `init`.

### OKLCH vs HSL — recommendation: **OKLCH** (default, don't fight it)

- Perceptually uniform — color manipulation predictable
- Better wide-gamut display support (P3)
- shadcn dropped HSL as default Mar 2025
- VN market has growing OLED/P3 device share — colors look correct
- Trade-off: not supported in IE/old Android <8 — irrelevant for SBF target users (SEO operators, modern browsers)

### Dark mode — next-themes

```bash
pnpm add next-themes
```

```tsx
// app/providers.tsx
"use client";
import { ThemeProvider } from "next-themes";
export function Providers({ children }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      {children}
    </ThemeProvider>
  );
}
```

```tsx
// app/layout.tsx
<html lang={locale} suppressHydrationWarning>
  <body><Providers>{children}</Providers></body>
</html>
```

`suppressHydrationWarning` on `<html>` is critical — next-themes flashes class before hydration.

### Component → Radix primitive map (for SBF Phase 3)

| Component | Radix Primitive | Notes |
|---|---|---|
| Form | react-hook-form (peer) | Pair with Zod resolver |
| Input | none | Native input |
| Button | none | Plain element |
| Dialog | @radix-ui/react-dialog | Modal for "Top up wallet", "Create campaign" |
| Tabs | @radix-ui/react-tabs | Dashboard sections |
| Table | TanStack Table v8 (peer) | Campaign list, transaction history |
| Toast | sonner (newer, replaces shadcn/toast) | Use `sonner` — toast deprecated |
| Card | none | Pure CSS |
| Skeleton | none | Pure CSS, animation only |
| Select | @radix-ui/react-select | WordPress site picker, language picker |
| Dropdown | @radix-ui/react-dropdown-menu | User menu, action menus |
| Tooltip | @radix-ui/react-tooltip | Form help text |

**Critical:** Use **`sonner` not `toast`** — shadcn migrated. Native React 19 + better mobile.

---

## Topic 4 — next-intl vs next-i18next

### Winner: **next-intl v4** (no contest, decisively)

**Reasons (brutal):**
1. next-i18next was built for Pages Router. App Router support = compatibility shim. Maintainer himself recommends next-intl for new App Router projects.
2. next-intl is the **only** library with native RSC support — no Provider wrapper needed for Server Components. `getTranslations()` is awaitable in RSC; `useTranslations()` for client.
3. Bundle size: next-intl precompiles ICU messages → runtime <1KB after icu-minify. next-i18next ships full i18next + react-i18next + next-i18next ≈ 30KB.
4. TypeScript-safe keys via module augmentation — autocomplete + compile-time errors on missing keys.
5. Growth: next-intl ~4x growth past 12 months; next-i18next flat-to-declining.
6. ICU MessageFormat = industry standard; works with any TMS (Crowdin, Lokalise, Locize) if you ever scale beyond solo.

**When next-i18next would still win:** existing i18next infrastructure, Pages Router migration, need for i18next backend plugins. **None apply to SBF.**

### Setup snippet (VN/EN, path-based routing)

```bash
pnpm add next-intl
```

```ts
// i18n/routing.ts
import {defineRouting} from 'next-intl/routing';

export const routing = defineRouting({
  locales: ['vi', 'en'],
  defaultLocale: 'vi',
  localePrefix: 'always' // /vi/dashboard, /en/dashboard — clearer SEO
});
```

```ts
// middleware.ts
import createMiddleware from 'next-intl/middleware';
import {routing} from './i18n/routing';
export default createMiddleware(routing);
export const config = {
  matcher: ['/((?!api|_next|_vercel|.*\\..*).*)']
};
```

```ts
// i18n/request.ts
import {getRequestConfig} from 'next-intl/server';
import {routing} from './routing';

export default getRequestConfig(async ({requestLocale}) => {
  let locale = await requestLocale;
  if (!locale || !routing.locales.includes(locale as any)) {
    locale = routing.defaultLocale;
  }
  return {
    locale,
    messages: (await import(`../messages/${locale}.json`)).default
  };
});
```

```ts
// next.config.ts
import createNextIntlPlugin from 'next-intl/plugin';
const withNextIntl = createNextIntlPlugin();
export default withNextIntl({/* next config */});
```

```tsx
// app/[locale]/layout.tsx
import {NextIntlClientProvider} from 'next-intl';
import {getMessages} from 'next-intl/server';

export default async function LocaleLayout({children, params}) {
  const {locale} = await params;
  const messages = await getMessages();
  return (
    <html lang={locale}>
      <body>
        <NextIntlClientProvider messages={messages}>
          {children}
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
```

```tsx
// Server Component
import {getTranslations} from 'next-intl/server';
const t = await getTranslations('Dashboard');
return <h1>{t('title')}</h1>;

// Client Component
"use client";
import {useTranslations} from 'next-intl';
const t = useTranslations('Dashboard');
return <button>{t('submit')}</button>;
```

### TypeScript-safe keys

```ts
// global.d.ts
import en from './messages/en.json';
type Messages = typeof en;
declare global {
  interface IntlMessages extends Messages {}
}
```

Now `t('foo.bar')` autocompletes; `t('typo')` is a TS error.

### Locale strategy: **path-based `/vi` and `/en`** (recommended)

- SEO: separate URLs indexed independently (Google.com.vn ranks /vi/, Google.com ranks /en/)
- Domain-based (`.vn` vs `.com`) overkill for solo founder; needs 2 SSL certs, 2 DNS, 2 Vercel projects
- Defer domain split until product-market fit + multiple regions

---

## Architectural fit summary

| Concern | Verdict |
|---|---|
| Existing stack alignment | Phase 2 Go Fiber backend untouched; Next.js purely UI layer |
| Solo founder maintenance | All 3 picks have low-config defaults + active maintenance |
| Fintech-adjacent risk | Route Handlers > Server Actions for explicit auth boundary |
| Time-to-ship | Tailwind v4 CSS-first + shadcn CLI = hours not days for full design system |
| VN market | OKLCH renders correctly on modern VN devices; next-intl handles VN ICU plurals |

---

## Unresolved questions

1. **TanStack Query version** — v5 confirmed for React 19 compat? (Not covered in 5 fetches; quick check before implementation.)
2. **Sonner vs shadcn `toast` migration** — confirm shadcn-replaced toast component status circa Apr 2026. (Suggest verifying when running `shadcn add`.)
3. **Next.js 16 timeline** — does it GA before SBF Phase 3 ships? If so, plan upgrade window. (Master prompt should specify upgrade cadence.)
4. **`localePrefix: 'as-needed'` vs `'always'`** — `'as-needed'` keeps Vietnamese URLs clean (no `/vi/`) since it's default locale, but `'always'` simpler for SEO consistency. Recommend `'always'` but flag for product decision.
5. **WordPress sites listing UI** — does R2 API research confirm cursor or offset pagination? Affects TanStack Query infinite-scroll vs page-based pattern.

---

## Sources

- [Next.js 15 release notes (nextjs.org)](https://nextjs.org/blog/next-15)
- [Next.js Releasebot Apr 2026 updates](https://releasebot.io/updates/vercel/next-js)
- [Upgrading: Version 15 (nextjs.org)](https://nextjs.org/docs/app/guides/upgrading/version-15)
- [shadcn/ui Tailwind v4 docs](https://ui.shadcn.com/docs/tailwind-v4)
- [shadcn/ui Next.js install](https://ui.shadcn.com/docs/installation/next)
- [shadcn/ui React 19 + Next.js 15](https://ui.shadcn.com/docs/react-19)
- [next-intl docs — App Router](https://next-intl.dev/docs/getting-started/app-router)
- [next-intl middleware](https://next-intl.dev/docs/routing/middleware)
- [Locize: next-intl vs next-i18next](https://www.locize.com/blog/next-intl-vs-next-i18next/)
- [Better-i18n: App Router i18n RSC patterns 2026](https://better-i18n.com/en/blog/nextjs-app-router-i18n-server-components/)

---

**Status:** DONE
**Summary:** Three foundation picks confirmed with concrete versions and install flows: Next.js 15.5 (RSC + Route Handlers, skip Server Actions), shadcn/ui latest CLI on Tailwind v4 + OKLCH, next-intl v4 (next-i18next is dead for App Router).
**Concerns/Blockers:** 5 unresolved questions listed above — none blocking, all minor verifications during implementation.
