import { cn } from "@/lib/utils";

/**
 * Hero right-top: CLI snippet matching mockup style (E:\ui-b-developer-tool.html lines 237-253).
 *
 * Mockup signature elements replicated here:
 *   - Title bar: 3 traffic light dots + "~/sbf-cli" path + "bash" lang label (right side)
 *   - Mac-style window framing via .glass-card-strong glass surface
 *   - Multi-line output:
 *       $ sbf campaign create --topic "forex vn"   ← prompt + cmd + arg
 *       → Researching 1,247 keywords...            ← progress (muted)
 *       → Drafting 47 articles via Claude Opus...  ← progress (muted)
 *       ✓ Queued to 12 WordPress sites ▌          ← success (green) + blink cursor
 *
 * Why custom JSX instead of CodeWindow + Shiki:
 *   - Mockup uses semantic colors (violet for cmd, yellow for string, green for success)
 *     that don't map to bash language tokens. Hand-rolled gives us the exact mockup palette.
 *   - Blink cursor at end requires animation — CSS @keyframes blink defined in globals.css.
 *   - Zero client JS, full SSR. */
export function HeroCliSnippet() {
  return (
    <figure
      aria-label="Snake Backlink Forge CLI demo"
      className={cn("glass-card-strong overflow-hidden text-sm")}
    >
      {/* Title bar: traffic lights + path + lang */}
      <figcaption className="flex items-center justify-between border-b border-white/5 bg-black/30 px-3.5 py-2">
        <div className="flex items-center gap-2">
          <span className="size-[9px] rounded-full bg-red-500/60" aria-hidden="true" />
          <span className="size-[9px] rounded-full bg-yellow-500/60" aria-hidden="true" />
          <span className="size-[9px] rounded-full bg-emerald-500/60" aria-hidden="true" />
          <span className="ml-1.5 font-mono text-[11px] text-foreground/50 dark:text-white/50">
            ~/sbf-cli
          </span>
        </div>
        <span className="font-mono text-[10px] text-foreground/40 dark:text-white/40">bash</span>
      </figcaption>

      {/* Body: prompt + multi-line output, mono 12.5px / leading 1.7 (mockup) */}
      <div className="px-4 py-3.5 font-mono text-[12.5px] leading-[1.7]">
        <div>
          <span className="text-foreground/40 dark:text-white/40">$</span>{" "}
          <span className="text-violet-700 dark:text-violet-300">sbf</span>{" "}
          <span className="text-foreground/85 dark:text-white/85">campaign create --topic</span>{" "}
          <span className="text-amber-600 dark:text-amber-300">{`"forex vn"`}</span>
        </div>
        <div className="pl-3.5 text-foreground/55 dark:text-white/50">
          → Researching 1,247 keywords...
        </div>
        <div className="pl-3.5 text-foreground/55 dark:text-white/50">
          → Drafting 47 articles via Claude Opus...
        </div>
        <div className="pl-3.5 text-emerald-700 dark:text-emerald-300">
          ✓ Queued to 12 WordPress sites <span className="cli-blink-cursor">▌</span>
        </div>
      </div>
    </figure>
  );
}
