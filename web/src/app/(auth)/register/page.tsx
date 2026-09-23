import { Card } from "@/components/ui/card";
import { RegisterForm } from "@/features/auth/register-form";
export default function RegisterPage() {
  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-4">
      <Card className="w-full max-w-md p-6 sm:p-8">
        <RegisterForm />
      </Card>
    </main>
  );
}
