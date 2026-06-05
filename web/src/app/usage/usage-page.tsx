"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, CheckCircle2, Clock3, Coins, History, ImageIcon, LoaderCircle, RefreshCw, XCircle } from "lucide-react";
import { toast } from "sonner";

import { AdminPage } from "@/components/admin-layout";
import { AppSelect, AppTimeSeg } from "@/components/app-controls";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import { fetchAllBusinessUsage, fetchBusinessUsage, fetchBusinessUsers, type BusinessUsageRecord, type BusinessUser, type PaginationMeta } from "@/lib/api";
import { cn } from "@/lib/utils";

type UsagePageProps = {
  scope: "self" | "admin";
};

function formatDateTime(value: string | undefined) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatDuration(ms: number | undefined) {
  const value = Math.max(0, Number(ms || 0));
  if (value <= 0) {
    return "-";
  }
  if (value < 1000) {
    return `${value} ms`;
  }
  const seconds = value / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(seconds >= 10 ? 1 : 2)} 秒`;
  }
  const minutes = Math.floor(seconds / 60);
  const remain = Math.round(seconds % 60);
  return `${minutes} 分 ${remain} 秒`;
}

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function statusBadgeClass(status: string) {
  if (status === "succeeded") return "app-badge ok";
  if (status === "failed") return "app-badge fail";
  return "app-badge run";
}

function statusText(status: string) {
  if (status === "succeeded") return "成功";
  if (status === "failed") return "失败";
  return status || "-";
}

function sourceBadgeClass(record: BusinessUsageRecord) {
  return record.api_key_id ? "app-badge run" : "app-badge";
}

function sourceText(record: BusinessUsageRecord) {
  return record.api_key_id ? "API" : "网页";
}

function promptPreview(value: string) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "-";
  }
  return raw.length > 72 ? `${raw.slice(0, 72)}...` : raw;
}

function userMatchesQuery(user: BusinessUser, query: string) {
  if (!query) {
    return true;
  }
  return [user.id, String(user.uid || ""), user.email, user.username].some((value) => String(value || "").toLowerCase().includes(query));
}

function pageNumbers(current: number, total: number) {
  const pages: number[] = [];
  for (let i = 1; i <= total; i++) {
    if (i <= 3 || i > total - 2 || Math.abs(i - current) <= 1) {
      pages.push(i);
    } else if (pages.length > 0 && pages[pages.length - 1] !== -1) {
      pages.push(-1);
    }
  }
  return pages;
}

export default function UsagePage({ scope }: UsagePageProps) {
  const [items, setItems] = useState<BusinessUsageRecord[]>([]);
  const [page, setPage] = useState<PaginationMeta>({ page: 1, pageSize: 20, total: 0 });
  const [status, setStatus] = useState("all");
  const [source, setSource] = useState("all");
  const [model, setModel] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRangeValue>({ preset: "last7", from: "", to: "" });
  const [userId, setUserId] = useState("all");
  const [users, setUsers] = useState<BusinessUser[]>([]);
  const [userSearch, setUserSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const isAdminScope = scope === "admin";
  const pageRef = useRef(page);

  useEffect(() => {
    pageRef.current = page;
  }, [page]);

  const loadItems = useCallback(async (nextPage?: number) => {
    const currentPage = pageRef.current;
    const targetPage = nextPage ?? currentPage.page;
    setLoading(true);
    try {
      const query = {
        page: targetPage,
        pageSize: currentPage.pageSize,
        status: status === "all" ? undefined : status,
        source: source === "all" ? undefined : source,
        model: model.trim() || undefined,
        ...timeRangeQuery(timeRange),
        userId: isAdminScope && userId !== "all" ? userId : undefined,
        userQuery: isAdminScope && userId === "all" ? userSearch.trim() || undefined : undefined,
      };
      const payload = isAdminScope ? await fetchAllBusinessUsage(query) : await fetchBusinessUsage(query);
      setItems(payload.items || []);
      setPage(payload.page || { page: targetPage, pageSize: currentPage.pageSize, total: payload.items?.length || 0 });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取使用记录失败");
    } finally {
      setLoading(false);
    }
  }, [isAdminScope, model, status, source, timeRange, userId, userSearch]);

  const loadUsers = useCallback(async () => {
    if (!isAdminScope) {
      return;
    }
    try {
      const payload = await fetchBusinessUsers({ includeDeleted: true });
      setUsers(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取用户列表失败");
    }
  }, [isAdminScope]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  useEffect(() => {
    void loadItems(1);
  }, [loadItems]);

  const usersByID = useMemo(() => new Map(users.map((user) => [user.id, user])), [users]);

  const visibleUsers = useMemo(() => {
    const query = userSearch.trim().toLowerCase();
    return users.filter((user) => userMatchesQuery(user, query));
  }, [userSearch, users]);

  const stats = useMemo(() => {
    return items.reduce(
      (acc, item) => {
        acc.records += 1;
        if (item.status === "succeeded") acc.succeeded += 1;
        if (item.status === "failed") acc.failed += 1;
        acc.credits += item.credits_used || 0;
        acc.durationMs += item.duration_ms || 0;
        return acc;
      },
      { records: 0, succeeded: 0, failed: 0, credits: 0, durationMs: 0 },
    );
  }, [items]);

  const totalPages = Math.max(1, Math.ceil(page.total / page.pageSize));

  return (
    <AdminPage>
      <div className="app-page-head">
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <h1 style={{ margin: 0 }}>使用记录</h1>
            <span className="app-stat-ic mono" style={{ width: 32, height: 32, flexShrink: 0 }}>
              <History className="size-4" />
            </span>
          </div>
          <p>{isAdminScope ? "所有用户的生成与扣点记录。" : "查看当前账户的图片生成消耗。"}</p>
        </div>
        <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading}>
          {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
          刷新
        </button>
      </div>

      <div className="app-stats">
        <div className="app-stat">
          <span className="app-stat-ic"><Activity className="size-5" /></span>
          <div><b>{numberText(stats.records)}</b><small>总请求</small></div>
        </div>
        <div className="app-stat">
          <span className="app-stat-ic ok"><CheckCircle2 className="size-5" /></span>
          <div><b>{numberText(stats.succeeded)}</b><small>成功</small></div>
        </div>
        <div className="app-stat">
          <span className="app-stat-ic fail"><XCircle className="size-5" /></span>
          <div><b>{numberText(stats.failed)}</b><small>失败</small></div>
        </div>
        {isAdminScope ? (
          <div className="app-stat">
            <span className="app-stat-ic"><Clock3 className="size-5" /></span>
            <div><b>{formatDuration(stats.durationMs)}</b><small>总耗时</small></div>
          </div>
        ) : null}
      </div>

      <div className="app-toolbar">
        <AppTimeSeg value={timeRange} onChange={setTimeRange} />
        <AppSelect
          value={status}
          onChange={setStatus}
          options={[
            { value: "all", label: "全部状态" },
            { value: "succeeded", label: "成功" },
            { value: "failed", label: "失败" },
          ]}
        />
        <AppSelect
          value={source}
          onChange={setSource}
          options={[
            { value: "all", label: "全部来源" },
            { value: "api", label: "API" },
            { value: "web", label: "网页" },
          ]}
        />
        <input className="app-input" type="text" value={model} onChange={(e) => setModel(e.target.value)} placeholder="全部模型" />
        {isAdminScope ? (
          <>
            <input className="app-input" type="search" value={userSearch} onChange={(e) => { setUserSearch(e.target.value); setUserId("all"); }} placeholder="搜索用户 / UID…" />
            <AppSelect
              value={userId}
              onChange={setUserId}
              options={[
                { value: "all", label: "全部用户" },
                ...visibleUsers.map((user) => ({ value: user.id, label: `UID ${user.uid || "-"} · ${user.username || user.email || "-"}` })),
              ]}
            />
          </>
        ) : null}
        <span className="spacer" />
        <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading}>
          {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
          刷新
        </button>
      </div>

      <div className="app-panel">
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                {isAdminScope ? <th>用户</th> : null}
                <th>时间</th>
                <th>提示词</th>
                <th>模型</th>
                <th>图片</th>
                <th>耗时</th>
                <th>扣点</th>
                <th>来源</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={isAdminScope ? 9 : 8} style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                    <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" /> 读取中
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={isAdminScope ? 9 : 8} style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                    暂无使用记录
                  </td>
                </tr>
              ) : (
                items.map((item) => {
                  const user = usersByID.get(item.user_id);
                  const uid = item.uid || user?.uid;
                  const username = item.username || user?.username;
                  const email = item.email || user?.email;
                  return (
                    <tr key={item.id}>
                      {isAdminScope ? (
                        <td>
                          <div className="app-user">
                            <div className="app-user-meta">
                              <b>{username || "-"}</b>
                              <small>uid_{uid || "-"} · {email || item.user_id}</small>
                            </div>
                          </div>
                        </td>
                      ) : null}
                      <td>{formatDateTime(item.created_at)}</td>
                      <td className="prompt" title={item.prompt}>{promptPreview(item.prompt)}</td>
                      <td>{item.model || "-"}</td>
                      <td>{numberText(item.count)}{item.size ? ` · ${item.size}` : ""}</td>
                      <td>{formatDuration(item.duration_ms)}</td>
                      <td className="strong">{numberText(item.credits_used)}</td>
                      <td><span className={sourceBadgeClass(item)}>{sourceText(item)}</span></td>
                      <td><span className={statusBadgeClass(item.status)}>{statusText(item.status)}</span></td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
        <div className="app-pager">
          <span className="info">共 {page.total} 条 · 每页 {page.pageSize} 条</span>
          <div className="pages">
            <button type="button" disabled={loading || page.page <= 1} onClick={() => void loadItems(page.page - 1)}>上一页</button>
            {pageNumbers(page.page, totalPages).map((p, i) =>
              p === -1 ? <span key={`gap-${i}`} style={{ color: "var(--app-text-muted)", padding: "0 4px" }}>…</span> : (
                <button key={p} type="button" className={p === page.page ? "on" : ""} onClick={() => void loadItems(p)}>{p}</button>
              )
            )}
            <button type="button" disabled={loading || page.page >= totalPages} onClick={() => void loadItems(page.page + 1)}>下一页</button>
          </div>
        </div>
      </div>
    </AdminPage>
  );
}
