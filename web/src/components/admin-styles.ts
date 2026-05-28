import { cn } from "@/lib/utils";

export const adminPanelClass = "app-panel";

export const adminSubPanelClass =
  "rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] backdrop-blur-[18px]";

export const adminLabelClass = "text-xs font-medium text-[var(--app-text-muted)]";

export const adminInputClass =
  "border-[var(--app-border)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)] placeholder:text-[var(--app-text-muted)] shadow-none backdrop-blur-xl focus-visible:ring-[var(--app-focus-ring)]";

export const adminInputPillClass = cn("h-10 rounded-[var(--app-radius-pill)]", adminInputClass);

export const adminTableClass = "app-table";

export const adminTableHeadClass = "";

export const adminTableBodyClass = "";

export const adminTableRowClass = "";
