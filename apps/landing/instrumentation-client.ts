import * as Sentry from "@sentry/nextjs";

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart;

const sensitiveHeaders = new Set(["authorization", "cookie", "set-cookie"]);
const keyPattern = /sbf_live_[A-Za-z0-9]+/g;

function redactKeyMatches(value: string | undefined): string | undefined {
  return value?.replace(keyPattern, "sbf_live_[REDACTED]");
}

function redactHeaders(headers: unknown): unknown {
  if (!headers || typeof headers !== "object") {
    return headers;
  }
  const nextHeaders = { ...(headers as Record<string, unknown>) };
  for (const key of Object.keys(nextHeaders)) {
    if (sensitiveHeaders.has(key.toLowerCase())) {
      nextHeaders[key] = "[REDACTED]";
    }
  }
  return nextHeaders;
}

Sentry.init({
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: process.env.VERCEL_ENV || process.env.NODE_ENV || "development",
  replaysSessionSampleRate: 0,
  replaysOnErrorSampleRate: 1.0,
  integrations: [
    Sentry.replayIntegration({
      maskAllInputs: true,
      blockAllMedia: true,
    }),
  ],
  tracesSampler(samplingContext) {
    const path = samplingContext.transactionContext?.name || "";
    if (path.startsWith("/login") || path.startsWith("/api/auth/verify")) {
      return 0;
    }
    return process.env.NODE_ENV === "production" ? 0.1 : 1;
  },
  beforeSend(event) {
    if (event.request) {
      event.request.headers = redactHeaders(event.request.headers) as
        | Record<string, string>
        | undefined;
      event.request.cookies = undefined;
      if (typeof event.request.data === "string") {
        event.request.data = redactKeyMatches(event.request.data);
      }
      event.request.url = redactKeyMatches(event.request.url);
    }
    event.message = redactKeyMatches(event.message) ?? event.message;
    return event;
  },
  beforeBreadcrumb(breadcrumb) {
    if (breadcrumb.data) {
      breadcrumb.data.request_headers = redactHeaders(breadcrumb.data.request_headers);
      breadcrumb.data.response_headers = redactHeaders(breadcrumb.data.response_headers);
      if (typeof breadcrumb.data.url === "string") {
        breadcrumb.data.url = redactKeyMatches(breadcrumb.data.url);
      }
    }
    return breadcrumb;
  },
});
