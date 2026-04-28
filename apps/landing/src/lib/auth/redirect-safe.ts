const SAFE_PATH = /^\/[a-zA-Z0-9/_-]*$/;

export function sanitizeNext(raw: string | null | undefined): string {
  if (!raw) {
    return "/dashboard";
  }
  if (raw.includes("//") || raw.includes("\\") || raw.includes("%2F") || raw.includes("%5C")) {
    return "/dashboard";
  }

  let decoded: string;
  try {
    decoded = decodeURIComponent(raw);
  } catch {
    return "/dashboard";
  }

  if (decoded !== raw || raw.startsWith("/login") || raw.startsWith("/api/")) {
    return "/dashboard";
  }
  if (!SAFE_PATH.test(raw)) {
    return "/dashboard";
  }
  return raw;
}
