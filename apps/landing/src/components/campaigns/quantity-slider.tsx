"use client";

type Props = {
  value: number;
  onChange: (v: number) => void;
  pool: "standard" | "premium";
  min?: number;
  max?: number;
};

const CREDIT_RATE: Record<"standard" | "premium", number> = {
  standard: 1,
  premium: 2,
};

/**
 * QuantitySlider — native range input 1-100 with live credit cost preview.
 * Cost: quantity * 1 credit (standard) or quantity * 2 credits (premium).
 */
export function QuantitySlider({ value, onChange, pool, min = 1, max = 100 }: Props) {
  const rate = CREDIT_RATE[pool] ?? 1;
  const cost = value * rate;
  const pct = ((value - min) / (max - min)) * 100;

  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between text-sm">
        <span className="font-medium">Quantity</span>
        <span className="tabular-nums text-muted-foreground">
          <span className="text-foreground font-semibold">{value}</span> backlinks
        </span>
      </div>

      {/* Styled range input */}
      <div className="relative">
        <input
          type="range"
          min={min}
          max={max}
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
          className="w-full h-2 rounded-full appearance-none cursor-pointer bg-muted [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:shadow [&::-moz-range-thumb]:size-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-primary [&::-moz-range-thumb]:border-0"
          style={{
            background: `linear-gradient(to right, hsl(var(--primary)) ${pct}%, hsl(var(--muted)) ${pct}%)`,
          }}
          aria-valuenow={value}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-label="Quantity"
        />
      </div>

      {/* Cost preview */}
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{min}</span>
        <span className="rounded-full border border-primary/30 bg-primary/10 px-2 py-0.5 text-primary font-medium">
          {cost} credit{cost !== 1 ? "s" : ""} ({pool})
        </span>
        <span>{max}</span>
      </div>
    </div>
  );
}
