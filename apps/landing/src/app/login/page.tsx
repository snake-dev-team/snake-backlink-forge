import { ArrowLeft, Bot, Terminal } from "lucide-react";
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

        {/* CLI hint — Developer Tool aesthetic; helps users find their API key */}
        <div className="mt-6 w-full overflow-hidden rounded-md border border-white/10 bg-card/40 backdrop-blur-md">
          <div className="flex items-center gap-2 border-b border-white/5 bg-white/[0.02] px-3 py-2">
            <span className="flex gap-1.5" aria-hidden="true">
              <span className="size-2.5 rounded-full bg-red-500/70" />
              <span className="size-2.5 rounded-full bg-yellow-500/70" />
              <span className="size-2.5 rounded-full bg-green-500/70" />
            </span>
            <Terminal className="ml-1 size-3.5 text-muted-foreground" aria-hidden="true" />
            <span className="font-mono text-[11px] text-muted-foreground">how to get key</span>
          </div>
          <div className="space-y-2 px-4 py-3 font-mono text-xs">
            <div className="flex items-start gap-2">
              <span className="text-violet-400">$</span>
              <span className="text-foreground/85">
                Mở{" "}
                <a
                  href="https://t.me/SnakeBacklinkForgeBot"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-cyan-300 underline-offset-2 hover:underline"
                >
                  <Bot className="size-3" aria-hidden="true" />
                  @SnakeBacklinkForgeBot
                </a>
              </span>
            </div>
            <div className="flex items-start gap-2">
              <span className="text-violet-400">$</span>
              <span className="text-foreground/85">
                Gõ <span className="rounded bg-cyan-300/15 px-1.5 py-0.5 text-cyan-300">/key</span>{" "}
                → bot trả về <span className="text-amber-300">sbf_live_...</span>
              </span>
            </div>
            <div className="flex items-start gap-2">
              <span className="text-violet-400">$</span>
              <span className="text-foreground/85">Dán key bên trên → đăng nhập</span>
            </div>
          </div>
        </div>
      </div>
    </PageShell>
  );
}
