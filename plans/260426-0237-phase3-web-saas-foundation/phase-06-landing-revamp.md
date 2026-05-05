---
name: "Phase 06 — Landing Revamp (VN-only, simplified)"
phase: 6
priority: P1
effort: 2h
status: implemented; typecheck/build/local smoke passed; independent reviewer deferred by 429
created: 2026-04-26
updated: 2026-04-28
---

<!-- RT-R1: F8 — MAJOR SIMPLIFICATION. Drop next-intl entirely from Phase 3. Drop [locale]/ route group. Drop sitemap/robots/OG. VN-only landing. Combined-middleware risk eliminated. -->
<!-- Effort delta: 5h → 2h. EN expansion + sitemap + OG metadata deferred to Phase 9 when pricing flow ships. -->

## Context Links

- Research R1: `plans/260426-0237-phase3-web-saas-foundation/research/researcher-r1-frontend-foundation.md` (lines 91-205 — shadcn + Tailwind v4 OKLCH used; lines 208-322 next-intl DEFERRED)
- Phase 01: `phase-01-setup-and-deps.md` (no next-intl install; `next.config.ts` minimal)
- Phase 03: `phase-03-frontend-auth-flow.md` (auth-only middleware, no intl combination)
- Phase 04: `phase-04-app-shell-and-dashboard.md` (`(public)` group reserved for landing)
- Master prompt: `docs/MASTER_PROMPT.md` §5.5 lines 1352-1357 (engineering bar)
- Scout: `plans/260426-0237-phase3-web-saas-foundation/reports/scout-codebase-state.md` (§1 — bare landing currently `<h1>placeholder</h1>`)

## Overview

- **Priority:** P1 (revenue funnel; brand presence)
- **Status:** implemented; typecheck/build/local smoke passed; independent reviewer deferred by 429
- **Brief:** Replace landing placeholder with single VN-only page at `apps/landing/src/app/page.tsx`. Hero (headline + sub + CTA "Mở Telegram bot"), 3 feature cards, pricing teaser ($30/$59/$99 with "Liên hệ qua bot" CTA), footer. Hardcoded VN copy. NO `[locale]/` route group, NO next-intl, NO messages JSON, NO sitemap/robots/OG metadata, NO locale switcher. EN expansion + SEO metadata + sitemap deferred to Phase 9 when pricing real flow ships.

## Key Insights (RT-R1: F8)

- next-intl deferred to Phase 9 — combined intl+auth middleware was a Critical regression risk; eliminating the combination removes that bug class from Phase 3 entirely
- Single landing route `/` (no locale prefix). Visitors see VN-only. Browser native lang detection irrelevant for v1
- Sitemap + robots + OG metadata add SEO value but landing is not yet ranking target — Phase 9 SEO push covers it together with EN expansion
- shadcn `Button`, `Card` (already installed Phase 1) cover all UI needs — no new components
- Footer copyright year hardcoded to 2026 (update annually OR use small server component)
- Hardcoded VN copy is a deliberate choice — extracting to `useTranslations` later (Phase 9) means just wrapping strings in `t()`

## Requirements

### Functional

- Visiting `/` renders full landing page (no redirect, no locale param)
- Hero section: headline, subheadline, primary CTA "Mở Telegram bot" (deep link `t.me/<bot>`), secondary CTA "Xem giá" (anchor link to `#pricing`)
- 3 feature cards: "AI tạo bài viết chuẩn SEO", "Đăng tự động lên WordPress", "Phân tích từ khóa thông minh"
- Pricing teaser (3 cards): $30/$59/$99 with "Liên hệ qua bot" CTA each, deep link
- Footer: copyright + Telegram link + email contact
- Auth-gated routes (`/dashboard`, `/sites`, `/settings`, `/campaigns`) untouched — middleware from Phase 3 still applies

### Non-functional

- Bundle size landing: target < 30KB JS shipped (RSC default, minimal client interactivity)
- LCP < 2.5s on 4G simulated (no images this phase, just text + Tailwind)
- WCAG 2.1 AA: focus rings, semantic landmarks, skip-to-main link, alt text on icons (or aria-hidden)
- Lighthouse mobile ≥ 90 on Performance / Accessibility / SEO / Best Practices (validated in Phase 8 CI)

## Architecture

### Route tree (final, simplified — RT-R1: F8)

```
apps/landing/src/app/
├── layout.tsx                 ← root <html lang="vi">
├── globals.css
├── providers.tsx
├── page.tsx                   ← THIS PHASE: full landing
├── (auth)/
│   └── login/                 ← Phase 3
├── (app)/                     ← Phase 4
│   └── ...
├── api/                       ← Phase 3
└── middleware.ts              ← auth-only, no intl (Phase 3)
```

**No `[locale]/` segment. No `(public)` group needed (only landing lives here, layout.tsx handles it).**

### Component breakdown

| Component | Type | Role |
|-----------|------|------|
| `<HeroSection>` | Server | Headline + CTAs |
| `<FeaturesSection>` | Server | 3-card grid |
| `<PricingTeaserSection>` | Server | 3 plan cards |
| `<LandingFooter>` | Server | Copyright + links |
| `<LandingNav>` | Server | Sticky top nav with login link |
| `<SkipToMainLink>` | Server | A11y skip link |

## Related Code Files

### Create

- `apps/landing/src/components/landing/hero-section.tsx`
- `apps/landing/src/components/landing/features-section.tsx`
- `apps/landing/src/components/landing/pricing-teaser-section.tsx`
- `apps/landing/src/components/landing/landing-nav.tsx`
- `apps/landing/src/components/landing/landing-footer.tsx`
- `apps/landing/src/components/landing/skip-to-main-link.tsx`

### Modify

- `apps/landing/src/app/page.tsx` — replace placeholder with full landing
- `apps/landing/src/app/layout.tsx` — confirm `<html lang="vi">` (set in Phase 1)

### Delete

- `apps/landing/src/app/page.tsx` placeholder content (replaced)

### Deferred to Phase 9 (RT-R1: F8)

- `apps/landing/src/app/[locale]/` route group
- `apps/landing/messages/{vi,en}.json`
- `apps/landing/i18n/{routing,request}.ts`
- `apps/landing/global.d.ts` (next-intl message augmentation)
- `apps/landing/src/app/sitemap.ts`
- `apps/landing/src/app/robots.ts`
- `<LocaleSwitcher>` component
- `generateMetadata` per locale
- `next-intl` package install

## Implementation Steps

### Step 1 — Landing page (45 min)

`apps/landing/src/app/page.tsx`:

```tsx
import { LandingNav } from '@/components/landing/landing-nav'
import { HeroSection } from '@/components/landing/hero-section'
import { FeaturesSection } from '@/components/landing/features-section'
import { PricingTeaserSection } from '@/components/landing/pricing-teaser-section'
import { LandingFooter } from '@/components/landing/landing-footer'
import { SkipToMainLink } from '@/components/landing/skip-to-main-link'

export default function HomePage() {
  return (
    <>
      <SkipToMainLink />
      <LandingNav />
      <main id="main" tabIndex={-1}>
        <HeroSection />
        <FeaturesSection />
        <PricingTeaserSection />
      </main>
      <LandingFooter />
    </>
  )
}
```

### Step 2 — Hero (15 min)

`hero-section.tsx`:

```tsx
import Link from 'next/link'
import { Button } from '@/components/ui/button'

export function HeroSection() {
  const botUrl = `https://t.me/${process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME}`
  return (
    <section className="max-w-6xl mx-auto px-4 py-20 md:py-28 text-center">
      <h1 className="text-4xl md:text-6xl font-bold tracking-tight">
        AI viết bài + tự động đăng WordPress
      </h1>
      <p className="mt-6 text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto">
        Tạo nội dung SEO bằng AI, tự động xuất bản lên WordPress site của bạn. Bắt đầu trong 1 phút qua Telegram bot.
      </p>
      <div className="mt-10 flex gap-3 justify-center">
        <Button asChild size="lg">
          <Link href={botUrl} target="_blank" rel="noopener noreferrer">Mở Telegram bot</Link>
        </Button>
        <Button asChild size="lg" variant="outline">
          <Link href="#pricing">Xem giá</Link>
        </Button>
      </div>
    </section>
  )
}
```

### Step 3 — Features (15 min)

`features-section.tsx`:

```tsx
import { Sparkles, Globe2, Search } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

const items = [
  { icon: Sparkles, title: 'AI tạo bài viết chuẩn SEO', body: 'Haiku/Sonnet/Opus — chọn chất lượng phù hợp ngân sách.' },
  { icon: Globe2, title: 'Đăng tự động lên WordPress', body: 'Kết nối site, AI đăng bài tự động — không thủ công.' },
  { icon: Search, title: 'Phân tích từ khóa thông minh', body: 'DataForSEO + tối ưu keyword cluster trước khi viết.' },
]

export function FeaturesSection() {
  return (
    <section className="max-w-6xl mx-auto px-4 py-16">
      <h2 className="text-3xl font-bold text-center mb-12">Tính năng</h2>
      <div className="grid md:grid-cols-3 gap-6">
        {items.map(({ icon: Icon, title, body }) => (
          <Card key={title}>
            <CardContent className="pt-6">
              <Icon className="h-8 w-8 mb-4" aria-hidden="true" />
              <h3 className="font-semibold text-lg mb-2">{title}</h3>
              <p className="text-sm text-muted-foreground">{body}</p>
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  )
}
```

### Step 4 — Pricing teaser (20 min)

`pricing-teaser-section.tsx`:

```tsx
import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const plans = [
  { name: 'Starter', price: '$30/tháng', features: ['50 bài/tháng', 'Haiku', '1 WP site'] },
  { name: 'Pro', price: '$59/tháng', features: ['200 bài/tháng', 'Sonnet', '5 WP sites'] },
  { name: 'Business', price: '$99/tháng', features: ['500 bài/tháng', 'Opus', 'Không giới hạn site'] },
]

export function PricingTeaserSection() {
  const botUrl = `https://t.me/${process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME}`
  return (
    <section id="pricing" className="max-w-6xl mx-auto px-4 py-16">
      <h2 className="text-3xl font-bold text-center mb-2">Bảng giá</h2>
      <p className="text-center text-muted-foreground mb-12">Đăng ký qua Telegram bot.</p>
      <div className="grid md:grid-cols-3 gap-6">
        {plans.map(p => (
          <Card key={p.name}>
            <CardHeader>
              <CardTitle>{p.name}</CardTitle>
              <p className="text-2xl font-bold">{p.price}</p>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm mb-6">
                {p.features.map(f => <li key={f}>• {f}</li>)}
              </ul>
              <Button asChild className="w-full">
                <Link href={botUrl} target="_blank" rel="noopener noreferrer">Liên hệ qua bot</Link>
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  )
}
```

### Step 5 — Nav + Footer + Skip link (15 min)

`landing-nav.tsx`:

```tsx
import Link from 'next/link'
import { Button } from '@/components/ui/button'

export function LandingNav() {
  return (
    <header className="sticky top-0 bg-background/80 backdrop-blur border-b z-40">
      <nav className="max-w-6xl mx-auto px-4 h-14 flex items-center" aria-label="Điều hướng chính">
        <Link href="/" className="font-semibold">Snake Backlink Forge</Link>
        <div className="ml-auto flex items-center gap-2">
          <Link href="#pricing" className="text-sm hover:underline">Bảng giá</Link>
          <Button asChild size="sm"><Link href="/login">Đăng nhập</Link></Button>
        </div>
      </nav>
    </header>
  )
}
```

`landing-footer.tsx`:

```tsx
export function LandingFooter() {
  const botUrl = `https://t.me/${process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME}`
  const year = new Date().getFullYear()
  return (
    <footer className="border-t mt-16">
      <div className="max-w-6xl mx-auto px-4 py-8 flex flex-col md:flex-row gap-4 justify-between text-sm text-muted-foreground">
        <p>© {year} Snake Backlink Forge</p>
        <div className="flex gap-4">
          <a href={botUrl} target="_blank" rel="noopener noreferrer" className="hover:underline">Telegram</a>
          <a href="mailto:support@snakebacklink.com" className="hover:underline">Liên hệ</a>
        </div>
      </div>
    </footer>
  )
}
```

`skip-to-main-link.tsx`:

```tsx
export function SkipToMainLink() {
  return (
    <a href="#main" className="sr-only focus:not-sr-only fixed top-2 left-2 z-50 bg-background px-3 py-2 rounded-md border">
      Bỏ qua tới nội dung chính
    </a>
  )
}
```

### Step 6 — Smoke (5 min)

```bash
pnpm --filter @sbf/web dev
# Open http://localhost:3000
# Verify: hero renders, features grid, pricing cards, footer
# Tab through page → focus rings visible, skip link works
# Click "Mở Telegram bot" → opens t.me/SnakeBacklinkForgeBot
# Click "Đăng nhập" → /login
```

## Todo List

- [x] Step 1 — `app/page.tsx` replaces placeholder with full landing layout (RT-R1: F8 — single route, no locale group)
- [x] Step 2 — Hero area with VN copy + 2 CTAs
- [x] Step 3 — Features area 3-card grid with lucide icons
- [x] Step 4 — Pricing teaser 3 plans with "Liên hệ qua bot" CTAs
- [x] Step 5 — Landing nav + footer + skip-to-main link
- [x] Step 6 — Local smoke + stable HTML marker check

## Success Criteria

- Visiting `/` renders full landing (no redirect, no locale param) — RT-R1: F8
- All 5 sections render correctly desktop + mobile
- Tab order logical; focus rings visible (`:focus-visible` styled)
- Skip-to-main link visible only on focus, jumps to `<main id="main">`
- Lighthouse mobile ≥ 90 on Performance / Accessibility / Best Practices / SEO (validated in Phase 8 CI)
- Bundle size for landing < 30KB JS shipped
- All Telegram CTAs use `target="_blank" rel="noopener noreferrer"`
- No console errors on landing in dev mode

## Risk Assessment

<!-- RT-R1: F8 — combined-middleware regression risk eliminated; Phase 6 → just landing content -->

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Pricing teaser implies live billing — user confusion | Med | Med | Subtitle says "Đăng ký qua Telegram bot"; CTA is clearly "Liên hệ qua bot" |
| Future EN expansion requires refactor to extract VN strings | High | Low | Accept — Phase 9 wraps strings in `t()` when next-intl ships; not blocking |
| Lighthouse < 90 due to web fonts | Low | Med | Use `next/font/google` with `display: swap`; fallback system fonts; no images |
| Browser missing `lucide-react` icons (CSP) | Low | Low | lucide-react is bundled JS, not external script; CSP `script-src 'self'` covers |
| User in EN-speaking market sees only VN | Med | Med | Accept for v1 — Phase 9 ships EN. VN market is primary launch market per master prompt |

## Security Considerations

- No secrets in landing — all client-rendered
- Telegram deep links use `target="_blank" rel="noopener noreferrer"` — prevents tabnabbing
- No third-party scripts on landing in this phase (Plausible added in Phase 07; opt-in cookieless)
- CSP header set in Phase 07; landing must work under `script-src 'self'` (no inline scripts)

## Next Steps

- **Depends on:** Phase 01 (apps/landing infra), Phase 03 (auth middleware unchanged)
- **Unblocks:** Phase 07 (deploy includes landing); Phase 08 (Lighthouse CI runs against `/` not `/vi`)
- **Follow-up (Phase 9):** install next-intl + add `[locale]/` segment + EN messages; add sitemap + robots; add `generateMetadata` with hreflang per locale; replace pricing CTAs with real subscription flow

## Resolved unresolved questions (from R1)

- **R1 Q4 (`localePrefix: 'always'` vs `'as-needed'`):** DEFERRED — not relevant in Phase 3 (no intl). Phase 9 will resolve with EN expansion.

## Deferred from earlier plan revision

- next-intl install + locale routing (RT-R1: F8 — Phase 9)
- `messages/vi.json` + `messages/en.json` (RT-R1: F8 — Phase 9)
- Sitemap.xml + robots.txt + hreflang (RT-R1: F8 — Phase 9)
- `generateMetadata` per locale + Open Graph (RT-R1: F8 — Phase 9)
- Locale switcher (RT-R1: F8 — Phase 9)

## Deferred / Do Later

- Independent `code-reviewer` agent review is deferred because the agent call hit account rate limit `429`; retry when quota resets.
- Full Lighthouse/mobile visual browser audit remains Phase 8 CI scope; this pass validated typecheck, production build, local HTTP render, and required HTML markers.
