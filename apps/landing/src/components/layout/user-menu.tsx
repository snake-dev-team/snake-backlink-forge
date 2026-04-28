"use client";

import { KeyRound, LogOut, RotateCcw, User } from "lucide-react";
import { useRouter } from "next/navigation";
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

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.replace("/login");
    router.refresh();
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
        <DropdownMenuItem onClick={logout}>
          <LogOut className="h-4 w-4" />
          Đăng xuất
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
