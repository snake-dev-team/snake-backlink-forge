"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { type CampaignActionState, createCampaignAction } from "./actions";

const initialState: CampaignActionState = {};

export function CampaignCreateForm() {
  const [state, formAction, pending] = useActionState(createCampaignAction, initialState);

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
          <Field label="Campaign name" name="name" placeholder="SaaS launch backlinks" />
          <Field
            label="Money site URL"
            name="money_site_url"
            placeholder="https://example.com/service"
          />
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
          <Field label="Daily limit" name="daily_limit" placeholder="5" type="number" />
          <Field
            label="Credits allocated"
            name="credits_allocated"
            placeholder="20"
            type="number"
          />
          <label className="grid gap-2 text-sm font-medium">
            Pool
            <select
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              name="pool"
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
          <label className="flex items-center gap-2 text-sm text-muted-foreground md:col-span-2">
            <input className="size-4" name="start_now" type="checkbox" />
            Start ngay sau khi tạo
          </label>
          {state.error ? (
            <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive md:col-span-2">
              {state.error}
            </div>
          ) : null}
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
