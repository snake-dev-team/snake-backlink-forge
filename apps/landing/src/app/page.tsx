import { ApiShowcase } from "@/components/landing/api-showcase";
import { FeaturesBento } from "@/components/landing/features-bento";
import { Hero } from "@/components/landing/hero";
import { LandingNav } from "@/components/landing/landing-nav";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { botUrl } from "@/lib/telegram/bot-url";

/**
 * Landing page — Phase 05 features bento + api showcase.
 *
 * Layout layers:
 *   <PageShell decorative> — bg-background + fixed bg-vignette + bg-grid overlays (Phase 01)
 *   <LandingNav>           — extracted nav with .glass-card token (Phase 04)
 *   <Hero>                 — 60/40 split hero with CLI snippet + dashboard mockup (Phase 04)
 *   <FeaturesBento>        — 6-cell asymmetric bento grid (Phase 05)
 *   <ApiShowcase>          — tabbed REST API showcase with pre-rendered snippets (Phase 05)
 *   <section id="pricing"> — PRESERVED verbatim; Phase 06 will replace with <Pricing />
 *   <footer id="contact">  — PRESERVED verbatim; Phase 06 will add <Footer />
 *
 * F13: pricing section MUST NOT be deleted until Phase 06 ships.
 * Anchor links from nav (#pricing, #contact) depend on them.
 */

// Pricing tiers data — preserved for existing <section id="pricing">
const pricing = [
  { name: "Starter", price: "$30", detail: "Test flow, ít site, volume thấp" },
  { name: "Growth", price: "$59", detail: "Đội SEO nhỏ, chạy đều mỗi tuần" },
  { name: "Scale", price: "$99", detail: "Nhiều site, cần tự động hóa mạnh hơn" },
];

export default function HomePage() {
  return (
    <PageShell decorative>
      {/* Skip-to-content link for keyboard/screen-reader users */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-full focus:bg-cyan-300 focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-slate-950"
      >
        Bỏ qua tới nội dung chính
      </a>

      <div className="relative mx-auto flex w-full max-w-7xl flex-col px-5 py-6 sm:px-8 lg:px-10">
        <LandingNav />

        <div id="main-content" tabIndex={-1}>
          {/* === Hero (Phase 04) === */}
          <Hero />

          {/* === Features bento grid (Phase 05) === */}
          <FeaturesBento />

          {/* === API showcase with tabbed snippets (Phase 05) === */}
          <ApiShowcase />

          {/*
            ===================================================================
            PRESERVED — Phase 06 will REPLACE <section id="pricing"> with
            <Pricing /> + <CtaSection /> + <Footer />. DO NOT delete until Phase 06 ships.
            Anchor link #pricing from LandingNav depends on this id.
            ===================================================================
          */}
          <section id="pricing" className="py-16">
            <div className="mb-8 max-w-2xl space-y-3">
              <p className="text-sm font-medium uppercase tracking-[0.24em] text-amber-200">
                Pricing teaser
              </p>
              <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                Bắt đầu nhỏ, scale khi workflow ổn.
              </h2>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              {pricing.map((plan) => (
                <Card
                  key={plan.name}
                  className="border-white/10 bg-white/[0.05] text-white shadow-none"
                >
                  <CardHeader>
                    <CardTitle className="flex items-end justify-between gap-4">
                      <span>{plan.name}</span>
                      <span className="text-3xl text-cyan-200">{plan.price}</span>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-5 text-sm text-white/62">
                    <p>{plan.detail}</p>
                    <Button
                      asChild
                      variant="outline"
                      className="w-full rounded-full border-white/15 bg-white/5 text-white hover:bg-white/10 hover:text-white"
                    >
                      {/* F6: botUrl() helper — no inline https://t.me/${username} */}
                      <a href={botUrl()} target="_blank" rel="noopener noreferrer">
                        Liên hệ qua bot
                      </a>
                    </Button>
                  </CardContent>
                </Card>
              ))}
            </div>
          </section>

          {/*
            PRESERVED — Phase 06 will replace/extend footer. DO NOT delete.
            Anchor #contact from existing nav links depends on this id.
          */}
          <footer
            id="contact"
            className="flex flex-col gap-4 border-t border-white/10 py-8 text-sm text-white/55 md:flex-row md:items-center md:justify-between"
          >
            <p>© 2026 Snake Backlink Forge. SEO automation cho operator Việt Nam.</p>
            <div className="flex gap-4">
              {/* F6: botUrl() helper — no inline https://t.me/${username} */}
              <a
                className="hover:text-cyan-200"
                href={botUrl()}
                target="_blank"
                rel="noopener noreferrer"
              >
                Telegram
              </a>
              <a className="hover:text-cyan-200" href="mailto:support@snakepremiumhub.com">
                support@snakepremiumhub.com
              </a>
            </div>
          </footer>
        </div>
      </div>
    </PageShell>
  );
}
