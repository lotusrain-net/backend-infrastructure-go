"use client";

import { useRef, useState } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/client";
import { EmailCode, validEmailCode } from "./email-code";
import { VerificationCodeInput } from "./verification-code-input";

export function requiresEmailVerification(error: unknown) {
  return (
    error instanceof ApiError &&
    error.status === 422 &&
    ((error.details as Record<string, unknown> | undefined)?.email_code === "required" ||
      error.fieldErrors?.email_code === "required")
  );
}

export type VerificationProof =
  | { code: string; recovery_code?: never }
  | { recovery_code: string; code?: never };

type VerificationDialogProps = {
  purpose: "login" | "register";
  onVerify: (proof: VerificationProof) => Promise<void>;
  onCancel: () => void;
} & ({ method: "email"; email: string } | { method: "totp"; email?: never });

export function VerificationDialog(props: VerificationDialogProps) {
  const { purpose, onVerify, onCancel } = props;
  const [code, setCode] = useState("");
  const [useRecovery, setUseRecovery] = useState(false);
  const [message, setMessage] = useState("");
  const [isPending, setIsPending] = useState(false);
  const pending = useRef(false);
  const action = purpose === "login" ? "登录" : "注册";
  const valid = useRecovery ? Boolean(code.trim()) : validEmailCode(code);

  function changeCode(value: string) {
    setCode(value);
    setMessage("");
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending.current) onCancel();
      }}
    >
      <DialogContent showClose={!isPending}>
        <DialogHeader>
          <DialogTitle>身份验证</DialogTitle>
          <DialogDescription>
            {props.method === "email"
              ? `请获取发送至 ${props.email} 的六位邮箱验证码，完成${action}。`
              : useRecovery
                ? "请输入一个尚未使用的恢复码，完成登录。"
                : "请输入身份验证器中的六位一次性代码，完成登录。"}
          </DialogDescription>
        </DialogHeader>
        <form
          className="min-h-0 overflow-y-auto"
          noValidate
          onSubmit={async (event) => {
            event.preventDefault();
            if (pending.current || !valid) return;
            pending.current = true;
            setIsPending(true);
            setMessage("");
            try {
              await onVerify(
                useRecovery ? { recovery_code: code.trim() } : { code },
              );
            } catch (error) {
              setMessage(
                error instanceof ApiError && error.status === 401
                  ? "验证码无效或已过期，请重新输入。"
                  : error instanceof Error
                    ? error.message
                    : "验证失败，请稍后重试。",
              );
            } finally {
              pending.current = false;
              setIsPending(false);
            }
          }}
        >
          <div className="space-y-4 px-5 py-4">
            {props.method === "email" ? (
              <EmailCode
                email={props.email}
                purpose={purpose}
                value={code}
                onChange={changeCode}
                disabled={isPending}
              />
            ) : (
              <div className="space-y-2">
                <Label htmlFor="login-verification-code">
                  {useRecovery ? "恢复码" : "一次性代码"}
                </Label>
                {useRecovery ? (
                  <Input
                    id="login-verification-code"
                    value={code}
                    onChange={(event) => changeCode(event.target.value)}
                    autoComplete="one-time-code"
                    disabled={isPending}
                  />
                ) : (
                  <VerificationCodeInput
                    id="login-verification-code"
                    value={code}
                    onChange={changeCode}
                    disabled={isPending}
                  />
                )}
              </div>
            )}
            {message && (
              <Alert variant="destructive">
                <AlertDescription>{message}</AlertDescription>
              </Alert>
            )}
            {props.method === "totp" && (
              <Button
                type="button"
                variant="ghost"
                disabled={isPending}
                onClick={() => {
                  setUseRecovery(!useRecovery);
                  changeCode("");
                }}
              >
                {useRecovery ? "使用一次性代码" : "使用恢复码"}
              </Button>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={isPending}
              onClick={onCancel}
            >
              返回{action}
            </Button>
            <Button type="submit" disabled={isPending || !valid}>
              {isPending ? "正在验证…" : `验证并${action}`}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
