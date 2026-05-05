"use client";

import { useEffect, useState } from "react";

export type PaletteTx = {
  id: string;
  package_code: string | null;
  amount_vnd: number;
  status: string;
  created_at: string;
};

/**
 * Fetches up to 100 recent transactions on first palette open.
 * Returns cached result on subsequent opens within the same browser session.
 * Client-side filter via cmdk handles search — no server pagination needed at this scale.
 *
 * Fetch path: /api/proxy/api/v1/transactions
 * The landing proxy strips "/api/proxy/" and forwards the remainder to the backend,
 * which mounts its API at /api/v1/... (confirmed: server-fetch.ts:99).
 */
export function useTransactionsCache(open: boolean) {
  const [data, setData] = useState<PaletteTx[] | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    // Only fetch on first open; null means "not yet fetched"
    if (!open || data !== null) return;

    setLoading(true);
    fetch("/api/proxy/api/v1/transactions?limit=100")
      .then((r) => (r.ok ? r.json() : Promise.resolve({ items: [] })))
      .then((j: { items?: PaletteTx[] }) => setData(j.items ?? []))
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  }, [open, data]);

  return { data: data ?? [], loading };
}
