"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { STATIC_ACTIONS } from "@/lib/command-palette/actions";
import { useTransactionsCache } from "@/lib/command-palette/use-transactions-cache";
import { packageLabel } from "@/lib/constants/packages";
import { formatVnd } from "@/lib/format/currency";

/**
 * Global ⌘K / Ctrl+K command palette.
 * Mounted once in (app)/layout.tsx — only active on auth-gated routes.
 *
 * Security:
 * - All action payloads are static constants (no user-controlled redirects).
 * - External links use noopener to prevent reverse tabnabbing.
 * - Transaction list is scoped to the authenticated user via /api/proxy cookie auth.
 *
 * IME safety (F9): keydown handler skips while e.isComposing or keyCode 229
 * so Vietnamese tone/diacritic input (Telex, VNI) never accidentally fires ⌘K.
 */
export function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const { data: txs } = useTransactionsCache(open);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // F9: Vietnamese IME composition safety.
      // e.isComposing is the standard check; keyCode 229 is the legacy "Process" signal
      // sent by some IMEs (Windows TSF). Skip both to prevent accidental palette triggers
      // while typing tones with Telex/VNI/VIQR input methods.
      if (e.isComposing || e.keyCode === 229) return;

      // Meta+K (Mac) or Ctrl+K without Alt (Win/Linux — avoids AltGr conflict)
      const isCmdK = (e.metaKey || (e.ctrlKey && !e.altKey)) && e.key === "k";
      if (isCmdK) {
        e.preventDefault();
        setOpen((prev) => !prev);
        return;
      }

      // Escape closes without toggling — avoids double-fire with Dialog's own handler
      if (e.key === "Escape" && open) {
        setOpen(false);
      }
    };

    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  const runAction = async (action: (typeof STATIC_ACTIONS)[number]) => {
    setOpen(false);
    switch (action.kind) {
      case "navigate":
        if (action.payload) router.push(action.payload);
        break;
      case "external":
        if (action.payload) window.open(action.payload, "_blank", "noopener,noreferrer");
        break;
      case "logout":
        try {
          await fetch("/api/auth/logout", { method: "POST" });
        } catch {
          // Cookie cleared on next auth-required hit by middleware — safe to proceed
        }
        router.push("/login");
        break;
    }
  };

  const navActions = STATIC_ACTIONS.filter((a) => a.kind === "navigate");
  const otherActions = STATIC_ACTIONS.filter((a) => a.kind !== "navigate");

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        className="glass-card-modal w-full max-w-2xl p-0 shadow-2xl shadow-violet-950/40"
        showCloseButton={false}
      >
        {/* sr-only title + description satisfy Radix Dialog WCAG 2.1 AA */}
        <DialogTitle className="sr-only">Command palette</DialogTitle>
        <DialogDescription className="sr-only">
          Tìm và mở nhanh trang, hành động hoặc giao dịch gần đây bằng bàn phím.
        </DialogDescription>
        <Command>
          <CommandInput placeholder="Tìm trang, hành động hoặc giao dịch..." autoFocus />
          <CommandList>
            <CommandEmpty>Không tìm thấy kết quả.</CommandEmpty>

            <CommandGroup heading="Trang">
              {navActions.map((action) => (
                <CommandItem
                  key={action.id}
                  value={action.label}
                  onSelect={() => runAction(action)}
                >
                  <action.icon className="mr-2 size-4" aria-hidden="true" />
                  {action.label}
                </CommandItem>
              ))}
            </CommandGroup>

            <CommandSeparator />

            <CommandGroup heading="Hành động">
              {otherActions.map((action) => (
                <CommandItem
                  key={action.id}
                  value={action.label}
                  onSelect={() => runAction(action)}
                >
                  <action.icon className="mr-2 size-4" aria-hidden="true" />
                  {action.label}
                </CommandItem>
              ))}
            </CommandGroup>

            {txs.length > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup heading="Giao dịch gần đây">
                  {txs.slice(0, 10).map((tx) => {
                    const label = packageLabel(tx.package_code);
                    return (
                      <CommandItem
                        key={tx.id}
                        value={`${label} ${tx.package_code ?? ""} ${tx.id} ${formatVnd(tx.amount_vnd)}`}
                        onSelect={() => {
                          setOpen(false);
                          router.push(`/dashboard?tx=${tx.id}`);
                        }}
                      >
                        <span className="mr-2 flex-1 truncate text-sm">{label}</span>
                        <span className="font-mono text-xs">{formatVnd(tx.amount_vnd)}</span>
                        <span className="ml-2 text-xs text-muted-foreground">{tx.status}</span>
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
