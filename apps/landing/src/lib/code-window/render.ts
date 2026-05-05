import "server-only"; // Hard guard: this module imports Shiki (~3MB grammars). MUST stay server-side.

import { cache } from "react";
import { codeToHtml } from "shiki";

export type CodeLang = "bash" | "typescript" | "javascript" | "json" | "python" | "yaml";

export const SHIKI_THEME = "vitesse-dark";

/**
 * Build-time Shiki render, cached per (code, lang) tuple within a single render pass.
 * Same snippet rendered in multiple components => single Shiki invocation.
 */
export const renderCode = cache(async (code: string, lang: CodeLang): Promise<string> => {
  return codeToHtml(code, { lang, theme: SHIKI_THEME });
});
