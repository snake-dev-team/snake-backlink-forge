import { ArrowRight, Bot } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { botUrl } from "@/lib/telegram/bot-url";

/**
 * Hero left column: badge chip, gradient headline, subhead, 2 CTAs, ⌘K hint.
 * F6: uses botUrl() helper — no inline t.me template literals.
 * F10: uses .gradient-text CSS class — no Tailwind bg-clip-text utilities.
 */
export function HeroText() {
  return (
    <div className="space-y-7">
      {/* Badge chip — violet chip white text (avoids cyan-on-cyan contrast fail) */}
      <div className="inline-flex items-center gap-2 rounded-full border border-violet-500/40 bg-violet-500/20 px-3 py-1 text-sm text-white">
        <Bot className="size-4" aria-hidden="true" />
        SEO automation stack cho team thích dashboard rõ ràng
      </div>

      {/* Headline + subhead */}
      <div className="space-y-5">
        <h1 className="max-w-3xl text-5xl font-semibold tracking-[-0.05em] sm:text-6xl lg:text-[64px] lg:leading-[1.05]">
          {/* .gradient-text from Phase 01 globals.css — violet→cyan→amber with -webkit- prefix + @supports fallback */}
          <span className="gradient-text">Tạo, kiểm soát và đăng backlink</span>
          <span className="text-foreground dark:text-white"> WordPress từ một cockpit.</span>
        </h1>
        <p className="max-w-xl text-lg leading-8 text-foreground/75 dark:text-white/70">
          Snake Backlink Forge gom credit Telegram, AI content và WordPress publishing vào một luồng
          vận hành gọn cho SEO operator Việt Nam.
        </p>
      </div>

      {/* CTAs */}
      <div className="flex flex-col gap-3 sm:flex-row">
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
