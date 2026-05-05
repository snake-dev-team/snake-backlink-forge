"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { z } from "zod";
import { postProxyServer } from "@/lib/api/server-fetch";

const campaignSchema = z.object({
  name: z.string().trim().min(2).max(120),
  money_site_url: z.string().url(),
  niche_keywords: z
    .string()
    .transform((value) => splitList(value))
    .pipe(z.array(z.string()).min(1).max(12)),
  anchor_texts: z
    .string()
    .transform((value) => splitList(value).map((text) => ({ text, type: "branded", weight: 1 })))
    .pipe(
      z
        .array(z.object({ text: z.string().min(1), type: z.string(), weight: z.number() }))
        .min(1)
        .max(20),
    ),
  pool: z.enum(["standard", "premium"]),
  source_mode: z.enum(["custom", "prebuilt", "autofind", "mixed"]),
  daily_limit: z.coerce.number().int().min(5).max(50),
  credits_allocated: z.coerce.number().int().min(1).max(100000),
  start_now: z.coerce.boolean().default(false),
});

export type CampaignActionState = {
  error?: string;
};

export async function createCampaignAction(
  _prevState: CampaignActionState,
  formData: FormData,
): Promise<CampaignActionState> {
  const parsed = campaignSchema.safeParse({
    name: formData.get("name"),
    money_site_url: formData.get("money_site_url"),
    niche_keywords: formData.get("niche_keywords"),
    anchor_texts: formData.get("anchor_texts"),
    pool: formData.get("pool") || "standard",
    source_mode: formData.get("source_mode") || "custom",
    daily_limit: formData.get("daily_limit") || 5,
    credits_allocated: formData.get("credits_allocated") || 10,
    start_now: formData.get("start_now") === "on",
  });
  if (!parsed.success) {
    return { error: "Nhập campaign hợp lệ: URL public, keyword/anchor cách nhau bằng dấu phẩy." };
  }

  const response = await postProxyServer("/campaigns", parsed.data);
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    return { error: campaignErrorMessage(body?.error ?? "create_failed") };
  }

  revalidatePath("/campaigns");
  redirect("/campaigns?status=created");
}

export async function campaignStatusAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  const action = String(formData.get("action") ?? "");
  if (!id || !["start", "pause", "archive"].includes(action)) {
    return;
  }
  const response = await postProxyServer(`/campaigns/${id}/${action}`);
  ensureMutationOk(response, `/campaigns?error=${action}_failed`);
  revalidatePath("/campaigns");
  const status = action === "start" ? "started" : action === "pause" ? "paused" : "archived";
  redirect(`/campaigns?status=${status}`);
}

export async function enqueueCampaignAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  // Cap count at 50 (matches campaign.daily_limit max) — defense in depth against
  // bypassing the UI input[max=50] via direct POST (F20).
  const rawCount = Number(formData.get("count") ?? 5);
  const count = Number.isFinite(rawCount) ? Math.min(Math.max(1, Math.floor(rawCount)), 50) : 5;
  if (!id) {
    return;
  }
  const response = await postProxyServer(`/campaigns/${id}/enqueue`, { count });
  ensureMutationOk(response, "/campaigns?error=enqueue_failed");
  revalidatePath("/campaigns");
  redirect("/campaigns?status=enqueued");
}

function ensureMutationOk(response: Response, failureRedirect: string) {
  if (!response.ok) {
    redirect(failureRedirect);
  }
}

function splitList(value: string) {
  return value
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function campaignErrorMessage(code: string) {
  switch (code) {
    case "bad_request":
      return "Campaign chưa hợp lệ hoặc vượt giới hạn credit/daily limit.";
    case "automation_unavailable":
      return "Campaign automation service chưa sẵn sàng.";
    default:
      return "Không tạo được campaign. Kiểm tra lại dữ liệu hoặc phiên đăng nhập.";
  }
}
