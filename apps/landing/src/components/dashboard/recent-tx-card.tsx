import { CreditCard } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
      <Card>
        <CardHeader>
          <CardTitle>Giao dịch gần đây</CardTitle>
          <CardDescription>5 giao dịch credit mới nhất.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Chưa tải được giao dịch. Đăng xuất rồi đăng nhập lại nếu lỗi kéo dài.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Giao dịch gần đây</CardTitle>
        <CardDescription>5 giao dịch credit mới nhất.</CardDescription>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Chưa có giao dịch.</p>
        ) : (
          <ul className="divide-y divide-border">
            {items.map((tx, index) => {
              const id = String(tx.id ?? index);
              const code = (tx.package_code as string | undefined) ?? null;
              const amount = Number(tx.amount_vnd ?? tx.amount ?? 0);
              const status = String(tx.status ?? "manual_review");
              const createdAt = tx.created_at;
              const meta = statusMeta(status);

              return (
                <li key={id} className="flex items-center gap-3 py-3">
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                    <CreditCard className="h-4 w-4" />
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{packageLabel(code)}</p>
                    <p className="text-xs text-muted-foreground">{formatDate(createdAt)}</p>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <span className="text-sm font-mono">{formatVnd(amount)}</span>
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
      </CardContent>
    </Card>
  );
}
