import { describe, expect, it, vi } from "vitest";
import {
  createRole,
  createUser,
  deleteRole,
  getRole,
  replaceRolePermissions,
  replaceUserRoles,
  resetUserPassword,
  setUserActive,
  updateRole,
  updateUser,
} from "@/features/iam/api";

function apiResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify({ code: status, msg: "success", data }), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("IAM management API", () => {
  it("uses the documented mutation endpoints and replacement bodies", async () => {
    const user = {
      id: "u1",
      email: "ops@example.com",
      username: "ops",
      display_name: "Operations",
      is_active: true,
      roles: [],
    };
    const role = { id: "r1", name: "operator", description: "Operations", is_system: false };
    const roleDetail = { ...role, permissions: [] };
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(apiResponse(user, 201))
      .mockResolvedValueOnce(apiResponse(user))
      .mockResolvedValueOnce(apiResponse({ updated: true }))
      .mockResolvedValueOnce(apiResponse({ reset: true }))
      .mockResolvedValueOnce(apiResponse({ replaced: true }))
      .mockResolvedValueOnce(apiResponse(role, 201))
      .mockResolvedValueOnce(apiResponse(roleDetail))
      .mockResolvedValueOnce(apiResponse(role))
      .mockResolvedValueOnce(apiResponse({ replaced: true }))
      .mockResolvedValueOnce(apiResponse({ deleted: true }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(createUser({ email: user.email, username: user.username, password: "password-1234", display_name: user.display_name })).resolves.toEqual(user);
    await expect(updateUser(user.id, { email: user.email, username: user.username, display_name: "Ops" })).resolves.toEqual(user);
    await expect(setUserActive(user.id, false)).resolves.toEqual({ updated: true });
    await expect(resetUserPassword(user.id, "password-5678")).resolves.toEqual({ reset: true });
    await expect(replaceUserRoles(user.id, [role.id])).resolves.toEqual({ replaced: true });
    await expect(createRole({ name: role.name, description: role.description })).resolves.toEqual(role);
    await expect(getRole(role.id)).resolves.toEqual(roleDetail);
    await expect(updateRole(role.id, { name: role.name, description: "Updated" })).resolves.toEqual(role);
    await expect(replaceRolePermissions(role.id, ["p1"])).resolves.toEqual({ replaced: true });
    await expect(deleteRole(role.id)).resolves.toEqual({ deleted: true });

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/v1/users", expect.objectContaining({ method: "POST", body: JSON.stringify({ email: user.email, username: user.username, password: "password-1234", display_name: user.display_name }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/v1/users/u1", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ email: user.email, username: user.username, display_name: "Ops" }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(3, "/api/v1/users/u1/active", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ is_active: false }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(4, "/api/v1/users/u1/password/reset", expect.objectContaining({ method: "POST", body: JSON.stringify({ new_password: "password-5678" }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(5, "/api/v1/users/u1/roles", expect.objectContaining({ method: "PUT", body: JSON.stringify({ role_ids: ["r1"] }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(6, "/api/v1/roles", expect.objectContaining({ method: "POST", body: JSON.stringify({ name: role.name, description: role.description }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(7, "/api/v1/roles/r1", expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(8, "/api/v1/roles/r1", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ name: role.name, description: "Updated" }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(9, "/api/v1/roles/r1/permissions", expect.objectContaining({ method: "PUT", body: JSON.stringify({ permission_ids: ["p1"] }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(10, "/api/v1/roles/r1", expect.objectContaining({ method: "DELETE" }));
  });
});
