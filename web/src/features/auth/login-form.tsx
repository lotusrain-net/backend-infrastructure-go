"use client";

import Link from "next/link";
import { validEmail } from "./email-code";
import {
  requiresEmailVerification,
  VerificationDialog,
} from "./verification-dialog";
import { useVerifyLoginMutation } from "./security-api";
import type { TOTPChallenge } from "@/types/api";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
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
    if (
      decodedPath.startsWith("//") ||
      /[\\\\\u0000-\u001F\u007F]/.test(decodedPath)
    ) {
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
  const verify = useVerifyLoginMutation();
  const [challenge, setChallenge] = useState<TOTPChallenge | null>(null);
  const [emailVerification, setEmailVerification] =
    useState<LoginValues | null>(null);
  const pending = useRef(false);
  const [message, setMessage] = useState<string | null>(null);
  useEffect(() => {
    if (!challenge) return;
    const timer = setTimeout(() => {
      setChallenge(null);
      setMessage("验证已过期，请重新登录。");
    }, challenge.expires_in * 1000);
    return () => clearTimeout(timer);
  }, [challenge]);
  const form = useForm<LoginValues>({
    defaultValues: { email: "", password: "" },
  });

  async function submitLogin(values: LoginValues, emailCode?: string) {
    try {
      const result = await login.mutateAsync({
        email: values.email.trim(),
        password: values.password,
        ...(emailCode ? { email_code: emailCode } : {}),
      });
      form.resetField("password");
      setEmailVerification(null);
      if (result && "status" in result && result.status === "totp_required") {
        setChallenge(result);
        return;
      }
      router.replace(resolveLoginDestination(nextPath));
    } finally {
      login.reset();
    }
  }

  async function handleSubmit(values: LoginValues) {
    if (pending.current || login.isPending || emailVerification || challenge)
      return;
    setMessage(null);
    if (!validEmail(values.email)) {
      setMessage("请填写有效邮箱。");
      return;
    }
    pending.current = true;
    try {
      await submitLogin(values);
    } catch (error) {
      if (requiresEmailVerification(error)) {
        setEmailVerification({ ...values, email: values.email.trim() });
      } else {
        setMessage(
          error instanceof Error ? error.message : "登录失败，请稍后重试。",
        );
      }
    } finally {
      pending.current = false;
    }
  }

  return (
    <>
      <Form {...form}>
        <form
          onSubmit={(event) => void form.handleSubmit(handleSubmit)(event)}
          className="space-y-5"
          noValidate
        >
          <div className="space-y-1">
            <h1 className="text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">
              登录平台控制台
            </h1>
            <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
              使用已分配的工作账号继续。
            </p>
          </div>
          <FormField
            control={form.control}
            name="email"
            rules={{ required: "请输入邮箱。" }}
            render={({ field }) => (
              <FormItem>
                <FormLabel>邮箱</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type="email"
                    autoComplete="email"
                    inputMode="email"
                    disabled={
                      login.isPending || Boolean(emailVerification || challenge)
                    }
                  />
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
                  <Input
                    {...field}
                    type="password"
                    autoComplete="current-password"
                    disabled={
                      login.isPending || Boolean(emailVerification || challenge)
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {message ? (
            <Alert variant="destructive">
              <AlertDescription>{message}</AlertDescription>
            </Alert>
          ) : null}
          <Button
            type="submit"
            className="w-full"
            disabled={
              login.isPending || Boolean(emailVerification || challenge)
            }
          >
            {login.isPending ? "正在登录..." : "登录"}
          </Button>
          <Link
            href="/register"
            className="block text-center text-sm underline"
          >
            创建账号
          </Link>
        </form>
      </Form>
      {emailVerification && (
        <VerificationDialog
          key="email"
          method="email"
          purpose="login"
          email={emailVerification.email}
          onVerify={async ({ code }) => {
            await submitLogin(emailVerification, code);
          }}
          onCancel={() => {
            setEmailVerification(null);
            setMessage(null);
          }}
        />
      )}
      {challenge && (
        <VerificationDialog
          key={challenge.challenge_id}
          method="totp"
          purpose="login"
          onVerify={async (proof) => {
            try {
              await verify.mutateAsync({
                challenge_id: challenge.challenge_id,
                ...proof,
              });
              setChallenge(null);
              router.replace(resolveLoginDestination(nextPath));
            } finally {
              verify.reset();
            }
          }}
          onCancel={() => {
            setChallenge(null);
            setMessage(null);
          }}
        />
      )}
    </>
  );
}
