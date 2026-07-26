import Link from "next/link";
import { ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function ForbiddenPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-6">
      <section className="max-w-md border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-6 text-center shadow-[0_12px_30px_var(--shadow-color)]">
        <ShieldAlert aria-hidden="true" className="mx-auto h-8 w-8 text-[color:var(--danger)]" />
        <h1 className="mt-4 text-xl font-semibold text-[color:var(--fg-default)]">无权访问</h1>
        <p className="mt-2 text-sm text-[color:var(--fg-muted)]">当前账号没有访问此页面所需的权限。</p>
        <Button asChild variant="secondary" className="mt-5"><Link href="/dashboard">返回概览</Link></Button>
      </section>
    </main>
  );
}
