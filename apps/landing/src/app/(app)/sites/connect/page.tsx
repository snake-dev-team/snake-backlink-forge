import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ConnectSiteForm } from "./connect-site-form";

export const dynamic = "force-dynamic";

export default function ConnectSitePage() {
  return (
    <div className="max-w-2xl space-y-6">
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">Phase 5</p>
        <h1 className="text-3xl font-semibold tracking-tight">Kết nối WordPress</h1>
        <p className="text-muted-foreground">
          Backend sẽ validate REST API và quyền publish_posts trước khi lưu.
        </p>
      </div>
      <ConnectSiteForm />
      <Button asChild variant="ghost">
        <Link href="/sites">Back to sites</Link>
      </Button>
    </div>
  );
}
