// Package code → Vietnamese display label
// Source of truth: services/api/internal/service/packages.go:25-66 (DisplayVI field)
// Defense-in-depth regex at packages.go:71 ENFORCES these exact 10 codes.
// Update both files together when packages change.

export const PACKAGE_LABELS: Record<string, string> = {
  standard_starter_50: "Standard Starter — 50 credit",
  standard_basic_100: "Standard Basic — 100 credit",
  standard_pro_200: "Standard Pro — 200 credit",
  standard_max_300: "Standard Max — 300 credit",
  premium_starter_50: "Premium Starter — 50 credit",
  premium_basic_100: "Premium Basic — 100 credit",
  premium_pro_200: "Premium Pro — 200 credit",
  premium_max_300: "Premium Max — 300 credit",
  combo_p100_s50: "Combo 100P+50S",
  combo_p200_s100: "Combo 200P+100S",
};

export function packageLabel(code: string | null | undefined): string {
  if (!code) return "—";
  return PACKAGE_LABELS[code] ?? code;
}
