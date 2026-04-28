import { NextResponse } from "next/server";
import { clearApiKeyCookie } from "@/lib/auth/cookies";

export async function POST() {
  await clearApiKeyCookie();
  return new NextResponse(null, { status: 204 });
}
