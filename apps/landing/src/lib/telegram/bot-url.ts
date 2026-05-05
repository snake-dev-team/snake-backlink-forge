/**
 * Telegram bot URL builder with username validation + start param encoding.
 * Used by command palette, hero CTA, pricing tier CTAs, CTA section, footer.
 *
 * Validates NEXT_PUBLIC_TELEGRAM_BOT_USERNAME at module load time —
 * a bad env value throws immediately rather than silently producing broken links.
 */

const TG_USERNAME_REGEX = /^[A-Za-z0-9_]{5,32}$/;
const FALLBACK_USERNAME = "SnakeBacklinkForgeBot";

const rawUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? FALLBACK_USERNAME;

if (!TG_USERNAME_REGEX.test(rawUsername)) {
  throw new Error(
    `Invalid NEXT_PUBLIC_TELEGRAM_BOT_USERNAME: "${rawUsername}". Must match ${TG_USERNAME_REGEX}`,
  );
}

const BOT_USERNAME = rawUsername;

/**
 * Build a Telegram bot deep link.
 * @param start - optional `?start=<param>` value; URL-encoded automatically.
 * @example botUrl()                        // https://t.me/SnakeBacklinkForgeBot
 * @example botUrl("topup_premium_pro_200") // https://t.me/SnakeBacklinkForgeBot?start=topup_premium_pro_200
 */
export function botUrl(start?: string): string {
  const base = `https://t.me/${BOT_USERNAME}`;
  if (!start) return base;
  return `${base}?start=${encodeURIComponent(start)}`;
}

/** Validated bot username for display purposes (no URL encoding). */
export const BOT_USERNAME_DISPLAY = BOT_USERNAME;
