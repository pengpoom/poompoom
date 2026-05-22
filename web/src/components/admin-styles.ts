import { cn } from "@/lib/utils";

export const adminPanelClass =
  "overflow-hidden rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-panel)] shadow-[var(--app-shadow-floating)] backdrop-blur-2xl";

export const adminSubPanelClass =
  "rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] backdrop-blur-xl";

export const adminLabelClass = "text-xs font-medium text-[var(--app-text-muted)]";

export const adminInputClass =
  "border-[var(--app-border)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)] placeholder:text-[var(--app-text-muted)] shadow-none backdrop-blur-xl focus-visible:ring-[var(--app-focus-ring)]";

export const adminInputPillClass = cn("h-10 rounded-[var(--app-radius-pill)]", adminInputClass);

export const adminTableClass = "min-w-full divide-y divide-[var(--app-border)] text-sm";

export const adminTableHeadClass =
  "bg-[var(--app-bg-surface)] text-left text-xs font-semibold text-[var(--app-text-muted)]";

export const adminTableBodyClass = "divide-y divide-[var(--app-border)]";

export const adminTableRowClass = "transition hover:bg-[var(--app-bg-surface-hover)]";
