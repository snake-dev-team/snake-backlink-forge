import { type NextRequest, NextResponse } from "next/server";
import { clearApiKeyCookie, getApiKeyCookie } from "@/lib/auth/cookies";

const BACKEND = process.env.NEXT_PUBLIC_API_BASE_URL;
const MAX_BODY_BYTES = 1024 * 1024;

async function readBodyWithLimit(req: NextRequest): Promise<string | null> {
  const body = await req.text();
  return new TextEncoder().encode(body).byteLength > MAX_BODY_BYTES ? null : body;
}

type ProxyContext = {
  params: Promise<{ path: string[] }>;
};

async function forward(req: NextRequest, ctx: ProxyContext) {
  const contentLength = Number.parseInt(req.headers.get("content-length") ?? "0", 10);
  if (contentLength > MAX_BODY_BYTES) {
    return NextResponse.json(
      { error: "payload_too_large", max_bytes: MAX_BODY_BYTES },
      { status: 413 },
    );
  }

  const key = await getApiKeyCookie();
  if (!key) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }
  if (!BACKEND) {
    return NextResponse.json({ error: "backend_not_configured" }, { status: 500 });
  }

  const { path } = await ctx.params;
  const url = new URL(req.url);
  const target = `${BACKEND}/${path.join("/")}${url.search}`;
  const headers: Record<string, string> = { Authorization: `Bearer ${key}` };
  const contentType = req.headers.get("content-type");
  if (contentType) {
    headers["Content-Type"] = contentType;
  }

  const init: RequestInit = {
    method: req.method,
    headers,
    cache: "no-store",
  };
  if (!["GET", "HEAD", "OPTIONS"].includes(req.method)) {
    const body = await readBodyWithLimit(req);
    if (body === null) {
      return NextResponse.json(
        { error: "payload_too_large", max_bytes: MAX_BODY_BYTES },
        { status: 413 },
      );
    }
    init.body = body;
  }

  const response = await fetch(target, init);
  if (response.status === 401) {
    await clearApiKeyCookie();
  }

  const text = await response.text();
  return new NextResponse(text, {
    status: response.status,
    headers: { "Content-Type": response.headers.get("content-type") ?? "application/json" },
  });
}

export const GET = forward;
export const POST = forward;
export const PATCH = forward;
export const PUT = forward;
export const DELETE = forward;
