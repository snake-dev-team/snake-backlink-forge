import { readFile } from "node:fs/promises";

const manifest = JSON.parse(
  await readFile(new URL("../dist/manifest.json", import.meta.url), "utf8"),
);
const entries = manifest.web_accessible_resources ?? [];

for (const entry of entries) {
  const matches = entry.matches ?? [];
  const resources = entry.resources ?? [];
  if (matches.includes("<all_urls>")) {
    throw new Error("web_accessible_resources must not match <all_urls>");
  }
  if (resources.includes("*") || resources.includes("**/*")) {
    throw new Error("web_accessible_resources must not expose wildcard resources");
  }
  if (entry.use_dynamic_url === false) {
    throw new Error("web_accessible_resources must not disable dynamic URLs");
  }
}
