import "server-only";
import { cookies } from "next/headers";

const COOKIE_NAME = process.env.NODE_ENV === "development" ? "sbf_key" : "__Host-sbf_key";
const MAX_AGE = 60 * 60 * 24 * 30;
const isSecure = process.env.NODE_ENV !== "development";

export async function getApiKeyCookie(): Promise<string | null> {
  return (await cookies()).get(COOKIE_NAME)?.value ?? null;
}

export async function setApiKeyCookie(key: string) {
  (await cookies()).set(COOKIE_NAME, key, {
    httpOnly: true,
    secure: isSecure,
    sameSite: "strict",
    path: "/",
    maxAge: MAX_AGE,
  });
}

export async function clearApiKeyCookie() {
  (await cookies()).set(COOKIE_NAME, "", {
    httpOnly: true,
    secure: isSecure,
    sameSite: "lax",
    path: "/",
    maxAge: 0,
  });
}

export { COOKIE_NAME };
