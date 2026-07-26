import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-6">
      <section className="max-w-md border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-6 text-center shadow-[0_12px_30px_var(--shadow-color)]">
        <h1 className="text-xl font-semibold text-[color:var(--fg-default)]">页面不存在</h1>
        <p className="mt-2 text-sm text-[color:var(--fg-muted)]">该地址未对应可访问的控制台页面。</p>
        <Button asChild variant="secondary" className="mt-5"><Link href="/dashboard">返回概览</Link></Button>
      </section>
    </main>
  );
}
