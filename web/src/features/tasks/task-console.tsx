"use client";

import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { Eye, Play } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import type { PageMeta } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { PaginationControls } from "@/components/patterns/pagination-controls";
import { type TableQueryCodec, useTableQueryState } from "@/components/patterns/use-table-query-state";
import { PermissionGate } from "@/components/providers/permission-gate";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/toaster";
import { useSubmitTaskMutation, useTaskExecutionQuery, useTaskExecutionsQuery, useTaskTypesQuery } from "@/features/tasks/api";
import { hasPermission } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";
import type { ListTaskExecutionsParams, TaskExecution, TaskExecutionStatus } from "@/types/api";

const emptyPage: PageMeta = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };
const allTaskStatusesValue = "__all_task_statuses__";
const taskStatuses = new Set<TaskExecutionStatus>(["queued", "running", "succeeded", "failed", "cancelled"]);

interface TaskQueryState extends Required<Pick<ListTaskExecutionsParams, "page" | "size">> {
  task_type: string;
  status: "" | TaskExecutionStatus;
}

interface TaskSubmissionValues {
  task_type: string;
  payload: string;
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
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [selectedExecutionID, setSelectedExecutionID] = useState("");
  const submissionForm = useForm<TaskSubmissionValues>({ defaultValues: { task_type: "", payload: "{}" } });
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
  const selectedSubmissionType = useWatch({ control: submissionForm.control, name: "task_type" });
  const activeSubmissionType = selectedSubmissionType || defaultSubmissionType;

  async function handleSubmit(values: TaskSubmissionValues) {
    submissionForm.clearErrors();
    setSubmitError(null);

    let decodedPayload: Record<string, unknown>;
    try {
      const parsed: unknown = JSON.parse(values.payload);
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new Error("payload");
      }
      decodedPayload = parsed as Record<string, unknown>;
    } catch {
      submissionForm.setError("payload", { type: "validate", message: "任务负载必须是 JSON 对象。" });
      return;
    }

    if (!activeSubmissionType) {
      submissionForm.setError("task_type", { type: "validate", message: "请选择任务类型。" });
      return;
    }

    try {
      const execution = await submit.mutateAsync({ task_type: activeSubmissionType, payload: decodedPayload });
      toast.success("任务已提交", { description: `任务 ${execution.id} 已提交。` });
      setState({ page: 1 });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "任务提交失败，请稍后重试。");
    }
  }

  const data = executions.data;
  return (
    <PermissionGate permission="tasks:read">
      <div className="space-y-6">
        <PageHeader title="任务执行" description="查看注册任务的运行状态，并在具备写权限时提交新执行。" />
        {canSubmit ? (
          <Card className="p-0 sm:p-0">
            <Form {...submissionForm}>
              <form onSubmit={submissionForm.handleSubmit(handleSubmit)} className="grid gap-4 p-5 sm:p-6 lg:grid-cols-[minmax(13rem,0.8fr)_minmax(0,1.6fr)_auto] lg:items-end">
                <FormField
                  control={submissionForm.control}
                  name="task_type"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>任务类型</FormLabel>
                      <Select value={activeSubmissionType} onValueChange={field.onChange} disabled={taskTypes.isPending || taskTypes.isError}>
                        <FormControl>
                          <SelectTrigger><SelectValue placeholder="选择任务类型" /></SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {taskTypes.data?.map((type) => <SelectItem key={type.task_type} value={type.task_type}>{type.task_type}</SelectItem>)}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={submissionForm.control}
                  name="payload"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>JSON 负载</FormLabel>
                      <FormControl><Textarea rows={3} {...field} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button type="submit" disabled={submit.isPending || taskTypes.isPending}>
                  <Play aria-hidden="true" className="size-4" />
                  {submit.isPending ? "正在提交" : "提交任务"}
                </Button>
                {submitError ? (
                  <Alert variant="destructive" className="lg:col-span-3">
                    <AlertTitle>无法提交任务</AlertTitle>
                    <AlertDescription>{submitError}</AlertDescription>
                  </Alert>
                ) : null}
              </form>
            </Form>
          </Card>
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
          <div className="grid gap-1.5">
            <Label htmlFor="task-status-filter" className="text-[length:var(--text-caption)] text-[color:var(--muted-foreground)]">执行状态</Label>
            <Select value={state.status || allTaskStatusesValue} onValueChange={(value) => setState({ status: value === allTaskStatusesValue ? "" : value as TaskQueryState["status"] })}>
              <SelectTrigger id="task-status-filter"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value={allTaskStatusesValue}>全部</SelectItem>
                <SelectItem value="queued">已排队</SelectItem>
                <SelectItem value="running">执行中</SelectItem>
                <SelectItem value="succeeded">成功</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
                <SelectItem value="cancelled">已取消</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </FilterBar>
        <TaskExecutionTable
          rows={data?.items ?? []}
          page={data?.meta ?? emptyPage}
          isLoading={executions.isPending}
          error={executions.isError ? (executions.error instanceof Error ? executions.error.message : "加载任务执行失败。") : null}
          onPageChange={(page) => setState({ page })}
          onSelect={(execution) => setSelectedExecutionID(execution.id)}
        />
      </div>
      <TaskDetailDialog execution={detail.data} isLoading={detail.isPending} error={detail.isError ? detail.error : null} open={selectedExecutionID !== ""} onOpenChange={(open) => { if (!open) setSelectedExecutionID(""); }} />
    </PermissionGate>
  );
}

function TaskExecutionTable({
  rows,
  page,
  isLoading,
  error,
  onPageChange,
  onSelect,
}: {
  rows: TaskExecution[];
  page: PageMeta;
  isLoading: boolean;
  error: string | null;
  onPageChange: (page: number) => void;
  onSelect: (execution: TaskExecution) => void;
}) {
  const columnCount = 7;

  return (
    <Card className="overflow-hidden p-0 sm:p-0">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[color:var(--border)] px-5 py-3">
        <p aria-live="polite" className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">共 {page.total} 条记录</p>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">第 {page.page} / {Math.max(page.pages, 1)} 页</p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead scope="col">任务类型</TableHead>
            <TableHead scope="col">状态</TableHead>
            <TableHead scope="col">尝试次数</TableHead>
            <TableHead scope="col">处理行数</TableHead>
            <TableHead scope="col">开始时间</TableHead>
            <TableHead scope="col">错误摘要</TableHead>
            <TableHead scope="col">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? <LoadingRows columnCount={columnCount} /> : null}
          {!isLoading && error ? (
            <TableRow>
              <TableCell colSpan={columnCount} className="p-5 sm:p-6">
                <Alert variant="destructive"><AlertTitle>加载任务执行失败</AlertTitle><AlertDescription>{error}</AlertDescription></Alert>
              </TableCell>
            </TableRow>
          ) : null}
          {!isLoading && !error && rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columnCount} className="p-10 text-center">
                <div role="status" className="mx-auto max-w-md space-y-1">
                  <p className="font-medium text-[color:var(--foreground)]">暂无任务执行</p>
                  <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">提交任务后，状态会在此处自动更新。</p>
                </div>
              </TableCell>
            </TableRow>
          ) : null}
          {!isLoading && !error ? rows.map((execution) => (
            <TableRow key={execution.id}>
              <TableCell><code className="font-mono">{execution.task_type}</code></TableCell>
              <TableCell><Badge variant={statusVariant(execution.status)}>{statusLabel(execution.status)}</Badge></TableCell>
              <TableCell>{execution.attempt}</TableCell>
              <TableCell>{execution.processed_rows}</TableCell>
              <TableCell>{formatDate(execution.started_at)}</TableCell>
              <TableCell>{execution.error_summary || "-"}</TableCell>
              <TableCell>
                <Button type="button" size="icon" variant="ghost" aria-label="查看任务执行详情" title="查看详情" onClick={() => onSelect(execution)}>
                  <Eye aria-hidden="true" className="size-4" />
                </Button>
              </TableCell>
            </TableRow>
          )) : null}
        </TableBody>
      </Table>
      <PaginationControls className="border-t border-[color:var(--border)] px-5 py-3" page={page} onPageChange={onPageChange} />
    </Card>
  );
}

function LoadingRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: 5 }, (_, rowIndex) => (
        <TableRow key={rowIndex} aria-busy="true">
          {Array.from({ length: columnCount }, (_, cellIndex) => <TableCell key={cellIndex}><Skeleton className="h-4 w-4/5" /></TableCell>)}
        </TableRow>
      ))}
    </>
  );
}

function TaskDetailDialog({ execution, isLoading, error, open, onOpenChange }: { execution?: TaskExecution; isLoading: boolean; error: unknown; open: boolean; onOpenChange: (open: boolean) => void }) {
  const errorMessage = error instanceof Error ? error.message : "加载任务详情失败。";
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(48rem,calc(100vw-2rem))] p-0">
        <DialogHeader>
          <DialogTitle>任务执行详情</DialogTitle>
          <DialogDescription>查看任务负载、重试状态和执行结果。</DialogDescription>
        </DialogHeader>
        <div className="overflow-y-auto px-5 pb-5 sm:px-6 sm:pb-6">
          {isLoading ? <DetailSkeleton /> : null}
          {error ? <Alert variant="destructive" className="mt-5"><AlertTitle>加载详情失败</AlertTitle><AlertDescription>{errorMessage}</AlertDescription></Alert> : null}
          {execution ? (
            <div className="mt-5 space-y-5">
              <dl className="grid gap-3 text-[length:var(--text-body-sm)] sm:grid-cols-2">
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
                <h3 className="text-[length:var(--text-label)] font-medium text-[color:var(--foreground)]">JSON 负载</h3>
                <pre className="mt-2 overflow-x-auto border border-[color:var(--border)] bg-[color:var(--muted)] p-3 font-mono text-[length:var(--text-caption)] leading-5 text-[color:var(--foreground)] [border-radius:var(--radius-md)]">{formatPayload(execution.payload)}</pre>
              </div>
            </div>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function DetailSkeleton() {
  return (
    <div role="status" className="mt-5 grid gap-3 sm:grid-cols-2">
      <span className="sr-only">正在加载详情...</span>
      {Array.from({ length: 8 }, (_, index) => <Skeleton key={index} className="h-12" />)}
    </div>
  );
}

function TaskDetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[length:var(--text-caption)] font-medium text-[color:var(--muted-foreground)]">{label}</dt>
      <dd className="mt-1 break-words text-[color:var(--foreground)]">{value}</dd>
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
