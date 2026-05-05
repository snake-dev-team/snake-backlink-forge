/**
 * Canonical 10 SKU registry — mirrored from services/api/internal/service/packages.go:25-66.
 *
 * SOURCE OF TRUTH: the backend Go map. This TS mirror MUST stay in sync on every
 * backend bump. Deferred CI smoke test (phase 8+) will diff this dict vs Go source
 * on PR and fail on AmountVND or DisplayVI drift.
 *
 * Do NOT invent codes. Valid codes are ONLY the 10 entries below.
 */

export type PackagePool = "standard" | "premium" | "combo";

export type LandingPackage = {
  /** Canonical code matching packages.go key */
  code: string;
  pool: PackagePool;
  /** Verbatim DisplayVI from packages.go */
  label: string;
  /** Canonical AmountVND from packages.go */
  priceVnd: number;
  /** Credit descriptor — matches DisplayVI suffix (Vietnamese: "50 credit" not "50 credits") */
  detail: string;
  /** Featured flag from packages.go */
  featured?: boolean;
};

/**
 * Mirrored from services/api/internal/service/packages.go — verify on every backend bump.
 * @see services/api/internal/service/packages.go:25-66
 */
export const LANDING_PACKAGES: LandingPackage[] = [
  // Standard pool — 4 SKUs
  {
    code: "standard_starter_50",
    pool: "standard",
    label: "Standard Starter — 50 credit",
    priceVnd: 99_000,
    detail: "50 credit",
  },
  {
    code: "standard_basic_100",
    pool: "standard",
    label: "Standard Basic — 100 credit",
    priceVnd: 179_000,
    detail: "100 credit",
  },
  {
    code: "standard_pro_200",
    pool: "standard",
    label: "Standard Pro — 200 credit",
    priceVnd: 329_000,
    detail: "200 credit",
    featured: true,
  },
  {
    code: "standard_max_300",
    pool: "standard",
    label: "Standard Max — 300 credit",
    priceVnd: 459_000,
    detail: "300 credit",
  },
  // Premium pool — 4 SKUs
  {
    code: "premium_starter_50",
    pool: "premium",
    label: "Premium Starter — 50 credit",
    priceVnd: 499_000,
    detail: "50 credit",
  },
  {
    code: "premium_basic_100",
    pool: "premium",
    label: "Premium Basic — 100 credit",
    priceVnd: 899_000,
    detail: "100 credit",
  },
  {
    code: "premium_pro_200",
    pool: "premium",
    label: "Premium Pro — 200 credit",
    priceVnd: 1_699_000,
    detail: "200 credit",
    featured: true,
  },
  {
    code: "premium_max_300",
    pool: "premium",
    label: "Premium Max — 300 credit",
    priceVnd: 2_399_000,
    detail: "300 credit",
  },
  // Combo pool — 2 SKUs
  {
    code: "combo_p100_s50",
    pool: "combo",
    label: "Combo 100P+50S",
    priceVnd: 999_000,
    detail: "100P+50S",
  },
  {
    code: "combo_p200_s100",
    pool: "combo",
    label: "Combo 200P+100S",
    priceVnd: 1_799_000,
    detail: "200P+100S",
  },
];

export type PricingTier = {
  id: string;
  /** Optional badge text shown above name */
  badge?: string;
  name: string;
  /** Package code used as representative for price + CTA */
  representative: string;
  features: string[];
  highlighted?: boolean;
};

/** 3-tier display — representative SKUs: standard_pro_200 / premium_pro_200 / combo_p200_s100 */
export const PRICING_TIERS: PricingTier[] = [
  {
    id: "standard",
    name: "Standard",
    representative: "standard_pro_200",
    features: [
      "200 credit standard pool",
      "5 site WordPress kết nối",
      "Daily limit + ethical mode",
      "Hỗ trợ qua Telegram",
    ],
  },
  {
    id: "premium",
    name: "Premium",
    representative: "premium_pro_200",
    badge: "Phổ biến nhất",
    highlighted: true,
    features: [
      "200 credit premium pool",
      "20 site WordPress kết nối",
      "Anti-abuse + footprint randomization",
      "Multi-pool credit audit",
      "Hỗ trợ ưu tiên",
    ],
  },
  {
    id: "combo",
    name: "Combo",
    representative: "combo_p200_s100",
    features: [
      "200 credit premium + 100 standard",
      "Tách audit theo pool",
      "Phù hợp team scale",
      "Hỗ trợ ưu tiên",
    ],
  },
];

/** Find a package by its canonical code. Returns undefined if code is invalid. */
export function findPackage(code: string): LandingPackage | undefined {
  return LANDING_PACKAGES.find((p) => p.code === code);
}
