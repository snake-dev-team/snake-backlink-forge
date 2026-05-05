"use client";

/**
 * Visible trigger button for the ⌘K command palette.
 * Dispatches a synthetic Ctrl+K keydown that CommandPalette's existing
 * window.addEventListener("keydown") handler picks up — no state lift needed.
 * Hidden on mobile/tablet; shown at lg+ breakpoint only.
 */
export function CmdKButton() {
  function handleClick() {
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true }));
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label="Mở command palette (Ctrl+K)"
      className="hidden lg:flex items-center gap-1.5 rounded-md border border-border/50 px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-primary/5 hover:text-foreground"
    >
      <kbd className="font-sans opacity-70">⌘K</kbd>
      <span>Tìm</span>
    </button>
  );
}
