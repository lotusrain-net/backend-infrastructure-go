import { keepPreviousData, queryOptions, useQuery } from "@tanstack/react-query";
import { apiClient, withQuery } from "@/lib/api/client";
import type { AuditEvent, ListAuditLogsParams, PaginatedResponse } from "@/types/api";

export const auditKeys = {
  list: (params: Required<Pick<ListAuditLogsParams, "page" | "size">> & Omit<ListAuditLogsParams, "page" | "size">) =>
    ["audit", "logs", params] as const,
};

function normalizeAuditParams(params: ListAuditLogsParams = {}) {
  return {
    page: params.page ?? 1,
    size: params.size ?? 20,
    request_id: params.request_id,
    actor_id: params.actor_id,
    action: params.action,
    result: params.result,
    resource_type: params.resource_type,
    resource_id: params.resource_id,
    from: params.from,
    to: params.to,
  };
}

export async function listAuditLogs(params: ListAuditLogsParams = {}): Promise<PaginatedResponse<AuditEvent>> {
  const normalized = normalizeAuditParams(params);
  return apiClient.request<PaginatedResponse<AuditEvent>>(withQuery("/api/v1/audit-logs", normalized));
}

export const auditLogQueryOptions = (params: ListAuditLogsParams = {}) => {
  const normalized = normalizeAuditParams(params);
  return queryOptions({
    queryKey: auditKeys.list(normalized),
    queryFn: () => listAuditLogs(normalized),
    placeholderData: keepPreviousData,
  });
};

export function useAuditLogsQuery(params: ListAuditLogsParams = {}) {
  return useQuery(auditLogQueryOptions(params));
}
