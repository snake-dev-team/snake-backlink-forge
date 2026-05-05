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

type VerificationState =
  | { verified: true; anchorVerified: true }
  | { verified: true; anchorVerified: false }
  | { verified: false; attempts: number }
  | null;

type Props = {
  status: string;
  className?: string;
  /** Phase 7.07: optional verification data from the jobs row. */
  verified?: boolean | null;
  anchorVerified?: boolean | null;
  verificationAttempts?: number | null;
};

/**
 * Derives the verification state from the nullable DB fields.
 * Returns null when status is not 'success' or verification hasn't started.
 */
function getVerificationState(
  status: string,
  verified?: boolean | null,
  anchorVerified?: boolean | null,
  attempts?: number | null,
): VerificationState {
  if (status !== "success") return null;
  if (verified == null) return null; // not yet attempted
  if (verified === true && anchorVerified === true) {
    return { verified: true, anchorVerified: true };
  }
  if (verified === true && anchorVerified === false) {
    return { verified: true, anchorVerified: false };
  }
  return { verified: false, attempts: attempts ?? 0 };
}

/**
 * VerificationBadge — secondary pill showing post-publish verification result.
 * Only rendered for jobs with status='success' that have been verified.
 */
function VerificationBadge({ state }: { state: VerificationState }) {
  if (!state) return null;

  // Discriminated union narrowing on the `verified` field.
  if (!state.verified) {
    // URL unreachable — show retry progress or hard failure.
    const maxAttempts = 3;
    if (state.attempts < maxAttempts) {
      return (
        <span className="inline-flex items-center rounded-full border border-muted-foreground/30 bg-muted/30 px-2 py-0.5 text-xs font-medium text-muted-foreground uppercase tracking-wide">
          Verifying...
        </span>
      );
    }
    return (
      <span className="inline-flex items-center rounded-full border border-destructive/50 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive uppercase tracking-wide">
        Unreachable
      </span>
    );
  }

  // verified=true: check anchor presence.
  if (state.anchorVerified) {
    return (
      <span className="inline-flex items-center rounded-full border border-emerald-400/50 bg-emerald-400/10 px-2 py-0.5 text-xs font-medium text-emerald-400 uppercase tracking-wide">
        Verified
      </span>
    );
  }

  return (
    <span className="inline-flex items-center rounded-full border border-yellow-400/50 bg-yellow-400/10 px-2 py-0.5 text-xs font-medium text-yellow-400 uppercase tracking-wide">
      Live, anchor missing
    </span>
  );
}

/**
 * JobStatusBadge — color-coded pill for job lifecycle status.
 * When status=success and verification data is provided, also renders
 * a verification sub-badge (Phase 7.07).
 * Unknown statuses fall back to muted styling.
 */
export function JobStatusBadge({
  status,
  className,
  verified,
  anchorVerified,
  verificationAttempts,
}: Props) {
  const known = status as JobStatusValue;
  const styles = STATUS_STYLES[known] ?? STATUS_STYLES.skipped;
  const label = STATUS_LABEL[known] ?? status;

  const verState = getVerificationState(status, verified, anchorVerified, verificationAttempts);

  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        className={cn(
          "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium uppercase tracking-wide",
          styles,
          className,
        )}
      >
        {label}
      </span>
      <VerificationBadge state={verState} />
    </span>
  );
}
