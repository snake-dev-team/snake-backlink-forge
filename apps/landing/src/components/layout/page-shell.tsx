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
          {/* Ambient mesh: 3 radial gradients (violet TL, cyan BR, amber center)
              dark-only — mirrors mockup body bg. Fixed full-viewport. */}
          <div
            className="pointer-events-none fixed inset-0 hidden bg-ambient-mesh dark:block"
            aria-hidden="true"
          />
          {/* Floating ambient orbs (dark-only, 4 drifting blurred discs).
              CSS animation respects prefers-reduced-motion via .orb rule. */}
          <div
            className="pointer-events-none fixed inset-0 hidden overflow-hidden dark:block"
            aria-hidden="true"
          >
            <div className="orb orb-violet" />
            <div className="orb orb-cyan" />
            <div className="orb orb-amber" />
            <div className="orb orb-violet-2" />
          </div>
          {/* Subtle grid pattern with radial mask (mockup signature). */}
          <div
            className="pointer-events-none fixed inset-0 bg-grid opacity-40"
            aria-hidden="true"
          />
          {/* Vignette darkens edges. Sits above grid + orbs, below content. */}
          <div className="pointer-events-none fixed inset-0 bg-vignette" aria-hidden="true" />
        </>
      )}
      <div className="relative z-[2]">{children}</div>
    </main>
  );
}
