"use client";

import { useActionState, useState } from "react";
import { QuantitySlider } from "@/components/campaigns/quantity-slider";
import { SiteMultiSelect } from "@/components/campaigns/site-multi-select";
import { type TonePreference, ToneSelect } from "@/components/campaigns/tone-select";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { WpSite } from "@/lib/api/server-fetch";
import { type CampaignActionState, createCampaignAction } from "./actions";

const initialState: CampaignActionState = {};

type Props = {
  sites: WpSite[];
};

export function CampaignCreateForm({ sites }: Props) {
  const [state, formAction, pending] = useActionState(createCampaignAction, initialState);

  // Controlled state for components that can't use plain FormData inputs
  const [selectedSiteIds, setSelectedSiteIds] = useState<string[]>([]);
  const [quantity, setQuantity] = useState(10);
  const [tone, setTone] = useState<TonePreference>("professional");
  const [pool, setPool] = useState<"standard" | "premium">("standard");

  // Show site error either from server-side fieldErrors or after a submit attempt with 0 selected.
  const siteValidationError: string | null =
    state.fieldErrors?.site_ids ??
    (state.attempted === true && selectedSiteIds.length === 0 ? "Chọn ít nhất 1 site." : null);

  return (
    <Card className="overflow-hidden border-primary/20 bg-gradient-to-br from-card via-card to-primary/5">
      <CardHeader>
        <CardTitle>Launch campaign</CardTitle>
        <CardDescription>
          Tạo queue backlink có giới hạn credit, ethical/niche filter mặc định bật.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form action={formAction} className="grid gap-4 md:grid-cols-2">
          {/* --- Hidden fields for controlled state --- */}
          {selectedSiteIds.map((id) => (
            <input key={id} name="site_ids" type="hidden" value={id} />
          ))}
          <input name="quantity" type="hidden" value={quantity} />
          <input name="tone_preference" type="hidden" value={tone} />

          {/* --- Row 1: name + money_site_url --- */}
          <Field label="Campaign name" name="name" placeholder="SaaS launch backlinks" />
          <Field
            label="Money site URL"
            name="money_site_url"
            placeholder="https://example.com/service"
          />

          {/* --- Row 2: keywords + anchors --- */}
          <Field
            label="Niche keywords"
            name="niche_keywords"
            placeholder="seo, automation, wordpress"
          />
          <Field
            label="Anchor texts"
            name="anchor_texts"
            placeholder="dịch vụ seo, backlink chất lượng"
          />

          {/* --- Row 3: daily_limit + credits_allocated --- */}
          <Field label="Daily limit" name="daily_limit" placeholder="5" type="number" />
          <Field
            label="Credits allocated"
            name="credits_allocated"
            placeholder="20"
            type="number"
          />

          {/* --- Row 4: pool + source_mode --- */}
          <label className="grid gap-2 text-sm font-medium">
            Pool
            <select
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              name="pool"
              value={pool}
              onChange={(e) => setPool(e.target.value as "standard" | "premium")}
            >
              <option value="standard">Standard</option>
              <option value="premium">Premium</option>
            </select>
          </label>
          <label className="grid gap-2 text-sm font-medium">
            Source mode
            <select
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              name="source_mode"
            >
              <option value="custom">Custom targets</option>
              <option value="prebuilt">System pool</option>
              <option value="autofind">Auto-find</option>
              <option value="mixed">Mixed</option>
            </select>
          </label>

          {/* --- Quantity slider: full-width --- */}
          <div className="md:col-span-2">
            <QuantitySlider value={quantity} onChange={setQuantity} pool={pool} />
          </div>

          {/* --- Tone preference: full-width --- */}
          <div className="md:col-span-2">
            <ToneSelect value={tone} onChange={setTone} />
          </div>

          {/* --- Site multi-select: full-width --- */}
          <div className="grid gap-2 md:col-span-2">
            <Label>Target WP sites</Label>
            <SiteMultiSelect
              sites={sites}
              selected={selectedSiteIds}
              onChange={setSelectedSiteIds}
            />
            {siteValidationError && (
              <p className="text-xs text-destructive">{siteValidationError}</p>
            )}
          </div>

          {/* --- Start now checkbox --- */}
          <label className="flex items-center gap-2 text-sm text-muted-foreground md:col-span-2">
            <input className="size-4" name="start_now" type="checkbox" />
            Start ngay sau khi tạo
          </label>

          {/* --- Error display --- */}
          {state.error ? (
            <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive md:col-span-2">
              {state.error}
            </div>
          ) : null}

          {/* --- Submit --- */}
          <div className="md:col-span-2">
            <Button disabled={pending} type="submit">
              {pending ? "Creating..." : "Create campaign"}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function Field({
  label,
  name,
  placeholder,
  type = "text",
}: {
  label: string;
  name: string;
  placeholder: string;
  type?: string;
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={name}>{label}</Label>
      <Input id={name} name={name} placeholder={placeholder} type={type} />
    </div>
  );
}
