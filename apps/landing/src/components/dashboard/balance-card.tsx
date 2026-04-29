import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { fetchMeServer } from "@/lib/api/server-fetch";
import { formatCredits } from "@/lib/format/currency";

export async function BalanceCard() {
  let me: Awaited<ReturnType<typeof fetchMeServer>> | null = null;
  try {
    me = await fetchMeServer();
  } catch {
    me = null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Số dư credit</CardTitle>
        <CardDescription>Dùng cho campaign và publishing workflow.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {me ? (
          <>
            <div className="text-4xl font-semibold tracking-tight">
              {formatCredits(me.balance_credits)}
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm text-muted-foreground">
              <div>Premium: {formatCredits(me.premium_credits)}</div>
              <div>Standard: {formatCredits(me.standard_credits)}</div>
            </div>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">
            Chưa tải được số dư. Đăng xuất rồi đăng nhập lại nếu lỗi kéo dài.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
