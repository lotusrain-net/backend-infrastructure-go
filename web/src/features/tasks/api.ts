import { keepPreviousData, queryOptions, useMutation, useQuery } from "@tanstack/react-query";
import { apiClient, withQuery } from "@/lib/api/client";
import { queryClient } from "@/lib/query/client";
import type { ListTaskExecutionsParams, PaginatedResponse, TaskExecution, TaskSubmissionRequest, TaskTypeRef } from "@/types/api";

export const taskKeys = {
  all: () => ["tasks"] as const,
  lists: () => ["tasks", "executions"] as const,
  list: (
    params: Required<Pick<ListTaskExecutionsParams, "page" | "size">> & Omit<ListTaskExecutionsParams, "page" | "size">,
  ) => ["tasks", "executions", params] as const,
  detail: (executionID: string) => ["tasks", "execution", executionID] as const,
  types: () => ["tasks", "types"] as const,
};

function normalizeExecutionParams(params: ListTaskExecutionsParams = {}) {
  return {
    page: params.page ?? 1,
    size: params.size ?? 20,
    task_type: params.task_type,
    status: params.status,
  };
}

export async function listTaskExecutions(params: ListTaskExecutionsParams = {}): Promise<PaginatedResponse<TaskExecution>> {
  const normalized = normalizeExecutionParams(params);
  return apiClient.request<PaginatedResponse<TaskExecution>>(withQuery("/api/v1/task-executions", normalized));
}

export async function getTaskExecution(executionID: string): Promise<TaskExecution> {
  return apiClient.request<TaskExecution>(`/api/v1/task-executions/${executionID}`);
}

export async function listTaskTypes(): Promise<TaskTypeRef[]> {
  return apiClient.request<TaskTypeRef[]>("/api/v1/task-types");
}

export async function submitTask(input: TaskSubmissionRequest): Promise<TaskExecution> {
  const execution = await apiClient.request<TaskExecution>("/api/v1/task-executions", {
    method: "POST",
    body: input,
  });

  queryClient.setQueryData(taskKeys.detail(execution.id), execution);
  void queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
  return execution;
}

export const taskExecutionListQueryOptions = (params: ListTaskExecutionsParams = {}, polling = false) => {
  const normalized = normalizeExecutionParams(params);
  return queryOptions({
    queryKey: taskKeys.list(normalized),
    queryFn: () => listTaskExecutions(normalized),
    placeholderData: keepPreviousData,
    refetchInterval: polling ? 5_000 : false,
  });
};

export const taskExecutionDetailQueryOptions = (executionID: string) =>
  queryOptions({
    queryKey: taskKeys.detail(executionID),
    queryFn: () => getTaskExecution(executionID),
    enabled: executionID.length > 0,
  });

export const taskTypesQueryOptions = () =>
  queryOptions({
    queryKey: taskKeys.types(),
    queryFn: listTaskTypes,
  });

export function useTaskExecutionsQuery(params: ListTaskExecutionsParams = {}, polling = false) {
  return useQuery(taskExecutionListQueryOptions(params, polling));
}

export function useTaskExecutionQuery(executionID: string) {
  return useQuery(taskExecutionDetailQueryOptions(executionID));
}

export function useTaskTypesQuery() {
  return useQuery(taskTypesQueryOptions());
}

export function useSubmitTaskMutation() {
  return useMutation({
    mutationFn: submitTask,
  });
}
