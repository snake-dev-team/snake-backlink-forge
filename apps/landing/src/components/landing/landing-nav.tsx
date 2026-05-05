import { Code2 } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";

/**
 * Top navigation bar extracted from page.tsx for hero redesign (Phase 04).
 * Uses Phase 01 .glass-card token instead of inline backdrop-blur classes.
 */
export function LandingNav() {
  return (
    <nav className="glass-card flex items-center justify-between rounded-full px-4 py-3 shadow-2xl shadow-violet-950/30 md:px-5">
      <Link href="/" className="flex items-center gap-2 text-sm font-semibold tracking-tight">
        <span className="flex size-8 items-center justify-center rounded-full bg-cyan-300 text-slate-950">
          <Code2 className="size-4" aria-hidden="true" />
        </span>
        Snake Backlink Forge
      </Link>
      <div className="hidden items-center gap-6 text-sm text-foreground/85 dark:text-white/85 md:flex">
        <a href="#features" className="hover:text-primary dark:hover:text-cyan-200">
          Tính năng
        </a>
        <a href="#pricing" className="hover:text-primary dark:hover:text-cyan-200">
          Giá
        </a>
        <a href="#contact" className="hover:text-primary dark:hover:text-cyan-200">
          Liên hệ
        </a>
      </div>
      <Button asChild size="sm" className="rounded-full bg-white text-slate-950 hover:bg-cyan-100">
        <Link href="/login">Đăng nhập</Link>
      </Button>
    </nav>
  );
}
