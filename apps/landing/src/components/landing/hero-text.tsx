import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { botUrl } from "@/lib/telegram/bot-url";

/**
 * Hero left column: NEW-chip badge, 2-line gradient h1, mockup typography, 2 CTAs, ⌘K hint.
 * Mockup ground truth (E:\ui-b-developer-tool.html lines 215-235):
 *   - badge: pill with embedded "NEW" mono chip + label + arrow
 *   - h1: 72px, line-height 1.0, tracking -0.04em, gradient on line 2 only
 *   - sub: 17px, max-w-480px, leading 1.55
 *   - margins: 28/24/36 (badge, h1, sub) — generous breathing
 * F6: uses botUrl() helper — no inline t.me template literals.
 * F10: uses .gradient-text CSS class — no Tailwind bg-clip-text utilities.
 */
export function HeroText() {
  return (
    <div className="flex flex-col">
      {/* NEW-chip badge — pill with embedded mono chip + label + arrow (mockup signature).
          Light mode: violet bg/border maintain WCAG AA via violet-700+ text. */}
      <div className="mb-7 inline-flex w-fit items-center gap-2 rounded-full border border-violet-500/30 bg-violet-500/10 py-1 pl-1.5 pr-3">
        <span className="rounded-full bg-violet-500/25 px-2 py-0.5 font-mono text-[10px] font-medium text-violet-700 dark:text-violet-200">
          NEW
        </span>
        <span className="text-xs text-foreground/85 dark:text-white/85">
          AI multi-agent content engine — Claude Opus 4.7 + GPT-5
        </span>
        <span className="text-[11px] text-foreground/40 dark:text-white/40" aria-hidden="true">
          →
        </span>
      </div>

      {/* Headline — 2-line break, gradient ONLY on second line (mockup) */}
      <h1 className="mb-6 max-w-3xl text-5xl font-bold tracking-[-0.04em] text-foreground dark:text-white sm:text-6xl lg:text-[68px] lg:leading-[1.0] xl:text-[72px]">
        Tạo và đăng backlink
        <br />
        <span className="gradient-text">WordPress từ terminal.</span>
      </h1>

      {/* Subhead — mockup max-w-480px, 17px, leading 1.55 */}
      <p className="mb-9 max-w-[480px] text-[17px] leading-[1.55] text-foreground/70 dark:text-white/70">
        SEO automation cho team kỹ thuật Việt Nam. Type-safe API, OpenAPI spec, webhooks. Stop
        clicking dashboards — start shipping campaigns from your terminal.
      </p>

      {/* CTAs */}
      <div className="mb-9 flex flex-col gap-3 sm:flex-row">
        <Button
          asChild
          size="lg"
          className="rounded-full bg-cyan-300 text-slate-950 hover:bg-cyan-200"
        >
          {/* F6: botUrl() helper — no inline https://t.me/${username} */}
          <a href={botUrl()} target="_blank" rel="noopener noreferrer">
            Mở Telegram bot
            <ArrowRight className="ml-1 size-4" aria-hidden="true" />
          </a>
        </Button>
        <Button
          asChild
          size="lg"
          variant="outline"
          className="rounded-full border-foreground/20 text-foreground hover:bg-foreground/10 dark:border-white/20 dark:text-white dark:hover:bg-white/10"
        >
          <Link href="/login">Xem demo</Link>
        </Button>
      </div>

      {/* ⌘K hint chip */}
      <p className="text-xs text-foreground/50 dark:text-white/40">
        Bấm{" "}
        <kbd className="rounded border border-foreground/15 bg-foreground/5 px-1.5 py-0.5 font-mono text-[10px] dark:border-white/15 dark:bg-white/5">
          ⌘K
        </kbd>{" "}
        sau khi đăng nhập để mở bảng điều khiển nhanh.
      </p>
    </div>
  );
}
