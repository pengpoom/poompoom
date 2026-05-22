import {
  adminInputClass,
  adminSubPanelClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { cn } from "@/lib/utils";

export const settingsInputClass = cn(
  "h-11 rounded-[var(--app-radius-md)]",
  adminInputClass,
);

export const settingsSelectClass = cn(settingsInputClass, "focus-visible:ring-0");

export const settingsSmallButtonClass = "h-9 px-3 text-xs";

export const settingsActionButtonClass = "h-10 px-4";

export const settingsCounterClass =
  "rounded-full bg-[var(--app-bg-surface)] px-2.5 py-1 text-[var(--app-text-muted)]";

export const settingsTableWrapClass =
  "overflow-x-auto rounded-[var(--app-radius-lg)] border border-[var(--app-border)]";

export {
  adminSubPanelClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
};
