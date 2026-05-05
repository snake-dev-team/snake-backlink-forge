/**
 * Hero right-bottom: Browser-framed dashboard mockup matching UI B mockup chrome.
 * Mockup ground truth (E:\ui-b-developer-tool.html lines 257-364).
 *
 * Visible elements:
 *   - Browser title bar: 3 traffic lights + "app.snakebacklink.com" + "3 active" badge
 *   - Campaign header row: title "Q4 Forex Campaign" + "RUNNING" status pill (pulse)
 *   - 2 primary KPI cards (F11 constraint: keep 2 not 4 to avoid density mismatch)
 *   - Inline DR-progression sparkline SVG (decorative)
 *   - 1 activity log row (recent publish event)
 *
 * F11 compliance: 2 KPI cards at full opacity. The browser chrome + sparkline + activity row
 * are decorative storytelling that mirror the real /dashboard route's vibe without
 * faking metric density that doesn't exist there.
 *
 * aria-hidden: decorative mockup, real values live on /dashboard.
 * Zero client JS — pulse animation driven by CSS keyframes in globals.css.
 */
export function HeroDashboardMockup() {
  return (
    <div
      role="presentation"
      aria-hidden="true"
      className="glass-card-strong overflow-hidden rounded-xl shadow-2xl shadow-violet-950/40 ring-1 ring-violet-500/10"
    >
      {/* Browser title bar (mockup signature: traffic lights + URL + active count) */}
      <div className="flex items-center justify-between border-b border-white/5 bg-black/30 px-3.5 py-2.5">
        <div className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-full bg-red-500/70" />
          <span className="size-2.5 rounded-full bg-yellow-500/70" />
          <span className="size-2.5 rounded-full bg-emerald-500/70" />
          <span className="ml-2 font-mono text-[11px] text-foreground/50 dark:text-white/50">
            app.snakebacklink.com
          </span>
        </div>
        <span className="font-mono text-[10px] text-foreground/45 dark:text-white/45">
          3 active
        </span>
      </div>

      {/* Main panel (no sidebar — saves vertical space; sidebar is implied by chrome) */}
      <div className="p-4">
        {/* Campaign header row: title + status pill */}
        <div className="mb-4 flex items-start justify-between">
          <div>
            <div className="text-[15px] font-semibold text-foreground dark:text-white">
              Q4 Forex Campaign
            </div>
            <div className="font-mono text-[11px] text-foreground/55 dark:text-white/50">
              847 keywords · 12 sites · running
            </div>
          </div>
          <div className="flex items-center gap-1.5 rounded-md border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1 font-mono text-[10px] text-emerald-700 dark:text-emerald-300">
            <span className="pulse-dot size-1.5 rounded-full bg-emerald-600 dark:bg-emerald-400" />
            RUNNING
          </div>
        </div>

        {/* 2 KPI cards (F11 lock) — mockup styling: mono uppercase label, big tabular num,
            mono delta. Second card uses violet-tinted surface for visual variation. */}
        <div className="mb-3 grid grid-cols-2 gap-2">
          <div className="rounded-lg border border-white/[0.06] bg-white/[0.02] px-3 py-2.5 dark:border-white/[0.06]">
            <div className="mb-1 font-mono text-[10px] uppercase tracking-wider text-foreground/50 dark:text-white/50">
              Credit
            </div>
            <div className="font-semibold text-[22px] leading-none tracking-tight tabular-nums text-foreground dark:text-white">
              12,500
            </div>
            <div className="mt-1.5 font-mono text-[10px] text-emerald-700 dark:text-emerald-300">
              ↑ 12.4%
            </div>
          </div>
          <div className="rounded-lg border border-violet-500/25 bg-violet-500/[0.06] px-3 py-2.5">
            <div className="mb-1 font-mono text-[10px] uppercase tracking-wider text-violet-700 dark:text-violet-300">
              Campaigns
            </div>
            <div className="font-semibold text-[22px] leading-none tracking-tight tabular-nums text-violet-700 dark:text-violet-200">
              3
            </div>
            <div className="mt-1.5 font-mono text-[10px] text-violet-700 dark:text-violet-300">
              ↑ 4.7×
            </div>
          </div>
        </div>

        {/* DR progression sparkline (mockup line 320-338) — inline SVG, no JS */}
        <div className="mb-3 rounded-lg border border-white/[0.06] bg-white/[0.02] p-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="font-mono text-[10px] uppercase tracking-wider text-foreground/50 dark:text-white/50">
              DR progression · 30D
            </span>
            <span className="font-mono text-[10px] text-foreground/45 dark:text-white/45">
              +47 pts
            </span>
          </div>
          <svg viewBox="0 0 280 60" className="h-12 w-full" aria-hidden="true">
            <defs>
              <linearGradient id="hero-dash-grad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#a78bfa" stopOpacity="0.4" />
                <stop offset="100%" stopColor="#a78bfa" stopOpacity="0" />
              </linearGradient>
            </defs>
            <path
              d="M 0 50 L 20 48 L 40 45 L 60 47 L 80 42 L 100 38 L 120 35 L 140 30 L 160 28 L 180 22 L 200 18 L 220 14 L 240 10 L 260 8 L 280 5 L 280 60 L 0 60 Z"
              fill="url(#hero-dash-grad)"
            />
            <path
              d="M 0 50 L 20 48 L 40 45 L 60 47 L 80 42 L 100 38 L 120 35 L 140 30 L 160 28 L 180 22 L 200 18 L 220 14 L 240 10 L 260 8 L 280 5"
              stroke="#a78bfa"
              strokeWidth="1.5"
              fill="none"
            />
            <circle cx="280" cy="5" r="3" fill="#a78bfa" />
            <circle cx="280" cy="5" r="6" fill="#a78bfa" opacity="0.3" />
          </svg>
        </div>

        {/* Single activity row (mockup has 3; we keep 1 to fit hero space) */}
        <div className="flex items-center gap-2.5 rounded-md bg-white/[0.02] px-2.5 py-2 text-[11.5px]">
          <span className="font-mono text-[9px] text-foreground/45 dark:text-white/40">14:32</span>
          <span className="size-1.5 rounded-full bg-emerald-500" />
          <span className="flex-1 truncate text-foreground/85 dark:text-white/85">
            muachung.vn · published{" "}
            <span className="font-mono text-foreground/55 dark:text-white/50">
              forex-broker-vn-2026.html
            </span>
          </span>
          <span className="font-mono text-[10px] text-emerald-700 dark:text-emerald-300">200</span>
        </div>
      </div>
    </div>
  );
}
