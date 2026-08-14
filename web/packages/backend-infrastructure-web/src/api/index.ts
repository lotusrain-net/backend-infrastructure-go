export interface ApiEnvelope<T> {
  code: number;
  msg: string;
  data: T;
}

export type ApiFieldErrors = Record<string, string>;

export interface PaginationMeta {
  page: number;
  size: number;
  total: number;
  pages: number;
  has_next: boolean;
  has_prev: boolean;
}

export interface PaginatedResponse<T> {
  items: T[];
  meta: PaginationMeta;
}

export type JSONPrimitive = null | boolean | number | string;
export type JSONValue = JSONPrimitive | JSONValue[] | { [key: string]: JSONValue };
export type QueryValue = JSONValue | undefined;

export interface ApiMessages {
  networkError: string;
  requestFailed: string;
  serverUnavailable: string;
  sessionExpired: string;
}

export const enUSApiMessages: Readonly<ApiMessages> = {
  networkError: "Network request failed. Check your connection and try again.",
  requestFailed: "The request failed. Try again.",
  serverUnavailable: "The service is temporarily unavailable. Try again later.",
  sessionExpired: "Your session has expired. Sign in again.",
};

export const zhCNApiMessages: Readonly<ApiMessages> = {
  networkError: "网络请求失败，请检查连接后重试",
  requestFailed: "请求失败，请稍后重试",
  serverUnavailable: "服务暂时不可用，请稍后重试",
  sessionExpired: "登录状态已失效，请重新登录",
};

export interface ApiClientOptions {
  baseURL?: string;
  credentials?: RequestCredentials;
  fetch?: typeof globalThis.fetch;
  refresh?: () => Promise<void>;
  onSessionExpired?: (reason: "session_expired") => void | Promise<void>;
  messages?: Partial<ApiMessages>;
}

export interface ApiRequestInit extends Omit<RequestInit, "body"> {
  body?: BodyInit | object | null;
  requiresAuth?: boolean;
}

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: number,
    public readonly details?: unknown,
    public readonly fieldErrors?: ApiFieldErrors,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export class ApiClient {
  private readonly configuredFetch?: typeof globalThis.fetch;
  private readonly baseURL: string;
  private readonly credentials: RequestCredentials;
  private readonly refresh?: () => Promise<void>;
  private readonly onSessionExpired?: ApiClientOptions["onSessionExpired"];
  private readonly messages: ApiMessages;
  private inFlightRefresh: Promise<void> | null = null;

  constructor(options: ApiClientOptions = {}) {
    this.configuredFetch = options.fetch;
    if (!this.configuredFetch && typeof globalThis.fetch !== "function") {
      throw new Error("ApiClient requires a fetch implementation.");
    }

    this.baseURL = options.baseURL?.replace(/\/+$/, "") ?? "";
    this.credentials = options.credentials ?? "include";
    this.refresh = options.refresh;
    this.onSessionExpired = options.onSessionExpired;
    this.messages = { ...enUSApiMessages, ...options.messages };
  }

  async request<T>(input: string, init: ApiRequestInit = {}): Promise<T> {
    const requiresAuth = init.requiresAuth ?? true;
    return this.performRequest<T>(input, init, requiresAuth, false);
  }

  private async performRequest<T>(
    input: string,
    init: ApiRequestInit,
    requiresAuth: boolean,
    didRefresh: boolean,
  ): Promise<T> {
    let response: Response;
    try {
      response = await (this.configuredFetch ?? globalThis.fetch)(resolveURL(this.baseURL, input), this.buildInit(init));
    } catch {
      throw new ApiError(this.messages.networkError, 0, 0);
    }

    if (response.status === 401 && requiresAuth && !didRefresh && this.refresh) {
      return this.retryAfterRefresh<T>(input, init);
    }

    const envelope = await parseEnvelope(response);
    if (!response.ok) {
      if (response.status === 401 && requiresAuth && didRefresh) {
        await this.expireSession();
        throw this.toApiError(response.status, envelope, this.messages.sessionExpired);
      }

      throw this.toApiError(response.status, envelope);
    }

    return (envelope?.data ?? undefined) as T;
  }

  private buildInit(init: ApiRequestInit): RequestInit {
    const { requiresAuth: _requiresAuth, ...requestInit } = init;
    const headers = new Headers(init.headers);
    let body: BodyInit | undefined;

    if (init.body !== undefined && init.body !== null) {
      if (isNativeBody(init.body)) {
        body = init.body;
      } else {
        if (!headers.has("content-type")) {
          headers.set("content-type", "application/json");
        }
        body = JSON.stringify(init.body);
      }
    }

    if (!body && requestInit.method?.toUpperCase() === "POST" && !headers.has("content-type")) {
      headers.set("content-type", "application/json");
    }

    return {
      ...requestInit,
      body,
      credentials: init.credentials ?? this.credentials,
      headers,
    };
  }

  private async retryAfterRefresh<T>(input: string, init: ApiRequestInit): Promise<T> {
    try {
      await this.refreshSession();
    } catch {
      await this.expireSession();
      throw new ApiError(this.messages.sessionExpired, 401, 401);
    }

    return this.performRequest<T>(input, init, true, true);
  }

  private async refreshSession(): Promise<void> {
    if (!this.refresh) {
      return;
    }

    if (!this.inFlightRefresh) {
      this.inFlightRefresh = this.refresh().finally(() => {
        this.inFlightRefresh = null;
      });
    }

    await this.inFlightRefresh;
  }

  private async expireSession(): Promise<void> {
    await this.onSessionExpired?.("session_expired");
  }

  private toApiError(status: number, envelope: ApiEnvelope<unknown> | null, fallback?: string): ApiError {
    const message = fallback ?? envelope?.msg ?? defaultErrorMessage(status, this.messages);
    const code = envelope?.code ?? status;
    const details = envelope?.data;
    return new ApiError(message, status, code, details, extractFieldErrors(details));
  }
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
  if (!query) {
    return input;
  }
  return `${input}${input.includes("?") ? "&" : "?"}${query}`;
}

function resolveURL(baseURL: string, input: string): string {
  if (!baseURL || /^(?:[a-z]+:)?\/\//i.test(input)) {
    return input;
  }
  return `${baseURL}${input.startsWith("/") ? "" : "/"}${input}`;
}

function isNativeBody(body: BodyInit | object): body is BodyInit {
  return (
    typeof body === "string" ||
    (typeof FormData !== "undefined" && body instanceof FormData) ||
    (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) ||
    (typeof Blob !== "undefined" && body instanceof Blob) ||
    (typeof ArrayBuffer !== "undefined" && body instanceof ArrayBuffer) ||
    (typeof ArrayBuffer !== "undefined" && ArrayBuffer.isView(body)) ||
    (typeof ReadableStream !== "undefined" && body instanceof ReadableStream)
  );
}

async function parseEnvelope(response: Response): Promise<ApiEnvelope<unknown> | null> {
  if (response.status === 204) {
    return null;
  }

  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("application/json")) {
    return null;
  }

  try {
    const payload: unknown = await response.json();
    return isApiEnvelope(payload) ? payload : null;
  } catch {
    return null;
  }
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

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function defaultErrorMessage(status: number, messages: ApiMessages): string {
  if (status === 401) {
    return messages.sessionExpired;
  }
  if (status >= 500) {
    return messages.serverUnavailable;
  }
  return messages.requestFailed;
}
