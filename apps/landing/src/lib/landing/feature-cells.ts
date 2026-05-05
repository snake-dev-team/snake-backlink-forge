import type { LucideIcon } from "lucide-react";
import { Bot, Globe, KeyRound, QrCode, Shield, Wallet } from "lucide-react";

export type BentoSize = "large" | "medium" | "wide";

export type FeatureCell = {
  id: string;
  size: BentoSize;
  icon: LucideIcon;
  title: string;
  description: string;
  /** Optional inline code snippet rendered as mini CodeWindow */
  snippet?: { lang: "bash" | "json"; code: string; title: string };
};

// API_BASE_URL is `https://snake-backlink-api.fly.dev` today; will move to a custom
// domain post-launch. Do NOT inline a fake api.sbf.io — that domain is unowned
// (security risk: token theft via copy-paste if a dev later uses real keys).
// biome-ignore lint/suspicious/noTemplateCurlyInString: literal placeholder string for code-window display, not a template literal
const API_BASE_URL = "${API_BASE_URL}";

export const FEATURE_CELLS: FeatureCell[] = [
  {
    id: "telegram-payment",
    size: "large",
    icon: Bot,
    title: "Thanh toán Telegram",
    description:
      "Nạp credit qua Telegram bot với SePay VietQR. Không cần tài khoản ngân hàng quốc tế.",
  },
  {
    id: "wordpress-connect",
    size: "large",
    icon: Globe,
    title: "Kết nối WordPress an toàn",
    description:
      "Application Password + REST API validation. Quyền publish_posts được kiểm trước khi lưu.",
  },
  {
    id: "sepay-vietqr",
    size: "medium",
    icon: QrCode,
    title: "SePay VietQR",
    description: "Quét QR, ghi có credit instant. Webhook reconcile mọi transaction.",
  },
  {
    id: "anti-abuse",
    size: "medium",
    icon: Shield,
    title: "Anti-abuse mặc định",
    description: "Daily limit, ethical mode, footprint randomization bật sẵn để tránh spam.",
  },
  {
    id: "multi-pool",
    size: "medium",
    icon: Wallet,
    title: "Multi-pool credit",
    description: "Tách Standard và Premium pool. Audit riêng từng nguồn credit.",
  },
  {
    id: "api-auth",
    size: "wide",
    icon: KeyRound,
    title: "API key authentication",
    description: "Bearer token qua header. Rotate key bất kỳ lúc nào từ Telegram bot.",
    snippet: {
      lang: "bash",
      title: "request",
      code: `curl ${API_BASE_URL}/api/v1/me \\\n  -H "Authorization: Bearer sbf_live_..."`,
    },
  },
];
