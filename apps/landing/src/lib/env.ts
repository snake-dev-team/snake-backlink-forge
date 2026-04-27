// Env loader — Zod-validated NEXT_PUBLIC_* vars.
// Module-load throw on parse fail prevents boot with misconfigured env.
//
// Convention: import { env } from '@/lib/env' — NEVER access process.env.NEXT_PUBLIC_* directly.
// Forward-declares Phase 3-8 env vars per RT-R2 F-bundled-env-list.

import { z } from "zod";

const envSchema = z.object({
  NEXT_PUBLIC_API_BASE_URL: z.string().url(),
  NEXT_PUBLIC_APP_URL: z.string().url(),
  NEXT_PUBLIC_TELEGRAM_BOT_USERNAME: z.string().min(1),
  NEXT_PUBLIC_SENTRY_DSN: z.string().url(),
  NEXT_PUBLIC_PLAUSIBLE_DOMAIN: z.string().min(1),
  NEXT_PUBLIC_VERCEL_URL: z.string().optional(), // auto-set by Vercel on previews
});

const parsed = envSchema.safeParse({
  NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,
  NEXT_PUBLIC_TELEGRAM_BOT_USERNAME: process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME,
  NEXT_PUBLIC_SENTRY_DSN: process.env.NEXT_PUBLIC_SENTRY_DSN,
  NEXT_PUBLIC_PLAUSIBLE_DOMAIN: process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN,
  NEXT_PUBLIC_VERCEL_URL: process.env.NEXT_PUBLIC_VERCEL_URL,
});

if (!parsed.success) {
  // Fail-loud: throws at module-load, prevents boot with misconfigured env.
  throw new Error(`Invalid env: ${JSON.stringify(parsed.error.flatten().fieldErrors)}`);
}

export const env = parsed.data;
