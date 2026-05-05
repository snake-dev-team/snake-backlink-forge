import { cn } from "@/lib/utils";

export type JobStatusValue =
  | "queued"
  | "content_ready"
  | "in_progress"
  | "dispatched"
  | "success"
  | "failed"
  | "skipped"
  | "dlq";

const STATUS_STYLES: Record<JobStatusValue, string> = {
  queued: "border-muted-foreground/30 bg-muted/40 text-muted-foreground",
  content_ready: "border-blue-400/40 bg-blue-400/10 text-blue-400",
  dispatched: "border-yellow-400/40 bg-yellow-400/10 text-yellow-400",
  in_progress: "border-primary/40 bg-primary/10 text-primary",
  success: "border-emerald-500/40 bg-emerald-500/10 text-emerald-500",
  failed: "border-destructive/40 bg-destructive/10 text-destructive",
  skipped: "border-muted-foreground/20 bg-muted/20 text-muted-foreground",
  dlq: "border-orange-500/40 bg-orange-500/10 text-orange-500",
};

const STATUS_LABEL: Record<JobStatusValue, string> = {
  queued: "Queued",
  content_ready: "Content ready",
  dispatched: "Dispatched",
  in_progress: "In progress",
  success: "Success",
  failed: "Failed",
  skipped: "Skipped",
  dlq: "DLQ",
};

type Props = {
  status: string;
  className?: string;
};

/**
 * JobStatusBadge — color-coded pill for job lifecycle status.
 * Unknown statuses fall back to muted styling.
 */
export function JobStatusBadge({ status, className }: Props) {
  const known = status as JobStatusValue;
  const styles = STATUS_STYLES[known] ?? STATUS_STYLES.skipped;
  const label = STATUS_LABEL[known] ?? status;

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium uppercase tracking-wide",
        styles,
        className,
      )}
    >
      {label}
    </span>
  );
}
