import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import { queryClient } from "@/lib/query/client";
import { authKeys, getCurrentUser } from "./api";
import type {
  BasicAuthSettings,
  RegisterRequest,
  SecurityMode,
  SecurityProof,
  SecuritySettings,
  TokenPair,
  TOTPEnrollment,
  User,
  EmailCodeRequest,
  EmailCodeResponse,
  VerifyLoginChallengeRequest,
} from "@/types/api";

export function requestEmailCode(input: EmailCodeRequest) {
  return apiClient.request<EmailCodeResponse>("/api/v1/auth/email-code", {
    method: "POST", body: input, requiresAuth: false,
  });
}
export function register(input: RegisterRequest) {
  return apiClient.request<User>("/api/v1/auth/register", {
    method: "POST", body: input, requiresAuth: false,
  });
}
export async function verifyLoginChallenge(input: VerifyLoginChallengeRequest) {
  await apiClient.request<TokenPair>("/api/v1/auth/login/totp/verify", {
    method: "POST", body: input, requiresAuth: false,
  });
  const user = await getCurrentUser();
  queryClient.setQueryData(authKeys.currentUser(), user);
  return user;
}

export function useEmailCodeMutation() {
  return useMutation({
    gcTime: 0,
    mutationFn: requestEmailCode,
  });
}
export function useRegisterMutation() {
  return useMutation({
    gcTime: 0,
    mutationFn: register,
  });
}
export function useVerifyLoginMutation() {
  return useMutation({
    gcTime: 0,
    mutationFn: verifyLoginChallenge,
  });
}
export function useBasicAuthQuery(enabled = true) {
  return useQuery({
    queryKey: ["system-settings", "basic-auth"],
    queryFn: () =>
      apiClient.request<BasicAuthSettings>(
        "/api/v1/system-settings/basic-auth",
      ),
    enabled,
  });
}
export function useSaveBasicAuthMutation() {
  const cache = useQueryClient();
  return useMutation({
    mutationFn: (input: BasicAuthSettings) =>
      apiClient.request<BasicAuthSettings>(
        "/api/v1/system-settings/basic-auth",
        { method: "PUT", body: input },
      ),
    onSuccess: (data) =>
      cache.setQueryData(["system-settings", "basic-auth"], data),
  });
}
export function useSecurityQuery() {
  return useQuery({
    queryKey: ["auth", "security"],
    queryFn: () =>
      apiClient.request<SecuritySettings>("/api/v1/users/me/security"),
  });
}
export function useSecurityMutations() {
  const cache = useQueryClient();
  const invalidate = () =>
    cache.invalidateQueries({ queryKey: ["auth", "security"] });
  const save = useMutation({
    mutationFn: (mode: SecurityMode) =>
      apiClient.request<SecuritySettings>("/api/v1/users/me/security", {
        method: "PUT",
        body: { mode },
      }),
    onSuccess: invalidate,
  });
  const enroll = useMutation({
    mutationFn: (proof: SecurityProof) =>
      apiClient.request<TOTPEnrollment>(
        "/api/v1/users/me/security/totp/enroll",
        { method: "POST", body: proof },
      ),
    gcTime: 0,
  });
  const confirm = useMutation({
    mutationFn: (code: string) =>
      apiClient.request<{ recovery_codes: string[] }>(
        "/api/v1/users/me/security/totp/confirm",
        { method: "POST", body: { code } },
      ),
    onSuccess: invalidate,
    gcTime: 0,
  });
  const disable = useMutation({
    gcTime: 0,
    mutationFn: (proof: SecurityProof) =>
      apiClient.request<{ disabled: boolean }>(
        "/api/v1/users/me/security/totp/disable",
        { method: "POST", body: proof },
      ),
    onSuccess: invalidate,
  });
  return { save, enroll, confirm, disable };
}
