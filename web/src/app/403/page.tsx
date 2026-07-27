import Link from "next/link";
import { ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function ForbiddenPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-6">
      <Card className="max-w-md p-6 text-center">
        <ShieldAlert aria-hidden="true" className="mx-auto h-8 w-8 text-[color:var(--destructive)]" />
        <h1 className="mt-4 text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">无权访问</h1>
        <p className="mt-2 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">当前账号没有访问此页面所需的权限。</p>
        <Button asChild variant="secondary" className="mt-5"><Link href="/dashboard">返回概览</Link></Button>
      </Card>
    </main>
  );
}
