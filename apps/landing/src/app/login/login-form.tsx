"use client";

import { useSearchParams } from "next/navigation";
import { type FormEvent, useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { sanitizeNext } from "@/lib/auth/redirect-safe";

const VERIFY_TIMEOUT_MS = 8_000;

const ERROR_MESSAGES: Record<string, string> = {
  invalid_format: "Khóa phải bắt đầu bằng sbf_live_",
  invalid_key: "Khóa không hợp lệ — kiểm tra lại",
  account_banned: "Tài khoản bị khóa — liên hệ hỗ trợ",
  too_many_attempts: "Quá nhiều lần thử — đợi 1 phút",
  request_timeout: "Hết thời gian chờ phản hồi — thử lại sau ít phút",
  backend_error: "Server lỗi — thử lại sau",
  backend_not_configured: "Server chưa được cấu hình",
  forbidden_origin: "Yêu cầu không hợp lệ",
};

export function LoginForm() {
  const searchParams = useSearchParams();
  const inputId = useId();
  const errorId = useId();
  const next = sanitizeNext(searchParams.get("next"));
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const formData = new FormData(event.currentTarget);
    const key = String(formData.get("key") ?? "").trim();
    if (!key || pending) {
      return;
    }

    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), VERIFY_TIMEOUT_MS);
    setPending(true);
    try {
      const response = await fetch("/api/auth/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key }),
        credentials: "same-origin",
        signal: controller.signal,
      });

      const data = await response.json().catch(() => ({}));
      if (response.ok) {
        window.location.assign(next);
        return;
      }

      setError(ERROR_MESSAGES[data.error as string] ?? "Đã có lỗi");
    } catch (error) {
      setError(error instanceof DOMException && error.name === "AbortError" ? ERROR_MESSAGES.request_timeout : ERROR_MESSAGES.backend_error);
    } finally {
      window.clearTimeout(timeout);
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-md border-border/70 bg-card/95 shadow-2xl">
      <CardHeader>
        <CardTitle>Đăng nhập SBF</CardTitle>
        <CardDescription>Dán API key từ Telegram bot để vào dashboard.</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={onSubmit}>
          <div className="space-y-2">
            <Label htmlFor={inputId}>API key</Label>
            <Input
              id={inputId}
              name="key"
              autoComplete="off"
              inputMode="text"
              placeholder="sbf_live_..."
              aria-describedby={error ? errorId : undefined}
              aria-invalid={Boolean(error)}
              disabled={pending}
            />
          </div>
          {error ? (
            <p className="text-sm text-destructive" id={errorId} role="alert">
              {error}
            </p>
          ) : null}
          <Button className="w-full" disabled={pending} type="submit">
            {pending ? "Đang kiểm tra..." : "Đăng nhập"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
