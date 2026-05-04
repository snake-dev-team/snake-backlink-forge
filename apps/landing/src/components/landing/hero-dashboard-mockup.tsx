import type { LucideIcon } from "lucide-react";
import { CreditCard, Megaphone } from "lucide-react";

type MockKpi = { label: string; value: string; icon: LucideIcon };

/**
 * Hero right-bottom: 2 mock KPI cards at FULL opacity (F11).
 * Mirrors real /dashboard layout density — not 4 faded cards.
 * Conversion-relevant KPIs: credit balance + campaigns running.
 * aria-hidden: decorative mockup, real values live on /dashboard.
 *
 * Phase 07 ships 4-card grid on real dashboard. Hero shows 2 to avoid
 * density mismatch between hero mockup and actual /dashboard.
 */
const MOCK_KPIS: MockKpi[] = [
  { label: "Số dư credit", value: "12,500", icon: CreditCard },
  { label: "Campaigns running", value: "3", icon: Megaphone },
];

export function HeroDashboardMockup() {
  return (
    <div role="presentation" aria-hidden="true" className="grid grid-cols-2 gap-3">
      {MOCK_KPIS.map(({ label, value, icon: Icon }) => (
        <div key={label} className="glass-card p-4">
          <div className="flex items-center justify-between">
            <span className="truncate text-[11px] uppercase tracking-wider text-white/50">
              {label}
            </span>
            <Icon className="size-3.5 shrink-0 text-white/40" aria-hidden="true" />
          </div>
          <span className="mt-1 block font-mono text-xl font-semibold tabular-nums text-white/90">
            {value}
          </span>
        </div>
      ))}
    </div>
  );
}
