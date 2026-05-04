"use client";

import { KeyRound, LogOut, RotateCcw, User } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type UserMenuProps = {
  botUsername: string;
  keyPrefix: string;
  username?: string | null;
};

export function UserMenu({ botUsername, keyPrefix, username }: UserMenuProps) {
  const router = useRouter();
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const [logoutPending, setLogoutPending] = useState(false);

  async function logout() {
    if (logoutPending) {
      return;
    }

    setLogoutError(null);
    setLogoutPending(true);
    try {
      const response = await fetch("/api/auth/logout", { method: "POST" });
      if (!response.ok) {
        setLogoutError("Đăng xuất chưa thành công. Thử lại nhé.");
        return;
      }

      router.replace("/login");
      router.refresh();
      window.location.assign("/login");
    } catch {
      setLogoutError("Không thể kết nối để đăng xuất. Thử lại nhé.");
    } finally {
      setLogoutPending(false);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button className="gap-2" type="button" variant="outline">
          <User className="h-4 w-4" />
          {username ? `@${username}` : "Tài khoản"}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel>Tài khoản SBF</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled>
          <KeyRound className="h-4 w-4" />
          <span className="font-mono text-xs">{keyPrefix}••••</span>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={`https://t.me/${botUsername}?start=regenkey`} rel="noreferrer" target="_blank">
            <RotateCcw className="h-4 w-4" />
            Tạo lại key
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={logoutPending} onClick={logout}>
          <LogOut className="h-4 w-4" />
          {logoutPending ? "Đang đăng xuất..." : "Đăng xuất"}
        </DropdownMenuItem>
        {logoutError ? <p className="px-2 py-1 text-xs text-destructive">{logoutError}</p> : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
