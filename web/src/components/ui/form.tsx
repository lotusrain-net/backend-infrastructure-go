"use client";

import * as LabelPrimitive from "@radix-ui/react-label";
import { Slot } from "@radix-ui/react-slot";
import * as React from "react";
import { Controller, FormProvider, useFormContext, useFormState, type ControllerProps, type FieldPath, type FieldValues } from "react-hook-form";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

export const Form = FormProvider;

type FieldContextValue<TValues extends FieldValues = FieldValues, TName extends FieldPath<TValues> = FieldPath<TValues>> = { name: TName };
const FormFieldContext = React.createContext<FieldContextValue>({} as FieldContextValue);
const FormItemContext = React.createContext<{ id: string }>({ id: "" });

export function FormField<TValues extends FieldValues = FieldValues, TName extends FieldPath<TValues> = FieldPath<TValues>>(props: ControllerProps<TValues, TName>) {
  return <FormFieldContext.Provider value={{ name: props.name }}><Controller {...props} /></FormFieldContext.Provider>;
}

function useFormField() {
  const field = React.useContext(FormFieldContext);
  const item = React.useContext(FormItemContext);
  const form = useFormContext();
  const state = useFormState({ name: field.name });
  const fieldState = form.getFieldState(field.name, state);
  if (!field.name || !item.id) throw new Error("Form field components must be used inside FormField and FormItem.");
  return { ...fieldState, name: field.name, formItemId: `${item.id}-control`, formDescriptionId: `${item.id}-description`, formMessageId: `${item.id}-message` };
}

export function FormItem({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  const id = React.useId();
  return <FormItemContext.Provider value={{ id }}><div className={cn("grid gap-2", className)} {...props} /></FormItemContext.Provider>;
}

export function FormLabel({ className, ...props }: React.ComponentProps<typeof LabelPrimitive.Root>) {
  const { error, formItemId } = useFormField();
  return <Label htmlFor={formItemId} className={cn(error && "text-[color:var(--destructive)]", className)} {...props} />;
}

export function FormControl(props: React.ComponentProps<typeof Slot>) {
  const { error, formItemId, formDescriptionId, formMessageId } = useFormField();
  return <Slot id={formItemId} aria-invalid={Boolean(error) || undefined} aria-describedby={error ? `${formDescriptionId} ${formMessageId}` : formDescriptionId} {...props} />;
}

export function FormDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  const { formDescriptionId } = useFormField();
  return <p id={formDescriptionId} className={cn("text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />;
}

export function FormMessage({ className, children, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  const { error, formMessageId } = useFormField();
  const body = error?.message ? String(error.message) : children;
  if (!body) return null;
  return <p id={formMessageId} role="alert" className={cn("text-[length:var(--text-body-sm)] text-[color:var(--destructive)]", className)} {...props}>{body}</p>;
}

export { useFormField };
