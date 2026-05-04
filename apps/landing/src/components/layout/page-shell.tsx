import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type PageShellProps = {
  children: ReactNode;
  className?: string;
  /** Apply decorative grid + vignette overlays. Default true for landing pages. */
  decorative?: boolean;
};

export function PageShell({ children, className, decorative = true }: PageShellProps) {
  return (
    <main
      id="main"
      tabIndex={-1}
      className={cn(
        "relative min-h-screen overflow-hidden bg-background text-foreground",
        className,
      )}
    >
      {decorative && (
        <>
          <div className="pointer-events-none fixed inset-0 bg-vignette" aria-hidden="true" />
          <div
            className="pointer-events-none fixed inset-0 bg-grid opacity-25"
            aria-hidden="true"
          />
        </>
      )}
      <div className="relative z-0">{children}</div>
    </main>
  );
}
