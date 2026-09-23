import { describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError, zhCNApiMessages } from "@purplevoid/backend-infrastructure-web/api";
import { formatApiError } from "../error-messages";

describe("Chinese API errors", () => {
  it.each([
    [400, "bad request", "请求参数有误，请检查后重试。"],
    [401, "invalid credentials", "账号、密码或验证码不正确，请检查后重试。"],
    [403, "operation not permitted", "当前操作不被允许，请检查账号权限或操作条件。"],
    [403, "email not verified", "请先验证邮箱，再启用邮箱二次验证。"],
    [404, "resource not found", "请求的内容不存在或已被删除。"],
    [409, "unknown conflict", "信息已发生变化或存在冲突，请刷新后重试。"],
    [422, "validation failed", "填写的信息有误，请检查后重试。"],
    [429, "too many requests", "操作过于频繁，请稍后重试。"],
    [503, "authentication service unavailable", "认证服务暂时不可用，请稍后重试。"],
    [500, "database secret diagnostic", "服务暂时不可用，请稍后重试。"],
  ])("localizes HTTP %s without exposing server diagnostics", (status, msg, expected) => {
    expect(formatApiError(new ApiError(msg, status, status)).message).toBe(expected);
  });
  it("keeps raw details for protocol decisions while translating field errors and Retry-After", async () => {
    const fetcher = vi.fn().mockImplementation(async () => new Response(JSON.stringify({code:422,msg:"validation failed",data:{email_code:"required"}}),{status:422,headers:{"content-type":"application/json"}}));
    const client = new ApiClient({fetch:fetcher, messages:zhCNApiMessages, formatError:formatApiError});
    await expect(client.request("/verify")).rejects.toMatchObject({details:{email_code:"required"},fieldErrors:{email_code:"此项为必填项。"}});
    fetcher.mockImplementation(async () => new Response(JSON.stringify({code:429,msg:"too many requests",data:null}),{status:429,headers:{"content-type":"application/json","Retry-After":"45"}}));
    await expect(client.request("/code")).rejects.toMatchObject({message:"操作过于频繁，请稍后重试。",retryAfterSeconds:45});
  });
});
