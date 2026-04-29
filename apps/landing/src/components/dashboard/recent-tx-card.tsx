import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { fetchTransactionsServer } from "@/lib/api/server-fetch";
import { formatDate } from "@/lib/format/date";

function cell(value: unknown): string {
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return "—";
}

export async function RecentTxCard() {
  let tx: Awaited<ReturnType<typeof fetchTransactionsServer>> | null = null;
  try {
    tx = await fetchTransactionsServer(5);
  } catch {
    tx = null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Giao dịch gần đây</CardTitle>
        <CardDescription>5 giao dịch credit mới nhất.</CardDescription>
      </CardHeader>
      <CardContent>
        {!tx ? (
          <p className="text-sm text-muted-foreground">
            Chưa tải được giao dịch. Đăng xuất rồi đăng nhập lại nếu lỗi kéo dài.
          </p>
        ) : tx.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Chưa có giao dịch.</p>
        ) : (
          <div className="space-y-3">
            {tx.items.map((item, index) => (
              <div className="rounded-lg border p-3 text-sm" key={`${cell(item.id)}-${index}`}>
                <div className="flex items-center justify-between gap-3">
                  <span className="font-medium">
                    {cell(item.package_code ?? item.type ?? item.status)}
                  </span>
                  <span className="text-muted-foreground">{cell(item.amount ?? item.credits)}</span>
                </div>
                <div className="mt-1 text-muted-foreground">{formatDate(item.created_at)}</div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
