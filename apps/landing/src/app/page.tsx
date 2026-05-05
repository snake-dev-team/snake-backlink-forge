import { ApiShowcase } from "@/components/landing/api-showcase";
import { CtaSection } from "@/components/landing/cta-section";
import { FeaturesBento } from "@/components/landing/features-bento";
import { Footer } from "@/components/landing/footer";
import { Hero } from "@/components/landing/hero";
import { LandingNav } from "@/components/landing/landing-nav";
import { Pricing } from "@/components/landing/pricing";
import { PageShell } from "@/components/layout/page-shell";

/**
 * Landing page — Phase 06 pricing + CTA + footer.
 *
 * Layout layers:
 *   <PageShell decorative> — bg-background + fixed bg-vignette + bg-grid overlays (Phase 01)
 *   <LandingNav>           — extracted nav with .glass-card token (Phase 04)
 *   <Hero>                 — 60/40 split hero with CLI snippet + dashboard mockup (Phase 04)
 *   <FeaturesBento>        — 6-cell asymmetric bento grid (Phase 05)
 *   <ApiShowcase>          — tabbed REST API showcase with pre-rendered snippets (Phase 05)
 *   <Pricing>              — 3-tier cards + collapsible 10-SKU table (Phase 06)
 *   <CtaSection>           — single conversion banner (Phase 06)
 *   <Footer>               — 3-column minimal footer with status indicator (Phase 06)
 */

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

          {/* === Pricing — 3-tier cards + collapsible 10-SKU table (Phase 06) === */}
          <Pricing />

          {/* === CTA banner — glass-card-strong conversion section (Phase 06) === */}
          <CtaSection />

          {/* === Footer — 3-column with status indicator (Phase 06) === */}
          <Footer />
        </div>
      </div>
    </PageShell>
  );
}
