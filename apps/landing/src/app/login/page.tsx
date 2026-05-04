import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { Suspense } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { LoginForm } from "./login-form";

/**
 * Login page — ambient parity with landing (Phase 6 UI B revamp).
 *
 * Layout layers:
 *   <PageShell decorative> — ambient mesh + 4 drifting orbs + grid + vignette (dark-only)
 *   ← back link            — top-left, returns to landing for hesitant users
 *   <main column>          — centered viewport, brand headline above card
 *   <LoginForm>             — wraps form in .glass-card-strong via className prop
 */
export default function LoginPage() {
  return (
    <PageShell decorative>
      {/* Back-to-landing link — top-left, escape hatch for users not yet committed */}
      <Link
        href="/"
        className="absolute left-5 top-5 inline-flex items-center gap-1.5 rounded-full border border-border/60 bg-background/40 px-3 py-1.5 text-xs font-medium text-muted-foreground backdrop-blur transition hover:border-border hover:text-foreground sm:left-8 sm:top-8"
      >
        <ArrowLeft className="size-3.5" aria-hidden="true" />
        Quay về trang chủ
      </Link>

      <div className="mx-auto flex min-h-screen w-full max-w-md flex-col items-center justify-center px-6 py-12">
        {/* Brand headline — gradient text matches landing hero */}
        <div className="mb-8 text-center">
          <h1 className="gradient-text text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
            Snake Backlink Forge
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">SEO automation cho người Việt</p>
        </div>

        <Suspense fallback={null}>
          <LoginForm />
        </Suspense>
      </div>
    </PageShell>
  );
}
