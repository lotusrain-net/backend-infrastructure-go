"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { validEmail } from "./email-code";
import { requiresEmailVerification, VerificationDialog } from "./verification-dialog";
import { authPageHref } from "./navigation";
import { applyAuthFormError } from "./form-errors";
import type { RegisterRequest } from "@/types/api";
import { useRegisterMutation } from "./security-api";
type RegisterValues = RegisterRequest & { confirm_password: string };
export function RegisterForm({ nextPath }: { nextPath?: string }) {
  const router = useRouter(); const register = useRegisterMutation();
  const form = useForm<RegisterValues>({ defaultValues: { username: "", email: "", password: "", confirm_password: "" } });
  const [verification, setVerification] = useState<RegisterRequest | null>(null); const [pending, setPending] = useState(false);
  async function submitRegistration(input: RegisterRequest) { try { await register.mutateAsync(input); form.resetField("password"); form.resetField("confirm_password"); setVerification(null); router.replace(authPageHref("/login", nextPath, true)); } finally { register.reset(); } }
  const onSubmit = form.handleSubmit(async (values) => { if (pending || register.isPending || verification) return; form.clearErrors(); const input: RegisterRequest = { username: values.username.trim(), email: values.email.trim(), password: values.password }; setPending(true); try { await submitRegistration(input); } catch (error) { if (requiresEmailVerification(error)) setVerification(input); else applyAuthFormError(error, form, ["username", "email", "password"]); } finally { setPending(false); } });
  return <><Form {...form}><form className="space-y-5" onSubmit={(event) => void onSubmit(event)} noValidate>
    <h1 className="text-[length:var(--text-heading)] font-semibold">创建账号</h1>
    <FormField control={form.control} name="username" rules={{ required: "请输入用户名。", maxLength: { value: 100, message: "用户名不能超过 100 个字符。" } }} render={({ field }) => <FormItem><FormLabel>用户名</FormLabel><FormControl><Input {...field} autoComplete="username" maxLength={100} disabled={Boolean(verification) || register.isPending} /></FormControl><FormMessage /></FormItem>} />
    <FormField control={form.control} name="email" rules={{ required: "请输入邮箱。", validate: (value) => validEmail(value) || "请输入有效邮箱。" }} render={({ field }) => <FormItem><FormLabel>邮箱</FormLabel><FormControl><Input {...field} type="email" autoComplete="email" disabled={Boolean(verification) || register.isPending} /></FormControl><FormMessage /></FormItem>} />
    <FormField control={form.control} name="password" rules={{ required: "请输入密码。", minLength: { value: 12, message: "密码至少 12 位。" } }} render={({ field }) => <FormItem><FormLabel>密码</FormLabel><FormControl><Input {...field} type="password" autoComplete="new-password" minLength={12} maxLength={1024} disabled={Boolean(verification) || register.isPending} /></FormControl><FormMessage /></FormItem>} />
    <FormField control={form.control} name="confirm_password" rules={{ required: "请确认密码。", validate: (value) => value === form.getValues("password") || "两次输入的密码不一致。" }} render={({ field }) => <FormItem><FormLabel>确认密码</FormLabel><FormControl><Input {...field} type="password" autoComplete="new-password" disabled={Boolean(verification) || register.isPending} /></FormControl><FormMessage /></FormItem>} />
    {form.formState.errors.root?.message ? <Alert variant="destructive"><AlertDescription>{form.formState.errors.root.message}</AlertDescription></Alert> : null}
    <Button className="w-full" type="submit" disabled={register.isPending || Boolean(verification)}>{register.isPending ? "正在注册…" : "注册"}</Button>
    <Link href={authPageHref("/login", nextPath)} className="block text-center text-sm underline">返回登录</Link>
  </form></Form>{verification ? <VerificationDialog method="email" purpose="register" email={verification.email} onVerify={async ({ code }) => submitRegistration({ ...verification, email_code: code })} onCancel={() => { setVerification(null); form.clearErrors(); }} /> : null}</>;
}
