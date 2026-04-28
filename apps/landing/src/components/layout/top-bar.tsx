import { ThemeToggle } from "@/components/theme-toggle";
import { fetchMeServer } from "@/lib/api/server-fetch";
import { UserMenu } from "./user-menu";

const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "SnakeBacklinkForgeBot";

export async function TopBar() {
  const me = await fetchMeServer();

  return (
    <div className="flex items-center gap-2">
      <ThemeToggle />
      <UserMenu
        botUsername={botUsername}
        keyPrefix={me.key_prefix}
        username={me.telegram_username}
      />
    </div>
  );
}
