import type { ReactNode } from "react";
import { CommandPalette } from "@/components/layout/command-palette";
import { MobileNav } from "@/components/layout/mobile-nav";
import { SidebarNav } from "@/components/layout/sidebar-nav";
import { TopBar } from "@/components/layout/top-bar";

export const dynamic = "force-dynamic";

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <CommandPalette />
      <header className="sticky top-0 z-40 flex h-14 items-center border-b bg-background/95 px-4 backdrop-blur">
        <MobileNav className="md:hidden" />
        <span className="ml-2 font-semibold md:ml-0">Snake Backlink Forge</span>
        <div className="ml-auto">
          <TopBar />
        </div>
      </header>
      <div className="flex flex-1">
        <aside className="hidden w-56 border-r md:block">
          <SidebarNav />
        </aside>
        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
