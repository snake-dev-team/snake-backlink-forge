import { cache } from "react";
import { getApiKeyCookie } from "@/lib/auth/cookies";

const BACKEND = process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://snake-backlink-api.fly.dev";
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

export type CampaignStats = {
  total: number;
  queued: number;
  dispatched: number;
  in_progress: number;
  success: number;
  failed: number;
  skipped: number;
};

export type Campaign = {
  id: string;
  name: string;
  money_site_url: string;
  niche_keywords: string[];
  anchor_texts: { text: string; type?: string; weight?: number }[];
  pool: string;
  source_mode: string;
  daily_limit: number;
  status: "draft" | "running" | "paused" | "completed" | "archived";
  ethical_mode: boolean;
  niche_filter: boolean;
  credits_allocated: number;
  credits_consumed: number;
  created_at: string;
  updated_at: string;
  stats?: CampaignStats;
};

export type CampaignsPage = {
  items: Campaign[];
};

export type CampaignJob = {
  id: string;
  campaign_id: string;
  target_url: string;
  anchor_text: string;
  anchor_type: string;
  status: string;
  pool: string;
  credits_cost: number;
  error_code?: string | null;
  error_message?: string | null;
  result_url?: string | null;
  created_at: string;
  completed_at?: string | null;
};

export type CampaignJobsPage = {
  items: CampaignJob[];
};

async function proxyFetch(path: string, init?: RequestInit): Promise<Response> {
  const key = await getApiKeyCookie();
  if (!key) {
    throw new Error("unauthorized");
  }
  return fetch(`${BACKEND}/api/v1${path}`, {
    ...init,
    headers: {
      Authorization: `Bearer ${key}`,
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

export type MeUsageResponse = {
  sites_connected: number;
  credits_consumed_month: number;
  campaigns_running: number;
};

export const fetchUsageServer = cache(async (): Promise<MeUsageResponse> => {
  if (E2E_API_MOCK) {
    return {
      sites_connected: 1,
      credits_consumed_month: 12,
      campaigns_running: 1,
    };
  }
  const response = await proxyFetch("/me/usage");
  if (!response.ok) {
    throw new Error("fetch_usage_failed");
  }
  return response.json() as Promise<MeUsageResponse>;
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

export async function fetchCampaignsServer(): Promise<CampaignsPage> {
  if (E2E_API_MOCK) {
    return {
      items: [
        {
          id: "00000000-0000-0000-0000-000000000101",
          name: "Money site launch",
          money_site_url: "https://example.com/seo-service",
          niche_keywords: ["seo", "backlink"],
          anchor_texts: [{ text: "dịch vụ seo", type: "partial", weight: 70 }],
          pool: "standard",
          source_mode: "custom",
          daily_limit: 5,
          status: "running",
          ethical_mode: true,
          niche_filter: true,
          credits_allocated: 20,
          credits_consumed: 4,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          stats: {
            total: 8,
            queued: 4,
            dispatched: 0,
            in_progress: 1,
            success: 3,
            failed: 0,
            skipped: 0,
          },
        },
      ],
    };
  }

  const response = await proxyFetch("/campaigns");
  if (!response.ok) {
    throw new Error("fetch_campaigns_failed");
  }
  return response.json();
}

export async function fetchCampaignJobsServer(campaignId: string): Promise<CampaignJobsPage> {
  if (E2E_API_MOCK) {
    return { items: [] };
  }

  const response = await proxyFetch(`/campaigns/${campaignId}/jobs?limit=20`);
  if (!response.ok) {
    throw new Error("fetch_campaign_jobs_failed");
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
