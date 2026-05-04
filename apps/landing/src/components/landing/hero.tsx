import { HeroCliSnippet } from "./hero-cli-snippet";
import { HeroDashboardMockup } from "./hero-dashboard-mockup";
import { HeroText } from "./hero-text";

/**
 * Hero section orchestrator — mockup-balanced 1fr / 1.1fr split.
 * Mockup (E:\ui-b-developer-tool.html line 214):
 *   grid-template-columns: 1fr 1.1fr; gap: 64px; padding 80px top/bottom
 * Right column slightly wider so dashboard mockup gets room to breathe.
 * Layout:
 *   lg+  → side-by-side [1fr / 1.1fr] with 64px gap
 *   md   → single col stacked: text → CLI snippet → mockup
 *   sm   → text only (hidden md:flex hides visual column on mobile)
 *
 * HeroCliSnippet + HeroDashboardMockup are pure SSR — zero client JS.
 */
export function Hero() {
  return (
    <section
      className="grid gap-10 py-20 lg:grid-cols-[1fr_1.1fr] lg:items-center lg:gap-16 lg:py-24"
      aria-label="Hero"
    >
      <HeroText />

      {/* Right column: hidden on mobile (sm), visible md+.
          min-w-0 prevents grid blowout when CodeWindow content is wide.
          gap-5 = mockup hero visual stack rhythm (CLI → dashboard mockup). */}
      <div className="hidden min-w-0 flex-col gap-5 md:flex">
        <HeroCliSnippet />
        <HeroDashboardMockup />
      </div>
    </section>
  );
}
