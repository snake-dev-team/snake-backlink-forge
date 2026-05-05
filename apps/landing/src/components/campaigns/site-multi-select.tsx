"use client";

import type { WpSite } from "@/lib/api/server-fetch";

type Props = {
  sites: WpSite[];
  /** Controlled: currently selected site IDs */
  selected: string[];
  onChange: (ids: string[]) => void;
};

/**
 * SiteMultiSelect — checkbox list of user's connected WP sites.
 * Renders a select-all toggle + per-site checkboxes.
 * Only shows connected sites; pending/error sites are disabled.
 */
export function SiteMultiSelect({ sites, selected, onChange }: Props) {
  const connected = sites.filter((s) => s.status === "connected");
  const allSelected = connected.length > 0 && selected.length === connected.length;

  function toggleAll() {
    if (allSelected) {
      onChange([]);
    } else {
      onChange(connected.map((s) => s.id));
    }
  }

  function toggle(id: string) {
    if (selected.includes(id)) {
      onChange(selected.filter((x) => x !== id));
    } else {
      onChange([...selected, id]);
    }
  }

  if (sites.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        Chưa có site nào kết nối.{" "}
        <a className="underline" href="/sites">
          Thêm site
        </a>{" "}
        trước.
      </p>
    );
  }

  return (
    <div className="space-y-2">
      {/* Select-all row */}
      <label className="flex cursor-pointer items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          className="size-4 accent-primary"
          checked={allSelected}
          onChange={toggleAll}
          aria-label="Chọn tất cả sites"
        />
        <span>
          Chọn tất cả{" "}
          <span className="text-muted-foreground">
            ({selected.length}/{connected.length} đã chọn)
          </span>
        </span>
      </label>

      <div className="max-h-48 overflow-y-auto rounded-md border border-input bg-muted/20 p-2 space-y-1">
        {sites.map((site) => {
          const isConnected = site.status === "connected";
          const isChecked = selected.includes(site.id);
          return (
            <label
              key={site.id}
              className={[
                "flex items-center gap-2 rounded px-2 py-1.5 text-sm transition-colors",
                isConnected
                  ? "cursor-pointer hover:bg-primary/10"
                  : "cursor-not-allowed opacity-50",
              ].join(" ")}
            >
              <input
                type="checkbox"
                className="size-4 accent-primary"
                checked={isChecked}
                disabled={!isConnected}
                onChange={() => isConnected && toggle(site.id)}
              />
              <span className="flex-1 truncate font-mono text-xs">{site.base_url}</span>
              {site.label && (
                <span className="shrink-0 text-xs text-muted-foreground">{site.label}</span>
              )}
              {!isConnected && (
                <span className="shrink-0 rounded-full bg-destructive/20 px-1.5 py-0.5 text-xs text-destructive">
                  {site.status}
                </span>
              )}
            </label>
          );
        })}
      </div>
    </div>
  );
}
