import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export function AdminPage({
  children,
  className,
  maxWidth = "",
}: {
  children: ReactNode;
  className?: string;
  maxWidth?: string;
}) {
  return (
    <main
      className="app-admin-page-bg relative text-[var(--app-text-primary)]"
      style={{ padding: "26px clamp(24px, 7.5vw, 144px) 40px", minHeight: "100%" }}
    >
      <div className={cn("relative z-10 flex w-full flex-col", maxWidth, className)} style={{ gap: 18 }}>{children}</div>
    </main>
  );
}

export function AdminHeader({
  title,
  description,
  actions,
  icon: Icon,
  iconVariant,
  children,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  icon?: LucideIcon;
  iconVariant?: "" | "sky" | "ok" | "amber" | "violet" | "fail";
  children?: ReactNode;
}) {
  return (
    <div className="app-page-head">
      <div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <h1 style={{ margin: 0 }}>{title}</h1>
          {Icon ? (
            <span className={cn("app-stat-ic", iconVariant || "mono")} style={{ width: 32, height: 32, flexShrink: 0 }}>
              <Icon className="size-4" />
            </span>
          ) : null}
          {children}
        </div>
        {description ? <p>{description}</p> : null}
      </div>
      {actions ? (
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          {actions}
        </div>
      ) : null}
    </div>
  );
}

export function AdminPanel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <section className={cn("app-panel", className)}>{children}</section>;
}

export function AdminSectionTitle({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="panel-title">
      <h3>{title}</h3>
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
    <section className={cn("app-toolbar", className)}>
      {children}
    </section>
  );
}

const iconVariantMap: Record<string, string> = {
  "text-cyan-300": "",
  "text-sky-300": "sky",
  "text-emerald-300": "ok",
  "text-green-300": "ok",
  "text-amber-300": "amber",
  "text-violet-300": "violet",
  "text-rose-300": "fail",
  "text-red-300": "fail",
};

function resolveIconVariant(color: string) {
  for (const [key, variant] of Object.entries(iconVariantMap)) {
    if (color.includes(key)) return variant;
  }
  return "";
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
  const variant = resolveIconVariant(color);
  return (
    <div className={cn("app-stat", className)}>
      <span className={cn("app-stat-ic", variant)}>
        <Icon className="size-5" />
      </span>
      <div>
        <b>{value}</b>
        <small>{label}</small>
        {sub ? <><br /><small>{sub}</small></> : null}
      </div>
    </div>
  );
}
