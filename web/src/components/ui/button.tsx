import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-[var(--app-radius-pill)] text-sm font-medium outline-none transition-all disabled:pointer-events-none disabled:opacity-50 focus-visible:border-[var(--app-border-strong)] focus-visible:ring-[3px] focus-visible:ring-[rgba(43,223,222,0.22)] aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "border border-cyan-200/35 bg-[linear-gradient(135deg,rgba(255,255,255,0.18),rgba(255,255,255,0.05)),linear-gradient(135deg,rgba(63,105,255,0.84),rgba(31,220,255,0.68))] text-white shadow-[0_12px_28px_rgba(41,152,255,0.18)] hover:border-cyan-100/50 hover:bg-[linear-gradient(135deg,rgba(255,255,255,0.22),rgba(255,255,255,0.07)),linear-gradient(135deg,rgba(63,105,255,0.92),rgba(31,220,255,0.76))]",
        destructive:
          "bg-destructive text-white shadow-xs hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40",
        outline:
          "border border-[var(--app-border)] bg-white/[0.045] text-[var(--app-text-secondary)] shadow-none backdrop-blur-xl hover:bg-white/[0.085] hover:text-[var(--app-text-primary)]",
        secondary:
          "border border-transparent bg-white/[0.06] text-[var(--app-text-primary)] shadow-none backdrop-blur-xl hover:bg-white/[0.1]",
        ghost:
          "text-[var(--app-text-secondary)] hover:bg-white/[0.075] hover:text-[var(--app-text-primary)]",
        link: "text-[var(--app-text-primary)] underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2 has-[>svg]:px-3",
        sm: "h-8 gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-10 px-6 has-[>svg]:px-4",
        icon: "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot : "button";

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button };
