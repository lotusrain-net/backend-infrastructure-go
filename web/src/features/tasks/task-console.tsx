"use client";

import { useState, type FormEvent } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Eye, Play, X } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { DataTable } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { type TableQueryCodec, useTableQueryState } from "@/components/patterns/use-table-query-state";
import { PermissionGate } from "@/components/providers/permission-gate";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { useSubmitTaskMutation, useTaskExecutionQuery, useTaskExecutionsQuery, useTaskTypesQuery } from "@/features/tasks/api";
import { hasPermission } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";
import type { ListTaskExecutionsParams, TaskExecution, TaskExecutionStatus } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };
const taskStatuses = new Set<TaskExecutionStatus>(["queued", "running", "succeeded", "failed", "cancelled"]);

interface TaskQueryState extends Required<Pick<ListTaskExecutionsParams, "page" | "size">> {
  task_type: string;
  status: "" | TaskExecutionStatus;
}

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export const taskQueryCodec: TableQueryCodec<TaskQueryState> = {
  defaultState: { page: 1, size: 20, task_type: "", status: "" },
  keys: ["page", "size", "task_type", "status"],
  resetPageOnChangeKeys: ["size", "task_type", "status"],
  parse(params) {
    const status = params.get("status") ?? "";
    return {
      page: positiveInteger(params.get("page"), 1),
      size: positiveInteger(params.get("size"), 20),
      task_type: params.get("task_type") ?? "",
      status: taskStatuses.has(status as TaskExecutionStatus) ? status as TaskExecutionStatus : "",
    };
  },
  serialize(state) {
    return {
      page: state.page === 1 ? undefined : String(state.page),
      size: state.size === 20 ? undefined : String(state.size),
      task_type: state.task_type || undefined,
      status: state.status || undefined,
    };
  },
};

export function TaskConsole() {
  const { state, setState, reset } = useTableQueryState(taskQueryCodec);
  const [submissionType, setSubmissionType] = useState("");
  const [payload, setPayload] = useState("{}");
  const [message, setMessage] = useState<string | null>(null);
  const [selectedExecutionID, setSelectedExecutionID] = useState("");
  const permissions = useAuthStore((store) => store.user?.permissions ?? []);
  const executions = useTaskExecutionsQuery({
    page: state.page,
    size: state.size,
    task_type: state.task_type || undefined,
    status: state.status || undefined,
  }, true);
  const taskTypes = useTaskTypesQuery();
  const detail = useTaskExecutionQuery(selectedExecutionID);
  const submit = useSubmitTaskMutation();
  const canSubmit = hasPermission({ permissions }, "tasks:write");
  const defaultSubmissionType = taskTypes.data?.[0]?.task_type ?? "";
  const activeSubmissionType = submissionType || defaultSubmissionType;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage(null);
    let decodedPayload: Record<string, unknown>;
    try {
      const parsed: unknown = JSON.parse(payload);
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new Error("payload");
      }
      decodedPayload = parsed as Record<string, unknown>;
    } catch {
      setMessage("任务负载必须是 JSON 对象。");
      return;
    }

    if (!activeSubmissionType) {
      setMessage("请选择任务类型。");
      return;
    }

    try {
      const execution = await submit.mutateAsync({ task_type: activeSubmissionType, payload: decodedPayload });
      setMessage(`任务 ${execution.id} 已提交。`);
      setState({ page: 1 });
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "任务提交失败，请稍后重试。");
    }
  }

  const data = executions.data;
  return (
    <PermissionGate permission="tasks:read">
      <div className="space-y-6">
        <PageHeader title="任务执行" description="查看注册任务的运行状态，并在具备写权限时提交新执行。" />
        {canSubmit ? (
          <form onSubmit={handleSubmit} className="grid gap-4 border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-5 lg:grid-cols-[minmax(13rem,0.8fr)_minmax(0,1.6fr)_auto] lg:items-end">
            <label className="grid gap-1.5">
              <span className="text-sm font-medium text-[color:var(--fg-default)]">任务类型</span>
              <Select value={activeSubmissionType} onChange={(event) => setSubmissionType(event.target.value)} disabled={taskTypes.isPending || taskTypes.isError}>
                {taskTypes.data?.map((type) => <option key={type.task_type} value={type.task_type}>{type.task_type}</option>)}
              </Select>
            </label>
            <label className="grid gap-1.5">
              <span className="text-sm font-medium text-[color:var(--fg-default)]">JSON 负载</span>
              <textarea value={payload} onChange={(event) => setPayload(event.target.value)} rows={3} className="w-full resize-y border border-[color:var(--border-strong)] bg-[color:var(--surface)] px-3 py-2 font-mono text-sm text-[color:var(--fg-default)] outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]" />
            </label>
            <Button type="submit" disabled={submit.isPending || taskTypes.isPending}>
              <Play aria-hidden="true" className="h-4 w-4" />
              {submit.isPending ? "正在提交" : "提交任务"}
            </Button>
            {message ? <p role="status" className="lg:col-span-3 text-sm text-[color:var(--fg-muted)]">{message}</p> : null}
          </form>
        ) : null}

        <FilterBar
          keyword={state.task_type}
          keywordLabel="任务类型"
          keywordPlaceholder="按任务类型筛选"
          pageSize={state.size}
          onKeywordChange={(task_type) => setState({ task_type })}
          onPageSizeChange={(size) => setState({ size })}
          onReset={reset}
        >
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-[color:var(--fg-muted)]">执行状态</span>
            <Select value={state.status} onChange={(event) => setState({ status: event.target.value as TaskQueryState["status"] })}>
              <option value="">全部</option>
              <option value="queued">已排队</option>
              <option value="running">执行中</option>
              <option value="succeeded">成功</option>
              <option value="failed">失败</option>
              <option value="cancelled">已取消</option>
            </Select>
          </label>
        </FilterBar>
        <DataTable<TaskExecution>
          columns={[
            { key: "task", label: "任务类型", render: (execution) => <code>{execution.task_type}</code> },
            { key: "status", label: "状态", render: (execution) => <Badge variant={statusVariant(execution.status)}>{statusLabel(execution.status)}</Badge> },
            { key: "attempt", label: "尝试次数", render: (execution) => String(execution.attempt) },
            { key: "processed", label: "处理行数", render: (execution) => String(execution.processed_rows) },
            { key: "started", label: "开始时间", render: (execution) => formatDate(execution.started_at) },
            { key: "failure", label: "错误摘要", render: (execution) => execution.error_summary || "-" },
            {
              key: "actions",
              label: "操作",
              render: (execution) => (
                <Button type="button" size="sm" variant="ghost" aria-label="查看任务执行详情" title="查看详情" onClick={() => setSelectedExecutionID(execution.id)}>
                  <Eye aria-hidden="true" className="h-4 w-4" />
                </Button>
              ),
            },
          ]}
          rows={data?.items ?? []}
          rowKey={(execution) => execution.id}
          page={data?.meta ?? emptyPage}
          isLoading={executions.isPending}
          error={executions.isError ? (executions.error instanceof Error ? executions.error.message : "加载任务执行失败。") : null}
          emptyTitle="暂无任务执行"
          emptyDescription="提交任务后，状态会在此处自动更新。"
          onPageChange={(page) => setState({ page })}
        />
      </div>
      <TaskDetailDialog execution={detail.data} isLoading={detail.isPending} error={detail.isError ? detail.error : null} open={selectedExecutionID !== ""} onOpenChange={(open) => { if (!open) setSelectedExecutionID(""); }} />
    </PermissionGate>
  );
}

function TaskDetailDialog({ execution, isLoading, error, open, onOpenChange }: { execution?: TaskExecution; isLoading: boolean; error: unknown; open: boolean; onOpenChange: (open: boolean) => void }) {
  const errorMessage = error instanceof Error ? error.message : "加载任务详情失败。";
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 max-h-[85vh] w-[min(48rem,92vw)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-5 shadow-xl focus:outline-none">
          <div className="flex items-start justify-between gap-4">
            <div>
              <Dialog.Title className="text-lg font-semibold text-[color:var(--fg-default)]">任务执行详情</Dialog.Title>
              <Dialog.Description className="mt-1 text-sm text-[color:var(--fg-muted)]">查看任务负载、重试状态和执行结果。</Dialog.Description>
            </div>
            <Button type="button" size="icon" variant="ghost" aria-label="关闭任务执行详情" onClick={() => onOpenChange(false)}>
              <X aria-hidden="true" className="h-4 w-4" />
            </Button>
          </div>
          {isLoading ? <p role="status" className="mt-5 text-sm text-[color:var(--fg-muted)]">正在加载详情...</p> : null}
          {error ? <p role="alert" className="mt-5 text-sm text-[color:var(--danger)]">{errorMessage}</p> : null}
          {execution ? (
            <div className="mt-5 space-y-5">
              <dl className="grid gap-3 text-sm sm:grid-cols-2">
                <TaskDetailItem label="执行 ID" value={execution.id} />
                <TaskDetailItem label="任务类型" value={execution.task_type} />
                <TaskDetailItem label="定义 ID" value={execution.definition_id ?? "-"} />
                <TaskDetailItem label="状态" value={statusLabel(execution.status)} />
                <TaskDetailItem label="尝试次数" value={String(execution.attempt)} />
                <TaskDetailItem label="处理行数" value={String(execution.processed_rows)} />
                <TaskDetailItem label="队列 ID" value={execution.queue_id ?? "-"} />
                <TaskDetailItem label="幂等键" value={execution.idempotency_key ?? "-"} />
                <TaskDetailItem label="开始时间" value={formatDate(execution.started_at)} />
                <TaskDetailItem label="结束时间" value={formatDate(execution.finished_at)} />
                <TaskDetailItem label="错误摘要" value={execution.error_summary ?? "-"} />
              </dl>
              <div>
                <h3 className="text-sm font-medium text-[color:var(--fg-default)]">JSON 负载</h3>
                <pre className="mt-2 overflow-x-auto border border-[color:var(--border-subtle)] bg-[color:var(--surface-subtle)] p-3 font-mono text-xs leading-5 text-[color:var(--fg-default)]">{formatPayload(execution.payload)}</pre>
              </div>
            </div>
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function TaskDetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs font-medium text-[color:var(--fg-muted)]">{label}</dt>
      <dd className="mt-1 break-words text-[color:var(--fg-default)]">{value}</dd>
    </div>
  );
}

function formatPayload(payload: Record<string, unknown>) {
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return "{}";
  }
}

function statusLabel(status: TaskExecutionStatus) {
  return { queued: "已排队", running: "执行中", succeeded: "成功", failed: "失败", cancelled: "已取消" }[status];
}

function statusVariant(status: TaskExecutionStatus): "info" | "success" | "warning" | "danger" | "neutral" {
  const variants: Record<TaskExecutionStatus, "info" | "success" | "warning" | "danger" | "neutral"> = {
    queued: "info",
    running: "warning",
    succeeded: "success",
    failed: "danger",
    cancelled: "neutral",
  };
  return variants[status];
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}
