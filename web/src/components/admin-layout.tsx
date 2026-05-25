import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import { adminLabelClass, adminPanelClass, adminSubPanelClass } from "@/components/admin-styles";

export function AdminPage({
  children,
  className,
  maxWidth = "max-w-[1400px]",
}: {
  children: ReactNode;
  className?: string;
  maxWidth?: string;
}) {
  return (
    <main className="app-admin-page-bg relative min-h-full px-5 py-5 text-[var(--app-text-primary)] sm:px-7 sm:py-7 lg:px-10 lg:py-8 xl:px-12">
      <div className={cn("relative z-10 mx-auto flex w-full flex-col gap-5 sm:gap-6", maxWidth, className)}>{children}</div>
    </main>
  );
}

export function AdminHeader({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-4 border-b border-[var(--app-border)] pb-5 lg:flex-row lg:items-end lg:justify-between">
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h1 className="text-[26px] font-semibold leading-tight tracking-normal text-[var(--app-text-primary)]">{title}</h1>
          {children ? (
            <div className="inline-flex shrink-0 items-center [&>*]:!mb-0 [&_.size-10]:!size-8 [&_.size-11]:!size-8 [&_.size-12]:!size-8 [&_svg]:!size-4">
              {children}
            </div>
          ) : null}
        </div>
        {description ? <p className="mt-2 max-w-3xl text-sm leading-6 text-[var(--app-text-muted)]">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">{actions}</div> : null}
    </section>
  );
}

export function AdminPanel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <section className={cn(adminPanelClass, className)}>{children}</section>;
}

export function AdminSectionTitle({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex min-h-12 items-center justify-between gap-3 border-b border-[var(--app-border)] px-4 py-3 sm:px-5">
      <h2 className="text-[15px] font-semibold text-[var(--app-text-primary)]">{title}</h2>
      {action}
    </div>
  );
}

export function AdminToolbar({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      className={cn(
        "rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-3 shadow-[var(--app-shadow-floating)] backdrop-blur-2xl",
        className,
      )}
    >
      {children}
    </section>
  );
}

export function AdminStatCard({
  label,
  value,
  sub,
  icon: Icon,
  color,
  className,
}: {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  icon: LucideIcon;
  color: string;
  className?: string;
}) {
  return (
    <div className={cn(adminSubPanelClass, "flex h-full min-h-[104px] items-center justify-between gap-3 p-4", className)}>
      <div className="min-w-0">
        <div className={adminLabelClass}>{label}</div>
        <div className={cn("mt-2 truncate text-2xl font-semibold leading-none", color)}>{value}</div>
        {sub ? <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]">{sub}</div> : null}
      </div>
      <Icon className={cn("size-5 shrink-0", color)} />
    </div>
  );
}
