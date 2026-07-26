import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 border text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-45",
  {
    variants: {
      variant: {
        primary: "border-[color:var(--accent-primary)] bg-[color:var(--accent-primary)] text-[color:var(--accent-contrast)] hover:bg-[color:var(--accent-primary-hover)]",
        secondary: "border-[color:var(--border-strong)] bg-[color:var(--surface)] text-[color:var(--fg-default)] hover:bg-[color:var(--surface-hover)]",
        ghost: "border-transparent bg-transparent text-[color:var(--fg-default)] hover:bg-[color:var(--surface-hover)]",
        danger: "border-[color:var(--danger)] bg-[color:var(--danger)] text-white hover:opacity-90",
      },
      size: {
        sm: "h-8 px-2.5",
        default: "h-10 px-3.5",
        lg: "h-11 px-4",
        icon: "h-9 w-9 p-0",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export function Button({ asChild = false, className, variant, size, ...props }: ButtonProps) {
  const Component = asChild ? Slot : "button";

  return <Component className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
