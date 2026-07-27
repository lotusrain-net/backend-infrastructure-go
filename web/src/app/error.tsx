"use client";

import { useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-6">
      <Card className="max-w-md p-6 text-center">
        <AlertTriangle aria-hidden="true" className="mx-auto h-8 w-8 text-[color:var(--warning)]" />
        <h1 className="mt-4 text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">页面暂时不可用</h1>
        <p className="mt-2 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">请重试，或稍后返回控制台。</p>
        <Button className="mt-5" onClick={reset}>重试</Button>
      </Card>
    </main>
  );
}
