import { CreditCard, Globe, TrendingUp, Zap } from "lucide-react";
import { Suspense } from "react";
import { KpiCard, KpiCardSkeleton } from "@/components/dashboard/kpi-card";
import { QuickActionsCard } from "@/components/dashboard/quick-actions-card";
import { RecentTxCard } from "@/components/dashboard/recent-tx-card";
import { fetchMeServer, fetchUsageServer } from "@/lib/api/server-fetch";

export const dynamic = "force-dynamic";

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
      />
    );
  } catch {
    // Graceful degradation: sentinel "—" if backend unreachable (matches BalanceCard pattern).
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

      {/* KPI grid: 1-col mobile → 2-col sm → 4-col lg */}
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

      <QuickActionsCard />

      {/* Recent transactions — full-width below KPI grid */}
      <section aria-label="Recent transactions" className="w-full">
        <Suspense fallback={null}>
          <RecentTxCard />
        </Suspense>
      </section>
    </div>
  );
}
