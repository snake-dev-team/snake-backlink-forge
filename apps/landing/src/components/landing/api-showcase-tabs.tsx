"use client";

import { type ReactNode, useState } from "react";
import { ENDPOINTS, LANG_LABELS, type SnippetLang } from "@/lib/landing/api-snippets";
import { cn } from "@/lib/utils";

type RenderedSnippet = { endpointId: string; lang: SnippetLang; node: ReactNode };

type ApiShowcaseTabsProps = { rendered: RenderedSnippet[] };

/**
 * Client island — manages active endpoint + language tab state.
 * Receives pre-rendered RSC CodeWindow nodes from ApiShowcase (server).
 * Zero network requests on tab switch: all 9 nodes already in HTML.
 *
 * Custom tab buttons (NOT shadcn Tabs) per F4: Radix Tabs expects React
 * children components, not arbitrary RSC nodes. Native role="tab" +
 * aria-selected passes WCAG 2.1 AA.
 */
export function ApiShowcaseTabs({ rendered }: ApiShowcaseTabsProps) {
  const [activeLang, setActiveLang] = useState<SnippetLang>("curl");
  const [activeEndpoint, setActiveEndpoint] = useState<string>(ENDPOINTS[0].id);

  const langs: SnippetLang[] = ["curl", "javascript", "python"];

  const activeNode = rendered.find(
    (r) => r.endpointId === activeEndpoint && r.lang === activeLang,
  )?.node;

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      {/* Endpoint selector chips */}
      <div role="tablist" aria-label="Endpoint" className="flex flex-wrap gap-2">
        {ENDPOINTS.map((ep) => (
          <button
            key={ep.id}
            role="tab"
            type="button"
            aria-selected={activeEndpoint === ep.id}
            onClick={() => setActiveEndpoint(ep.id)}
            className={cn(
              "rounded-md border px-3 py-1.5 font-mono text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300",
              activeEndpoint === ep.id
                ? "border-cyan-300/40 bg-cyan-300/10 text-cyan-100"
                : "border-white/10 text-white/60 hover:bg-white/5",
            )}
          >
            {ep.label}
          </button>
        ))}
      </div>

      {/* Language tabs */}
      <div role="tablist" aria-label="Language" className="flex gap-1 border-b border-white/10">
        {langs.map((lang) => (
          <button
            key={lang}
            role="tab"
            type="button"
            aria-selected={activeLang === lang}
            onClick={() => setActiveLang(lang)}
            className={cn(
              "border-b-2 px-3 py-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300",
              activeLang === lang
                ? "border-violet-400 text-white"
                : "border-transparent text-white/50 hover:text-white/80",
            )}
          >
            {LANG_LABELS[lang]}
          </button>
        ))}
      </div>

      {/* Snippet display panel */}
      <div role="tabpanel" aria-label={`${activeEndpoint} ${activeLang} example`}>
        {activeNode}
      </div>
    </div>
  );
}
