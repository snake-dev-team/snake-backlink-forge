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
  // Phase 7.06: multi-site targeting + auto-enqueue fields.
  // site_ids is optional: empty means use legacy target-pool picker.
  site_ids: z
    .array(z.string().uuid("site_id phải là UUID hợp lệ"))
    .min(1, "Chọn ít nhất 1 site.")
    .optional(),
  quantity: z.coerce.number().int().min(1).max(100).default(10),
  tone_preference: z
    .enum(["professional", "casual", "storytelling", "technical"])
    .default("professional"),
});

export type CampaignActionState = {
  error?: string;
  /** Per-field validation messages for client display. */
  fieldErrors?: Record<string, string>;
  /** True after first submit attempt — enables showing site-select validation error. */
  attempted?: boolean;
};

export async function createCampaignAction(
  _prevState: CampaignActionState,
  formData: FormData,
): Promise<CampaignActionState> {
  // site_ids submitted as multiple hidden inputs — getAll collects all values.
  const rawSiteIds = formData.getAll("site_ids").map(String).filter(Boolean);

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
    // Pass undefined when empty so the min(1) optional validation is skipped.
    site_ids: rawSiteIds.length > 0 ? rawSiteIds : undefined,
    quantity: formData.get("quantity") || 10,
    tone_preference: formData.get("tone_preference") || "professional",
  });
  if (!parsed.success) {
    const fieldErrors: Record<string, string> = {};
    for (const issue of parsed.error.issues) {
      const key = issue.path[0]?.toString() ?? "form";
      if (!fieldErrors[key]) fieldErrors[key] = issue.message;
    }
    return {
      error: "Nhập campaign hợp lệ: URL public, keyword/anchor cách nhau bằng dấu phẩy.",
      fieldErrors,
      attempted: true,
    };
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
