import { cache } from "react";
import { getApiKeyCookie } from "@/lib/auth/cookies";

const APP_ORIGIN = process.env.NEXT_PUBLIC_APP_URL ?? "http://localhost:3000";
const COOKIE_NAME = process.env.NODE_ENV === "development" ? "sbf_key" : "__Host-sbf_key";
const E2E_API_MOCK = process.env.E2E_AUTH_MOCK === "1";

export type MeResponse = {
  user_id: string;
  telegram_id: number;
  telegram_username?: string | null;
  language: string;
  is_verified: boolean;
  balance_credits: number;
  premium_credits: number;
  standard_credits: number;
  key_prefix: string;
};

export type TransactionsPage = {
  items: Record<string, unknown>[];
  limit: number;
  offset: number;
};

export type WpSite = {
  id: string;
  base_url: string;
  app_username: string;
  label: string;
  status: "pending" | "connected" | "error";
  last_validated_at?: string | null;
  last_error?: string | null;
  created_at: string;
  updated_at: string;
};

export type WpSitesPage = {
  items: WpSite[];
};

async function proxyFetch(path: string, init?: RequestInit): Promise<Response> {
  const key = await getApiKeyCookie();
  if (!key) {
    throw new Error("unauthorized");
  }
  return fetch(`${APP_ORIGIN}/api/proxy/api/v1${path}`, {
    ...init,
    headers: {
      Cookie: `${COOKIE_NAME}=${encodeURIComponent(key)}`,
      ...init?.headers,
    },
    cache: "no-store",
  });
}

export const fetchMeServer = cache(async (): Promise<MeResponse> => {
  if (E2E_API_MOCK) {
    return {
      user_id: "00000000-0000-0000-0000-000000000001",
      telegram_id: 990000000001,
      telegram_username: "phase8_e2e",
      language: "vi",
      is_verified: true,
      balance_credits: 42,
      premium_credits: 7,
      standard_credits: 35,
      key_prefix: "sbf_live_pha",
    };
  }

  const response = await proxyFetch("/me");
  if (!response.ok) {
    throw new Error("fetch_me_failed");
  }
  return response.json();
});

export async function fetchTransactionsServer(limit = 5): Promise<TransactionsPage> {
  if (E2E_API_MOCK) {
    return { items: [], limit, offset: 0 };
  }

  const response = await proxyFetch(`/transactions?limit=${limit}`);
  if (!response.ok) {
    throw new Error("fetch_transactions_failed");
  }
  return response.json();
}

export async function fetchWpSitesServer(): Promise<WpSitesPage> {
  if (E2E_API_MOCK) {
    return { items: [] };
  }

  const response = await proxyFetch("/wp-sites");
  if (!response.ok) {
    throw new Error("fetch_wp_sites_failed");
  }
  return response.json();
}

export async function postProxyServer(path: string, payload?: unknown): Promise<Response> {
  return proxyFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
}

export async function deleteProxyServer(path: string): Promise<Response> {
  return proxyFetch(path, { method: "DELETE" });
}
