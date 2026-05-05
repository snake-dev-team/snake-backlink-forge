// Vanilla TS popup — KISS, no framework. ~80 LOC.
// Shows current settings status, last-5-jobs history, Pause/Resume, Run-now.

type SettingsResponse = {
  ok: boolean;
  settings: {
    apiKey: string;
    enabled: boolean;
    apiBaseUrl: string;
    pollMinutes: number;
  };
};

type JobCacheEntry = {
  result_url: string;
  ts: number;
};

async function init(): Promise<void> {
  const statusEl = document.getElementById("status") as HTMLDivElement;
  const toggleBtn = document.getElementById("toggle") as HTMLButtonElement;
  const runBtn = document.getElementById("run-now") as HTMLButtonElement;
  const jobsEl = document.getElementById("jobs") as HTMLUListElement;

  // Load current settings from background SW.
  let settingsResp: SettingsResponse;
  try {
    settingsResp = (await chrome.runtime.sendMessage({
      type: "sbf:get-settings",
    })) as SettingsResponse;
  } catch {
    statusEl.textContent = "Error: could not reach background worker.";
    return;
  }

  const { settings } = settingsResp;
  const keyMasked = settings.apiKey ? `${settings.apiKey.slice(0, 6)}…` : "(not set)";
  statusEl.textContent = `${settings.enabled ? "Active" : "Paused"} · API ${keyMasked}`;
  toggleBtn.textContent = settings.enabled ? "Pause" : "Resume";

  // Toggle enabled state.
  toggleBtn.addEventListener("click", () => {
    toggleBtn.disabled = true;
    chrome.runtime
      .sendMessage({ type: "sbf:save-settings", settings: { enabled: !settings.enabled } })
      .then(() => window.location.reload())
      .catch((err: unknown) => {
        statusEl.textContent = `Toggle failed: ${String(err)}`;
        toggleBtn.disabled = false;
      });
  });

  // Force poll now.
  runBtn.addEventListener("click", () => {
    runBtn.disabled = true;
    statusEl.textContent = "Polling…";
    chrome.runtime
      .sendMessage({ type: "sbf:poll-now" })
      .then((r: { ok: boolean; error?: string }) => {
        statusEl.textContent = r.ok ? "Polled OK" : `Error: ${r.error ?? "unknown"}`;
      })
      .catch((err: unknown) => {
        statusEl.textContent = `Poll failed: ${String(err)}`;
      })
      .finally(() => {
        runBtn.disabled = false;
      });
  });

  // Render last-5-jobs from chrome.storage.local cache.
  const all = await chrome.storage.local.get(null);
  const jobEntries = (Object.entries(all) as [string, JobCacheEntry][])
    .filter(([k]) => k.startsWith("job:"))
    .sort(([, a], [, b]) => b.ts - a.ts)
    .slice(0, 5);

  if (jobEntries.length === 0) {
    const li = document.createElement("li");
    li.textContent = "No jobs completed yet.";
    jobsEl.appendChild(li);
  } else {
    for (const [k, v] of jobEntries) {
      const jobId = k.slice(4, 12);
      const li = document.createElement("li");
      const link = document.createElement("a");
      link.href = v.result_url;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.textContent = v.result_url;
      li.textContent = `${jobId}… → `;
      li.appendChild(link);
      jobsEl.appendChild(li);
    }
  }
}

init().catch((err: unknown) => {
  const statusEl = document.getElementById("status");
  if (statusEl) statusEl.textContent = `Fatal: ${String(err)}`;
});
