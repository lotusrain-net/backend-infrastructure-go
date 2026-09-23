"use client";
import { Card } from "@/components/ui/card";
import { RegisterForm } from "@/features/auth/register-form";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
export default function RegisterPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-4">
      <Card className="w-full max-w-md p-6 sm:p-8">
        <Suspense fallback={<RegisterForm />}><RegisterPageContent /></Suspense>
      </Card>
    </main>
  );
}

function RegisterPageContent() {
  const params = useSearchParams();
  return <RegisterForm nextPath={params.get("next") ?? undefined} />;
}
