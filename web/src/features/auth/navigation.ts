export function resolveLoginDestination(nextPath?: string) {
  if (!nextPath) return "/dashboard";
  try {
    const decodedPath = decodeURIComponent(nextPath);
    if (decodedPath.startsWith("//") || /[\\\u0000-\u001F\u007F]/.test(decodedPath)) return "/dashboard";
    const origin = "http://local.invalid";
    const target = new URL(nextPath, origin);
    if (target.origin !== origin || !target.pathname.startsWith("/")) return "/dashboard";
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return "/dashboard";
  }
}

export function authPageHref(page: "/login" | "/register", nextPath?: string, registered = false) {
  const params = new URLSearchParams();
  if (registered) params.set("registered", "1");
  if (nextPath) params.set("next", resolveLoginDestination(nextPath));
  return `${page}${params.size ? `?${params}` : ""}`;
}
