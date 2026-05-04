/**
 * Footer — 3-column minimal footer (Sản phẩm / Tài nguyên / Liên hệ).
 *
 * F6: Telegram link via botUrl() helper — no inline template literals.
 * F15: /legal/privacy and /legal/tos links point to existing stub pages.
 * Status indicator: green dot + "Tất cả hệ thống hoạt động bình thường".
 */

import Link from "next/link";
import { botUrl } from "@/lib/telegram/bot-url";

const SUPPORT_EMAIL = process.env.NEXT_PUBLIC_SUPPORT_EMAIL ?? "support@snakepremiumhub.com";

const GITHUB_URL =
  process.env.NEXT_PUBLIC_GITHUB_URL ?? "https://github.com/HermeticUS/tool_backlink";

type NavLink = {
  label: string;
  href: string;
  external?: boolean;
};

type FooterColumn = {
  heading: string;
  links: NavLink[];
};

const COLUMNS: FooterColumn[] = [
  {
    heading: "Sản phẩm",
    links: [
      { label: "Tính năng", href: "/#features" },
      { label: "API", href: "/#api" },
      { label: "Pricing", href: "/#pricing" },
    ],
  },
  {
    heading: "Tài nguyên",
    links: [
      { label: "GitHub", href: GITHUB_URL, external: true },
      {
        label: "Status",
        href: "https://snake-backlink-api.fly.dev/healthz",
        external: true,
      },
      // F15: stub pages exist at /legal/privacy and /legal/tos — no 404
      { label: "Privacy", href: "/legal/privacy" },
      { label: "Điều khoản", href: "/legal/tos" },
    ],
  },
  {
    heading: "Liên hệ",
    links: [
      // F6: botUrl() — no inline https://t.me/${username}
      { label: "Telegram bot", href: botUrl(), external: true },
      { label: SUPPORT_EMAIL, href: `mailto:${SUPPORT_EMAIL}` },
    ],
  },
];

export function Footer() {
  return (
    <footer className="mt-12 border-t border-foreground/10 py-12 dark:border-white/10">
      {/* 3-column grid — stacks on mobile */}
      <div className="grid gap-10 sm:grid-cols-3">
        {COLUMNS.map((col) => (
          <div key={col.heading} className="space-y-3">
            <p className="text-sm font-semibold uppercase tracking-wider text-foreground/60 dark:text-white/50">
              {col.heading}
            </p>
            <ul className="space-y-2 text-sm">
              {col.links.map((link) =>
                link.external ? (
                  <li key={link.label}>
                    <a
                      href={link.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-foreground/90 hover:text-primary dark:text-white/85 dark:hover:text-cyan-200"
                    >
                      {link.label}
                    </a>
                  </li>
                ) : (
                  <li key={link.label}>
                    <Link
                      href={link.href}
                      className="text-foreground/90 hover:text-primary dark:text-white/85 dark:hover:text-cyan-200"
                    >
                      {link.label}
                    </Link>
                  </li>
                ),
              )}
            </ul>
          </div>
        ))}
      </div>

      {/* Status indicator */}
      <div className="mt-10 flex flex-col items-center gap-4 sm:flex-row sm:justify-between">
        <p className="flex items-center gap-2 text-xs text-foreground/50 dark:text-white/40">
          <span className="inline-block size-2 rounded-full bg-emerald-400" aria-hidden="true" />
          Tất cả hệ thống hoạt động bình thường
        </p>
        <p className="text-xs text-foreground/45 dark:text-white/35">
          © 2026 Snake Backlink Forge · SEO automation cho operator Việt Nam
        </p>
      </div>
    </footer>
  );
}
