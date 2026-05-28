"use client";

import { useEffect, useMemo, useState } from "react";
import { Activity, AlertTriangle, BarChart3, CheckCircle2, Coins, Database, ImageIcon, LoaderCircle, RefreshCw, UsersRound } from "lucide-react";
import { Link } from "react-router-dom";
import { toast } from "sonner";

import { AdminPage } from "@/components/admin-layout";
import { AppTimeSeg } from "@/components/app-controls";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import { fetchBusinessDashboard, type BusinessDashboard } from "@/lib/api";
import { cn } from "@/lib/utils";

import "./dashboard.css";

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let amount = bytes / 1024;
  let unitIndex = 0;
  while (amount >= 1024 && unitIndex < units.length - 1) { amount /= 1024; unitIndex += 1; }
  return `${amount >= 10 ? amount.toFixed(1) : amount.toFixed(2)} ${units[unitIndex]}`;
}

function successRate(success: number | undefined, total: number | undefined) {
  const all = Number(total || 0);
  return all <= 0 ? "0%" : `${Math.round((Number(success || 0) / all) * 100)}%`;
}

export default function DashboardPage() {
  const [dashboard, setDashboard] = useState<BusinessDashboard | null>(null);
  const [timeRange, setTimeRange] = useState<TimeRangeValue>({ preset: "last7", from: "", to: "" });
  const [loading, setLoading] = useState(true);
  const dashboardQuery = useMemo(() => timeRangeQuery(timeRange), [timeRange]);

  const loadDashboard = async () => {
    setLoading(true);
    try {
      setDashboard(await fetchBusinessDashboard(dashboardQuery));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取管理员总览失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadDashboard(); }, [dashboardQuery.from, dashboardQuery.timeRange, dashboardQuery.timezone, dashboardQuery.to]);

  const summary = dashboard?.summary;
  const issueCount = Number(summary?.disabledUserCount || 0) + Number(summary?.failedCount || 0);
  const maxModelCount = useMemo(() => Math.max(1, ...(dashboard?.modelUsage || []).map((m) => m.generation_count || 0)), [dashboard]);

  const v = (val: ReactNode) => (loading ? "-" : val);
  type ReactNode = string | number;

  return (
    <AdminPage>
      {/* ── 标题区(原型 app-page-head) ── */}
      <div className="app-page-head">
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <h1 style={{ margin: 0 }}>管理员总览</h1>
            <span className="app-stat-ic mono" style={{ width: 32, height: 32, flexShrink: 0 }}>
              <BarChart3 className="size-4" />
            </span>
          </div>
          <p>生成、图片、存储、模型和 Top 用户按时间范围统计；用户数与余额为当前状态。</p>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <AppTimeSeg value={timeRange} onChange={setTimeRange} />
          <button className="app-btn" type="button" onClick={() => void loadDashboard()} disabled={loading}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        </div>
      </div>

      {/* ── 6 指标卡(原型 dash-metrics) ── */}
      <section className="dash-metrics">
        {([
          { label: "用户", value: numberText(summary?.userCount), sub: `${numberText(summary?.activeUserCount)} 启用 / ${numberText(summary?.adminCount)} 管理员`, icon: UsersRound, color: "c-pri" },
          { label: "生成", value: numberText(summary?.generationCount), sub: `成功率 ${successRate(summary?.successCount, summary?.generationCount)}`, icon: Activity, color: "c-sky" },
          { label: "图片", value: numberText(summary?.imageCount), sub: `${numberText(summary?.conversationCount)} 个会话`, icon: ImageIcon, color: "c-violet" },
          { label: "存储", value: formatBytes(summary?.storageBytes), sub: "业务图片资产", icon: Database, color: "c-emerald" },
          { label: "点数消耗", value: numberText(summary?.creditSpent), sub: `余额 ${numberText(summary?.creditBalance)}`, icon: Coins, color: "c-amber" },
          { label: "待关注", value: numberText(issueCount), sub: `${numberText(summary?.failedCount)} 失败 / ${numberText(summary?.disabledUserCount)} 禁用`, icon: AlertTriangle, color: issueCount > 0 ? "c-rose" : "c-emerald" },
        ] as const).map((item) => {
          const Icon = item.icon;
          return (
            <div className="dash-metric" key={item.label}>
              <div>
                <div className="lbl">{item.label}</div>
                <div className={cn("val", item.color)}>{v(item.value)}</div>
                <div className="sub">{v(item.sub)}</div>
              </div>
              <Icon className={cn(item.color)} />
            </div>
          );
        })}
      </section>

      {/* ── 模型分布 + 消耗最高用户 ── */}
      <section className="dash-row">
        <div className="app-panel">
          <div className="panel-title"><h3>模型分布</h3></div>
          <div className="dash-bars">
            {loading ? (
              <div style={{ padding: "40px 0", textAlign: "center", color: "var(--app-text-muted)" }}>
                <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" /> 读取中
              </div>
            ) : !dashboard?.modelUsage.length ? (
              <div style={{ padding: "40px 0", textAlign: "center", color: "var(--app-text-muted)" }}>暂无模型数据</div>
            ) : (
              dashboard.modelUsage.map((item) => (
                <div className="dash-bar" key={item.model || "unknown"}>
                  <div className="bar-top">
                    <b>{item.model || "-"}</b>
                    <span>{numberText(item.generation_count)} 次 / {numberText(item.credits_used)} 点</span>
                  </div>
                  <div className="bar-track">
                    <div className="bar-fill" style={{ width: `${Math.max(4, Math.round(((item.generation_count || 0) / maxModelCount) * 100))}%` }} />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div className="app-panel">
          <div className="panel-title">
            <h3>消耗最高用户</h3>
            <Link to="/users" className="app-btn">用户管理</Link>
          </div>
          <div className="dash-rank">
            {loading ? (
              <div style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" /> 读取中
              </div>
            ) : !dashboard?.topUsers.length ? (
              <div style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无用户数据</div>
            ) : (
              dashboard.topUsers.map((user) => (
                <Link key={user.id} to={`/users/${encodeURIComponent(user.id)}`}>
                  <div className="u">
                    <b>{user.username}</b>
                    <small>生成 {numberText(user.usage.generation_count)} / 图片 {numberText(user.usage.image_count)}</small>
                  </div>
                  <div className="v">
                    <b>{numberText(user.credit.spent)}</b>
                    <small>余额 {numberText(user.credit.balance)}</small>
                  </div>
                </Link>
              ))
            )}
          </div>
        </div>
      </section>

      {/* ── 范围内概况 + 待关注摘要(反向布局) ── */}
      <section className="dash-row rev">
        <div className="app-panel">
          <div className="panel-title">
            <h3>范围内概况</h3>
            <Link to="/admin/usage" className="app-btn">查看使用记录</Link>
          </div>
          <div className="dash-subgrid">
            {([
              { label: "生成记录", value: numberText(summary?.generationCount), icon: Activity, color: "c-pri" },
              { label: "成功记录", value: numberText(summary?.successCount), icon: CheckCircle2, color: "c-emerald" },
              { label: "图片产出", value: numberText(summary?.imageCount), icon: ImageIcon, color: "c-violet" },
              { label: "扣点", value: numberText(summary?.creditSpent), icon: Coins, color: "c-amber" },
            ] as const).map((item) => {
              const Icon = item.icon;
              return (
                <div className="dash-subcard" key={item.label}>
                  <div className="top">
                    <span>{item.label}</span>
                    <Icon className={item.color} />
                  </div>
                  <div className={cn("num", item.color)}>{v(item.value)}</div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="app-panel">
          <div className="panel-title"><h3>待关注摘要</h3></div>
          <div className="dash-subgrid three">
            {([
              { label: "失败记录", value: numberText(summary?.failedCount), icon: AlertTriangle, color: Number(summary?.failedCount || 0) > 0 ? "c-rose" : "c-emerald" },
              { label: "禁用用户", value: numberText(summary?.disabledUserCount), icon: UsersRound, color: Number(summary?.disabledUserCount || 0) > 0 ? "c-amber" : "c-emerald" },
              { label: "存储占用", value: formatBytes(summary?.storageBytes), icon: Database, color: "c-emerald" },
            ] as const).map((item) => {
              const Icon = item.icon;
              return (
                <div className="dash-subcard" key={item.label}>
                  <div className="top">
                    <span>{item.label}</span>
                    <Icon className={item.color} />
                  </div>
                  <div className={cn("num", item.color)}>{v(item.value)}</div>
                </div>
              );
            })}
          </div>
          <div className="panel-foot">
            <Link to="/admin/operations" className="app-btn">查看运维监控</Link>
          </div>
        </div>
      </section>
    </AdminPage>
  );
}
