import { Suspense } from "react";
import { BalanceCard } from "@/components/dashboard/balance-card";
import { QuickActionsCard } from "@/components/dashboard/quick-actions-card";
import { RecentTxCard } from "@/components/dashboard/recent-tx-card";
import { Skeleton } from "@/components/ui/skeleton";

export const dynamic = "force-dynamic";

function CardSkeleton() {
  return <Skeleton className="h-40 w-full" />;
}

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm text-muted-foreground">Snake Backlink Forge</p>
        <h1 className="text-3xl font-semibold tracking-tight">Dashboard</h1>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Suspense fallback={<CardSkeleton />}>
          <BalanceCard />
        </Suspense>
        <Suspense fallback={<CardSkeleton />}>
          <RecentTxCard />
        </Suspense>
      </div>
      <QuickActionsCard />
    </div>
  );
}
