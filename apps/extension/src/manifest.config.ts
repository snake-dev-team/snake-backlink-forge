import { defineManifest } from "@crxjs/vite-plugin";

// Phase 3 will add: storage, alarms, offscreen, scripting, tabs, cookies,
// declarativeNetRequest per MASTER_PROMPT §6.1. Phase 1 ships empty permissions.
export default defineManifest({
  manifest_version: 3,
  name: "Snake Backlink Forge",
  version: "0.1.0",
  description: "Professional backlink automation with AI content",
  permissions: [],
  host_permissions: [],
  background: {
    service_worker: "src/background/index.ts",
    type: "module",
  },
});
