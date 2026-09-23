"use client";
import { useState } from "react";
import { KeyRound } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { hasPermission } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";
import { useBasicAuthQuery, useSaveBasicAuthMutation } from "./security-api";
import type { BasicAuthSettings } from "@/types/api";
const defaults: BasicAuthSettings = {
  password_login_enabled: true,
  registration_enabled: false,
  registration_email_verification_required: true,
  allowed_email_domains: [],
};
export function BasicAuthSettingsPage() {
  const user = useAuthStore((state) => state.user);
  const canRead = hasPermission(
    { permissions: user?.permissions ?? [] },
    "system-settings:read",
  );
  const canWrite = hasPermission(
    { permissions: user?.permissions ?? [] },
    "system-settings:write",
  );
  const query = useBasicAuthQuery(canRead);
  const save = useSaveBasicAuthMutation();
  const [draft, setDraft] = useState<BasicAuthSettings | null>(null);
  const [domains, setDomains] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const value = draft ?? query.data ?? defaults;
  const disabled =
    !canWrite || query.isPending || !!query.error || save.isPending;
  if (!canRead) return <p role="alert">你没有查看基本身份验证设置的权限。</p>;
  return (
    <div className="space-y-6">
      <PageHeader
        title="基本身份验证"
        description="管理密码登录、公开注册与注册邮箱验证策略。"
      />
      <Card className="max-w-3xl">
        <form
          className="space-y-6"
          onSubmit={async (event) => {
            event.preventDefault();
            setMessage("");
            try {
              await save.mutateAsync({
                ...value,
                allowed_email_domains: (
                  domains ?? value.allowed_email_domains.join(", ")
                )
                  .split(/[,，\s]+/)
                  .map((domain) => domain.trim().toLowerCase())
                  .filter(Boolean),
              });
              setDraft(null);
              setDomains(null);
              setMessage("设置已保存。");
            } catch (error) {
              setMessage(error instanceof Error ? error.message : "保存失败。");
            }
          }}
        >
          <h2 className="flex items-center gap-2 font-semibold">
            <KeyRound aria-hidden="true" className="size-5" />
            基本认证设置
          </h2>
          {(
            [
              ["password_login_enabled", "启用密码登录"],
              ["registration_enabled", "允许公开注册"],
              [
                "registration_email_verification_required",
                "注册时要求邮箱验证",
              ],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={value[key]}
                onChange={(event) =>
                  setDraft({ ...value, [key]: event.target.checked })
                }
                disabled={disabled}
                className="size-4 accent-[color:var(--primary)]"
              />
              <span>{label}</span>
            </label>
          ))}
          <div className="space-y-2">
            <Label htmlFor="allowed-domains">允许的邮箱域名</Label>
            <Input
              id="allowed-domains"
              value={domains ?? value.allowed_email_domains.join(", ")}
              onChange={(event) => setDomains(event.target.value)}
              placeholder="example.com, company.com"
              disabled={disabled}
            />
            <p className="text-sm text-[color:var(--muted-foreground)]">
              留空表示不限制；多个域名用逗号分隔。
            </p>
          </div>
          {query.error && <p role="alert">无法加载设置，请稍后重试。</p>}
          {message && <p role="status">{message}</p>}
          <Button type="submit" disabled={disabled}>
            {save.isPending ? "正在保存…" : "保存设置"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
