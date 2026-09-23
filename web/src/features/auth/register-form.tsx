"use client";
import { useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { validEmail } from "./email-code";
import {
  requiresEmailVerification,
  VerificationDialog,
} from "./verification-dialog";
import type { RegisterRequest } from "@/types/api";
import { useRegisterMutation } from "./security-api";
export function RegisterForm() {
  const router = useRouter();
  const register = useRegisterMutation();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [verification, setVerification] = useState<RegisterRequest | null>(
    null,
  );
  const pending = useRef(false);
  const [message, setMessage] = useState("");
  async function submitRegistration(input: RegisterRequest) {
    try {
      await register.mutateAsync(input);
      setPassword("");
      setVerification(null);
      router.replace("/login?registered=1");
    } finally {
      register.reset();
    }
  }
  return (
    <>
      <form
        className="space-y-5"
        onSubmit={async (event) => {
          event.preventDefault();
          if (pending.current || register.isPending || verification) return;
          setMessage("");
          if (!validEmail(email) || !username.trim() || password.length < 12) {
            setMessage("请填写有效邮箱、用户名及至少 12 位密码。");
            return;
          }
          const input = {
            username: username.trim(),
            email: email.trim(),
            password,
          };
          pending.current = true;
          try {
            await submitRegistration(input);
          } catch (error) {
            if (requiresEmailVerification(error)) {
              setVerification(input);
            } else {
              setMessage(
                error instanceof Error
                  ? error.message
                  : "注册失败，请稍后重试。",
              );
            }
          } finally {
            pending.current = false;
          }
        }}
      >
        <h1 className="text-[length:var(--text-heading)] font-semibold">
          创建账号
        </h1>
        <div className="space-y-2">
          <Label htmlFor="register-username">用户名</Label>
          <Input
            id="register-username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            maxLength={100}
            required
            disabled={register.isPending || Boolean(verification)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="register-email">邮箱</Label>
          <Input
            id="register-email"
            type="email"
            value={email}
            onChange={(event) => {
              setEmail(event.target.value);
            }}
            autoComplete="email"
            required
            disabled={register.isPending || Boolean(verification)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="register-password">密码</Label>
          <Input
            id="register-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="new-password"
            minLength={12}
            maxLength={1024}
            required
            disabled={register.isPending || Boolean(verification)}
          />
        </div>
        {message && <p role="alert">{message}</p>}
        <Button
          className="w-full"
          type="submit"
          disabled={register.isPending || Boolean(verification)}
        >
          {register.isPending ? "正在注册…" : "注册"}
        </Button>
        <Link href="/login" className="block text-center text-sm underline">
          返回登录
        </Link>
      </form>
      {verification && (
        <VerificationDialog
          method="email"
          purpose="register"
          email={verification.email}
          onVerify={async ({ code }) => {
            await submitRegistration({ ...verification, email_code: code });
          }}
          onCancel={() => {
            setVerification(null);
            setMessage("");
          }}
        />
      )}
    </>
  );
}
