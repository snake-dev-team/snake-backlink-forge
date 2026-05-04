import { type CodeLang, renderCode } from "@/lib/code-window/render";
import { cn } from "@/lib/utils";
import { CodeWindowCopyButton } from "./code-window-copy-button";

type CodeWindowProps = {
  title?: string;
  lang?: CodeLang;
  code: string;
  showTrafficLights?: boolean;
  copyable?: boolean;
  className?: string;
};

/**
 * Async RSC code window with mac-style title bar and Shiki syntax highlighting.
 * Shiki runs at build time — zero client JS for syntax coloring.
 * Glass surface via .glass-card; copy button is the only client-side island.
 */
export async function CodeWindow({
  title = "terminal",
  lang = "bash",
  code,
  showTrafficLights = true,
  copyable = false,
  className,
}: CodeWindowProps) {
  const html = await renderCode(code, lang);

  return (
    <figure
      aria-label={`Code example: ${title}`}
      className={cn("glass-card overflow-hidden text-sm", className)}
    >
      <figcaption className="flex items-center justify-between gap-3 border-b border-white/5 px-4 py-2.5">
        <div className="flex items-center gap-3">
          {showTrafficLights && (
            <div className="flex gap-1.5" aria-hidden="true">
              <span className="size-3 rounded-full bg-red-500/80" />
              <span className="size-3 rounded-full bg-yellow-500/80" />
              <span className="size-3 rounded-full bg-green-500/80" />
            </div>
          )}
          <span className="font-mono text-xs text-white/50">{title}</span>
        </div>
        {copyable && <CodeWindowCopyButton code={code} />}
      </figcaption>

      {/* [&>pre]:!bg-transparent overrides Shiki inline background-color so glass-card surface shows through */}
      <div
        className="overflow-x-auto p-4 font-mono text-sm leading-relaxed [&>pre]:!bg-transparent"
        // biome-ignore lint/security/noDangerouslySetInnerHtml: Shiki output is build-time pre-rendered HTML, code is author-controlled
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </figure>
  );
}
