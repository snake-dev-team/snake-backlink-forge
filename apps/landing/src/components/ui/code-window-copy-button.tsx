"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";

type CodeWindowCopyButtonProps = { code: string; className?: string };

/**
 * Client island: copy-to-clipboard button for CodeWindow.
 * Silently fails if clipboard API is unavailable (non-HTTPS or denied).
 */
export function CodeWindowCopyButton({ code, className }: CodeWindowCopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleClick = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      /* clipboard denied — silently ignore; works on https:// prod */
    }
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label="Copy code"
      aria-pressed={copied}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md border border-white/10 px-2 py-1 text-xs text-white/60 transition-colors hover:bg-white/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300",
        copied && "border-emerald-400/40 text-emerald-300",
        className,
      )}
    >
      {copied ? (
        <Check className="size-3.5" aria-hidden="true" />
      ) : (
        <Copy className="size-3.5" aria-hidden="true" />
      )}
      {copied ? "Đã copy" : "Copy"}
    </button>
  );
}
