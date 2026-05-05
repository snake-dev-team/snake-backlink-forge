import { NextResponse } from "next/server";

const UNSAFE_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

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

export function forbiddenOriginResponse(req: Request): NextResponse | null {
  if (!UNSAFE_METHODS.has(req.method)) {
    return null;
  }
  const origin = req.headers.get("origin");
  if (!origin || !allowedOrigins(req).includes(origin)) {
    return NextResponse.json({ error: "forbidden_origin" }, { status: 403 });
  }
  return null;
}
