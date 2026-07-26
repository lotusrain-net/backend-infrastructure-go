import { describe, expect, it } from "vitest";
import { appNavigation } from "@/config/navigation";
import { filterNavigation } from "@/lib/rbac";

describe("RBAC navigation", () => {
  it("hides restricted links when permissions are missing", () => {
    const visible = filterNavigation(appNavigation, { permissions: ["users:read"] });

    expect(visible.find((item) => item.href === "/tasks")).toBeUndefined();
    expect(visible.find((item) => item.href === "/audit")).toBeUndefined();

    const identity = visible.find((item) => item.label === "身份与访问");
    expect(identity?.children?.map((item) => item.href)).toEqual(["/iam/users"]);
  });

  it("shows all links for wildcard permissions", () => {
    const visible = filterNavigation(appNavigation, { permissions: ["*"] });
    const flat = visible.flatMap((item) => item.children ?? [item]).map((item) => item.href);

    expect(flat).toContain("/audit");
    expect(flat).toContain("/tasks");
    expect(flat).toContain("/iam/roles");
    expect(flat).toContain("/iam/permissions");
  });

  it("removes parent routes that have no own permission and no visible children", () => {
    const visible = filterNavigation(appNavigation, { permissions: [] });

    expect(visible.find((item) => item.label === "身份与访问")).toBeUndefined();
  });
});
