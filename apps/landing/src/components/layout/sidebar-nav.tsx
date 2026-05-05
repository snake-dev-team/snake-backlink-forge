"use client";

import { Globe2, LayoutDashboard, Sparkles } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/sites", label: "WordPress Sites", icon: Globe2 },
  { href: "/campaigns", label: "Campaigns", icon: Sparkles, comingSoon: true },
];

type SidebarNavProps = {
  className?: string;
  onNavigate?: () => void;
};

export function SidebarNav({ className, onNavigate }: SidebarNavProps) {
  const pathname = usePathname();

  return (
    <nav className={cn("flex flex-col gap-1 p-3", className)}>
      {navItems.map(({ href, label, icon: Icon, comingSoon }) => {
        const isActive = pathname?.startsWith(href) ?? false;

        if (comingSoon) {
          return (
            <div
              key={href}
              className="flex items-center gap-2 rounded-md px-3 py-2 text-sm opacity-50 cursor-not-allowed pointer-events-none transition-colors duration-150 border-l-2 border-l-transparent text-muted-foreground"
              aria-disabled="true"
            >
              <Icon className="h-4 w-4" />
              <span>{label}</span>
              <span className="ml-auto text-xs text-muted-foreground">Soon</span>
            </div>
          );
        }

        return (
          <Link
            key={href}
            href={href}
            onClick={onNavigate}
            className={cn(
              "flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors duration-150 border-l-2",
              isActive
                ? "bg-primary/10 border-l-primary text-foreground"
                : "border-l-transparent text-muted-foreground hover:bg-primary/5 hover:text-foreground",
            )}
          >
            <Icon className="h-4 w-4" />
            <span>{label}</span>
          </Link>
        );
      })}
    </nav>
  );
}
