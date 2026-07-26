"use client";

import { useState, type FormEvent } from "react";
import { Play } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { PermissionGate } from "@/components/providers/permission-gate";
import { DataTable } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { useSubmitTaskMutation, useTaskExecutionsQuery, useTaskTypesQuery } from "@/features/tasks/api";
import { hasPermission } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";
import type { TaskExecution, TaskExecutionStatus } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };

export function TaskConsole() {
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [taskType, setTaskType] = useState("");
  const [status, setStatus] = useState<"" | TaskExecutionStatus>("");
  const [submissionType, setSubmissionType] = useState("");
  const [payload, setPayload] = useState("{}");
  const [message, setMessage] = useState<string | null>(null);
  const permissions = useAuthStore((state) => state.user?.permissions ?? []);
  const executions = useTaskExecutionsQuery({ page, size, task_type: taskType || undefined, status: status || undefined }, true);
  const taskTypes = useTaskTypesQuery();
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
      setPage(1);
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
          keyword={taskType}
          keywordLabel="任务类型"
          keywordPlaceholder="按任务类型筛选"
          pageSize={size}
          onKeywordChange={(value) => { setTaskType(value); setPage(1); }}
          onPageSizeChange={(value) => { setSize(value); setPage(1); }}
          onReset={() => { setTaskType(""); setStatus(""); setPage(1); }}
        >
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-[color:var(--fg-muted)]">执行状态</span>
            <Select value={status} onChange={(event) => { setStatus(event.target.value as "" | TaskExecutionStatus); setPage(1); }}>
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
          ]}
          rows={data?.items ?? []}
          rowKey={(execution) => execution.id}
          page={data?.meta ?? emptyPage}
          isLoading={executions.isPending}
          error={executions.isError ? (executions.error instanceof Error ? executions.error.message : "加载任务执行失败。") : null}
          emptyTitle="暂无任务执行"
          emptyDescription="提交任务后，状态会在此处自动更新。"
          onPageChange={setPage}
        />
      </div>
    </PermissionGate>
  );
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
