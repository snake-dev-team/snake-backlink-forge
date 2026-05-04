import type { LucideIcon } from "lucide-react";
import { Sparkline } from "@/components/ui/sparkline";
import { cn } from "@/lib/utils";

type KpiCardProps = {
  label: string;
  value: string | number;
  icon: LucideIcon;
  /** Optional trend data; renders inline SVG sparkline at card bottom-right. */
  sparklineData?: number[];
  /** Stroke color for sparkline (CSS color or `currentColor`). */
  sparklineStroke?: string;
  className?: string;
};

// F7: KPI cards use solid bg-card (NOT glass). Dashboard renders 4 KPIs simultaneously;
// backdrop-blur on all 4 exceeds Snapdragon 7 cap of 3 simultaneous glass surfaces.
// Glass is reserved for wider main cards (RecentTxCard) where elevation matters more.
export function KpiCard({
  label,
  value,
  icon: Icon,
  sparklineData,
  sparklineStroke,
  className,
}: KpiCardProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-2 rounded-lg border border-border/50 bg-card p-4 transition-colors hover:bg-card/80",
        className,
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs uppercase tracking-wider text-muted-foreground">{label}</span>
        <Icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
      </div>
      <div className="flex items-end justify-between gap-3">
        <span className="font-mono text-2xl font-semibold tabular-nums">{value}</span>
        {sparklineData && (
          <Sparkline
            data={sparklineData}
            stroke={sparklineStroke ?? "currentColor"}
            height={28}
            width={80}
          />
        )}
      </div>
    </div>
  );
}

export function KpiCardSkeleton() {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border/50 bg-card p-4">
      <div className="flex items-center justify-between">
        <div className="h-3 w-20 animate-pulse rounded bg-muted/40" />
        <div className="h-4 w-4 animate-pulse rounded bg-muted/40" />
      </div>
      <div className="h-8 w-24 animate-pulse rounded bg-muted/40" />
    </div>
  );
}
