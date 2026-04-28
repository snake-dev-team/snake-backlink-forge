import { crx } from "@crxjs/vite-plugin";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";
import manifest from "./src/manifest.config";

// svelte() MUST come before crx() — CRXJS requires Svelte transforms to run first
export default defineConfig({
  plugins: [svelte(), crx({ manifest })],
});
