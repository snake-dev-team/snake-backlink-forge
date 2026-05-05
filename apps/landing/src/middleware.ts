import { type NextRequest, NextResponse } from "next/server";

const COOKIE_NAME = process.env.NODE_ENV === "development" ? "sbf_key" : "__Host-sbf_key";
const PUBLIC_PATHS = ["/", "/login", "/api/auth", "/_next", "/favicon.ico"];
const APP_PATH_PREFIXES = ["/dashboard", "/sites", "/campaigns"];
const SAFE_NEXT = /^\/[a-zA-Z0-9/_-]*$/;

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;
  const hasKey = req.cookies.has(COOKIE_NAME);

  if (pathname === "/login" && hasKey) {
    const url = req.nextUrl.clone();
    url.pathname = "/dashboard";
    return NextResponse.redirect(url);
  }

  if (PUBLIC_PATHS.some((path) => pathname === path || pathname.startsWith(`${path}/`))) {
    return NextResponse.next();
  }

  const requiresAuth = APP_PATH_PREFIXES.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`),
  );
  if (requiresAuth && !hasKey) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    if (SAFE_NEXT.test(pathname)) {
      url.searchParams.set("next", pathname);
    }
    return NextResponse.redirect(url);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|.*\\..*).*)"],
};
