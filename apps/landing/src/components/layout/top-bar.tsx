import { ThemeToggle } from "@/components/theme-toggle";
import { fetchMeServer } from "@/lib/api/server-fetch";
import { CmdKButton } from "./cmd-k-button";
import { UserMenu } from "./user-menu";

const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "SnakeBacklinkForgeBot";

export async function TopBar() {
  let me: Awaited<ReturnType<typeof fetchMeServer>> | null = null;
  try {
    me = await fetchMeServer();
  } catch {
    me = null;
  }

  return (
    <div className="flex items-center gap-2">
      <CmdKButton />
      <ThemeToggle />
      <UserMenu
        botUsername={botUsername}
        keyPrefix={me?.key_prefix ?? "sbf_live"}
        username={me?.telegram_username}
      />
    </div>
  );
}
