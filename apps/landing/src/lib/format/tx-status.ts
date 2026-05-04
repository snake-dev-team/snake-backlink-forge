// Mirrors services/api Postgres enum transaction_status (7 values):
// pending | paid | failed | refunded | manual_review | cancelled | recovered_by_late_payment

export type TxStatus =
  | "pending"
  | "paid"
  | "failed"
  | "refunded"
  | "manual_review"
  | "cancelled"
  | "recovered_by_late_payment";

type StatusMeta = { label: string; pillClass: string };

export const TX_STATUS_META: Record<TxStatus, StatusMeta> = {
  paid: {
    label: "Đã thanh toán",
    pillClass:
      "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30",
  },
  recovered_by_late_payment: {
    label: "Thanh toán bù trễ",
    pillClass:
      "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30",
  },
  pending: {
    label: "Đang chờ",
    pillClass: "bg-amber-500/15 text-amber-600 dark:text-amber-400 border border-amber-500/30",
  },
  failed: {
    label: "Thất bại",
    pillClass: "bg-red-500/15 text-red-600 dark:text-red-400 border border-red-500/30",
  },
  refunded: {
    label: "Hoàn tiền",
    pillClass: "bg-red-500/15 text-red-600 dark:text-red-400 border border-red-500/30",
  },
  cancelled: {
    label: "Đã hủy",
    pillClass: "bg-red-500/15 text-red-600 dark:text-red-400 border border-red-500/30",
  },
  manual_review: {
    label: "Đang kiểm tra",
    pillClass: "bg-muted text-muted-foreground border border-border",
  },
};

export function txStatusLabel(status: string): string {
  return TX_STATUS_META[status as TxStatus]?.label ?? status;
}

export function txStatusPillClass(status: string): string {
  return TX_STATUS_META[status as TxStatus]?.pillClass ?? TX_STATUS_META.manual_review.pillClass;
}

export function statusMeta(status: string): StatusMeta {
  return TX_STATUS_META[status as TxStatus] ?? TX_STATUS_META.manual_review;
}
