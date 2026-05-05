import Link from "next/link";
import { Button } from "@/components/ui/button";

const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "SnakeBacklinkForgeBot";

export function QuickActionsCard() {
  return (
    <article className="glass-card p-6">
      <header className="space-y-1">
        <h2 className="text-lg font-semibold tracking-tight">Thao tác nhanh</h2>
        <p className="text-sm text-muted-foreground">
          Mở bot, nạp credit, hoặc chuẩn bị kết nối WordPress.
        </p>
      </header>
      <div className="mt-4 grid gap-3 sm:grid-cols-3">
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
      </div>
    </article>
  );
}
