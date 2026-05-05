import { cn } from "@/lib/utils";

type Props = {
  done: number;
  total: number;
  className?: string;
};

/**
 * CampaignProgressBar — animated gradient fill showing "done/total" progress.
 * Renders "12/20 backlinks done · 60%" with a filled bar.
 */
export function CampaignProgressBar({ done, total, className }: Props) {
  const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-baseline justify-between text-xs text-muted-foreground">
        <span>
          <span className="font-semibold text-foreground">{done}</span>/{total} backlinks done
        </span>
        <span className="tabular-nums font-medium text-foreground">{pct}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full transition-all duration-500 ease-out"
          style={{
            width: `${pct}%`,
            background: "linear-gradient(to right, hsl(var(--primary)), hsl(var(--primary) / 0.7))",
          }}
          role="progressbar"
          aria-valuenow={pct}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${pct}% complete`}
        />
      </div>
    </div>
  );
}
