"use client";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { VerificationCodeInput } from "./verification-code-input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/client";
import { useEmailCodeMutation } from "./security-api";
export const validEmail = (value: string) =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
export const validEmailCode = (value: string) => /^\d{6}$/.test(value);
export function EmailCode({
  email,
  purpose,
  value,
  onChange,
  disabled = false,
}: {
  email: string;
  purpose: "register" | "login" | "verify_email";
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const send = useEmailCodeMutation();
  const [seconds, setSeconds] = useState(0);
  const [error, setError] = useState("");
  const pending = useRef(false);
  const identity = `${email.trim().toLowerCase()}\u0000${purpose}`;
  const previousIdentity = useRef(identity);
  useEffect(() => {
    if (previousIdentity.current !== identity) {
      previousIdentity.current = identity;
      setSeconds(0);
      setError("");
      onChange("");
    }
  }, [identity, onChange]);
  useEffect(() => {
    if (seconds <= 0) return;
    const timer = setTimeout(() => setSeconds(seconds - 1), 1000);
    return () => clearTimeout(timer);
  }, [seconds]);
  async function request() {
    if (pending.current || seconds > 0 || !validEmail(email)) return;
    pending.current = true;
    setError("");
    try {
      const requestIdentity = identity;
      const result = await send.mutateAsync({ email: email.trim(), purpose });
      if (requestIdentity !== previousIdentity.current) return;
      setSeconds(result.resend_after_seconds);
    } catch (error) {
      if (error instanceof ApiError && error.status === 429)
        setSeconds(error.retryAfterSeconds ?? 60);
      setError(
        error instanceof ApiError && error.status === 503
          ? "验证码邮件暂时无法发送，请稍后重试或联系管理员检查邮件服务。"
          : error instanceof Error ? error.message : "发送失败，请稍后重试。",
      );
    } finally {
      pending.current = false;
      send.reset();
    }
  }
  return (
    <div className="space-y-2">
      <Label htmlFor={`${purpose}-email-code`}>邮箱验证码</Label>
      <div className="flex gap-2">
        <VerificationCodeInput
          id={`${purpose}-email-code`}
          value={value}
          onChange={onChange}
          disabled={disabled}
        />
        <Button
          type="button"
          variant="outline"
          onClick={request}
          disabled={
            disabled || send.isPending || seconds > 0 || !validEmail(email)
          }
        >
          {seconds > 0
            ? `${seconds} 秒后重发`
            : send.isPending
              ? "正在发送…"
              : "发送验证码"}
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-[color:var(--destructive)]">
          {error}
        </p>
      )}
    </div>
  );
}

export const EmailCodeField = EmailCode;
