import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { fetchWpSitesServer, type WpSite } from "@/lib/api/server-fetch";
import { deleteWpSiteAction, revalidateWpSiteAction } from "./actions";
import { DeleteSiteButton, RevalidateSiteButton } from "./site-action-buttons";

export const dynamic = "force-dynamic";

export default async function SitesPage() {
  let items: WpSite[] | null = null;
  try {
    items = (await fetchWpSitesServer()).items;
  } catch {
    items = null;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm text-muted-foreground">Phase 5</p>
          <h1 className="text-3xl font-semibold tracking-tight">WordPress Sites</h1>
          <p className="text-muted-foreground">Kết nối WordPress bằng Application Password.</p>
        </div>
        <Button asChild>
          <Link href="/sites/connect">Connect site</Link>
        </Button>
      </div>

      {items === null ? (
        <SitesUnavailableCard />
      ) : items.length === 0 ? (
        <EmptySitesCard />
      ) : (
        <SitesTable items={items} />
      )}
    </div>
  );
}

function SitesUnavailableCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Chưa tải được WordPress sites</CardTitle>
        <CardDescription>
          Backend WordPress service chưa sẵn sàng hoặc phiên đăng nhập cần làm mới.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button asChild variant="outline">
          <Link href="/sites/connect">Connect WordPress</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

function EmptySitesCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Chưa có WordPress site</CardTitle>
        <CardDescription>Thêm site đầu tiên để chuẩn bị campaign automation.</CardDescription>
      </CardHeader>
      <CardContent>
        <Button asChild variant="outline">
          <Link href="/sites/connect">Connect WordPress</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

function SitesTable({ items }: { items: WpSite[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Connected sites</CardTitle>
        <CardDescription>
          Application Password được mã hoá ở backend, không hiển thị lại.
        </CardDescription>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <table className="w-full min-w-[760px] text-left text-sm">
          <thead className="border-b text-muted-foreground">
            <tr>
              <th className="py-3 pr-4 font-medium">Site</th>
              <th className="py-3 pr-4 font-medium">Username</th>
              <th className="py-3 pr-4 font-medium">Status</th>
              <th className="py-3 pr-4 font-medium">Last error</th>
              <th className="py-3 pr-4 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((site) => (
              <tr className="border-b last:border-0" key={site.id}>
                <td className="py-4 pr-4">
                  <div className="font-medium">{site.label || site.base_url}</div>
                  <div className="text-xs text-muted-foreground">{site.base_url}</div>
                </td>
                <td className="py-4 pr-4">{site.app_username}</td>
                <td className="py-4 pr-4">
                  <span className="rounded-full border px-2 py-1 text-xs">{site.status}</span>
                </td>
                <td className="max-w-[220px] truncate py-4 pr-4 text-muted-foreground">
                  {site.last_error ?? "—"}
                </td>
                <td className="py-4 pr-4">
                  <div className="flex justify-end gap-2">
                    <form action={revalidateWpSiteAction}>
                      <input name="id" type="hidden" value={site.id} />
                      <RevalidateSiteButton />
                    </form>
                    <form action={deleteWpSiteAction}>
                      <input name="id" type="hidden" value={site.id} />
                      <DeleteSiteButton />
                    </form>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
