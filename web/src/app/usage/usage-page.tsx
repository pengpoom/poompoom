"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, Clock3, Coins, ImageIcon, LoaderCircle, RefreshCw, UserRound } from "lucide-react";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminStatCard,
} from "@/components/admin-layout";
import {
  adminInputPillClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TimeRangeFilter } from "@/components/time-range-filter";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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

function statusVariant(status: string) {
  return status === "succeeded" ? "success" : status === "failed" ? "danger" : "secondary";
}

function statusText(status: string) {
  if (status === "succeeded") return "成功";
  if (status === "failed") return "失败";
  return status || "-";
}

function paginationText(page: PaginationMeta) {
  if (!page.total) {
    return "0 / 0";
  }
  const start = (page.page - 1) * page.pageSize + 1;
  const end = Math.min(page.page * page.pageSize, page.total);
  return `${start}-${end} / ${page.total}`;
}

function promptPreview(value: string) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "-";
  }
  return raw.length > 72 ? `${raw.slice(0, 72)}...` : raw;
}

function userOptionText(user: BusinessUser) {
  const suffix = user.status === "deleted" ? " · 已删除" : "";
  return `UID ${user.uid || "-"} · ${user.email || "-"} · ${user.username || "-"}${suffix}`;
}

function userSelectedText(user: BusinessUser | undefined) {
  if (!user) {
    return "全部用户";
  }
  const suffix = user.status === "deleted" ? " · 已删除" : "";
  return `UID ${user.uid || "-"} · ${user.username || user.email || "-"}${suffix}`;
}

function userMatchesQuery(user: BusinessUser, query: string) {
  if (!query) {
    return true;
  }
  return [user.id, String(user.uid || ""), user.email, user.username].some((value) => String(value || "").toLowerCase().includes(query));
}

export default function UsagePage({ scope }: UsagePageProps) {
  const [items, setItems] = useState<BusinessUsageRecord[]>([]);
  const [page, setPage] = useState<PaginationMeta>({ page: 1, pageSize: 20, total: 0 });
  const [status, setStatus] = useState("all");
  const [model, setModel] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRangeValue>({ preset: "last24h", from: "", to: "" });
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
  }, [isAdminScope, model, status, timeRange, userId, userSearch]);

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

  const handleUserSearchChange = useCallback((value: string) => {
    setUserSearch(value);
    setUserId("all");
  }, []);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  useEffect(() => {
    void loadItems(1);
  }, [loadItems]);

  const usersByID = useMemo(() => new Map(users.map((user) => [user.id, user])), [users]);
  const selectedUser = usersByID.get(userId);

  const visibleUsers = useMemo(() => {
    const query = userSearch.trim().toLowerCase();
    const matched = users.filter((user) => userMatchesQuery(user, query));
    if (userId === "all" || matched.some((user) => user.id === userId)) {
      return matched;
    }
    return selectedUser ? [selectedUser, ...matched] : matched;
  }, [selectedUser, userId, userSearch, users]);

  const stats = useMemo(() => {
    return items.reduce(
      (acc, item) => {
        acc.records += 1;
        acc.images += item.count || 0;
        acc.credits += item.credits_used || 0;
        acc.durationMs += item.duration_ms || 0;
        return acc;
      },
      { records: 0, images: 0, credits: 0, durationMs: 0 },
    );
  }, [items]);

  return (
    <AdminPage>
        <AdminHeader
          title="使用记录"
          description={isAdminScope ? "按用户、状态、模型和时间范围筛选业务生成流水。" : "查看当前账户的图片生成消耗和执行状态。"}
          actions={
            <Button type="button" variant="outline" className="h-10 w-full px-4 sm:w-auto" onClick={() => void loadItems()} disabled={loading}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
            </Button>
          }
        >
          <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            {isAdminScope ? <UserRound className="size-5" /> : <Activity className="size-5" />}
          </div>
        </AdminHeader>

        <AdminPanel className="p-4">
        <section className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
            {isAdminScope ? (
              <>
                <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
                  用户搜索
                  <Input value={userSearch} onChange={(event) => handleUserSearchChange(event.target.value)} className={adminInputPillClass} placeholder="UID / 邮箱 / 用户名" />
                </label>
                <label className="grid min-w-0 gap-1.5 text-xs text-[var(--app-text-muted)]">
                  用户
                  <Select value={userId} onValueChange={setUserId}>
                    <SelectTrigger className={`${adminInputPillClass} w-full min-w-0 [&>span]:block [&>span]:max-w-[calc(100%-1.5rem)] [&>span]:truncate`}>
                      <SelectValue>{userSelectedText(selectedUser)}</SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">全部用户</SelectItem>
                      {visibleUsers.map((user) => (
                        <SelectItem key={user.id} value={user.id}>{userOptionText(user)}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </label>
              </>
            ) : null}
            <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
              状态
              <Select value={status} onValueChange={setStatus}>
                <SelectTrigger className={`${adminInputPillClass} w-full`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="succeeded">成功</SelectItem>
                  <SelectItem value="failed">失败</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
              模型
              <Input value={model} onChange={(event) => setModel(event.target.value)} className={adminInputPillClass} placeholder="全部模型" />
            </label>
            <div className="grid gap-1.5 text-xs text-[var(--app-text-muted)] lg:col-span-2">
              <span>时间范围</span>
              <TimeRangeFilter value={timeRange} onApply={setTimeRange} />
            </div>
          </div>
          <div className="flex items-center justify-end gap-2 text-sm text-[var(--app-text-muted)]">
            <span>{paginationText(page)}</span>
            <Button type="button" variant="outline" size="sm" disabled={loading || page.page <= 1} onClick={() => void loadItems(page.page - 1)}>上一页</Button>
            <Button type="button" variant="outline" size="sm" disabled={loading || page.page * page.pageSize >= page.total} onClick={() => void loadItems(page.page + 1)}>下一页</Button>
          </div>
        </section>
        </AdminPanel>

        <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { label: "记录数", value: numberText(stats.records), icon: Activity, color: "text-[var(--app-text-primary)]" },
            { label: "图片数", value: numberText(stats.images), icon: ImageIcon, color: "text-violet-300" },
            { label: "扣点", value: numberText(stats.credits), icon: Coins, color: "text-amber-300" },
            { label: "总耗时", value: formatDuration(stats.durationMs), icon: Clock3, color: "text-[var(--app-text-muted)]" },
          ].map((item) => {
            return <AdminStatCard key={item.label} {...item} />;
          })}
        </section>

        <AdminPanel>
          <div className="overflow-x-auto">
            <table className={adminTableClass}>
              <thead className={adminTableHeadClass}>
                <tr>
                  {isAdminScope ? <th className="px-4 py-3">用户</th> : null}
                  <th className="px-4 py-3">时间</th>
                  <th className="px-4 py-3">提示词</th>
                  <th className="px-4 py-3">模型</th>
                  <th className="px-4 py-3">图片</th>
                  <th className="px-4 py-3">耗时</th>
                  <th className="px-4 py-3">扣点</th>
                  <th className="px-4 py-3">状态</th>
                </tr>
              </thead>
              <tbody className={adminTableBodyClass}>
                {loading ? (
                  <tr>
                    <td colSpan={isAdminScope ? 8 : 7} className="px-4 py-12 text-center text-[var(--app-text-muted)]">
                      <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                      读取中
                    </td>
                  </tr>
                ) : items.length === 0 ? (
                  <tr>
                    <td colSpan={isAdminScope ? 8 : 7} className="px-4 py-12 text-center text-[var(--app-text-muted)]">
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
                      <tr key={item.id} className={adminTableRowClass}>
                        {isAdminScope ? (
                          <td className="px-4 py-3">
                            <div className="font-medium text-[var(--app-text-primary)]">{username || "-"}</div>
                            <div className="mt-0.5 font-mono text-xs text-[var(--app-text-muted)]">UID {uid || "-"}</div>
                            <div className="mt-0.5 max-w-[200px] truncate text-xs text-[var(--app-text-muted)]">{email || item.user_id}</div>
                          </td>
                        ) : null}
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                        <td className="min-w-[280px] px-4 py-3 text-[var(--app-text-primary)]" title={item.prompt}>{promptPreview(item.prompt)}</td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{item.model || "-"}</td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">
                          {numberText(item.count)}
                          {item.size ? <span className="ml-2 text-xs text-[var(--app-text-muted)]">{item.size}</span> : null}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDuration(item.duration_ms)}</td>
                        <td className="whitespace-nowrap px-4 py-3 font-medium text-[var(--app-text-primary)]">{numberText(item.credits_used)}</td>
                        <td className="whitespace-nowrap px-4 py-3">
                          <Badge variant={statusVariant(item.status)}>{statusText(item.status)}</Badge>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </AdminPanel>
    </AdminPage>
  );
}
