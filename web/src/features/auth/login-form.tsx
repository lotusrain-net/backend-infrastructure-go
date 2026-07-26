"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useLoginMutation } from "@/features/auth/api";

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
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage(null);

    if (!email.trim() || !password) {
      setMessage("请输入邮箱和密码。");
      return;
    }

    try {
      await login.mutateAsync({ email: email.trim(), password });
      router.replace(resolveLoginDestination(nextPath));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "登录失败，请稍后重试。");
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5" noValidate>
      <div className="space-y-1">
        <h1 className="text-xl font-semibold text-[color:var(--fg-default)]">登录平台控制台</h1>
        <p className="text-sm text-[color:var(--fg-muted)]">使用已分配的工作账号继续。</p>
      </div>
      <label className="grid gap-1.5 text-sm font-medium text-[color:var(--fg-default)]">
        邮箱
        <Input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="email" inputMode="email" />
      </label>
      <label className="grid gap-1.5 text-sm font-medium text-[color:var(--fg-default)]">
        密码
        <Input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" />
      </label>
      {message ? <p role="alert" className="border-l-2 border-[color:var(--danger)] bg-[color:var(--danger-subtle)] px-3 py-2 text-sm text-[color:var(--danger)]">{message}</p> : null}
      <Button type="submit" className="w-full" disabled={login.isPending}>
        {login.isPending ? "正在登录..." : "登录"}
      </Button>
    </form>
  );
}
