import { NextResponse } from "next/server";
import { clearApiKeyCookie } from "@/lib/auth/cookies";
import { forbiddenOriginResponse } from "@/lib/auth/origin";

export async function POST(req: Request) {
  const forbiddenOrigin = forbiddenOriginResponse(req);
  if (forbiddenOrigin) {
    return forbiddenOrigin;
  }
  await clearApiKeyCookie();
  return new NextResponse(null, { status: 204 });
}
