/**
 * CtaSection — single conversion banner between pricing and footer.
 *
 * Uses .glass-card-strong for visual elevation above surrounding sections.
 * F6: all Telegram links via botUrl() helper — no inline template literals.
 */

import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { botUrl } from "@/lib/telegram/bot-url";

const SUPPORT_EMAIL = process.env.NEXT_PUBLIC_SUPPORT_EMAIL ?? "support@snakepremiumhub.com";

export function CtaSection() {
  return (
    <section className="py-16">
      <div className="glass-card-strong mx-auto max-w-4xl space-y-6 p-10 text-center sm:p-14">
        <h2 className="gradient-text text-balance text-4xl font-semibold tracking-tight">
          Nâng cấp SEO của bạn ngay
        </h2>
        <p className="mx-auto max-w-xl text-foreground/70 dark:text-white/65">
          Mở Telegram bot, nạp credit và kết nối WordPress site đầu tiên trong 60 giây.
        </p>

        <div className="flex flex-col items-center justify-center gap-3 sm:flex-row">
          {/* Primary CTA — F6: botUrl() with "start" param */}
          <Button
            asChild
            size="lg"
            className="rounded-full bg-cyan-300 text-slate-950 hover:bg-cyan-200"
          >
            <a href={botUrl("start")} target="_blank" rel="noopener noreferrer">
              Bắt đầu free
              <ArrowRight className="ml-1 size-4" aria-hidden="true" />
            </a>
          </Button>

          {/* Secondary CTA — mailto, no JS */}
          <Button
            asChild
            size="lg"
            variant="outline"
            className="rounded-full border-foreground/20 text-foreground hover:bg-foreground/10 dark:border-white/20 dark:text-white dark:hover:bg-white/10 dark:hover:text-white"
          >
            <a href={`mailto:${SUPPORT_EMAIL}`}>Liên hệ hỗ trợ</a>
          </Button>
        </div>
      </div>
    </section>
  );
}
