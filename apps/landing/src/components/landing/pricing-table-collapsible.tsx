/**
 * PricingTableCollapsible — native <details> collapsible revealing full 10-SKU table.
 *
 * Zero client JS — uses native browser <details>/<summary> for SSR/SEO-friendly collapse.
 * F8: <summary> has min-h-11 px-4 py-3 for 44px WCAG 2.5.5 touch target compliance.
 * a11y: browser manages aria-expanded on <summary> automatically; no explicit attr needed.
 * Chevron rotation uses CSS group-open: — no JS state required.
 */

import { ChevronDown } from "lucide-react";
import { formatVnd } from "@/lib/format/currency";
import { LANDING_PACKAGES } from "@/lib/landing/pricing-data";

const POOL_LABEL: Record<string, string> = {
  standard: "Standard",
  premium: "Premium",
  combo: "Combo",
};

export function PricingTableCollapsible() {
  return (
    <details className="group mx-auto max-w-4xl">
      {/*
        F8 mandate: min-h-11 (44px) + px-4 py-3 for WCAG 2.5.5 touch target.
        sm:py-2 reduces vertical padding on larger screens where pointer precision is higher.
      */}
      <summary className="flex min-h-11 cursor-pointer list-none items-center justify-center gap-2 rounded-md px-4 py-3 text-sm text-cyan-200 hover:text-cyan-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 [&::-webkit-details-marker]:hidden sm:py-2">
        Xem 10 gói chi tiết
        <ChevronDown
          className="size-4 transition-transform group-open:rotate-180"
          aria-hidden="true"
        />
      </summary>

      {/* Full 10-SKU table — verbatim DisplayVI + canonical VND from pricing-data.ts */}
      <div className="glass-card mt-4 overflow-hidden">
        <table className="w-full border-collapse text-sm">
          <thead className="border-b border-white/10 bg-white/[0.03] text-left">
            <tr>
              <th scope="col" className="px-4 py-3 font-medium text-white/70">
                Mã gói
              </th>
              <th scope="col" className="px-4 py-3 font-medium text-white/70">
                Pool
              </th>
              <th scope="col" className="px-4 py-3 font-medium text-white/70">
                Mô tả
              </th>
              <th scope="col" className="px-4 py-3 text-right font-medium text-white/70">
                Giá VND
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-white/5">
            {LANDING_PACKAGES.map((p) => (
              <tr key={p.code} className="hover:bg-white/[0.02]">
                <td className="px-4 py-3 font-mono text-xs text-white/60">{p.code}</td>
                <td className="px-4 py-3 text-white/75">{POOL_LABEL[p.pool] ?? p.pool}</td>
                <td className="px-4 py-3 text-white/75">{p.label}</td>
                <td className="px-4 py-3 text-right font-mono tabular-nums text-white/85">
                  {formatVnd(p.priceVnd)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}
