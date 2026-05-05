import { defineManifest } from "@crxjs/vite-plugin";

export default defineManifest({
  manifest_version: 3,
  name: "Snake Backlink Forge",
  version: "0.2.0",
  description: "Professional backlink automation with AI content",
  permissions: ["alarms", "storage", "tabs"],
  host_permissions: [
    "http://localhost:8080/*",
    "https://snake-backlink-api.fly.dev/*",
    "https://*.snakebacklinkforge.com/*",
  ],
  // F24: optional_host_permissions allows runtime grant per user's wp_site domain.
  // Chrome Web Store risk: "https://*/*" is broad; document for reviewer.
  // Backup: use chrome.permissions.request({ origins: [domain] }) at site-add time.
  optional_host_permissions: ["https://*/*", "http://*/*"],
  background: {
    service_worker: "src/background/index.ts",
    type: "module",
  },
  action: {
    default_popup: "src/popup/popup.html",
    default_title: "Snake Backlink Forge",
  },
});
