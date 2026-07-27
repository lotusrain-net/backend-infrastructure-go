"use client";

import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Download, Eye, X } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { DataTable } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { type TableQueryCodec, useTableQueryState } from "@/components/patterns/use-table-query-state";
import { PermissionGate } from "@/components/providers/permission-gate";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { useAuditLogsQuery } from "@/features/audit/api";
import type { AuditEvent, ListAuditLogsParams } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };
const auditResults = new Set(["success", "failure"]);

interface AuditQueryState extends Required<Pick<ListAuditLogsParams, "page" | "size">> {
  request_id: string;
  actor_id: string;
  action: string;
  result: "" | "success" | "failure";
  resource_type: string;
  resource_id: string;
  from: string;
  to: string;
}

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export const auditQueryCodec: TableQueryCodec<AuditQueryState> = {
  defaultState: {
    page: 1,
    size: 20,
    request_id: "",
    actor_id: "",
    action: "",
    result: "",
    resource_type: "",
    resource_id: "",
    from: "",
    to: "",
  },
  keys: ["page", "size", "request_id", "actor_id", "action", "result", "resource_type", "resource_id", "from", "to"],
  resetPageOnChangeKeys: ["size", "request_id", "actor_id", "action", "result", "resource_type", "resource_id", "from", "to"],
  parse(params) {
    const result = params.get("result") ?? "";
    return {
      page: positiveInteger(params.get("page"), 1),
      size: positiveInteger(params.get("size"), 20),
      request_id: params.get("request_id") ?? "",
      actor_id: params.get("actor_id") ?? "",
      action: params.get("action") ?? "",
      result: auditResults.has(result) ? result as AuditQueryState["result"] : "",
      resource_type: params.get("resource_type") ?? "",
      resource_id: params.get("resource_id") ?? "",
      from: params.get("from") ?? "",
      to: params.get("to") ?? "",
    };
  },
  serialize(state) {
    return {
      page: state.page === 1 ? undefined : String(state.page),
      size: state.size === 20 ? undefined : String(state.size),
      request_id: state.request_id || undefined,
      actor_id: state.actor_id || undefined,
      action: state.action || undefined,
      result: state.result || undefined,
      resource_type: state.resource_type || undefined,
      resource_id: state.resource_id || undefined,
      from: state.from || undefined,
      to: state.to || undefined,
    };
  },
};

export function AuditConsole() {
  const { state, setState, reset } = useTableQueryState(auditQueryCodec);
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  const query = useAuditLogsQuery({
    page: state.page,
    size: state.size,
    request_id: state.request_id || undefined,
    actor_id: state.actor_id || undefined,
    action: state.action || undefined,
    result: state.result || undefined,
    resource_type: state.resource_type || undefined,
    resource_id: state.resource_id || undefined,
    from: state.from || undefined,
    to: state.to || undefined,
  });
  const data = query.data;
  const events = data?.items ?? [];

  function exportCurrentPage() {
    const blob = new Blob(["\uFEFF", auditEventsToCSV(events)], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `audit-logs-${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.append(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  return (
    <PermissionGate permission="audit:read">
      <div className="space-y-6">
        <PageHeader
          title="审计日志"
          description="按主体、资源、操作和时间范围查询平台操作记录。"
          actions={
            <Button type="button" variant="secondary" onClick={exportCurrentPage} disabled={events.length === 0}>
              <Download aria-hidden="true" className="h-4 w-4" />
              导出当前页
            </Button>
          }
        />
        <FilterBar
          keyword={state.request_id}
          keywordLabel="请求 ID"
          keywordPlaceholder="输入请求 ID"
          pageSize={state.size}
          onKeywordChange={(request_id) => setState({ request_id })}
          onPageSizeChange={(size) => setState({ size })}
          onReset={reset}
        >
          <AuditFilter label="主体 ID">
            <Input value={state.actor_id} onChange={(event) => setState({ actor_id: event.target.value })} placeholder="用户 UUID" />
          </AuditFilter>
          <AuditFilter label="操作">
            <Input value={state.action} onChange={(event) => setState({ action: event.target.value })} placeholder="例如 task.submit" />
          </AuditFilter>
          <AuditFilter label="结果">
            <Select value={state.result} onChange={(event) => setState({ result: event.target.value as AuditQueryState["result"] })}>
              <option value="">全部</option>
              <option value="success">成功</option>
              <option value="failure">失败</option>
            </Select>
          </AuditFilter>
          <AuditFilter label="资源类型">
            <Input value={state.resource_type} onChange={(event) => setState({ resource_type: event.target.value })} placeholder="例如 task_execution" />
          </AuditFilter>
          <AuditFilter label="资源 ID">
            <Input value={state.resource_id} onChange={(event) => setState({ resource_id: event.target.value })} />
          </AuditFilter>
          <AuditFilter label="开始时间">
            <Input type="datetime-local" value={toDateTimeLocal(state.from)} onChange={(event) => setState({ from: fromDateTimeLocal(event.target.value) })} />
          </AuditFilter>
          <AuditFilter label="结束时间">
            <Input type="datetime-local" value={toDateTimeLocal(state.to)} onChange={(event) => setState({ to: fromDateTimeLocal(event.target.value) })} />
          </AuditFilter>
        </FilterBar>
        <DataTable<AuditEvent>
          columns={[
            { key: "created", label: "时间", render: (event) => formatDate(event.created_at) },
            { key: "action", label: "操作", render: (event) => <code>{event.action}</code> },
            { key: "actor", label: "主体", render: (event) => event.actor_id ?? "-" },
            { key: "resource", label: "资源", render: (event) => event.resource_id ? `${event.resource_type} / ${event.resource_id}` : event.resource_type },
            { key: "result", label: "结果", render: (event) => <Badge variant={event.result === "success" ? "success" : "danger"}>{event.result === "success" ? "成功" : "失败"}</Badge> },
            { key: "request", label: "请求 ID", render: (event) => <code className="text-xs">{event.request_id}</code> },
            {
              key: "actions",
              label: "操作",
              render: (event) => (
                <Button type="button" size="sm" variant="ghost" aria-label="查看审计事件详情" title="查看详情" onClick={() => setSelectedEvent(event)}>
                  <Eye aria-hidden="true" className="h-4 w-4" />
                </Button>
              ),
            },
          ]}
          rows={events}
          rowKey={(event) => event.id}
          page={data?.meta ?? emptyPage}
          isLoading={query.isPending}
          error={query.isError ? (query.error instanceof Error ? query.error.message : "加载审计日志失败。") : null}
          emptyTitle="暂无审计日志"
          emptyDescription="当前筛选条件没有匹配的记录。"
          onPageChange={(page) => setState({ page })}
        />
      </div>
      <AuditEventDialog event={selectedEvent} onOpenChange={(open) => { if (!open) setSelectedEvent(null); }} />
    </PermissionGate>
  );
}

function AuditFilter({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid min-w-[12rem] flex-1 gap-1.5">
      <span className="text-xs font-medium text-[color:var(--fg-muted)]">{label}</span>
      {children}
    </label>
  );
}

function AuditEventDialog({ event, onOpenChange }: { event: AuditEvent | null; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog.Root open={event !== null} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 max-h-[85vh] w-[min(48rem,92vw)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-5 shadow-xl focus:outline-none">
          <div className="flex items-start justify-between gap-4">
            <div>
              <Dialog.Title className="text-lg font-semibold text-[color:var(--fg-default)]">审计事件详情</Dialog.Title>
              <Dialog.Description className="mt-1 text-sm text-[color:var(--fg-muted)]">事件上下文和记录的元数据。</Dialog.Description>
            </div>
            <Button type="button" size="icon" variant="ghost" aria-label="关闭审计事件详情" onClick={() => onOpenChange(false)}>
              <X aria-hidden="true" className="h-4 w-4" />
            </Button>
          </div>
          {event ? (
            <div className="mt-5 space-y-5">
              <dl className="grid gap-3 text-sm sm:grid-cols-2">
                <DetailItem label="事件 ID" value={event.id} />
                <DetailItem label="请求 ID" value={event.request_id} />
                <DetailItem label="主体 ID" value={event.actor_id ?? "-"} />
                <DetailItem label="操作" value={event.action} />
                <DetailItem label="结果" value={event.result} />
                <DetailItem label="资源" value={event.resource_id ? `${event.resource_type} / ${event.resource_id}` : event.resource_type} />
                <DetailItem label="IP 地址" value={event.ip_address ?? "-"} />
                <DetailItem label="用户代理" value={event.user_agent ?? "-"} />
                <DetailItem label="发生时间" value={formatDate(event.created_at)} />
              </dl>
              <div>
                <h3 className="text-sm font-medium text-[color:var(--fg-default)]">元数据</h3>
                <pre className="mt-2 overflow-x-auto border border-[color:var(--border-subtle)] bg-[color:var(--surface-subtle)] p-3 font-mono text-xs leading-5 text-[color:var(--fg-default)]">{formatMetadata(event.metadata)}</pre>
              </div>
            </div>
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs font-medium text-[color:var(--fg-muted)]">{label}</dt>
      <dd className="mt-1 break-words text-[color:var(--fg-default)]">{value}</dd>
    </div>
  );
}

function toDateTimeLocal(value: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}

function fromDateTimeLocal(value: string) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function formatMetadata(metadata: Record<string, unknown>) {
  try {
    return JSON.stringify(metadata, null, 2);
  } catch {
    return "{}";
  }
}

function compactMetadata(metadata: Record<string, unknown>) {
  try {
    return JSON.stringify(metadata);
  } catch {
    return "{}";
  }
}

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function csvCell(value: unknown) {
  let text = value === null || value === undefined ? "" : String(value);
  if (/^[\t\r\n ]*[=+\-@]/.test(text)) {
    text = `'${text}`;
  }
  return /[",\r\n]/.test(text) ? `"${text.replaceAll("\"", "\"\"")}"` : text;
}

export function auditEventsToCSV(events: AuditEvent[]) {
  const header = ["事件 ID", "请求 ID", "主体 ID", "操作", "结果", "资源类型", "资源 ID", "IP 地址", "用户代理", "元数据", "发生时间"];
  const rows = events.map((event) => [
    event.id,
    event.request_id,
    event.actor_id,
    event.action,
    event.result,
    event.resource_type,
    event.resource_id,
    event.ip_address,
    event.user_agent,
    compactMetadata(event.metadata),
    event.created_at,
  ]);
  return [header, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n");
}
