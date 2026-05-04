import { FEATURE_CELLS } from "@/lib/landing/feature-cells";
import { BentoCell } from "./bento-cell";

/**
 * Server component — 6-cell asymmetric bento grid.
 * Grid: 12-col desktop, 2-col tablet (sm), 1-col mobile.
 * Replaces the old flat 3-col features array (Phase 05).
 */
export function FeaturesBento() {
  return (
    <section id="features" className="space-y-10 py-20">
      <div className="space-y-3 text-center">
        <p className="text-sm uppercase tracking-[0.2em] text-cyan-300/80">Tính năng</p>
        <h2 className="mx-auto max-w-2xl text-balance text-4xl font-semibold tracking-tight">
          Tất cả công cụ SEO operator cần, gom vào một cockpit
        </h2>
      </div>
      <div className="grid grid-cols-12 gap-4 md:gap-6">
        {FEATURE_CELLS.map((cell) => (
          <BentoCell key={cell.id} cell={cell} />
        ))}
      </div>
    </section>
  );
}
