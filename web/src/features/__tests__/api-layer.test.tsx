import { describe, expect, it, vi } from "vitest";
import { auditLogQueryOptions, listAuditLogs } from "@/features/audit/api";
import { authKeys, currentUserQueryOptions, login, logout } from "@/features/auth/api";
import { listPermissions, listRoles, usersQueryOptions } from "@/features/iam/api";
import { listTaskTypes, submitTask, taskExecutionListQueryOptions, taskKeys } from "@/features/tasks/api";
import { queryClient } from "@/lib/query/client";
import { getAuthState, setCurrentUser } from "@/stores/auth-store";
import type { AuditEvent, AuthenticatedUser, Permission, Role, TaskExecution, TaskTypeRef, User } from "@/types/api";

const currentUser: AuthenticatedUser = {
  id: "u1",
  email: "admin@example.com",
  username: "admin",
  display_name: "Administrator",
  is_active: true,
  permissions: ["users:read", "tasks:read", "tasks:write"],
};

const listUser: User = {
  id: currentUser.id,
  email: currentUser.email,
  username: currentUser.username,
  display_name: currentUser.display_name,
  is_active: currentUser.is_active,
};

describe("feature API layer", () => {
  it("enables five-second polling for active task execution views", () => {
    const options = taskExecutionListQueryOptions({ page: 1, size: 20 }, true);

    expect(options.refetchInterval).toBe(5_000);
  });

  it("logs in with cookies, loads the current user, and keeps it in the auth cache", async () => {
    queryClient.clear();

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 200,
            msg: "success",
            data: { access_token: "access", token_type: "Bearer", expires_in: 900 },
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 200, msg: "success", data: currentUser }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(login({ email: currentUser.email, password: "password" })).resolves.toEqual(currentUser);

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/v1/auth/login",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ email: currentUser.email, password: "password" }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/v1/users/me", expect.any(Object));
    expect(getAuthState().user).toEqual(currentUser);
    expect(queryClient.getQueryData(authKeys.currentUser())).toEqual(currentUser);
  });

  it("clears local authentication state and cache when logout fails", async () => {
    queryClient.clear();
    setCurrentUser(currentUser);
    queryClient.setQueryData(authKeys.currentUser(), currentUser);
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 500, msg: "logout unavailable", data: null }), {
          status: 500,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    await expect(logout()).rejects.toMatchObject({ status: 500 });

    expect(getAuthState().user).toBeNull();
    expect(queryClient.getQueryData(authKeys.currentUser())).toBeUndefined();
  });

  it("serializes user and task execution filters for query hooks", async () => {
    queryClient.clear();

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 200,
            msg: "success",
            data: {
              items: [listUser],
              meta: { page: 2, size: 5, total: 1, pages: 1, has_next: false, has_prev: true },
            },
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 200,
            msg: "success",
            data: {
              items: [
                {
                  id: "execution-1",
                  task_type: "report.generate",
                  payload: { dry_run: true },
                  status: "queued",
                  attempt: 0,
                  processed_rows: 0,
                },
              ],
              meta: { page: 3, size: 10, total: 1, pages: 1, has_next: false, has_prev: true },
            },
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const usersQuery = await queryClient.fetchQuery(usersQueryOptions({ page: 2, size: 5, query: "admin", is_active: true }));
    const tasksQuery = await queryClient.fetchQuery(
      taskExecutionListQueryOptions({ page: 3, size: 10, task_type: "report.generate", status: "queued" }),
    );

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/v1/users?page=2&size=5&query=admin&is_active=true",
      expect.any(Object),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/v1/task-executions?page=3&size=10&task_type=report.generate&status=queued",
      expect.any(Object),
    );
    expect(usersQuery.meta.page).toBe(2);
    expect(tasksQuery.items[0]?.task_type).toBe("report.generate");
  });

  it("loads auxiliary collections and accepts dynamic task submissions", async () => {
    queryClient.clear();

    const roles: Role[] = [{ id: "r1", name: "admin", description: "Administrator", is_system: true }];
    const permissions: Permission[] = [{ id: "p1", name: "users:read", description: "Read users" }];
    const auditLogs = {
      items: [
        {
          id: "a1",
          request_id: "req-1",
          action: "task.submit",
          result: "success",
          resource_type: "task_execution",
          metadata: {},
          created_at: "2026-07-26T00:00:00Z",
        } satisfies AuditEvent,
      ],
      meta: { page: 1, size: 20, total: 1, pages: 1, has_next: false, has_prev: false },
    };
    const taskTypes: TaskTypeRef[] = [{ task_type: "report.generate" }];
    const execution: TaskExecution = {
      id: "execution-1",
      task_type: "report.generate",
      payload: { dry_run: true },
      status: "queued",
      attempt: 0,
      processed_rows: 0,
    };

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: roles }), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: permissions }), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: auditLogs }), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: taskTypes }), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 202, msg: "success", data: execution }), { status: 202, headers: { "content-type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(listRoles()).resolves.toEqual(roles);
    await expect(listPermissions()).resolves.toEqual(permissions);
    await expect(queryClient.fetchQuery(auditLogQueryOptions({ page: 1, size: 20, action: "task.submit" }))).resolves.toEqual(auditLogs);
    await expect(listTaskTypes()).resolves.toEqual(taskTypes);
    await expect(
      submitTask({
        task_type: "report.generate",
        payload: { dry_run: true },
        max_retries: 1,
      }),
    ).resolves.toEqual(execution);

    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/v1/audit-logs?page=1&size=20&action=task.submit",
      expect.any(Object),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(4, "/api/v1/task-types", expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(
      5,
      "/api/v1/task-executions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ task_type: "report.generate", payload: { dry_run: true }, max_retries: 1 }),
      }),
    );
    expect(queryClient.getQueryData(taskKeys.detail(execution.id))).toEqual(execution);
  });

  it("loads the current user through the query hook", async () => {
    queryClient.clear();

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 200, msg: "success", data: currentUser }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    const result = await queryClient.fetchQuery(currentUserQueryOptions());

    expect(result).toEqual(currentUser);
    expect(getAuthState().user).toEqual(currentUser);
  });
});
