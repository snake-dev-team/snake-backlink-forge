import { Globe2, LayoutDashboard, Sparkles } from "lucide-react";
import Link from "next/link";
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
  return (
    <nav className={cn("flex flex-col gap-1 p-3", className)}>
      {navItems.map(({ href, label, icon: Icon, comingSoon }) => (
        <Link
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition hover:bg-accent hover:text-accent-foreground"
          href={href}
          key={href}
          onClick={onNavigate}
        >
          <Icon className="h-4 w-4" />
          <span>{label}</span>
          {comingSoon ? <span className="ml-auto text-xs text-muted-foreground">Soon</span> : null}
        </Link>
      ))}
    </nav>
  );
}
