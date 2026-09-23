"use client";

import { Input } from "@/components/ui/input";

export function VerificationCodeInput({
  value,
  onChange,
  ...props
}: Omit<React.ComponentProps<typeof Input>, "value" | "onChange"> & {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Input
      {...props}
      value={value}
      onChange={(event) =>
        onChange(event.target.value.replace(/\D/g, "").slice(0, 6))
      }
      inputMode="numeric"
      autoComplete="one-time-code"
      maxLength={6}
    />
  );
}
