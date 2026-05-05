import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  type Campaign,
  fetchCampaignJobsServer,
  fetchCampaignsServer,
} from "@/lib/api/server-fetch";
import { campaignStatusAction, enqueueCampaignAction } from "./actions";
import { CampaignCreateForm } from "./campaign-create-form";

export const dynamic = "force-dynamic";

type CampaignsPageProps = {
  searchParams?: Promise<{ status?: string; error?: string }>;
};

export default async function CampaignsPage({ searchParams }: CampaignsPageProps) {
  const params = await searchParams;
  const status = params?.status;
  const error = params?.error;
  let items: Campaign[] | null = null;
  try {
    items = (await fetchCampaignsServer()).items;
  } catch {
    items = null;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2">
        <p className="text-sm text-muted-foreground">Phase 3 automation</p>
        <h1 className="text-3xl font-semibold tracking-tight">Campaigns</h1>
        <p className="max-w-2xl text-muted-foreground">
          Điều phối backlink jobs, target queue và extension handoff từ dashboard.
        </p>
      </div>

      <CampaignStatusMessage error={error} status={status} />
      <CampaignCreateForm />

      {items === null ? (
        <CampaignsUnavailableCard />
      ) : items.length === 0 ? (
        <EmptyCampaignsCard />
      ) : (
        <CampaignList campaigns={items} />
      )}
    </div>
  );
}

function CampaignStatusMessage({ error, status }: { error?: string; status?: string }) {
  const errorMessage =
    error === "start_failed"
      ? "Không thể chạy campaign. Kiểm tra lại phiên đăng nhập hoặc trạng thái backend."
      : error === "pause_failed"
        ? "Không thể tạm dừng campaign lúc này."
        : error === "archive_failed"
          ? "Không thể archive campaign lúc này."
          : error === "enqueue_failed"
            ? "Không thể enqueue jobs mới cho campaign."
            : null;

  if (errorMessage) {
    return (
      <div className="rounded-md border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
        {errorMessage}
      </div>
    );
  }

  const successMessage =
    status === "created"
      ? "Đã tạo campaign."
      : status === "started"
        ? "Campaign đã chạy."
        : status === "paused"
          ? "Campaign đã tạm dừng."
          : status === "archived"
            ? "Campaign đã archive."
            : status === "enqueued"
              ? "Đã enqueue jobs mới."
              : null;
  if (!successMessage) {
    return null;
  }
  return (
    <div className="rounded-md border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700 dark:text-emerald-300">
      {successMessage}
    </div>
  );
}

function CampaignsUnavailableCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Chưa tải được campaigns</CardTitle>
        <CardDescription>
          Backend automation chưa sẵn sàng hoặc phiên đăng nhập cần làm mới.
        </CardDescription>
      </CardHeader>
    </Card>
  );
}

function EmptyCampaignsCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Chưa có campaign</CardTitle>
        <CardDescription>Tạo campaign đầu tiên để sinh queue cho extension/worker.</CardDescription>
      </CardHeader>
    </Card>
  );
}

// CampaignList fetches all campaign job lists in parallel (Promise.all) to avoid
// N+1 sequential server round-trips — one concurrent fetch per campaign (F21).
async function CampaignList({ campaigns }: { campaigns: Campaign[] }) {
  const jobResults = await Promise.all(
    campaigns.map((c) => fetchCampaignJobsServer(c.id).catch(() => null)),
  );
  return (
    <div className="grid gap-4">
      {campaigns.map((campaign, i) => (
        <CampaignCard campaign={campaign} jobsPage={jobResults[i]} key={campaign.id} />
      ))}
    </div>
  );
}

type CampaignJobsPage = {
  items: {
    id: string;
    target_url: string;
    anchor_text: string;
    status: string;
    result_url?: string | null;
    error_message?: string | null;
  }[];
};

function CampaignCard({
  campaign,
  jobsPage,
}: {
  campaign: Campaign;
  jobsPage: CampaignJobsPage | null;
}) {
  const jobs = jobsPage ?? { items: [] };
  const jobsError = jobsPage === null;

  const stats = campaign.stats;
  const progress =
    stats && stats.total > 0
      ? Math.round(((stats.success + stats.failed + stats.skipped) / stats.total) * 100)
      : 0;

  return (
    <Card className="overflow-hidden">
      <CardHeader className="gap-3 sm:flex-row sm:items-start sm:justify-between sm:space-y-0">
        <div>
          <CardTitle className="text-xl">{campaign.name}</CardTitle>
          <CardDescription>{campaign.money_site_url}</CardDescription>
        </div>
        <span className="w-fit rounded-full border px-3 py-1 text-xs font-medium uppercase tracking-wide">
          {campaign.status}
        </span>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="grid gap-3 md:grid-cols-4">
          <Metric label="Jobs" value={stats?.total ?? 0} />
          <Metric label="Success" value={stats?.success ?? 0} />
          <Metric label="Queued" value={stats?.queued ?? 0} />
          <Metric
            label="Credits"
            value={`${campaign.credits_consumed}/${campaign.credits_allocated}`}
          />
        </div>
        <div>
          <div className="mb-2 flex justify-between text-xs text-muted-foreground">
            <span>Completion</span>
            <span>{progress}%</span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary" style={{ width: `${progress}%` }} />
          </div>
        </div>
        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
          {campaign.niche_keywords.map((keyword) => (
            <span className="rounded-full bg-muted px-2 py-1" key={keyword}>
              {keyword}
            </span>
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          <CampaignAction id={campaign.id} action="start" label="Start" />
          <CampaignAction id={campaign.id} action="pause" label="Pause" />
          <CampaignAction id={campaign.id} action="archive" label="Archive" />
          <form action={enqueueCampaignAction} className="flex gap-2">
            <input name="id" type="hidden" value={campaign.id} />
            <input
              className="h-9 w-20 rounded-md border border-input bg-background px-3 text-sm"
              name="count"
              type="number"
              defaultValue={5}
              min={1}
              max={50}
            />
            <Button size="sm" type="submit" variant="outline">
              Enqueue
            </Button>
          </form>
        </div>
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full min-w-[680px] text-left text-sm">
            <thead className="border-b bg-muted/40 text-muted-foreground">
              <tr>
                <th className="p-3 font-medium">Target</th>
                <th className="p-3 font-medium">Anchor</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 font-medium">Result</th>
              </tr>
            </thead>
            <tbody>
              {jobsError ? (
                <tr>
                  <td className="p-3 text-destructive" colSpan={4}>
                    Không tải được jobs của campaign này.
                  </td>
                </tr>
              ) : jobs.items.length === 0 ? (
                <tr>
                  <td className="p-3 text-muted-foreground" colSpan={4}>
                    Chưa có job.
                  </td>
                </tr>
              ) : (
                jobs.items.map((job) => (
                  <tr className="border-b last:border-0" key={job.id}>
                    <td className="max-w-[240px] truncate p-3">{job.target_url}</td>
                    <td className="p-3">{job.anchor_text}</td>
                    <td className="p-3">{job.status}</td>
                    <td className="max-w-[240px] truncate p-3 text-muted-foreground">
                      {job.result_url ?? job.error_message ?? "-"}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );
}

function Metric({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="rounded-lg border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="text-2xl font-semibold">{value}</div>
    </div>
  );
}

function CampaignAction({ id, action, label }: { id: string; action: string; label: string }) {
  return (
    <form action={campaignStatusAction}>
      <input name="id" type="hidden" value={id} />
      <input name="action" type="hidden" value={action} />
      <Button size="sm" type="submit" variant="outline">
        {label}
      </Button>
    </form>
  );
}
