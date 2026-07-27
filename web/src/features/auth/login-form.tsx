"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { useLoginMutation } from "@/features/auth/api";

interface LoginValues {
  email: string;
  password: string;
}

export function resolveLoginDestination(nextPath?: string) {
  if (!nextPath) {
    return "/dashboard";
  }

  try {
    const decodedPath = decodeURIComponent(nextPath);
    if (decodedPath.startsWith("//") || /[\\\\\u0000-\u001F\u007F]/.test(decodedPath)) {
      return "/dashboard";
    }

    const localOrigin = "http://local.invalid";
    const target = new URL(nextPath, localOrigin);
    if (target.origin !== localOrigin || !target.pathname.startsWith("/")) {
      return "/dashboard";
    }

    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return "/dashboard";
  }
}

export function LoginForm({ nextPath }: { nextPath?: string }) {
  const router = useRouter();
  const login = useLoginMutation();
  const [message, setMessage] = useState<string | null>(null);
  const form = useForm<LoginValues>({
    defaultValues: { email: "", password: "" },
  });

  async function handleSubmit(values: LoginValues) {
    setMessage(null);

    try {
      await login.mutateAsync({ email: values.email.trim(), password: values.password });
      router.replace(resolveLoginDestination(nextPath));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "登录失败，请稍后重试。");
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5" noValidate>
        <div className="space-y-1">
          <h1 className="text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">登录平台控制台</h1>
          <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">使用已分配的工作账号继续。</p>
        </div>
        <FormField
          control={form.control}
          name="email"
          rules={{ required: "请输入邮箱。" }}
          render={({ field }) => (
            <FormItem>
              <FormLabel>邮箱</FormLabel>
              <FormControl>
                <Input {...field} type="email" autoComplete="email" inputMode="email" disabled={login.isPending} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="password"
          rules={{ required: "请输入密码。" }}
          render={({ field }) => (
            <FormItem>
              <FormLabel>密码</FormLabel>
              <FormControl>
                <Input {...field} type="password" autoComplete="current-password" disabled={login.isPending} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        {message ? <Alert variant="destructive"><AlertDescription>{message}</AlertDescription></Alert> : null}
        <Button type="submit" className="w-full" disabled={login.isPending}>
          {login.isPending ? "正在登录..." : "登录"}
        </Button>
      </form>
    </Form>
  );
}
