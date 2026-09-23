"use client";
import { useEffect, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { ShieldCheck } from "lucide-react";
import { SingleSelect } from "@/components/patterns/single-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSecurityMutations, useSecurityQuery } from "./security-api";
import type { SecurityMode, TOTPEnrollment } from "@/types/api";
const options: { value: SecurityMode; label: string }[] = [
  { value: "default", label: "default" },
  { value: "email", label: "启用邮箱二次验证" },
  { value: "totp", label: "启用一次性代码二次验证" },
];
export function SecurityControls() {
  const query = useSecurityQuery();
  const mutations = useSecurityMutations();
  const [selection, setSelection] = useState<SecurityMode | null>(null);
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [recovery, setRecovery] = useState(false);
  const [enrollment, setEnrollment] = useState<TOTPEnrollment | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const selected = selection ?? query.data?.mode ?? "default";
  useEffect(() => {
    if (!enrollment) return;
    const timer = setTimeout(() => {
      setEnrollment(null);
      setMessage("绑定已过期，请重新开始。");
    }, enrollment.expires_in * 1000);
    return () => clearTimeout(timer);
  }, [enrollment]);
  async function perform(action: () => Promise<void>) {
    if (pending) return;
    setPending(true);
    setMessage("");
    try {
      await action();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "操作失败，请重试。");
    } finally {
      setPending(false);
      setPassword("");
      setCode("");
      mutations.enroll.reset();
      mutations.confirm.reset();
      mutations.disable.reset();
    }
  }
  async function choose(mode: SecurityMode) {
    setSelection(mode);
    setEnrollment(null);
    setRecoveryCodes([]);
    setPassword("");
    setCode("");
    if (mode === "totp" && !query.data?.totp_enabled) return;
    if (query.data?.totp_enabled && mode !== "totp") return;
    await perform(async () => {
      await mutations.save.mutateAsync(mode);
      setSelection(null);
      setMessage("两步验证设置已保存。");
    });
  }
  const disabling = query.data?.totp_enabled && selected !== "totp";
  return (
    <section
      aria-label="两步验证"
      className="mt-6 space-y-4 border-t border-[color:var(--border)] pt-6"
    >
      <h2 className="flex items-center gap-2 font-semibold">
        <ShieldCheck aria-hidden="true" className="size-5" />
        两步验证
      </h2>
      <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
        default 使用密码登录；邮箱二次验证需要已验证的邮箱。
      </p>
      <SingleSelect
        value={selected}
        onChange={choose}
        options={options}
        label="两步验证方式"
        disabled={pending || query.isPending || !!query.error}
      />
      {query.error && <p role="alert">无法读取安全设置，请稍后重试。</p>}
      {((selected === "totp" &&
        !query.data?.totp_enabled &&
        !enrollment &&
        recoveryCodes.length === 0) ||
        disabling) && (
        <div className="space-y-3">
          <Label htmlFor="security-password">当前密码</Label>
          <Input
            id="security-password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            disabled={pending}
          />
          {disabling ? (
            <>
              <Label htmlFor="security-proof">
                {recovery ? "恢复码" : "一次性代码"}
              </Label>
              <Input
                id="security-proof"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                autoComplete="off"
                disabled={pending}
              />
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setRecovery(!recovery);
                  setCode("");
                }}
              >
                {recovery ? "使用一次性代码" : "使用恢复码"}
              </Button>
              <Button
                type="button"
                disabled={pending || !password || !code}
                onClick={() =>
                  perform(async () => {
                    await mutations.disable.mutateAsync({
                      password,
                      ...(recovery ? { recovery_code: code } : { code }),
                    });
                    if (selected !== "default")
                      await mutations.save.mutateAsync(selected);
                    setSelection(null);
                    setMessage("一次性代码二次验证已停用。");
                  })
                }
              >
                验证并停用
              </Button>
            </>
          ) : (
            <Button
              type="button"
              disabled={pending || !password}
              onClick={() =>
                perform(async () => {
                  setEnrollment(
                    await mutations.enroll.mutateAsync({ password }),
                  );
                })
              }
            >
              开始绑定
            </Button>
          )}
        </div>
      )}
      {enrollment && (
        <div className="space-y-3">
          <p>使用身份验证器扫描二维码，或手动输入密钥。</p>
          <QRCodeSVG
            value={enrollment.otpauth_uri}
            size={192}
            marginSize={4}
            role="img"
            aria-label="身份验证器绑定二维码"
            className="max-w-full"
          />
          <p className="break-all font-mono text-sm">{enrollment.secret}</p>
          <Label htmlFor="totp-confirm">一次性代码</Label>
          <Input
            id="totp-confirm"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            value={code}
            onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
            disabled={pending}
          />
          <Button
            type="button"
            disabled={pending || !/^\d{6}$/.test(code)}
            onClick={() =>
              perform(async () => {
                const result = await mutations.confirm.mutateAsync(code);
                setEnrollment(null);
                setRecoveryCodes(result.recovery_codes);
                setSelection(null);
              })
            }
          >
            确认绑定
          </Button>
        </div>
      )}
      {recoveryCodes.length > 0 && (
        <div className="space-y-3" role="status">
          <p>恢复码仅展示这一次。请安全保存，每个恢复码只能使用一次。</p>
          <ul className="grid gap-2 sm:grid-cols-2">
            {recoveryCodes.map((value) => (
              <li key={value} className="break-all font-mono text-sm">
                {value}
              </li>
            ))}
          </ul>
          <Button type="button" onClick={() => setRecoveryCodes([])}>
            我已安全保存恢复码
          </Button>
        </div>
      )}
      {query.data?.totp_enabled && (
        <p className="text-sm">
          剩余恢复码：{query.data.recovery_codes_remaining}
        </p>
      )}
      {query.data?.recovery_codes_expires_at && (
        <p className="text-sm">
          恢复码到期时间：
          {new Date(query.data.recovery_codes_expires_at).toLocaleString()}
        </p>
      )}
      {message && (
        <p role="status" className="text-sm">
          {message}
        </p>
      )}
    </section>
  );
}
