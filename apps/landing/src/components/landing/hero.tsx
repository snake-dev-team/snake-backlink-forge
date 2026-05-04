import { HeroCliSnippet } from "./hero-cli-snippet";
import { HeroDashboardMockup } from "./hero-dashboard-mockup";
import { HeroText } from "./hero-text";

/**
 * Hero section orchestrator — 60/40 desktop split (Phase 04).
 * Layout:
 *   lg+  → side-by-side [1.4fr / 0.6fr]: text left, visuals right
 *   md   → single col stacked: text → CLI snippet → mockup (~250px)
 *   sm   → text only (hidden md:flex hides visual column on mobile)
 *
 * HeroCliSnippet is an async RSC (Shiki at build time) — no client JS.
 */
export function Hero() {
  return (
    <section
      className="grid gap-12 py-20 lg:grid-cols-[1.4fr_0.6fr] lg:items-center lg:py-28"
      aria-label="Hero"
    >
      <HeroText />

      {/* Right column: hidden on mobile (sm), visible md+.
          min-w-0 prevents grid blowout when CodeWindow content is wide. */}
      <div className="hidden min-w-0 flex-col gap-4 md:flex">
        <HeroCliSnippet />
        <HeroDashboardMockup />
      </div>
    </section>
  );
}
