"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { connectWpSiteAction, type WpSiteActionState } from "../actions";

const initialState: WpSiteActionState = {};

export function ConnectSiteForm() {
  const [state, formAction, pending] = useActionState(connectWpSiteAction, initialState);

  return (
    <Card>
      <CardHeader>
        <CardTitle>WordPress credentials</CardTitle>
        <CardDescription>
          Dùng Application Password, không dùng mật khẩu đăng nhập chính.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form action={formAction} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="base_url">Site URL</Label>
            <Input id="base_url" name="base_url" placeholder="https://example.com" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="app_username">Username</Label>
            <Input id="app_username" name="app_username" placeholder="admin" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="app_password">Application Password</Label>
            <Input
              autoComplete="off"
              id="app_password"
              name="app_password"
              placeholder="xxxx xxxx xxxx xxxx xxxx xxxx"
              required
              type="password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="label">Label</Label>
            <Input id="label" name="label" placeholder="Money site / PBN 01" />
          </div>
          {state.error ? <p className="text-sm text-destructive">{state.error}</p> : null}
          <Button disabled={pending} type="submit">
            {pending ? "Validating..." : "Connect WordPress"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
