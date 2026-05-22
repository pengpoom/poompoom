import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-[var(--app-radius-pill)] border px-2.5 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default: "border-transparent bg-[var(--app-text-primary)]/10 text-[var(--app-text-primary)]",
        secondary: "border-transparent bg-[var(--app-bg-surface)] text-[var(--app-text-secondary)]",
        outline: "border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-[var(--app-text-primary)]",
        success:
          "border-emerald-400/24 bg-emerald-400/10 text-emerald-200",
        warning:
          "border-amber-400/24 bg-amber-400/10 text-amber-200",
        danger:
          "border-rose-400/24 bg-rose-400/10 text-rose-200",
        info: "border-sky-400/24 bg-sky-400/10 text-sky-200",
        violet:
          "border-violet-400/24 bg-violet-400/10 text-violet-200",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge };
