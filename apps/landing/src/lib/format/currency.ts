export function formatCredits(value: number): string {
  return new Intl.NumberFormat("vi-VN").format(value);
}

const VND_FORMATTER = new Intl.NumberFormat("vi-VN", {
  style: "currency",
  currency: "VND",
  maximumFractionDigits: 0,
});

/** Expects positive integer VND amount. Negative values render with leading minus. */
export function formatVnd(amountInVnd: number): string {
  // amountInVnd is whole units (no sub-units in VND)
  return VND_FORMATTER.format(amountInVnd);
}
