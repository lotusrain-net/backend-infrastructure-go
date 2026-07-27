"use client";

import type { ReactNode } from "react";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AppearanceControls } from "@/features/preferences/appearance-controls";
import { useAuthStore } from "@/stores/auth-store";

export function Profile() {
  const user = useAuthStore((state) => state.user);

  return (
    <div className="space-y-6">
      <PageHeader title="个人资料" description="查看当前浏览器会话对应的账号与授权范围。" />
      <Tabs defaultValue="profile" className="space-y-5">
        <TabsList aria-label="个人资料设置">
          <TabsTrigger value="profile">资料</TabsTrigger>
          <TabsTrigger value="appearance">外观</TabsTrigger>
        </TabsList>

        <TabsContent value="profile" className="space-y-6">
          <Card className="max-w-3xl">
            <dl className="grid gap-5 sm:grid-cols-2">
              <Detail label="显示名" value={user?.display_name || user?.username || "-"} />
              <Detail label="邮箱" value={user?.email || "-"} />
              <Detail label="账号状态" value={<Badge variant={user?.is_active ? "success" : "danger"}>{user?.is_active ? "启用" : "停用"}</Badge>} />
              <Detail label="用户 ID" value={<code className="break-all text-[length:var(--text-caption)]">{user?.id || "-"}</code>} />
            </dl>
          </Card>
          <div className="space-y-3">
            <h2 className="text-[length:var(--text-heading-sm)] font-semibold text-[color:var(--foreground)]">当前权限</h2>
            <div className="flex flex-wrap gap-2">
              {user?.permissions.length ? user.permissions.map((permission) => <Badge key={permission} variant="info"><code>{permission}</code></Badge>) : <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">当前账号没有已返回的权限码。</p>}
            </div>
          </div>
        </TabsContent>

        <TabsContent value="appearance" className="max-w-3xl">
          <Card>
            <AppearanceControls />
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: ReactNode }) {
  return <div className="space-y-1"><dt className="text-[length:var(--text-label)] text-[color:var(--muted-foreground)]">{label}</dt><dd className="text-[length:var(--text-body-sm)] text-[color:var(--foreground)]">{value}</dd></div>;
}
