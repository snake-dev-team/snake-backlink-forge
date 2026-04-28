import { NextResponse } from "next/server";
import { COOKIE_NAME } from "@/lib/auth/cookies";

const BACKEND = process.env.NEXT_PUBLIC_API_BASE_URL;
const VERIFY_TIMEOUT_MS = 5_000;
const E2E_AUTH_MOCK = process.env.E2E_AUTH_MOCK === "1";
const COOKIE_MAX_AGE = 60 * 60 * 24 * 30;
const secureCookie = process.env.NODE_ENV !== "development";

function verifiedResponse(key: string, user: unknown) {
  const response = NextResponse.json({ ok: true, user });
  response.cookies.set(COOKIE_NAME, key, {
    httpOnly: true,
    secure: secureCookie,
    sameSite: "strict",
    path: "/",
    maxAge: COOKIE_MAX_AGE,
  });
  return response;
}

function allowedOrigins(req: Request): string[] {
  const origins = new Set<string>();
  const host = req.headers.get("host");
  if (host) {
    origins.add(`https://${host}`);
  }
  if (process.env.NEXT_PUBLIC_APP_URL) {
    origins.add(process.env.NEXT_PUBLIC_APP_URL);
  }
  if (process.env.VERCEL_ENV === "preview" && process.env.VERCEL_URL) {
    origins.add(`https://${process.env.VERCEL_URL}`);
  }
  if (process.env.NODE_ENV === "development") {
    origins.add("http://localhost:3000");
    origins.add("http://127.0.0.1:3000");
  }
  return [...origins];
}

export async function POST(req: Request) {
  const origin = req.headers.get("origin");
  if (!origin || !allowedOrigins(req).includes(origin)) {
    return NextResponse.json({ error: "forbidden_origin" }, { status: 403 });
  }

  let body: { key?: string };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "bad_request" }, { status: 400 });
  }

  const key = body.key?.trim();
  if (!key?.startsWith("sbf_live_")) {
    return NextResponse.json({ error: "invalid_format" }, { status: 400 });
  }
  if (!BACKEND) {
    return NextResponse.json({ error: "backend_not_configured" }, { status: 500 });
  }
  if (E2E_AUTH_MOCK) {
    if (key === "sbf_live_bad_e2e_key") {
      return NextResponse.json({ error: "invalid_key" }, { status: 401 });
    }
    return verifiedResponse(key, {
      id: "00000000-0000-0000-0000-000000000001",
      key_prefix: "sbf_live_pha",
    });
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), VERIFY_TIMEOUT_MS);
  let response: Response;
  try {
    response = await fetch(`${BACKEND}/api/v1/auth/verify`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key }),
      cache: "no-store",
      signal: controller.signal,
    });
  } catch {
    return NextResponse.json({ error: "backend_error" }, { status: 502 });
  } finally {
    clearTimeout(timeout);
  }

  if (response.status === 401) {
    return NextResponse.json({ error: "invalid_key" }, { status: 401 });
  }
  if (response.status === 403) {
    return NextResponse.json({ error: "account_banned" }, { status: 403 });
  }
  if (response.status === 429) {
    return NextResponse.json({ error: "too_many_attempts" }, { status: 429 });
  }
  if (!response.ok) {
    return NextResponse.json({ error: "backend_error" }, { status: 502 });
  }

  const data = await response.json();
  return verifiedResponse(key, data.user);
}
