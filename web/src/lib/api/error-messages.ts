import { ApiError } from "@lotusrain-net/backend-infrastructure-web/api";

const responseMessages: Record<string, string> = {
  "invalid credentials": "账号、密码或验证码不正确，请检查后重试。",
  "invalid refresh token": "登录状态已失效，请重新登录。",
  "operation not permitted": "当前操作不被允许，请检查账号权限或操作条件。",
  "permission denied": "你没有执行此操作的权限，请联系管理员。",
  "user is inactive": "账号已停用，请联系管理员。",
  "email not verified": "请先验证邮箱，再启用邮箱二次验证。",
  "identity conflict": "邮箱或用户名已被使用，请更换后重试。",
  "security settings changed; retry": "账号信息或安全设置已发生变化，请刷新后重试。",
  "cannot deactivate the current user": "不能停用当前登录账号。",
  "cannot remove the last active administrator": "至少需要保留一名启用的管理员。",
  "system role is protected": "系统内置角色不支持此操作。",
  "authentication service unavailable": "认证服务暂时不可用，请稍后重试。",
};
const statusMessages: Record<number, string> = {
  400: "请求参数有误，请检查后重试。",
  401: "登录状态已失效，请重新登录。",
  403: "你没有执行此操作的权限，或尚未满足操作条件。",
  404: "请求的内容不存在或已被删除。",
  405: "当前操作暂不支持，请刷新页面后重试。",
  408: "请求超时，请稍后重试。",
  409: "信息已发生变化或存在冲突，请刷新后重试。",
  413: "提交的内容过大，请缩减后重试。",
  415: "提交的内容格式不受支持。",
  422: "填写的信息有误，请检查后重试。",
  429: "操作过于频繁，请稍后重试。",
};
const fieldMessages: Record<string, string> = {
  required: "此项为必填项。",
  "is required": "此项为必填项。",
  "must contain six digits": "请输入六位数字验证码。",
  "invalid or expired credential": "验证码不正确或已过期，请重新获取后再试。",
  "must be a valid email address": "请输入有效的邮箱地址。",
  "must contain 1 to 100 characters": "请输入 1 至 100 个字符。",
  "must contain 12 to 1024 characters": "密码长度应为 12 至 1024 个字符。",
  "invalid JSON body": "提交的数据格式有误，请刷新后重试。",
  "invalid user input": "填写的信息有误，请检查后重试。",
};
const isChinese = (message: string) => /[\u3400-\u9fff]/.test(message);

// Keep details unchanged for protocol checks; only display messages are localized.
export function formatApiError(error: ApiError): ApiError {
  const fields = error.fieldErrors && Object.fromEntries(
    Object.entries(error.fieldErrors).map(([field, message]) => [
      field,
      field === "proof" && message === "invalid or expired credential"
        ? "密码、一次性代码或恢复码不正确，请检查后重试。"
        : fieldMessages[message] ?? (isChinese(message) ? message : "此项填写有误，请检查后重试。"),
    ]),
  );
  const message = responseMessages[error.message]
    ?? (isChinese(error.message) ? error.message : undefined)
    ?? statusMessages[error.status]
    ?? (error.status >= 500 ? "服务暂时不可用，请稍后重试。" : "请求失败，请稍后重试。");
  return new ApiError(
    error.status === 422 && fields?.email_code ? fields.email_code : message,
    error.status, error.code, error.details, fields,
  );
}
