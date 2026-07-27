import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap border text-[length:var(--text-label)] font-medium transition-[color,background-color,border-color,box-shadow,transform] [border-radius:var(--radius-md)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-45 [&_svg]:pointer-events-none [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: "border-[color:var(--primary)] bg-[color:var(--primary)] text-[color:var(--primary-foreground)] hover:bg-[color:var(--primary-hover)]",
        secondary: "border-[color:var(--secondary)] bg-[color:var(--secondary)] text-[color:var(--secondary-foreground)] hover:brightness-[0.97]",
        outline: "border-[color:var(--input)] bg-[color:var(--card)] text-[color:var(--foreground)] hover:bg-[color:var(--accent)] hover:text-[color:var(--accent-foreground)]",
        ghost: "border-transparent bg-transparent text-[color:var(--foreground)] hover:bg-[color:var(--accent)] hover:text-[color:var(--accent-foreground)]",
        destructive: "border-[color:var(--destructive)] bg-[color:var(--destructive)] text-[color:var(--destructive-foreground)] hover:brightness-95",
        link: "border-transparent bg-transparent text-[color:var(--primary)] underline-offset-4 hover:underline",
      },
      size: {
        sm: "h-8 px-2.5",
        default: "h-10 px-3.5",
        lg: "h-11 px-4",
        icon: "h-9 w-9 p-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ asChild = false, className, variant, size, ...props }, ref) => {
    const Component = asChild ? Slot : "button";

    return <Component ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
  },
);

Button.displayName = "Button";
