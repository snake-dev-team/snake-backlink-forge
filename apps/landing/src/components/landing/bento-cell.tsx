import { CodeWindow } from "@/components/ui/code-window";
import type { FeatureCell } from "@/lib/landing/feature-cells";
import { cn } from "@/lib/utils";

const SIZE_CLASS: Record<FeatureCell["size"], string> = {
  large: "col-span-12 md:col-span-6 md:row-span-2",
  medium: "col-span-12 sm:col-span-6 md:col-span-4",
  wide: "col-span-12",
};

type BentoCellProps = { cell: FeatureCell };

/** Async RSC — CodeWindow highlight runs at build time if snippet present. */
export async function BentoCell({ cell }: BentoCellProps) {
  const Icon = cell.icon;
  return (
    <article className={cn("glass-card flex flex-col gap-4 p-6", SIZE_CLASS[cell.size])}>
      <div className="flex items-center gap-3">
        <span className="flex size-9 items-center justify-center rounded-md bg-violet-500/15 text-violet-200">
          <Icon className="size-4" aria-hidden="true" />
        </span>
        <h3 className="text-lg font-semibold tracking-tight">{cell.title}</h3>
      </div>
      <p className="text-sm leading-6 text-foreground dark:text-white/85">{cell.description}</p>
      {cell.snippet && (
        <CodeWindow
          title={cell.snippet.title}
          lang={cell.snippet.lang}
          code={cell.snippet.code}
          showTrafficLights={false}
          className="mt-auto !text-xs"
        />
      )}
    </article>
  );
}
