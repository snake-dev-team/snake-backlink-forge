import { withSentryConfig } from "@sentry/nextjs";
import type { NextConfig } from "next";

const scriptSrc =
  process.env.NODE_ENV === "development"
    ? "script-src 'self' 'unsafe-eval' 'unsafe-inline' https://plausible.io https://vercel.live"
    : "script-src 'self' 'unsafe-inline' https://plausible.io https://vercel.live";

const cspValue = [
  "default-src 'self'",
  scriptSrc,
  "connect-src 'self' https://plausible.io https://*.sentry.io https://snake-backlink-api.fly.dev",
  "img-src 'self' data: https:",
  "style-src 'self' 'unsafe-inline'",
  "font-src 'self' data:",
  "frame-src https://vercel.live",
  "frame-ancestors 'none'",
  "form-action 'self'",
  "base-uri 'self'",
].join("; ");

const config: NextConfig = {
  reactStrictMode: true,
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "Content-Security-Policy", value: cspValue },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
        ],
      },
    ];
  },
};

export default withSentryConfig(
  config,
  {
    silent: true,
    org: process.env.SENTRY_ORG,
    project: process.env.SENTRY_PROJECT,
    widenClientFileUpload: false,
    sourcemaps: { deleteSourcemapsAfterUpload: true },
  },
  { hideSourceMaps: true, disableLogger: true },
);
