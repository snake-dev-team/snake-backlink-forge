"use client";

import Link from "next/link";
import { type ReactNode, use } from "react";
import { CampaignProgressBar } from "@/components/campaigns/campaign-progress-bar";
import { JobStatusBadge } from "@/components/campaigns/job-status-badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useCampaignProgress } from "@/hooks/use-campaign-progress";

type Props = {
  params: Promise<{ id: string }>;
};

/**
 * Campaign detail page — polls /api/v1/campaigns/:id every 5 s via
 * useCampaignProgress hook. No force-dynamic needed: client component
 * owns all live data. SSR shell renders instantly; data streams in.
 */
export default function CampaignDetailPage({ params }: Props) {
  const { id } = use(params);
  const { campaign, jobs, isLoading, isPolling, error, refresh } = useCampaignProgress(id);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 animate-pulse rounded-md bg-muted" />
        <div className="h-32 animate-pulse rounded-lg bg-muted" />
      </div>
    );
  }

  if (error && !campaign) {
    return (
      <div className="space-y-4">
        <Link className="text-sm text-muted-foreground hover:underline" href="/campaigns">
          ← Campaigns
        </Link>
        <Card>
          <CardHeader>
            <CardTitle className="text-destructive">Không tải được campaign</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={refresh} size="sm" variant="outline">
              Retry
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!campaign) return null;

  const stats = {
    total: jobs.length,
    success: jobs.filter((j) => j.status === "success").length,
    failed: jobs.filter((j) => j.status === "failed").length,
    queued: jobs.filter((j) => j.status === "queued").length,
    in_progress: jobs.filter((j) => j.status === "in_progress").length,
  };
  const done = stats.success + stats.failed;

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2">
        <Link className="text-sm text-muted-foreground hover:underline" href="/campaigns">
          ← Campaigns
        </Link>
        {isPolling && (
          <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
            <span className="size-1.5 animate-pulse rounded-full bg-primary" />
            Live
          </span>
        )}
      </div>

      {/* Header */}
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">{campaign.name}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{campaign.money_site_url}</p>
      </div>

      {/* Progress + KPIs */}
      <div className="grid gap-4 sm:grid-cols-2">
        <Card className="sm:col-span-2">
          <CardContent className="pt-6 space-y-4">
            <CampaignProgressBar done={done} total={stats.total} />
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Kpi label="Total jobs" value={stats.total} />
              <Kpi label="Success" value={stats.success} highlight="emerald" />
              <Kpi label="Queued" value={stats.queued} />
              <Kpi label="In progress" value={stats.in_progress} highlight="primary" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Campaign meta */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Details</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2 text-sm sm:grid-cols-2">
          <MetaRow label="Status">
            <span className="rounded-full border px-2 py-0.5 text-xs font-medium uppercase tracking-wide">
              {campaign.status}
            </span>
          </MetaRow>
          <MetaRow label="Pool">{campaign.pool}</MetaRow>
          <MetaRow label="Daily limit">{campaign.daily_limit}</MetaRow>
          <MetaRow label="Credits">
            {campaign.credits_consumed}/{campaign.credits_allocated}
          </MetaRow>
          <MetaRow label="Keywords">
            <span className="flex flex-wrap gap-1">
              {campaign.niche_keywords.map((k) => (
                <span className="rounded bg-muted px-1.5 py-0.5 text-xs" key={k}>
                  {k}
                </span>
              ))}
            </span>
          </MetaRow>
          <MetaRow label="Created">{new Date(campaign.created_at).toLocaleString("vi-VN")}</MetaRow>
        </CardContent>
      </Card>

      {/* Error banner */}
      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Cập nhật thất bại: {error}.{" "}
          <button className="underline" onClick={refresh} type="button">
            Retry
          </button>
        </div>
      )}

      {/* Jobs table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Jobs ({jobs.length})</CardTitle>
          <CardDescription>Cập nhật mỗi 5 giây khi campaign đang chạy.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b bg-muted/40 text-muted-foreground">
                <tr>
                  <th className="p-3 font-medium">Target</th>
                  <th className="p-3 font-medium">Anchor</th>
                  <th className="p-3 font-medium">Status</th>
                  <th className="p-3 font-medium">Result</th>
                  <th className="p-3 font-medium">Created</th>
                </tr>
              </thead>
              <tbody>
                {jobs.length === 0 ? (
                  <tr>
                    <td className="p-3 text-muted-foreground" colSpan={5}>
                      Chưa có job nào.
                    </td>
                  </tr>
                ) : (
                  jobs.map((job) => (
                    <tr className="border-b last:border-0 hover:bg-muted/20" key={job.id}>
                      <td className="max-w-[200px] truncate p-3 font-mono text-xs">
                        {job.target_url}
                      </td>
                      <td className="max-w-[160px] truncate p-3">{job.anchor_text}</td>
                      <td className="p-3">
                        <JobStatusBadge status={job.status} />
                      </td>
                      <td className="max-w-[200px] truncate p-3 text-muted-foreground">
                        {job.result_url ? (
                          <a
                            className="text-primary hover:underline"
                            href={job.result_url}
                            rel="noopener noreferrer"
                            target="_blank"
                          >
                            {job.result_url}
                          </a>
                        ) : (
                          (job.error_message ?? "—")
                        )}
                      </td>
                      <td className="p-3 text-xs text-muted-foreground">
                        {new Date(job.created_at).toLocaleString("vi-VN")}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function Kpi({
  label,
  value,
  highlight,
}: {
  label: string;
  value: number;
  highlight?: "emerald" | "primary";
}) {
  const valueClass =
    highlight === "emerald"
      ? "text-emerald-500"
      : highlight === "primary"
        ? "text-primary"
        : "text-foreground";
  return (
    <div className="rounded-lg border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={`text-2xl font-semibold ${valueClass}`}>{value}</div>
    </div>
  );
}

function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className="w-28 shrink-0 text-muted-foreground">{label}</span>
      <span className="text-foreground">{children}</span>
    </div>
  );
}
