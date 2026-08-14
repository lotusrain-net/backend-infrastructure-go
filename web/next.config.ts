import type { NextConfig } from "next";

const localAPIProxyTarget = "http://127.0.0.1:8080";

function resolveAPIProxyTarget(): string {
  const configuredTarget = process.env.API_PROXY_TARGET?.trim();
  if (configuredTarget) {
    return configuredTarget.replace(/\/+$/, "");
  }

  if (process.env.NODE_ENV === "production") {
    throw new Error("API_PROXY_TARGET is required for production builds");
  }

  return localAPIProxyTarget;
}

const apiProxyTarget = resolveAPIProxyTarget();

const nextConfig: NextConfig = {
  output: "standalone",
  transpilePackages: ["@purplevoid/backend-infrastructure-web"],
  turbopack: {
    root: process.cwd(),
  },
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiProxyTarget}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
