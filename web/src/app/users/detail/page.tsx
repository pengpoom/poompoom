"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, ArrowLeft, Clock3, Coins, Database, ImageIcon, LoaderCircle, RefreshCw, UserRound } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
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
import { fetchBusinessUserDetail, type BusinessCreditLedgerEntry, type BusinessUsageRecord, type BusinessUserDetail, type PaginationMeta } from "@/lib/api";
import { cn } from "@/lib/utils";

function formatDateTime(value: string | undefined) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
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

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function roleText(value: string | undefined) {
  return value === "admin" ? "管理员" : "普通用户";
}

function statusText(value: string | undefined) {
  return value === "active" ? "启用" : value === "disabled" ? "禁用" : value === "deleted" ? "已删除" : value || "-";
}

function userStatusVariant(status: string | undefined) {
  return status === "active" ? "success" : status === "deleted" ? "danger" : "warning";
}

function statusVariant(status: string) {
  return status === "succeeded" ? "success" : status === "failed" ? "danger" : "secondary";
}

function generationStatusText(status: string) {
  if (status === "succeeded") return "成功";
  if (status === "failed") return "失败";
  return status || "-";
}

function reasonText(reason: string) {
  if (reason === "admin_adjustment") return "管理员调整";
  if (reason === "image_generation_reserve") return "生成预扣";
  if (reason === "image_generation_refund") return "生成退款";
  return reason || "-";
}

function promptPreview(value: string) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "-";
  }
  return raw.length > 64 ? `${raw.slice(0, 64)}...` : raw;
}

function defaultPage(pageSize = 20): PaginationMeta {
  return { page: 1, pageSize, total: 0 };
}

function paginationText(page: PaginationMeta | undefined) {
  if (!page || !page.total) {
    return "0 / 0";
  }
  const start = (page.page - 1) * page.pageSize + 1;
  const end = Math.min(page.page * page.pageSize, page.total);
  return `${start}-${end} / ${page.total}`;
}

export default function UserDetailPage() {
  const { id } = useParams();
  const userID = String(id || "").trim();
  const [detail, setDetail] = useState<BusinessUserDetail | null>(null);
  const [usagePage, setUsagePage] = useState(defaultPage());
  const [assetsPage, setAssetsPage] = useState(defaultPage());
  const [ledgerPage, setLedgerPage] = useState(defaultPage());
  const [usageModel, setUsageModel] = useState("");
  const [usageTimeRange, setUsageTimeRange] = useState<TimeRangeValue>({ preset: "last24h", from: "", to: "" });
  const [loading, setLoading] = useState(true);
  const pagesRef = useRef({
    usagePage,
    assetsPage,
    ledgerPage,
  });

  useEffect(() => {
    pagesRef.current = {
      usagePage,
      assetsPage,
      ledgerPage,
    };
  }, [assetsPage, ledgerPage, usagePage]);

  const loadDetail = useCallback(async (next?: {
    usagePage?: number;
    assetsPage?: number;
    ledgerPage?: number;
  }) => {
    if (!userID) {
      setLoading(false);
      return;
    }
    const currentPages = pagesRef.current;
    const target = {
      usagePage: next?.usagePage ?? currentPages.usagePage.page,
      assetsPage: next?.assetsPage ?? currentPages.assetsPage.page,
      ledgerPage: next?.ledgerPage ?? currentPages.ledgerPage.page,
    };
    setLoading(true);
    try {
      const payload = await fetchBusinessUserDetail(userID, {
        usagePage: target.usagePage,
        usagePageSize: currentPages.usagePage.pageSize,
        assetsPage: target.assetsPage,
        assetsPageSize: currentPages.assetsPage.pageSize,
        ledgerPage: target.ledgerPage,
        ledgerPageSize: currentPages.ledgerPage.pageSize,
        model: usageModel.trim() || undefined,
        ...timeRangeQuery(usageTimeRange),
      });
      setDetail(payload);
      setUsagePage(payload.recentUsagePage || { ...currentPages.usagePage, page: target.usagePage });
      setAssetsPage(payload.recentAssetsPage || { ...currentPages.assetsPage, page: target.assetsPage });
      setLedgerPage(payload.recentLedgerPage || { ...currentPages.ledgerPage, page: target.ledgerPage });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取用户详情失败");
    } finally {
      setLoading(false);
    }
  }, [usageModel, usageTimeRange, userID]);

  useEffect(() => {
    void loadDetail({ usagePage: 1, assetsPage: 1, ledgerPage: 1 });
  }, [loadDetail]);

  const stats = useMemo(() => {
    return {
      balance: detail?.credit.balance || 0,
      spent: detail?.credit.spent || 0,
      generations: detail?.usage.generation_count || 0,
      images: detail?.usage.image_count || 0,
      storageBytes: detail?.usage.storage_bytes || 0,
      conversations: detail?.conversationCount || 0,
    };
  }, [detail]);

  return (
    <AdminPage>
        <AdminHeader
          title={detail?.user.username || "用户详情"}
          description={<span className="font-mono">{userID}</span>}
          actions={
            <Button type="button" variant="outline" className="h-10 w-full px-4 sm:w-auto" onClick={() => void loadDetail()} disabled={loading}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
          }
        >
          <div className="mb-3 flex items-center gap-3">
            <Button type="button" variant="outline" size="icon" className="size-11" asChild>
              <Link to="/users">
                <ArrowLeft className="size-4" />
              </Link>
            </Button>
            <div className="inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
              <UserRound className="size-5" />
            </div>
          </div>
        </AdminHeader>

        <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
          {[
            { label: "余额", value: numberText(stats.balance), icon: Coins, color: "text-violet-300" },
            { label: "已消耗", value: numberText(stats.spent), icon: Coins, color: "text-amber-300" },
            { label: "生成记录", value: numberText(stats.generations), icon: Activity, color: "text-[var(--app-text-primary)]" },
            { label: "图片", value: numberText(stats.images), icon: ImageIcon, color: "text-sky-300" },
            { label: "存储", value: formatBytes(stats.storageBytes), icon: Database, color: "text-emerald-300" },
            { label: "会话", value: numberText(stats.conversations), icon: Clock3, color: "text-[var(--app-text-muted)]" },
          ].map((item) => {
            return <AdminStatCard key={item.label} {...item} value={loading ? "-" : item.value} />;
          })}
        </section>

        <AdminPanel className="p-5">
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">基础信息</h2>
            <Badge variant={userStatusVariant(detail?.user.status)}>{statusText(detail?.user.status)}</Badge>
          </div>
          {loading ? (
            <LoadingState />
          ) : !detail ? (
            <EmptyState text="没有读取到用户详情" />
          ) : (
            <div className="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
              <InfoItem label="UID" value={String(detail.user.uid || "-")} />
              <InfoItem label="用户名" value={detail.user.username} />
              <InfoItem label="角色" value={roleText(detail.user.role)} />
              <InfoItem label="创建时间" value={formatDateTime(detail.user.created_at)} />
              <InfoItem label="更新时间" value={formatDateTime(detail.user.updated_at)} />
            </div>
          )}
        </AdminPanel>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_minmax(360px,0.8fr)]">
          <UsageTable
            items={detail?.recentUsage || []}
            loading={loading}
            page={usagePage}
            model={usageModel}
            timeRange={usageTimeRange}
            onModelChange={setUsageModel}
            onTimeRangeApply={setUsageTimeRange}
            onPageChange={(page) => void loadDetail({ usagePage: page, assetsPage: assetsPage.page, ledgerPage: ledgerPage.page })}
          />
          <CreditLedgerTable
            items={detail?.recentCreditLedger || []}
            loading={loading}
            page={ledgerPage}
            onPageChange={(page) => void loadDetail({ usagePage: usagePage.page, assetsPage: assetsPage.page, ledgerPage: page })}
          />
        </div>

        <AssetsTable
          items={detail?.recentAssets || []}
          loading={loading}
          page={assetsPage}
          onPageChange={(page) => void loadDetail({ usagePage: usagePage.page, assetsPage: page, ledgerPage: ledgerPage.page })}
        />
    </AdminPage>
  );
}

function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-[var(--app-text-muted)]">{label}</div>
      <div className="mt-1 break-all text-[var(--app-text-primary)]">{value || "-"}</div>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="py-10 text-center text-[var(--app-text-muted)]">
      <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
      读取中
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <div className="px-4 py-10 text-center text-sm text-[var(--app-text-muted)]">{text}</div>;
}

function UsageTable({
  items,
  loading,
  page,
  model,
  timeRange,
  onModelChange,
  onTimeRangeApply,
  onPageChange,
}: {
  items: BusinessUsageRecord[];
  loading: boolean;
  page: PaginationMeta;
  model: string;
  timeRange: TimeRangeValue;
  onModelChange: (value: string) => void;
  onTimeRangeApply: (value: TimeRangeValue) => void;
  onPageChange: (page: number) => void;
}) {
  return (
    <AdminPanel>
      <TableTitle title="最近使用记录" count={page.total} page={page} loading={loading} onPageChange={onPageChange} />
      <div className="grid gap-3 border-b border-[var(--app-border)] px-4 py-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
          模型
          <Input value={model} onChange={(event) => onModelChange(event.target.value)} className={adminInputPillClass} placeholder="全部模型" />
        </label>
        <div className="grid gap-1.5 text-xs text-[var(--app-text-muted)] lg:col-span-3">
          <span>时间范围</span>
          <TimeRangeFilter value={timeRange} onApply={onTimeRangeApply} />
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className={adminTableClass}>
          <thead className={adminTableHeadClass}>
            <tr>
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
              <TableLoading colSpan={7} />
            ) : items.length === 0 ? (
              <TableEmpty colSpan={7} text="暂无使用记录" />
            ) : (
              items.map((item) => (
                <tr key={item.id} className={adminTableRowClass}>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                  <td className="min-w-[260px] px-4 py-3 text-[var(--app-text-primary)]" title={item.prompt}>{promptPreview(item.prompt)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{item.model || "-"}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{numberText(item.count)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDuration(item.duration_ms)}</td>
                  <td className="whitespace-nowrap px-4 py-3 font-medium text-[var(--app-text-primary)]">{numberText(item.credits_used)}</td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <Badge variant={statusVariant(item.status)}>{generationStatusText(item.status)}</Badge>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </AdminPanel>
  );
}

function CreditLedgerTable({ items, loading, page, onPageChange }: { items: BusinessCreditLedgerEntry[]; loading: boolean; page: PaginationMeta; onPageChange: (page: number) => void }) {
  return (
    <AdminPanel>
      <TableTitle title="点数流水" count={page.total} page={page} loading={loading} onPageChange={onPageChange} />
      <div className="overflow-x-auto">
        <table className={adminTableClass}>
          <thead className={adminTableHeadClass}>
            <tr>
              <th className="px-4 py-3">时间</th>
              <th className="px-4 py-3">原因</th>
              <th className="px-4 py-3">变化</th>
              <th className="px-4 py-3">余额</th>
            </tr>
          </thead>
          <tbody className={adminTableBodyClass}>
            {loading ? (
              <TableLoading colSpan={4} />
            ) : items.length === 0 ? (
              <TableEmpty colSpan={4} text="暂无点数流水" />
            ) : (
              items.map((item) => (
                <tr key={item.id} className={adminTableRowClass}>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-primary)]">{reasonText(item.reason)}</td>
                  <td className={cn("whitespace-nowrap px-4 py-3 font-medium", item.delta >= 0 ? "text-emerald-600 dark:text-emerald-300" : "text-rose-600 dark:text-rose-300")}>
                    {item.delta >= 0 ? "+" : ""}
                    {numberText(item.delta)}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{numberText(item.balance_after)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </AdminPanel>
  );
}

function AssetsTable({ items, loading, page, onPageChange }: { items: BusinessUserDetail["recentAssets"]; loading: boolean; page: PaginationMeta; onPageChange: (page: number) => void }) {
  return (
    <AdminPanel>
      <TableTitle title="最近图片资产" count={page.total} page={page} loading={loading} onPageChange={onPageChange} />
      <div className="overflow-x-auto">
        <table className={adminTableClass}>
          <thead className={adminTableHeadClass}>
            <tr>
              <th className="px-4 py-3">文件</th>
              <th className="px-4 py-3">生成记录</th>
              <th className="px-4 py-3">会话</th>
              <th className="px-4 py-3">大小</th>
              <th className="px-4 py-3">类型</th>
              <th className="px-4 py-3">创建时间</th>
            </tr>
          </thead>
          <tbody className={adminTableBodyClass}>
            {loading ? (
              <TableLoading colSpan={6} />
            ) : items.length === 0 ? (
              <TableEmpty colSpan={6} text="暂无图片资产" />
            ) : (
              items.map((item) => (
                <tr key={item.id} className={adminTableRowClass}>
                  <td className="min-w-[280px] px-4 py-3">
                    <div className="max-w-[360px] truncate font-medium text-[var(--app-text-primary)]" title={item.file_name}>{item.file_name}</div>
                    <div className="mt-0.5 max-w-[360px] truncate text-xs text-[var(--app-text-muted)]" title={item.url}>{item.url}</div>
                  </td>
                  <td className="max-w-[220px] truncate px-4 py-3 font-mono text-xs text-[var(--app-text-secondary)]" title={item.generation_id}>{item.generation_id}</td>
                  <td className="max-w-[220px] truncate px-4 py-3 font-mono text-xs text-[var(--app-text-secondary)]" title={item.conversation_id}>{item.conversation_id}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatBytes(item.size_bytes)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{item.mime_type || "-"}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </AdminPanel>
  );
}

function TableTitle({ title, count, page, loading, onPageChange }: { title: string; count: number; page: PaginationMeta; loading: boolean; onPageChange: (page: number) => void }) {
  return (
    <div className="flex flex-col gap-3 border-b border-[var(--app-border)] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
      <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">{title}</h2>
      <div className="flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
        <Badge variant="secondary">{numberText(count)}</Badge>
        <span>{paginationText(page)}</span>
        <Button type="button" variant="outline" size="sm" disabled={loading || page.page <= 1} onClick={() => onPageChange(page.page - 1)}>上一页</Button>
        <Button type="button" variant="outline" size="sm" disabled={loading || page.page * page.pageSize >= page.total} onClick={() => onPageChange(page.page + 1)}>下一页</Button>
      </div>
    </div>
  );
}

function TableLoading({ colSpan }: { colSpan: number }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
        <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
        读取中
      </td>
    </tr>
  );
}

function TableEmpty({ colSpan, text }: { colSpan: number; text: string }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
        {text}
      </td>
    </tr>
  );
}
