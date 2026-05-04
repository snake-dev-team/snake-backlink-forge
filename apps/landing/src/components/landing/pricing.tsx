/**
 * Pricing — 3-tier comparison section with expandable full SKU table.
 *
 * Layout:
 *   section#pricing
 *   ├── header (eyebrow + h2)
 *   ├── 3-column tier grid (<PricingTierCard> × 3)
 *   └── <PricingTableCollapsible> — native <details> zero-JS collapse
 *
 * Representative tiers: standard_pro_200 / premium_pro_200 / combo_p200_s100.
 * Premium tier (middle) is highlighted with violet ring + "Phổ biến nhất" badge.
 */

import { PRICING_TIERS } from "@/lib/landing/pricing-data";
import { PricingTableCollapsible } from "./pricing-table-collapsible";
import { PricingTierCard } from "./pricing-tier-card";

export function Pricing() {
  return (
    <section id="pricing" className="space-y-10 py-20">
      {/* Section header */}
      <div className="space-y-3 text-center">
        <p className="text-sm uppercase tracking-[0.2em] text-cyan-300/80">Giá</p>
        <h2 className="mx-auto max-w-2xl text-balance text-4xl font-semibold tracking-tight">
          Chọn pool credit phù hợp với scale của bạn
        </h2>
      </div>

      {/* 3-tier grid — stacks on mobile, 3 columns on lg+ */}
      <div className="grid gap-6 lg:grid-cols-3">
        {PRICING_TIERS.map((tier) => (
          <PricingTierCard key={tier.id} tier={tier} />
        ))}
      </div>

      {/* Expandable full 10-SKU table — F8: summary has 44px touch target */}
      <PricingTableCollapsible />
    </section>
  );
}
