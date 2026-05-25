"use client";

import { useEffect, useMemo, useState } from "react";
import { Activity, AlertTriangle, BarChart3, CheckCircle2, Coins, Database, ImageIcon, LoaderCircle, RefreshCw, UsersRound } from "lucide-react";
import { Link } from "react-router-dom";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import { adminSubPanelClass } from "@/components/admin-styles";
import { TimeRangeFilter } from "@/components/time-range-filter";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import { Button } from "@/components/ui/button";
import { fetchBusinessDashboard, type BusinessDashboard } from "@/lib/api";
import { cn } from "@/lib/utils";

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  const units = ["KB", "MB", "GB", "TB"];
  let amount = bytes / 1024;
  let unitIndex = 0;
  while (amount >= 1024 && unitIndex < units.length - 1) {
    amount /= 1024;
    unitIndex += 1;
  }
  return `${amount >= 10 ? amount.toFixed(1) : amount.toFixed(2)} ${units[unitIndex]}`;
}

function successRate(success: number | undefined, total: number | undefined) {
  const all = Number(total || 0);
  if (all <= 0) {
    return "0%";
  }
  return `${Math.round((Number(success || 0) / all) * 100)}%`;
}

export default function DashboardPage() {
  const [dashboard, setDashboard] = useState<BusinessDashboard | null>(null);
  const [timeRange, setTimeRange] = useState<TimeRangeValue>({ preset: "last7", from: "", to: "" });
  const [loading, setLoading] = useState(true);
  const dashboardQuery = useMemo(() => timeRangeQuery(timeRange), [timeRange]);

  const loadDashboard = async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessDashboard(dashboardQuery);
      setDashboard(payload);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取管理员总览失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadDashboard();
  }, [dashboardQuery.from, dashboardQuery.timeRange, dashboardQuery.timezone, dashboardQuery.to]);

  const summary = dashboard?.summary;
  const issueCount = Number(summary?.disabledUserCount || 0) + Number(summary?.failedCount || 0);
  const maxModelCount = useMemo(() => {
    return Math.max(1, ...(dashboard?.modelUsage || []).map((item) => item.generation_count || 0));
  }, [dashboard]);

  return (
    <AdminPage>
        <AdminHeader
          title="管理员总览"
          description="生成、图片、存储、模型和 Top 用户按时间范围统计；用户数与余额为当前状态。"
          actions={
            <div className="grid gap-3 sm:grid-cols-[minmax(220px,280px)_auto] sm:items-center">
            <TimeRangeFilter
              value={timeRange}
              onApply={setTimeRange}
              panelAlign="end"
              panelClassName="w-[min(calc(100vw-2rem),460px)]"
            />
            <Button type="button" variant="outline" onClick={() => void loadDashboard()} disabled={loading}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
            </div>
          }
        >
          <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            <BarChart3 className="size-5" />
          </div>
        </AdminHeader>

        <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
          {[
            { label: "用户", value: numberText(summary?.userCount), sub: `${numberText(summary?.activeUserCount)} 启用 / ${numberText(summary?.adminCount)} 管理员`, icon: UsersRound, color: "text-[var(--app-text-primary)]" },
            { label: "生成", value: numberText(summary?.generationCount), sub: `成功率 ${successRate(summary?.successCount, summary?.generationCount)}`, icon: Activity, color: "text-sky-300" },
            { label: "图片", value: numberText(summary?.imageCount), sub: `${numberText(summary?.conversationCount)} 个会话`, icon: ImageIcon, color: "text-violet-300" },
            { label: "存储", value: formatBytes(summary?.storageBytes), sub: "业务图片资产", icon: Database, color: "text-emerald-300" },
            { label: "点数消耗", value: numberText(summary?.creditSpent), sub: `余额 ${numberText(summary?.creditBalance)}`, icon: Coins, color: "text-amber-300" },
            { label: "待关注", value: numberText(issueCount), sub: `${numberText(summary?.failedCount)} 失败 / ${numberText(summary?.disabledUserCount)} 禁用`, icon: AlertTriangle, color: issueCount > 0 ? "text-rose-300" : "text-emerald-300" },
          ].map((item) => {
            return (
              <AdminStatCard key={item.label} {...item} value={loading ? "-" : item.value} sub={loading ? "-" : item.sub} />
            );
          })}
        </section>

        <section className="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.65fr)]">
          <AdminPanel>
            <AdminSectionTitle title="模型分布" />
            <div className="space-y-3 p-4">
              {loading ? (
                <div className="py-10 text-center text-[var(--app-text-muted)]">
                  <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                  读取中
                </div>
              ) : !dashboard?.modelUsage.length ? (
                <div className="py-10 text-center text-[var(--app-text-muted)]">暂无模型数据</div>
              ) : (
                dashboard.modelUsage.map((item) => (
                  <div key={item.model || "unknown"} className="grid gap-2">
                    <div className="flex items-center justify-between gap-3 text-sm">
                      <span className="min-w-0 truncate font-medium text-[var(--app-text-primary)]">{item.model || "-"}</span>
                      <span className="shrink-0 text-[var(--app-text-muted)]">
                        {numberText(item.generation_count)} 次 / {numberText(item.credits_used)} 点
                      </span>
                    </div>
                    <div className="h-2 overflow-hidden rounded-full bg-[var(--app-bg-surface)]">
                      <div className="h-full rounded-full bg-sky-500" style={{ width: `${Math.max(4, Math.round(((item.generation_count || 0) / maxModelCount) * 100))}%` }} />
                    </div>
                  </div>
                ))
              )}
            </div>
          </AdminPanel>

          <AdminPanel>
            <AdminSectionTitle title="消耗最高用户" action={<Button type="button" variant="outline" size="sm" asChild><Link to="/users">用户管理</Link></Button>} />
            <div className="divide-y divide-[var(--app-border)]">
              {loading ? (
                <div className="px-4 py-12 text-center text-[var(--app-text-muted)]">
                  <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                  读取中
                </div>
              ) : !dashboard?.topUsers.length ? (
                <div className="px-4 py-12 text-center text-[var(--app-text-muted)]">暂无用户数据</div>
              ) : (
                dashboard.topUsers.map((user) => (
                  <Link key={user.id} to={`/users/${encodeURIComponent(user.id)}`} className="flex items-center justify-between gap-3 px-4 py-3 transition hover:bg-[var(--app-bg-surface)]">
                    <div className="min-w-0">
                      <div className="truncate font-medium text-[var(--app-text-primary)]">{user.username}</div>
                      <div className="mt-0.5 truncate text-xs text-[var(--app-text-muted)]">
                        生成 {numberText(user.usage.generation_count)} / 图片 {numberText(user.usage.image_count)}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="font-semibold text-amber-300">{numberText(user.credit.spent)}</div>
                      <div className="mt-0.5 text-xs text-[var(--app-text-muted)]">余额 {numberText(user.credit.balance)}</div>
                    </div>
                  </Link>
                ))
              )}
            </div>
          </AdminPanel>
        </section>

        <section className="grid gap-6 xl:grid-cols-[minmax(360px,0.65fr)_minmax(0,1.35fr)]">
          <AdminPanel>
            <AdminSectionTitle title="范围内概况" action={<Button type="button" variant="outline" size="sm" asChild><Link to="/admin/usage">查看使用记录</Link></Button>} />
            <div className="grid gap-3 p-4 sm:grid-cols-2">
              {[
                { label: "生成记录", value: numberText(summary?.generationCount), icon: Activity, color: "text-[var(--app-text-primary)]" },
                { label: "成功记录", value: numberText(summary?.successCount), icon: CheckCircle2, color: "text-emerald-300" },
                { label: "图片产出", value: numberText(summary?.imageCount), icon: ImageIcon, color: "text-violet-300" },
                { label: "扣点", value: numberText(summary?.creditSpent), icon: Coins, color: "text-amber-300" },
              ].map((item) => {
                const Icon = item.icon;
                return (
                  <div key={item.label} className={cn(adminSubPanelClass, "p-4")}>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs text-[var(--app-text-muted)]">{item.label}</span>
                      <Icon className={cn("size-4", item.color)} />
                    </div>
                    <div className={cn("mt-2 text-xl font-semibold", item.color)}>{loading ? "-" : item.value}</div>
                  </div>
                );
              })}
            </div>
          </AdminPanel>

          <AdminPanel>
            <AdminSectionTitle title="待关注摘要" />
            <div className="grid gap-3 p-4 sm:grid-cols-3">
              {[
                { label: "失败记录", value: numberText(summary?.failedCount), icon: AlertTriangle, color: Number(summary?.failedCount || 0) > 0 ? "text-rose-300" : "text-emerald-300" },
                { label: "禁用用户", value: numberText(summary?.disabledUserCount), icon: UsersRound, color: Number(summary?.disabledUserCount || 0) > 0 ? "text-amber-300" : "text-emerald-300" },
                { label: "存储占用", value: formatBytes(summary?.storageBytes), icon: Database, color: "text-emerald-300" },
              ].map((item) => {
                const Icon = item.icon;
                return (
                  <div key={item.label} className={cn(adminSubPanelClass, "p-4")}>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs text-[var(--app-text-muted)]">{item.label}</span>
                      <Icon className={cn("size-4", item.color)} />
                    </div>
                    <div className={cn("mt-2 text-xl font-semibold", item.color)}>{loading ? "-" : item.value}</div>
                  </div>
                );
              })}
            </div>
            <div className="border-t border-[var(--app-border)] px-4 py-3 text-xs text-[var(--app-text-muted)]">
              <Button type="button" variant="outline" size="sm" asChild>
                <Link to="/admin/operations">查看运维监控</Link>
              </Button>
            </div>
          </AdminPanel>
        </section>
    </AdminPage>
  );
}
