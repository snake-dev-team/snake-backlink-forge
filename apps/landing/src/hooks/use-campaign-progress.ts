"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Campaign, CampaignJob } from "@/lib/api/server-fetch";

// POLL_INTERVAL_MS: 5 s between fetches, matching spec.
const POLL_INTERVAL_MS = 5_000;

// Terminal campaign statuses where polling must stop.
const TERMINAL_STATUSES = new Set<string>(["completed", "archived"]);

type ProgressData = {
  campaign: Campaign | null;
  jobs: CampaignJob[];
};

type UseCampaignProgressResult = ProgressData & {
  isLoading: boolean;
  isPolling: boolean;
  error: string | null;
  /** Manual refresh — resets error and fires an immediate fetch. */
  refresh: () => void;
};

/**
 * useCampaignProgress — polls GET /api/v1/campaigns/:id and
 * GET /api/v1/campaigns/:id/jobs every 5 s.
 *
 * Polling stops automatically when campaign.status reaches a terminal state
 * (completed | archived). Re-enables if campaignId changes.
 *
 * Uses native fetch via the Next.js proxy route (/api/proxy/...) so auth
 * cookies are forwarded automatically — same pattern as postProxyServer.
 */
export function useCampaignProgress(campaignId: string): UseCampaignProgressResult {
  const [data, setData] = useState<ProgressData>({ campaign: null, jobs: [] });
  const [isLoading, setIsLoading] = useState(true);
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Ref tracks whether polling should continue. Avoids stale closure in interval.
  const shouldPollRef = useRef(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchOnce = useCallback(async (): Promise<boolean> => {
    // Returns true when polling should stop (terminal status or hard error).
    try {
      // Proxy route: /api/proxy/<path> → BACKEND/<path>.
      // Backend API lives under /api/v1, so full path must include it.
      const [campRes, jobsRes] = await Promise.all([
        fetch(`/api/proxy/api/v1/campaigns/${campaignId}`, { credentials: "include" }),
        fetch(`/api/proxy/api/v1/campaigns/${campaignId}/jobs?limit=100`, {
          credentials: "include",
        }),
      ]);

      if (!campRes.ok) {
        setError(`api_${campRes.status}`);
        // Stop polling on 401/403/404 — retrying won't help.
        return campRes.status === 401 || campRes.status === 403 || campRes.status === 404;
      }

      const campBody = (await campRes.json()) as { item?: Campaign };
      const camp = campBody.item ?? null;

      let jobs: CampaignJob[] = [];
      if (jobsRes.ok) {
        const jobsBody = (await jobsRes.json()) as { items?: CampaignJob[] };
        jobs = jobsBody.items ?? [];
      }

      setData({ campaign: camp, jobs });
      setError(null);

      // Stop polling once campaign reaches a terminal state.
      return camp !== null && TERMINAL_STATUSES.has(camp.status);
    } catch (err) {
      setError(err instanceof Error ? err.message : "fetch_failed");
      return false; // transient network error — keep polling
    }
  }, [campaignId]);

  const scheduleNext = useCallback(() => {
    timerRef.current = setTimeout(async () => {
      if (!shouldPollRef.current) return;
      setIsPolling(true);
      const done = await fetchOnce();
      setIsPolling(false);
      if (!done && shouldPollRef.current) {
        scheduleNext();
      }
    }, POLL_INTERVAL_MS);
  }, [fetchOnce]);

  const refresh = useCallback(() => {
    // Cancel any pending timer, clear error, immediately re-fetch + restart polling.
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setError(null);
    setIsLoading(true);
    fetchOnce().then((done) => {
      setIsLoading(false);
      if (!done && shouldPollRef.current) {
        scheduleNext();
      }
    });
  }, [fetchOnce, scheduleNext]);

  useEffect(() => {
    shouldPollRef.current = true;
    setData({ campaign: null, jobs: [] });
    setError(null);
    setIsLoading(true);
    setIsPolling(false);

    let cancelled = false;

    fetchOnce().then((done) => {
      if (cancelled) return;
      setIsLoading(false);
      if (!done) {
        scheduleNext();
      }
    });

    return () => {
      cancelled = true;
      shouldPollRef.current = false;
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
    // fetchOnce identity transitively depends on campaignId — when the id changes,
    // fetchOnce is recreated and this effect re-fires. No need to list campaignId directly.
  }, [fetchOnce, scheduleNext]);

  return { ...data, isLoading, isPolling, error, refresh };
}
