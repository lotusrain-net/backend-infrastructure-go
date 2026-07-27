"use client";

import { useState, type ReactNode } from "react";
import { Download, Eye } from "lucide-react";
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
import { Input } from "@/components/ui/input";
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
import { useAuditLogsQuery } from "@/features/audit/api";
import type { AuditEvent, ListAuditLogsParams } from "@/types/api";

const emptyPage: PageMeta = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };
const allAuditResultsValue = "__all_audit_results__";
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
              <Download aria-hidden="true" className="size-4" />
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
          <AuditFilter id="audit-actor-filter" label="主体 ID">
            <Input id="audit-actor-filter" value={state.actor_id} onChange={(event) => setState({ actor_id: event.target.value })} placeholder="用户 UUID" />
          </AuditFilter>
          <AuditFilter id="audit-action-filter" label="操作">
            <Input id="audit-action-filter" value={state.action} onChange={(event) => setState({ action: event.target.value })} placeholder="例如 task.submit" />
          </AuditFilter>
          <AuditFilter id="audit-result-filter" label="结果">
            <Select value={state.result || allAuditResultsValue} onValueChange={(value) => setState({ result: value === allAuditResultsValue ? "" : value as AuditQueryState["result"] })}>
              <SelectTrigger id="audit-result-filter"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value={allAuditResultsValue}>全部</SelectItem>
                <SelectItem value="success">成功</SelectItem>
                <SelectItem value="failure">失败</SelectItem>
              </SelectContent>
            </Select>
          </AuditFilter>
          <AuditFilter id="audit-resource-type-filter" label="资源类型">
            <Input id="audit-resource-type-filter" value={state.resource_type} onChange={(event) => setState({ resource_type: event.target.value })} placeholder="例如 task_execution" />
          </AuditFilter>
          <AuditFilter id="audit-resource-id-filter" label="资源 ID">
            <Input id="audit-resource-id-filter" value={state.resource_id} onChange={(event) => setState({ resource_id: event.target.value })} />
          </AuditFilter>
          <AuditFilter id="audit-from-filter" label="开始时间">
            <Input id="audit-from-filter" type="datetime-local" value={toDateTimeLocal(state.from)} onChange={(event) => setState({ from: fromDateTimeLocal(event.target.value) })} />
          </AuditFilter>
          <AuditFilter id="audit-to-filter" label="结束时间">
            <Input id="audit-to-filter" type="datetime-local" value={toDateTimeLocal(state.to)} onChange={(event) => setState({ to: fromDateTimeLocal(event.target.value) })} />
          </AuditFilter>
        </FilterBar>
        <AuditEventsTable
          rows={events}
          page={data?.meta ?? emptyPage}
          isLoading={query.isPending}
          error={query.isError ? (query.error instanceof Error ? query.error.message : "加载审计日志失败。") : null}
          onPageChange={(page) => setState({ page })}
          onSelect={setSelectedEvent}
        />
      </div>
      <AuditEventDialog event={selectedEvent} onOpenChange={(open) => { if (!open) setSelectedEvent(null); }} />
    </PermissionGate>
  );
}

function AuditFilter({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="grid min-w-[12rem] flex-1 gap-1.5">
      <Label htmlFor={id} className="text-[length:var(--text-caption)] text-[color:var(--muted-foreground)]">{label}</Label>
      {children}
    </div>
  );
}

function AuditEventsTable({
  rows,
  page,
  isLoading,
  error,
  onPageChange,
  onSelect,
}: {
  rows: AuditEvent[];
  page: PageMeta;
  isLoading: boolean;
  error: string | null;
  onPageChange: (page: number) => void;
  onSelect: (event: AuditEvent) => void;
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
            <TableHead scope="col">时间</TableHead>
            <TableHead scope="col">操作</TableHead>
            <TableHead scope="col">主体</TableHead>
            <TableHead scope="col">资源</TableHead>
            <TableHead scope="col">结果</TableHead>
            <TableHead scope="col">请求 ID</TableHead>
            <TableHead scope="col">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? <LoadingRows columnCount={columnCount} /> : null}
          {!isLoading && error ? (
            <TableRow>
              <TableCell colSpan={columnCount} className="p-5 sm:p-6">
                <Alert variant="destructive"><AlertTitle>加载审计日志失败</AlertTitle><AlertDescription>{error}</AlertDescription></Alert>
              </TableCell>
            </TableRow>
          ) : null}
          {!isLoading && !error && rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columnCount} className="p-10 text-center">
                <div role="status" className="mx-auto max-w-md space-y-1">
                  <p className="font-medium text-[color:var(--foreground)]">暂无审计日志</p>
                  <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">当前筛选条件没有匹配的记录。</p>
                </div>
              </TableCell>
            </TableRow>
          ) : null}
          {!isLoading && !error ? rows.map((event) => (
            <TableRow key={event.id}>
              <TableCell>{formatDate(event.created_at)}</TableCell>
              <TableCell><code className="font-mono">{event.action}</code></TableCell>
              <TableCell>{event.actor_id ?? "-"}</TableCell>
              <TableCell>{event.resource_id ? `${event.resource_type} / ${event.resource_id}` : event.resource_type}</TableCell>
              <TableCell><Badge variant={event.result === "success" ? "success" : "danger"}>{event.result === "success" ? "成功" : "失败"}</Badge></TableCell>
              <TableCell><code className="font-mono text-[length:var(--text-caption)]">{event.request_id}</code></TableCell>
              <TableCell>
                <Button type="button" size="icon" variant="ghost" aria-label="查看审计事件详情" title="查看详情" onClick={() => onSelect(event)}>
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

function AuditEventDialog({ event, onOpenChange }: { event: AuditEvent | null; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={event !== null} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(48rem,calc(100vw-2rem))] p-0">
        <DialogHeader>
          <DialogTitle>审计事件详情</DialogTitle>
          <DialogDescription>事件上下文和记录的元数据。</DialogDescription>
        </DialogHeader>
        {event ? (
          <div className="overflow-y-auto px-5 pb-5 sm:px-6 sm:pb-6">
            <div className="mt-5 space-y-5">
              <dl className="grid gap-3 text-[length:var(--text-body-sm)] sm:grid-cols-2">
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
                <h3 className="text-[length:var(--text-label)] font-medium text-[color:var(--foreground)]">元数据</h3>
                <pre className="mt-2 overflow-x-auto border border-[color:var(--border)] bg-[color:var(--muted)] p-3 font-mono text-[length:var(--text-caption)] leading-5 text-[color:var(--foreground)] [border-radius:var(--radius-md)]">{formatMetadata(event.metadata)}</pre>
              </div>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[length:var(--text-caption)] font-medium text-[color:var(--muted-foreground)]">{label}</dt>
      <dd className="mt-1 break-words text-[color:var(--foreground)]">{value}</dd>
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
