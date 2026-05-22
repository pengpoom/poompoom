import * as React from "react";
import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";

import { cn } from "@/lib/utils";

function Checkbox({
  className,
  ...props
}: React.ComponentProps<typeof CheckboxPrimitive.Root>) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        "peer size-4 shrink-0 rounded-[4px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-[var(--app-bg-root)] shadow-xs outline-none transition-shadow data-[state=checked]:bg-[var(--app-text-primary)] data-[state=checked]:text-[var(--app-bg-root)] disabled:cursor-not-allowed disabled:opacity-50 focus-visible:border-[var(--app-border-strong)] focus-visible:ring-[3px] focus-visible:ring-[rgba(43,223,222,0.22)] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="flex items-center justify-center text-current transition-none">
        <Check className="size-3.5" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
