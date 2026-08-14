import type { Metadata } from "next";
import type { ReactNode } from "react";
import "@purplevoid/backend-infrastructure-web/styles.css";
import "./globals.css";
import { AppProviders } from "@/components/providers/app-providers";

export const metadata: Metadata = {
  title: "基础设施控制台",
  description: "可复用的后端基础设施平台控制台",
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body>
        <a className="skip-link" href="#main-content">跳到主要内容</a>
        <AppProviders>{children}</AppProviders>
      </body>
    </html>
  );
}
