import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function NotFound() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-6">
      <Card className="max-w-md p-6 text-center">
        <h1 className="text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">页面不存在</h1>
        <p className="mt-2 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">该地址未对应可访问的控制台页面。</p>
        <Button asChild variant="secondary" className="mt-5"><Link href="/dashboard">返回概览</Link></Button>
      </Card>
    </main>
  );
}
