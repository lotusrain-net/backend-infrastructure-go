import type { FieldValues, Path, UseFormReturn } from "react-hook-form";
import { ApiError } from "@/lib/api/client";

export function applyAuthFormError<T extends FieldValues>(
  error: unknown,
  form: UseFormReturn<T>,
  fields: readonly Path<T>[],
  aliases: Record<string, Path<T>> = {},
) {
  const unknown: string[] = [];
  let mapped = false;
  if (error instanceof ApiError && error.status === 422 && error.fieldErrors) {
    for (const [name, message] of Object.entries(error.fieldErrors)) {
      const field = aliases[name] ?? fields.find((field) => field === name);
      if (field) {
        form.setError(field, { type: "server", message });
        mapped = true;
      } else unknown.push(message);
    }
  }
  if (unknown.length || !mapped) {
    form.setError("root", {
      message: unknown.join("；") || (error instanceof Error ? error.message : "请求失败，请稍后重试。"),
    });
  }
}
