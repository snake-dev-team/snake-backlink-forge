import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "SnakeBacklinkForgeBot";

export function QuickActionsCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Thao tác nhanh</CardTitle>
        <CardDescription>Mở bot, nạp credit, hoặc chuẩn bị kết nối WordPress.</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-3">
        <Button asChild variant="outline">
          <a href={`https://t.me/${botUsername}`} rel="noreferrer" target="_blank">
            Mở Telegram bot
          </a>
        </Button>
        <Button asChild variant="outline">
          <a href={`https://t.me/${botUsername}?start=topup`} rel="noreferrer" target="_blank">
            Nạp credit
          </a>
        </Button>
        <Button asChild>
          <Link href="/sites/connect">Kết nối WordPress</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
