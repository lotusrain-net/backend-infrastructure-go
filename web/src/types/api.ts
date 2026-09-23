export type {
  ApiEnvelope,
  ApiFieldErrors,
  PaginatedResponse,
  PaginationMeta,
} from "@lotusrain-net/backend-infrastructure-web/api";
export type {
  AccountPreferences,
  ColorMode,
  FontScale,
  RadiusScale,
  ThemeName,
} from "@lotusrain-net/backend-infrastructure-web/theme";

export interface LoginRequest {
  email_code?: string;
  email: string;
  password: string;
}

export interface TokenPair {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
}

export interface User {
  email_verified_at?: string | null;
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

export type TaskExecutionStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";

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

export interface TOTPChallenge {
  status: "totp_required";
  challenge_id: string;
  expires_in: number;
}
export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  email_code?: string;
}
export interface BasicAuthSettings {
  password_login_enabled: boolean;
  registration_enabled: boolean;
  registration_email_verification_required: boolean;
  allowed_email_domains: string[];
}
export type SecurityMode = "default" | "email" | "totp";
export interface SecuritySettings {
  recovery_codes_expires_at?: string | null;
  mode: SecurityMode;
  totp_enabled: boolean;
  recovery_codes_remaining: number;
}
export interface TOTPEnrollment {
  secret: string;
  otpauth_uri: string;
  expires_in: number;
}
export interface SecurityProof {
  password: string;
  code?: string;
  recovery_code?: string;
}
