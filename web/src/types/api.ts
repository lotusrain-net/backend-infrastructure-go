export type ThemeName = "enterprise" | "cyberpunk";
export type ColorMode = "light" | "dark" | "system";
export type FontScale = "small" | "standard" | "large";
export type RadiusScale = "square" | "compact" | "rounded";

export interface AccountPreferences {
  theme: ThemeName;
  color_mode: ColorMode;
  accent_color: string | null;
  font_scale: FontScale;
  radius_scale: RadiusScale;
}

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

export interface LoginRequest {
  email: string;
  password: string;
}

export interface TokenPair {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
}

export interface User {
  id: string;
  email: string;
  username: string;
  display_name: string;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
  roles?: Role[];
}

export interface AuthenticatedUser extends User {
  permissions: string[];
}

export type UserProfile = AuthenticatedUser;

export interface Role {
  id: string;
  name: string;
  description: string;
  is_system: boolean;
}

export interface RoleDetail extends Role {
  permissions: Permission[];
}

export interface CreateUserRequest {
  email: string;
  username: string;
  password: string;
  display_name?: string;
}

export interface UpdateUserRequest {
  email: string;
  username: string;
  display_name: string;
}

export interface RoleRequest {
  name: string;
  description: string;
}

export interface Permission {
  id: string;
  name: string;
  description: string;
}

export interface AuditEvent {
  id: string;
  request_id: string;
  actor_id?: string;
  action: string;
  result: "success" | "failure";
  resource_type: string;
  resource_id?: string;
  ip_address?: string;
  user_agent?: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

export type TaskExecutionStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled";

export interface TaskExecution {
  id: string;
  definition_id?: string;
  task_type: string;
  queue_id?: string;
  idempotency_key?: string;
  payload: Record<string, unknown>;
  status: TaskExecutionStatus;
  attempt: number;
  processed_rows: number;
  error_summary?: string;
  started_at?: string;
  finished_at?: string;
}

export interface TaskTypeRef {
  task_type: string;
}

export interface TaskSubmissionRequest {
  definition_id?: string;
  task_type: string;
  payload: Record<string, unknown>;
  idempotency_key?: string;
  max_retries?: number;
  timeout_seconds?: number;
  unique_for_seconds?: number;
  process_after_seconds?: number;
}

export interface ListUsersParams {
  page?: number;
  size?: number;
  query?: string;
  is_active?: boolean;
}

export interface ListAuditLogsParams {
  page?: number;
  size?: number;
  request_id?: string;
  actor_id?: string;
  action?: string;
  result?: "success" | "failure";
  resource_type?: string;
  resource_id?: string;
  from?: string;
  to?: string;
}

export interface ListTaskExecutionsParams {
  page?: number;
  size?: number;
  task_type?: string;
  status?: TaskExecutionStatus;
}
