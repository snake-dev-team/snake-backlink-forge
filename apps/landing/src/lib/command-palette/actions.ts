/**
 * Static command palette action registry.
 * All navigation targets and external links are defined here —
 * no user-controllable redirect targets (security: closed set of payloads).
 */

import type { LucideIcon } from "lucide-react";
import { Globe, LayoutDashboard, LogOut, Megaphone, MessageCircle } from "lucide-react";
import { botUrl } from "@/lib/telegram/bot-url";

export type CommandActionKind = "navigate" | "external" | "logout";

export type CommandAction = {
  id: string;
  label: string;
  kind: CommandActionKind;
  icon: LucideIcon;
  /** navigate: pathname; external: full URL; logout: unused */
  payload?: string;
  shortcut?: string;
};

export const STATIC_ACTIONS: CommandAction[] = [
  {
    id: "nav-dashboard",
    label: "Đi tới Dashboard",
    kind: "navigate",
    icon: LayoutDashboard,
    payload: "/dashboard",
  },
  {
    id: "nav-sites",
    label: "Đi tới Sites",
    kind: "navigate",
    icon: Globe,
    payload: "/sites",
  },
  {
    id: "nav-campaigns",
    label: "Đi tới Campaigns",
    kind: "navigate",
    icon: Megaphone,
    payload: "/campaigns",
  },
  {
    id: "ext-telegram",
    label: "Mở Telegram bot",
    kind: "external",
    icon: MessageCircle,
    payload: botUrl(),
  },
  {
    id: "logout",
    label: "Đăng xuất",
    kind: "logout",
    icon: LogOut,
  },
];
