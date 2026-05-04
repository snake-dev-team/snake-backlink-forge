import { CreditCard } from "lucide-react";
import { fetchTransactionsServer } from "@/lib/api/server-fetch";
import { packageLabel } from "@/lib/constants/packages";
import { formatVnd } from "@/lib/format/currency";
import { formatDate } from "@/lib/format/date";
import { statusMeta } from "@/lib/format/tx-status";

export async function RecentTxCard() {
  let items: Record<string, unknown>[] = [];
  try {
    const data = await fetchTransactionsServer(5);
    items = data.items ?? [];
  } catch {
    return (
      <article className="glass-card p-6">
        <header className="space-y-1">
          <h2 className="text-lg font-semibold tracking-tight">Giao dịch gần đây</h2>
          <p className="text-sm text-muted-foreground">5 giao dịch credit mới nhất.</p>
        </header>
        <div className="mt-4">
          <p className="text-sm text-muted-foreground">
            Chưa tải được giao dịch. Đăng xuất rồi đăng nhập lại nếu lỗi kéo dài.
          </p>
        </div>
      </article>
    );
  }

  return (
    <article className="glass-card p-6">
      <header className="space-y-1">
        <h2 className="text-lg font-semibold tracking-tight">Giao dịch gần đây</h2>
        <p className="text-sm text-muted-foreground">5 giao dịch credit mới nhất.</p>
      </header>
      <div className="mt-4">
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Chưa có giao dịch.</p>
        ) : (
          <ul className="divide-y divide-border/50">
            {items.map((tx, index) => {
              const id = String(tx.id ?? index);
              const code = (tx.package_code as string | undefined) ?? null;
              const amount = Number(tx.amount_vnd ?? tx.amount ?? 0);
              const status = String(tx.status ?? "manual_review");
              const createdAt = tx.created_at;
              const meta = statusMeta(status);

              return (
                <li key={id} className="flex items-center gap-3 py-3">
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-muted/40 text-muted-foreground">
                    <CreditCard className="h-4 w-4" aria-hidden="true" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{packageLabel(code)}</p>
                    <p className="text-xs text-muted-foreground">{formatDate(createdAt)}</p>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <span className="font-mono text-sm">{formatVnd(amount)}</span>
                    <span
                      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs ${meta.pillClass}`}
                    >
                      {meta.label}
                    </span>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </article>
  );
}
