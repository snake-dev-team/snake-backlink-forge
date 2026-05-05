// Imports must appear at the top of ES module.

import { postBacklink } from "../wp/poster";
import type { WPSiteCredentials } from "../wp/types";

// F14: server claimedJobMap does not include `type`; make optional to avoid strict TS error.
// F17: target_url is non-optional; fallback to money_url removed.
// Phase 7.04: content_body/title/meta populated by AI generation step before dispatch.
type CampaignJob = {
  id: string;
  campaign_id: string;
  target_id?: string | null;
  type?: string | null;
  target_url: string;
  target_domain?: string | null;
  money_url: string;
  anchor_text?: string | null;
  status: string;
  metadata?: unknown;
  content_body?: string | null;
  content_title?: string | null;
  content_meta?: string | null;
};

type ExtensionSettings = {
  apiBaseUrl: string;
  apiKey: string;
  enabled: boolean;
  pollMinutes: number;
};

// F15: was "succeeded" — server normalizeReportStatus accepts success|failed|skipped only.
// F18: evidence field required for server-side Report validator.
type JobResult = {
  status: "success" | "failed" | "skipped";
  result_url?: string;
  error_code?: string;
  error_message?: string;
  evidence?: string;
};

const DEFAULT_SETTINGS: ExtensionSettings = {
  apiBaseUrl: "http://localhost:8080/api/v1",
  apiKey: "",
  enabled: false,
  pollMinutes: 5,
};

const ALARM_NAME = "sbf-campaign-poll";

async function getSettings(): Promise<ExtensionSettings> {
  const stored = await chrome.storage.sync.get(DEFAULT_SETTINGS);
  return {
    apiBaseUrl: String(stored.apiBaseUrl || DEFAULT_SETTINGS.apiBaseUrl).replace(/\/+$/, ""),
    apiKey: String(stored.apiKey || ""),
    enabled: Boolean(stored.enabled),
    pollMinutes: Math.max(1, Number(stored.pollMinutes || DEFAULT_SETTINGS.pollMinutes)),
  };
}

async function saveSettings(patch: Partial<ExtensionSettings>): Promise<ExtensionSettings> {
  const next = { ...(await getSettings()), ...patch };
  await chrome.storage.sync.set(next);
  await schedulePolling(next);
  return next;
}

async function schedulePolling(settings = DEFAULT_SETTINGS): Promise<void> {
  await chrome.alarms.clear(ALARM_NAME);
  if (!settings.enabled || !settings.apiKey) {
    return;
  }
  await chrome.alarms.create(ALARM_NAME, {
    delayInMinutes: 0.1,
    periodInMinutes: Math.max(1, settings.pollMinutes),
  });
}

/**
 * Build HMAC-SHA-256 signature headers required by server's ExtensionSignature middleware.
 *
 * Payload signed (matches server's signExtensionPayload):
 *   "${METHOD}\n${pathname}\n${timestampSeconds}\n${hex(sha256(body))}"
 *
 * - X-Timestamp: Unix seconds as decimal string
 * - X-Nonce: random UUID (server rejects replay within 5-min window)
 * - X-Signature: lowercase hex HMAC-SHA-256 over payload, keyed with the raw API key
 */
async function buildSignatureHeaders(
  apiKey: string,
  method: string,
  fullRequestUrl: string,
  bodyBytes: Uint8Array,
): Promise<Record<string, string>> {
  const timestampSeconds = String(Math.floor(Date.now() / 1000));
  const nonce = crypto.randomUUID();

  // Server uses c.Path() = full URL pathname (e.g. /api/v1/campaign/next)
  const urlPath = new URL(fullRequestUrl).pathname;

  const encoder = new TextEncoder();

  // sha256(body) hex. Cast to BufferSource — TS strict mode rejects Uint8Array<ArrayBufferLike>.
  const bodyHash = await crypto.subtle.digest("SHA-256", bodyBytes as BufferSource);
  const bodyHashHex = Array.from(new Uint8Array(bodyHash))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  // Payload mirrors server: METHOD\nPATH\nTIMESTAMP\nBODY_HASH_HEX
  const payload = `${method.toUpperCase()}\n${urlPath}\n${timestampSeconds}\n${bodyHashHex}`;

  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(apiKey) as BufferSource,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, encoder.encode(payload) as BufferSource);
  const signature = Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  return {
    "X-Timestamp": timestampSeconds,
    "X-Nonce": nonce,
    "X-Signature": signature,
  };
}

/**
 * Fetch wrapper. Pass signed=true for extension-only routes protected by
 * ExtensionSignature middleware (/campaign/next, /campaign/result/:id).
 */
async function apiFetch<T>(
  settings: ExtensionSettings,
  path: string,
  init: RequestInit = {},
  signed = false,
): Promise<T> {
  const fullUrl = `${settings.apiBaseUrl}${path}`;
  const method = (init.method || "GET").toUpperCase();
  // Body must be a string for fetch; encode to bytes only for hashing
  const bodyStr = typeof init.body === "string" ? init.body : "";
  const bodyBytes = new TextEncoder().encode(bodyStr);

  const extraHeaders: Record<string, string> = {};
  if (signed) {
    const sigHeaders = await buildSignatureHeaders(settings.apiKey, method, fullUrl, bodyBytes);
    Object.assign(extraHeaders, sigHeaders);
  }

  const response = await fetch(fullUrl, {
    ...init,
    headers: {
      "content-type": "application/json",
      authorization: `Bearer ${settings.apiKey}`,
      ...extraHeaders,
      ...(init.headers || {}),
    },
  });

  if (!response.ok) {
    throw new Error(`api_${response.status}`);
  }
  return (await response.json()) as T;
}

async function claimNext(settings: ExtensionSettings): Promise<CampaignJob | null> {
  // signed=true: route protected by ExtensionSignature middleware
  const payload = await apiFetch<{ item?: CampaignJob | null }>(
    settings,
    "/campaign/next",
    {},
    true,
  );
  return payload.item ?? null;
}

async function reportResult(
  settings: ExtensionSettings,
  jobId: string,
  result: JobResult,
): Promise<void> {
  // signed=true: route protected by ExtensionSignature middleware
  await apiFetch(
    settings,
    `/campaign/result/${jobId}`,
    {
      method: "POST",
      body: JSON.stringify(result),
    },
    true,
  );
}

// F16: real WP REST API worker — replaces silent handshake stub.
async function runJob(settings: ExtensionSettings, job: CampaignJob): Promise<void> {
  // F17: no fallback to money_url — assert target_url non-empty.
  if (!job.target_url) {
    await reportResult(settings, job.id, {
      status: "skipped",
      error_code: "missing_target_url",
    });
    return;
  }

  // Only blog_comment supported in v1; undefined/null type treated as blog_comment.
  if (job.type && job.type !== "blog_comment") {
    await reportResult(settings, job.id, {
      status: "skipped",
      error_code: "type_not_supported",
      error_message: `type ${job.type} not supported in v1`,
    });
    return;
  }

  if (!job.money_url || !/^https?:\/\//i.test(job.money_url)) {
    await reportResult(settings, job.id, {
      status: "failed",
      error_code: "invalid_money_url",
    });
    return;
  }

  // Fetch WP site credentials for the job's target domain via signed extension endpoint.
  let creds: WPSiteCredentials;
  const targetDomain = new URL(job.target_url).hostname;
  try {
    creds = await apiFetch<WPSiteCredentials>(
      settings,
      `/wp-sites/by-domain/${encodeURIComponent(targetDomain)}`,
      {},
      true,
    );
  } catch {
    await reportResult(settings, job.id, {
      status: "skipped",
      error_code: "no_wp_site",
      error_message: `no wp_site connected for ${targetDomain}`,
    });
    return;
  }

  // Phase 7.04: require AI-generated content; skip job if not yet populated.
  // Jobs without content_body were enqueued before generate-content was called.
  if (!job.content_body || !job.content_title) {
    await reportResult(settings, job.id, {
      status: "skipped",
      error_code: "no_content",
      error_message: "AI content not generated for this job",
    });
    return;
  }

  const postResult = await postBacklink(creds, {
    title: job.content_title,
    content_html: job.content_body,
  });

  if (!postResult.ok) {
    await reportResult(settings, job.id, {
      status: "failed",
      error_code: postResult.errorCode,
      error_message: postResult.errorMessage,
    });
    return;
  }

  // Cache for popup job history.
  await chrome.storage.local.set({
    [`job:${job.id}`]: { result_url: postResult.postUrl, ts: Date.now() },
  });

  // F18: evidence from WP REST content.rendered (trimmed to 16KB server-side limit).
  await reportResult(settings, job.id, {
    status: "success",
    result_url: postResult.postUrl,
    evidence: postResult.evidence,
  });
}

async function pollOnce(): Promise<void> {
  const settings = await getSettings();
  if (!settings.enabled || !settings.apiKey) {
    return;
  }

  const job = await claimNext(settings);
  if (job) {
    await runJob(settings, job);
  }
}

chrome.runtime.onInstalled.addListener(() => {
  void getSettings().then(schedulePolling);
});

chrome.runtime.onStartup.addListener(() => {
  void getSettings().then(schedulePolling);
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) {
    void pollOnce().catch((error: unknown) => {
      console.warn("[sbf] campaign poll failed", error);
    });
  }
});

chrome.runtime.onMessage.addListener((message: unknown, _sender, sendResponse) => {
  const request = message as { type?: string; settings?: Partial<ExtensionSettings> };
  if (request.type === "sbf:get-settings") {
    void getSettings().then((settings) => sendResponse({ ok: true, settings }));
    return true;
  }
  if (request.type === "sbf:save-settings") {
    void saveSettings(request.settings || {}).then((settings) =>
      sendResponse({ ok: true, settings }),
    );
    return true;
  }
  if (request.type === "sbf:poll-now") {
    void pollOnce()
      .then(() => sendResponse({ ok: true }))
      .catch((error: unknown) => sendResponse({ ok: false, error: String(error) }));
    return true;
  }
  return false;
});

console.log("[sbf] background ready", { ts: Date.now() });
