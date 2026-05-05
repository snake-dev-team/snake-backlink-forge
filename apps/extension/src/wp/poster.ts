// wp-poster: Posts backlink articles via WordPress REST API using Application Password auth.
// No DOM scraping — pure fetch() with Basic auth. Runs in Service Worker context.
import type { PostInput, PostResult, WPSiteCredentials } from "./types";

/** Maximum time per WP REST POST before AbortController fires. */
const WP_TIMEOUT_MS = 60_000;

/**
 * Post a backlink article to a WordPress site via the WP REST API.
 *
 * Auth: Authorization: Basic base64(app_username:app_password_plain)
 * Endpoint: POST {base_url}/wp-json/wp/v2/posts
 *
 * Returns PostResult discriminated on ok:
 *   ok=true  → { postUrl, evidence } (evidence = content.rendered, ≤16KB)
 *   ok=false → { errorCode, errorMessage? }
 *
 * Error codes match the failure-mode table in phase-7-01 plan:
 *   wp_auth_expired, wp_rest_disabled, wp_blocked, wp_unreachable,
 *   wp_validation_error, wp_timeout
 */
export async function postBacklink(creds: WPSiteCredentials, post: PostInput): Promise<PostResult> {
  const url = new URL("/wp-json/wp/v2/posts", creds.base_url).toString();
  const auth = btoa(`${creds.app_username}:${creds.app_password_plain}`);

  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), WP_TIMEOUT_MS);

  try {
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        authorization: `Basic ${auth}`,
      },
      body: JSON.stringify({
        title: post.title,
        content: post.content_html,
        status: "publish",
      }),
      signal: ctrl.signal,
    });

    // Auth failures — Application Password expired or revoked.
    if (res.status === 401 || res.status === 403) {
      return { ok: false, errorCode: "wp_auth_expired" };
    }

    // 404 on /wp-json/wp/v2/posts → REST API disabled by security plugin.
    if (res.status === 404) {
      return { ok: false, errorCode: "wp_rest_disabled" };
    }

    const bodyText = await res.text();

    // WAF / Cloudflare challenge page heuristic.
    if (/cloudflare|challenge|captcha/i.test(bodyText)) {
      return { ok: false, errorCode: "wp_blocked" };
    }

    if (!res.ok) {
      const code = res.status >= 500 ? "wp_unreachable" : "wp_validation_error";
      return { ok: false, errorCode: code, errorMessage: bodyText.slice(0, 200) };
    }

    // Parse 201 Created response.
    let data: { id: number; link: string; content: { rendered: string } };
    try {
      data = JSON.parse(bodyText) as typeof data;
    } catch {
      return { ok: false, errorCode: "wp_validation_error", errorMessage: "non-json response" };
    }

    if (!data.link || !data.content?.rendered) {
      return {
        ok: false,
        errorCode: "wp_validation_error",
        errorMessage: "missing link or content in response",
      };
    }

    // Trim evidence to 16KB — server Report validator enforces ≤16KB.
    return {
      ok: true,
      postUrl: data.link,
      evidence: data.content.rendered.slice(0, 16_000),
    };
  } catch (err) {
    if ((err as Error).name === "AbortError") {
      return { ok: false, errorCode: "wp_timeout" };
    }
    return { ok: false, errorCode: "wp_unreachable", errorMessage: String(err) };
  } finally {
    clearTimeout(timer);
  }
}
