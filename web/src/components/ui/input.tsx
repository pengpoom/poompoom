import * as React from "react";

import { cn } from "@/lib/utils";

function Input({ className, id, name, type, ...props }: React.ComponentProps<"input">) {
  const generatedId = React.useId().replace(/:/g, "");
  const fieldId = id || name || generatedId;
  const fieldName = name || id || generatedId;

  return (
    <input
      id={fieldId}
      name={fieldName}
      type={type}
      data-slot="input"
      className={cn(
        "flex h-11 w-full min-w-0 rounded-[var(--app-radius-pill)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-4 py-2 text-sm text-[var(--app-text-primary)] shadow-none outline-none backdrop-blur-xl transition-[color,box-shadow] selection:bg-[var(--app-text-primary)] selection:text-[var(--app-bg-root)] file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-[var(--app-text-primary)] placeholder:text-[var(--app-text-muted)] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 focus-visible:border-[var(--app-border-strong)] focus-visible:ring-[3px] focus-visible:ring-[rgba(91,214,255,0.18)]",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
