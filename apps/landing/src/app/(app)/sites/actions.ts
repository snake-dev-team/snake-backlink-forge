"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { z } from "zod";
import { deleteProxyServer, postProxyServer } from "@/lib/api/server-fetch";

const wpSiteSchema = z.object({
  base_url: z.string().url(),
  app_username: z.string().trim().min(1).max(120),
  app_password: z.string().trim().min(1),
  label: z.string().trim().max(120).optional(),
});

export type WpSiteActionState = {
  error?: string;
};

export async function connectWpSiteAction(
  _prevState: WpSiteActionState,
  formData: FormData,
): Promise<WpSiteActionState> {
  const parsed = wpSiteSchema.safeParse({
    base_url: formData.get("base_url"),
    app_username: formData.get("app_username"),
    app_password: formData.get("app_password"),
    label: formData.get("label") || undefined,
  });
  if (!parsed.success) {
    return { error: "Nhập đủ URL, username và Application Password hợp lệ." };
  }

  const response = await postProxyServer("/wp-sites", parsed.data);
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    return { error: wpSiteErrorMessage(body?.error ?? "connect_failed") };
  }

  revalidatePath("/sites");
  redirect("/sites");
}

export async function deleteWpSiteAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  if (!id) {
    return;
  }
  await deleteProxyServer(`/wp-sites/${id}`);
  revalidatePath("/sites");
}

export async function revalidateWpSiteAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  if (!id) {
    return;
  }
  await postProxyServer(`/wp-sites/${id}/revalidate`);
  revalidatePath("/sites");
}

function wpSiteErrorMessage(code: string) {
  switch (code) {
    case "wp_invalid_url":
      return "URL WordPress không hợp lệ. Dùng HTTPS public site.";
    case "wp_invalid_credentials":
      return "Username hoặc Application Password không đúng.";
    case "wp_insufficient_capability":
      return "Tài khoản WordPress thiếu quyền publish_posts.";
    case "wp_rest_api_not_found":
      return "Không tìm thấy WordPress REST API tại site này.";
    case "wp_private_address":
      return "Không thể kết nối tới địa chỉ private/internal.";
    case "wp_rate_limited":
      return "WordPress đang rate limit, thử lại sau.";
    case "wp_server_error_502":
    case "wp_server_error_503":
    case "wp_server_error_504":
      return "Site server tạm thời không phản hồi, thử lại sau.";
    default:
      return "Không kết nối được WordPress site. Kiểm tra lại thông tin.";
  }
}
