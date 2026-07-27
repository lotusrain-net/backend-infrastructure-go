"use client";

import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Activity, KeyRound, ShieldCheck } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { appNavigation } from "@/config/navigation";
import { filterNavigation } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";

interface PlatformHealth {
  status: "ok";
}

async function getPlatformHealth(): Promise<PlatformHealth> {
  const response = await fetch("/api/healthz", { cache: "no-store" });
  if (!response.ok) {
    throw new Error("平台健康检查不可用");
  }
  return response.json() as Promise<PlatformHealth>;
}

export function Dashboard() {
  const user = useAuthStore((state) => state.user);
  const navigation = filterNavigation(appNavigation, { permissions: user?.permissions ?? [] });
  const health = useQuery({
    queryKey: ["platform", "health"],
    queryFn: getPlatformHealth,
    refetchInterval: 30_000,
    retry: false,
  });

  const visibleModules = navigation.flatMap((item) => item.children ?? [item]).length;

  return (
    <div className="space-y-6">
      <PageHeader title="概览" description="查看平台会话、运行状态与当前可访问的管理模块。" />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Metric title="服务状态" icon={<Activity aria-hidden="true" className="h-4 w-4" />} value={health.isPending ? "检查中" : health.isError ? "不可用" : "正常"}>
          <Badge variant={health.isError ? "danger" : health.isPending ? "neutral" : "success"}>{health.isError ? "需检查" : health.isPending ? "正在检查" : "已就绪"}</Badge>
        </Metric>
        <Metric title="当前权限" icon={<KeyRound aria-hidden="true" className="h-4 w-4" />} value={`${user?.permissions.length ?? 0} 项`}>
          <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">按后端授权实时控制导航与页面访问。</p>
        </Metric>
        <Metric title="可用模块" icon={<ShieldCheck aria-hidden="true" className="h-4 w-4" />} value={`${visibleModules} 个`}>
          <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">当前会话可进入的控制台功能。</p>
        </Metric>
      </div>
    </div>
  );
}

function Metric({ title, icon, value, children }: { title: string; icon: ReactNode; value: string; children: ReactNode }) {
  return (
    <Card className="space-y-4">
      <div className="flex items-center gap-2 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{icon}{title}</div>
      <p className="text-[length:var(--text-display)] font-semibold text-[color:var(--foreground)]">{value}</p>
      {children}
    </Card>
  );
}
