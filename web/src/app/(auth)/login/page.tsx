"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { LoginForm } from "@/features/auth/login-form";

export default function LoginPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-4">
      <section className="w-full max-w-md border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-6 shadow-[0_12px_30px_var(--shadow-color)] sm:p-8">
        <Suspense fallback={<LoginForm />}>
          <LoginPageContent />
        </Suspense>
      </section>
    </main>
  );
}

function LoginPageContent() {
  const searchParams = useSearchParams();

  return <LoginForm nextPath={searchParams.get("next") ?? undefined} />;
}
