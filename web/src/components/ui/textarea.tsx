import * as React from "react";

import { cn } from "@/lib/utils";

function Textarea({ className, id, name, ...props }: React.ComponentProps<"textarea">) {
  const generatedId = React.useId().replace(/:/g, "");
  const fieldId = id || name || generatedId;
  const fieldName = name || id || generatedId;

  return (
    <textarea
      id={fieldId}
      name={fieldName}
      data-slot="textarea"
      className={cn(
        "flex min-h-32 w-full rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-4 py-3 text-sm text-[var(--app-text-primary)] shadow-sm outline-none placeholder:text-[var(--app-text-muted)] focus-visible:border-[var(--app-border-strong)] focus-visible:ring-[3px] focus-visible:ring-[rgba(43,223,222,0.22)] disabled:cursor-not-allowed disabled:opacity-50 [font:inherit] placeholder:[font:inherit]",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
