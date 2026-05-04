/**
 * PricingTierCard — single tier card primitive for the 3-tier pricing grid.
 *
 * Glass card surface with optional violet highlight ring for the "popular" tier.
 * CTA deep-links to Telegram bot with topup_<package_code> start param.
 */

import { Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatVnd } from "@/lib/format/currency";
import { findPackage, type PricingTier } from "@/lib/landing/pricing-data";
import { botUrl } from "@/lib/telegram/bot-url";
import { cn } from "@/lib/utils";

type Props = { tier: PricingTier };

export function PricingTierCard({ tier }: Props) {
  const pkg = findPackage(tier.representative);
  // Guard: should never fire with valid PRICING_TIERS, but avoids silent blank render
  if (!pkg) return null;

  return (
    <article
      className={cn(
        "glass-card flex flex-col gap-6 p-6",
        tier.highlighted && "ring-2 ring-violet-400/60 shadow-2xl shadow-violet-900/40",
      )}
    >
      {/* Popular badge — only on highlighted tier */}
      {tier.badge && (
        <span className="self-start rounded-full bg-violet-500/15 px-3 py-1 text-xs font-medium text-violet-200">
          {tier.badge}
        </span>
      )}

      {/* Tier name + price */}
      <div className="space-y-2">
        <h3 className="text-2xl font-semibold tracking-tight">{tier.name}</h3>
        <p className="text-sm text-white/55">
          {pkg.label} · {pkg.detail}
        </p>
        <div className="flex items-baseline gap-2 pt-2">
          <span className="font-mono text-4xl font-semibold tabular-nums">
            {formatVnd(pkg.priceVnd)}
          </span>
          <span className="text-sm text-white/50">/ gói</span>
        </div>
      </div>

      {/* Feature checklist */}
      <ul className="flex flex-1 flex-col gap-2 text-sm">
        {tier.features.map((f) => (
          <li key={f} className="flex items-start gap-2">
            <Check className="mt-0.5 size-4 shrink-0 text-cyan-300" aria-hidden="true" />
            <span className="text-foreground dark:text-white/90">{f}</span>
          </li>
        ))}
      </ul>

      {/* CTA — F6: botUrl() helper; deep-link with topup_<code> start param */}
      <Button
        asChild
        size="lg"
        className={cn(
          "rounded-full",
          tier.highlighted
            ? "bg-cyan-300 text-slate-950 hover:bg-cyan-200"
            : "bg-white/10 text-white hover:bg-white/20",
        )}
      >
        <a href={botUrl(`topup_${tier.representative}`)} target="_blank" rel="noopener noreferrer">
          Bắt đầu
        </a>
      </Button>
    </article>
  );
}
