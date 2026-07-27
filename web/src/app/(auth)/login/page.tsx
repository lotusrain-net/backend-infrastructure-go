"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Card } from "@/components/ui/card";
import { LoginForm } from "@/features/auth/login-form";

export default function LoginPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-4">
      <Card className="w-full max-w-md p-6 sm:p-8">
        <Suspense fallback={<LoginForm />}>
          <LoginPageContent />
        </Suspense>
      </Card>
    </main>
  );
}

function LoginPageContent() {
  const searchParams = useSearchParams();

  return <LoginForm nextPath={searchParams.get("next") ?? undefined} />;
}
