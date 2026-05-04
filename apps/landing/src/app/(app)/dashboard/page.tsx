import { CreditCard, Globe, TrendingUp, Zap } from "lucide-react";
import { Suspense } from "react";
import { KpiCard, KpiCardSkeleton } from "@/components/dashboard/kpi-card";
import { QuickActionsCard } from "@/components/dashboard/quick-actions-card";
import { RecentTxCard } from "@/components/dashboard/recent-tx-card";
import { fetchMeServer, fetchUsageServer } from "@/lib/api/server-fetch";

export const dynamic = "force-dynamic";

// ─── Static sparkline placeholder data (v1) ───────────────────────────────────
// Replace with real `usage_daily` Postgres query in phase 8+.
// Violet stroke = balance trend; amber = usage trend.
const BALANCE_TREND = [40, 38, 42, 45, 50, 55, 55];
const USAGE_TREND = [0, 5, 10, 8, 12, 12, 12];

// ─── KPI server components (each wraps fetchUsageServer / fetchMeServer) ──────
// React.cache() on fetchUsageServer deduplicates the HTTP call — all 3 usage
// cards share a single request per render. Verify via Fly access logs (NOT
// browser DevTools — server-side fetches are invisible there).

async function CreditBalanceCard() {
  try {
    const me = await fetchMeServer();
    return (
      <KpiCard
        label="Số dư credit"
        value={me.balance_credits.toLocaleString("vi-VN")}
        icon={CreditCard}
        sparklineData={BALANCE_TREND}
        sparklineStroke="rgb(167 139 250 / 0.7)"
      />
    );
  } catch {
    // Graceful degradation: sentinel "—" if backend unreachable.
    return <KpiCard label="Số dư credit" value="—" icon={CreditCard} />;
  }
}

async function SitesConnectedCard() {
  try {
    const usage = await fetchUsageServer();
    return <KpiCard label="Sites kết nối" value={usage.sites_connected} icon={Globe} />;
  } catch {
    return <KpiCard label="Sites kết nối" value="—" icon={Globe} />;
  }
}

async function CreditsConsumedCard() {
  try {
    const usage = await fetchUsageServer();
    return (
      <KpiCard
        label="Credits dùng tháng này"
        value={usage.credits_consumed_month.toLocaleString("vi-VN")}
        icon={TrendingUp}
        sparklineData={USAGE_TREND}
        sparklineStroke="rgb(251 191 36 / 0.7)"
      />
    );
  } catch {
    return <KpiCard label="Credits dùng tháng này" value="—" icon={TrendingUp} />;
  }
}

async function CampaignsRunningCard() {
  try {
    const usage = await fetchUsageServer();
    return <KpiCard label="Campaigns running" value={usage.campaigns_running} icon={Zap} />;
  } catch {
    return <KpiCard label="Campaigns running" value="—" icon={Zap} />;
  }
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm text-muted-foreground">Snake Backlink Forge</p>
        <h1 className="text-3xl font-semibold tracking-tight">Dashboard</h1>
      </div>

      {/* KPI grid: solid bg-card (NOT glass) — perf cap F7 */}
      <section aria-label="KPI" className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Suspense fallback={<KpiCardSkeleton />}>
          <CreditBalanceCard />
        </Suspense>
        <Suspense fallback={<KpiCardSkeleton />}>
          <SitesConnectedCard />
        </Suspense>
        <Suspense fallback={<KpiCardSkeleton />}>
          <CreditsConsumedCard />
        </Suspense>
        <Suspense fallback={<KpiCardSkeleton />}>
          <CampaignsRunningCard />
        </Suspense>
      </section>

      {/* Quick actions — glass-card surface */}
      <QuickActionsCard />

      {/* Recent transactions — glass-card, full-width (one heavy surface per scroll) */}
      <section aria-label="Recent transactions" className="w-full">
        <Suspense fallback={null}>
          <RecentTxCard />
        </Suspense>
      </section>
    </div>
  );
}
