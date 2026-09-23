import type { LucideIcon } from "lucide-react";
import {
  Activity,
  LayoutDashboard,
  KeyRound,
  ListChecks,
  Shield,
  UserCircle2,
  Users,
} from "lucide-react";

export interface NavigationItem {
  label: string;
  href: string;
  icon: LucideIcon;
  permission?: string;
  children?: NavigationItem[];
}

export const appNavigation: NavigationItem[] = [
  { label: "概览", href: "/dashboard", icon: LayoutDashboard },
  {
    label: "审计日志",
    href: "/audit",
    icon: Activity,
    permission: "audit:read",
  },
  {
    label: "任务执行",
    href: "/tasks",
    icon: ListChecks,
    permission: "tasks:read",
  },
  { label: "个人资料", href: "/profile", icon: UserCircle2 },
  {
    label: "系统设置",
    href: "/system-settings",
    icon: Shield,
    children: [
      {
        label: "用户",
        href: "/iam/users",
        icon: Users,
        permission: "users:read",
      },
      {
        label: "角色",
        href: "/iam/roles",
        icon: Shield,
        permission: "roles:read",
      },
      {
        label: "权限",
        href: "/iam/permissions",
        icon: KeyRound,
        permission: "roles:read",
      },
      {
        label: "基本身份验证",
        href: "/system-settings/basic-auth",
        icon: KeyRound,
        permission: "system-settings:read",
      },
    ],
  },
];
