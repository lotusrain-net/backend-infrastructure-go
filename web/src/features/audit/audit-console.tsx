"use client";

import { useState } from "react";
import { PageHeader } from "@/components/layout/page-header";
import { PermissionGate } from "@/components/providers/permission-gate";
import { DataTable } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { Badge } from "@/components/ui/badge";
import { Select } from "@/components/ui/select";
import { useAuditLogsQuery } from "@/features/audit/api";
import type { AuditEvent } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };

export function AuditConsole() {
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [requestID, setRequestID] = useState("");
  const [result, setResult] = useState<"" | "success" | "failure">("");
  const query = useAuditLogsQuery({ page, size, request_id: requestID, result: result || undefined });
  const data = query.data;

  return (
    <PermissionGate permission="audit:read">
      <div className="space-y-6">
        <PageHeader title="审计日志" description="按请求标识与结果查询平台操作记录。" />
        <FilterBar
          keyword={requestID}
          keywordLabel="请求 ID"
          keywordPlaceholder="输入请求 ID"
          pageSize={size}
          onKeywordChange={(value) => { setRequestID(value); setPage(1); }}
          onPageSizeChange={(value) => { setSize(value); setPage(1); }}
          onReset={() => { setRequestID(""); setResult(""); setPage(1); }}
        >
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-[color:var(--fg-muted)]">结果</span>
            <Select value={result} onChange={(event) => { setResult(event.target.value as "" | "success" | "failure"); setPage(1); }}>
              <option value="">全部</option>
              <option value="success">成功</option>
              <option value="failure">失败</option>
            </Select>
          </label>
        </FilterBar>
        <DataTable<AuditEvent>
          columns={[
            { key: "created", label: "时间", render: (event) => formatDate(event.created_at) },
            { key: "action", label: "操作", render: (event) => <code>{event.action}</code> },
            { key: "resource", label: "资源", render: (event) => event.resource_id ? `${event.resource_type} / ${event.resource_id}` : event.resource_type },
            { key: "result", label: "结果", render: (event) => <Badge variant={event.result === "success" ? "success" : "danger"}>{event.result === "success" ? "成功" : "失败"}</Badge> },
            { key: "request", label: "请求 ID", render: (event) => <code className="text-xs">{event.request_id}</code> },
          ]}
          rows={data?.items ?? []}
          rowKey={(event) => event.id}
          page={data?.meta ?? emptyPage}
          isLoading={query.isPending}
          error={query.isError ? (query.error instanceof Error ? query.error.message : "加载审计日志失败。") : null}
          emptyTitle="暂无审计日志"
          emptyDescription="当前筛选条件没有匹配的记录。"
          onPageChange={setPage}
        />
      </div>
    </PermissionGate>
  );
}

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}
