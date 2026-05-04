import { CodeWindow } from "@/components/ui/code-window";
import { ENDPOINTS } from "@/lib/landing/api-snippets";
import { ApiShowcaseTabs } from "./api-showcase-tabs";

/**
 * Server component — pre-renders all 9 CodeWindow nodes (3 endpoints × 3 langs)
 * at build time via Shiki. Passes rendered nodes to ApiShowcaseTabs client island.
 * Tab switching has zero network requests — all nodes are already in the HTML.
 */
export async function ApiShowcase() {
  // Build all snippet nodes concurrently. React cache() in renderCode dedupes
  // identical code+lang calls so repeated identical snippets cost zero extra work.
  const rendered = await Promise.all(
    ENDPOINTS.flatMap((ep) =>
      Object.values(ep.snippets).map(async (snip) => ({
        endpointId: ep.id,
        lang: snip.lang,
        node: (
          <CodeWindow
            key={`${ep.id}-${snip.lang}`}
            title={snip.title}
            lang={snip.shikiLang}
            code={snip.code}
            copyable
          />
        ),
      })),
    ),
  );

  return (
    <section id="api" className="space-y-10 py-20">
      <div className="space-y-3 text-center">
        <p className="text-sm uppercase tracking-[0.2em] text-cyan-300/80">API</p>
        <h2 className="mx-auto max-w-2xl text-balance text-4xl font-semibold tracking-tight">
          Tích hợp REST API trong vài dòng
        </h2>
        <p className="mx-auto max-w-xl text-foreground/70 dark:text-white/60">
          Lấy API key từ Telegram bot, gọi endpoints như ví dụ bên dưới. Authentication bằng Bearer
          token.
        </p>
        <p className="text-xs text-foreground/50 dark:text-white/40">
          {"API_BASE_URL = "}
          <code className="font-mono text-cyan-300/80">https://snake-backlink-api.fly.dev</code>
          {" (custom domain sẽ thay thế sau launch)"}
        </p>
      </div>

      {/* Left/right split on lg: checklist + tabbed code window */}
      <div className="mx-auto max-w-5xl lg:grid lg:grid-cols-2 lg:gap-12">
        {/* Left — feature checklist */}
        <div className="mb-10 space-y-6 lg:mb-0">
          <p className="text-sm font-semibold uppercase tracking-[0.18em] text-violet-300/80">
            Type-safe API · OpenAPI 3.1
          </p>
          <ul className="space-y-4">
            {[
              "OpenAPI 3.1 contract",
              "Idempotent operations",
              "Signed webhooks",
              "Rate limited per key",
            ].map((item) => (
              <li key={item} className="flex items-start gap-3">
                <span
                  className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-violet-500/20 text-violet-300"
                  aria-hidden="true"
                >
                  <svg
                    className="size-3"
                    viewBox="0 0 12 12"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                    aria-hidden="true"
                    focusable="false"
                  >
                    <title>check</title>
                    <path
                      d="M2 6l3 3 5-5"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </span>
                <span className="text-sm leading-6 text-foreground/80 dark:text-white/75">
                  {item}
                </span>
              </li>
            ))}
          </ul>
        </div>

        {/* Right — tabbed code window (client island) */}
        <ApiShowcaseTabs rendered={rendered} />
      </div>
    </section>
  );
}
