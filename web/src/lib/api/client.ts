import { queryClient } from "@/lib/query/client";
import { clearAuthState } from "@/stores/auth-store";
import { clearActiveThemeState } from "@/stores/theme-store";
import type { ApiEnvelope, ApiFieldErrors } from "@/types/api";

export interface ApiClientOptions {
  redirectToLogin?: (reason: string) => void;
}

export interface ApiRequestInit extends Omit<RequestInit, "body"> {
  body?: BodyInit | object | null;
  requiresAuth?: boolean;
}

let inFlightRefresh: Promise<void> | null = null;

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: number,
    public readonly details?: unknown,
    public readonly fieldErrors?: ApiFieldErrors,
  ) {
    super(message);
  }
}

type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };
type QueryValue = JsonValue | undefined;

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isApiEnvelope<T>(value: unknown): value is ApiEnvelope<T> {
  return (
    isPlainObject(value) &&
    typeof value.code === "number" &&
    typeof value.msg === "string" &&
    Object.prototype.hasOwnProperty.call(value, "data")
  );
}

function extractFieldErrors(value: unknown): ApiFieldErrors | undefined {
  if (!isPlainObject(value)) {
    return undefined;
  }

  const entries = Object.entries(value);
  if (entries.length === 0 || entries.some(([, fieldValue]) => typeof fieldValue !== "string")) {
    return undefined;
  }

  return Object.fromEntries(entries) as ApiFieldErrors;
}

function defaultErrorMessage(status: number): string {
  if (status === 401) {
    return "登录状态已失效，请重新登录";
  }
  if (status >= 500) {
    return "服务暂时不可用，请稍后重试";
  }
  return "请求失败，请稍后重试";
}

function hasBody(body: ApiRequestInit["body"]): body is NonNullable<ApiRequestInit["body"]> {
  return body !== undefined && body !== null;
}

function isNativeBody(body: NonNullable<ApiRequestInit["body"]>): body is BodyInit {
  return typeof body === "string" || body instanceof FormData || body instanceof URLSearchParams || body instanceof Blob || body instanceof ArrayBuffer;
}

function normalizeHeaders(headers?: HeadersInit): Headers {
  return new Headers(headers);
}

async function parseEnvelope(response: Response): Promise<ApiEnvelope<unknown> | null> {
  if (response.status === 204) {
    return null;
  }

  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.includes("application/json")) {
    return null;
  }

  const payload: unknown = await response.json();
  return isApiEnvelope(payload) ? payload : null;
}

export function withQuery(input: string, params?: Record<string, QueryValue>): string {
  if (!params) {
    return input;
  }

  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        search.append(key, String(item));
      }
      continue;
    }
    search.set(key, String(value));
  }

  const query = search.toString();
  return query ? `${input}?${query}` : input;
}

export class ApiClient {
  constructor(private readonly options: ApiClientOptions = {}) {}

  async request<T>(input: string, init: ApiRequestInit = {}): Promise<T> {
    return this.performRequest<T>(input, init, init.requiresAuth ?? true, false);
  }

  private async performRequest<T>(
    input: string,
    init: ApiRequestInit,
    requiresAuth: boolean,
    didRefresh: boolean,
  ): Promise<T> {
    let response: Response;
    try {
      response = await fetch(input, this.buildInit(init));
    } catch {
      throw new ApiError("网络请求失败，请检查连接后重试", 0, 0);
    }

    if (response.status === 401 && requiresAuth && !didRefresh) {
      return this.retryAfterRefresh<T>(input, init);
    }

    const envelope = await parseEnvelope(response);
    if (!response.ok) {
      if (response.status === 401 && requiresAuth && didRefresh) {
        this.handleSessionExpired();
        throw this.toApiError(response.status, envelope, "登录状态已失效，请重新登录");
      }

      throw this.toApiError(response.status, envelope);
    }

    return (envelope?.data ?? undefined) as T;
  }

  private buildInit(init: ApiRequestInit): RequestInit {
    const headers = normalizeHeaders(init.headers);
    let body: BodyInit | undefined;

    if (hasBody(init.body)) {
      if (isNativeBody(init.body)) {
        body = init.body;
      } else {
        headers.set("content-type", "application/json");
        body = JSON.stringify(init.body);
      }
    } else if (!headers.has("content-type")) {
      headers.set("content-type", "application/json");
    }

    return {
      ...init,
      body,
      credentials: "include",
      headers,
    };
  }

  private async retryAfterRefresh<T>(input: string, init: ApiRequestInit): Promise<T> {
    try {
      await this.refreshSession();
    } catch {
      this.handleSessionExpired();
      throw new ApiError("登录状态已失效，请重新登录", 401, 401);
    }

    return this.performRequest<T>(input, init, true, true);
  }

  private async refreshSession(): Promise<void> {
    if (!inFlightRefresh) {
      inFlightRefresh = (async () => {
        try {
          const refreshResponse = await fetch("/api/v1/auth/refresh", {
            method: "POST",
            credentials: "include",
          });

          if (!refreshResponse.ok) {
            const refreshEnvelope = await parseEnvelope(refreshResponse);
            throw this.toApiError(401, refreshEnvelope, "登录状态已失效，请重新登录");
          }
        } finally {
          inFlightRefresh = null;
        }
      })();
    }

    await inFlightRefresh;
  }

  private handleSessionExpired() {
    clearAuthState();
    clearActiveThemeState();
    queryClient.clear();
    this.options.redirectToLogin?.("session_expired");
  }

  private toApiError(status: number, envelope: ApiEnvelope<unknown> | null, fallback?: string): ApiError {
    const message = fallback || envelope?.msg || defaultErrorMessage(status);
    const code = envelope?.code ?? status;
    const details = envelope?.data;
    return new ApiError(message, status, code, details, extractFieldErrors(details));
  }
}

export const apiClient = new ApiClient({
  redirectToLogin: () => {
    if (typeof window === "undefined") {
      return;
    }

    const next = `${window.location.pathname}${window.location.search}`;
    window.location.assign(`/login?next=${encodeURIComponent(next)}`);
  },
});
