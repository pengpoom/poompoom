"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, Clock3, Coins, Database, ImageIcon, LoaderCircle } from "lucide-react";
import { toast } from "sonner";

import { TimeRangeFilter } from "@/components/time-range-filter";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import { fetchBusinessUserDetail, type BusinessCreditLedgerEntry, type BusinessUsageRecord, type BusinessUserDetail, type PaginationMeta } from "@/lib/api";

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
  if (value <= 0) return "-";
  if (value < 1000) return `${value} ms`;
  const seconds = value / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds >= 10 ? 1 : 2)} 秒`;
  const minutes = Math.floor(seconds / 60);
  const remain = Math.round(seconds % 60);
  return `${minutes} 分 ${remain} 秒`;
}

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) return `${bytes} B`;
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

function userStatusBadge(status: string | undefined) {
  return status === "active" ? "ok" : status === "deleted" ? "fail" : "warn";
}

function generationStatusBadge(status: string) {
  return status === "succeeded" ? "ok" : status === "failed" ? "fail" : "off";
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
  if (!raw) return "-";
  return raw.length > 64 ? `${raw.slice(0, 64)}...` : raw;
}

function defaultPage(pageSize = 20): PaginationMeta {
  return { page: 1, pageSize, total: 0 };
}

function paginationText(page: PaginationMeta | undefined) {
  if (!page || !page.total) return "0 / 0";
  const start = (page.page - 1) * page.pageSize + 1;
  const end = Math.min(page.page * page.pageSize, page.total);
  return `${start}-${end} / ${page.total}`;
}

export function UserDetailDrawerTitle({ userID, detail, loading }: { userID: string; detail: BusinessUserDetail | null; loading: boolean }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4, minWidth: 0 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
        <div style={{ fontSize: 16, fontWeight: 700, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {detail?.user.username || (loading ? "加载中…" : "用户详情")}
        </div>
        {detail ? (
          <span className={`app-badge ${userStatusBadge(detail.user.status)}`}>{statusText(detail.user.status)}</span>
        ) : null}
        {detail ? (
          <span style={{ fontSize: 12, color: "var(--app-text-muted)" }}>{roleText(detail.user.role)}</span>
        ) : null}
      </div>
      <div style={{ fontSize: 12, color: "var(--app-text-muted)", fontFamily: "var(--font-mono, ui-monospace, SFMono-Regular, monospace)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
        {userID || "—"}
      </div>
    </div>
  );
}

type StatTone = "mono" | "ok" | "sky" | "amber" | "violet" | "fail";

function Stat({ label, value, tone, Icon, loading }: { label: string; value: string; tone: StatTone; Icon: typeof Coins; loading: boolean }) {
  return (
    <div className="app-stat">
      <div className={`app-stat-ic ${tone}`}>
        <Icon className="size-4" />
      </div>
      <div style={{ minWidth: 0, flex: 1 }}>
        <b>{loading ? "-" : value}</b>
        <small>{label}</small>
      </div>
    </div>
  );
}

export function UserDetailDrawerContent({
  userID,
  onLoaded,
  onReady,
}: {
  userID: string;
  onLoaded?: (detail: BusinessUserDetail | null, loading: boolean) => void;
  onReady?: (api: { reload: () => void }) => void;
}) {
  const [detail, setDetail] = useState<BusinessUserDetail | null>(null);
  const [usagePage, setUsagePage] = useState(defaultPage());
  const [assetsPage, setAssetsPage] = useState(defaultPage());
  const [ledgerPage, setLedgerPage] = useState(defaultPage());
  const [usageModel, setUsageModel] = useState("");
  const [usageTimeRange, setUsageTimeRange] = useState<TimeRangeValue>({ preset: "last24h", from: "", to: "" });
  const [loading, setLoading] = useState(true);
  const pagesRef = useRef({ usagePage, assetsPage, ledgerPage });

  useEffect(() => {
    pagesRef.current = { usagePage, assetsPage, ledgerPage };
  }, [assetsPage, ledgerPage, usagePage]);

  useEffect(() => {
    onLoaded?.(detail, loading);
  }, [detail, loading, onLoaded]);

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

  const loadDetailRef = useRef(loadDetail);
  useEffect(() => {
    loadDetailRef.current = loadDetail;
  }, [loadDetail]);

  const reload = useCallback(() => {
    void loadDetailRef.current();
  }, []);

  useEffect(() => {
    onReady?.({ reload });
  }, [onReady, reload]);

  const stats = useMemo(() => ({
    balance: detail?.credit.balance || 0,
    spent: detail?.credit.spent || 0,
    generations: detail?.usage.generation_count || 0,
    images: detail?.usage.image_count || 0,
    storageBytes: detail?.usage.storage_bytes || 0,
    conversations: detail?.conversationCount || 0,
  }), [detail]);

  return (
    <>
      <section style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 12 }}>
        <Stat label="余额" value={numberText(stats.balance)} tone="violet" Icon={Coins} loading={loading} />
        <Stat label="已消耗" value={numberText(stats.spent)} tone="amber" Icon={Coins} loading={loading} />
        <Stat label="生成记录" value={numberText(stats.generations)} tone="mono" Icon={Activity} loading={loading} />
        <Stat label="图片" value={numberText(stats.images)} tone="sky" Icon={ImageIcon} loading={loading} />
        <Stat label="存储" value={formatBytes(stats.storageBytes)} tone="ok" Icon={Database} loading={loading} />
        <Stat label="会话" value={numberText(stats.conversations)} tone="mono" Icon={Clock3} loading={loading} />
      </section>

      <div className="app-panel">
        <div className="panel-title">
          <h3>基础信息</h3>
        </div>
        <div style={{ padding: 18 }}>
          {loading ? (
            <LoadingState />
          ) : !detail ? (
            <EmptyState text="没有读取到用户详情" />
          ) : (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 14, fontSize: 13 }}>
              <InfoItem label="UID" value={String(detail.user.uid || "-")} />
              <InfoItem label="用户名" value={detail.user.username} />
              <InfoItem label="角色" value={roleText(detail.user.role)} />
              <InfoItem label="状态" value={statusText(detail.user.status)} />
              <InfoItem label="创建时间" value={formatDateTime(detail.user.created_at)} />
              <InfoItem label="更新时间" value={formatDateTime(detail.user.updated_at)} />
            </div>
          )}
        </div>
      </div>

      <UsageTable
        items={detail?.recentUsage || []}
        loading={loading}
        page={usagePage}
        model={usageModel}
        timeRange={usageTimeRange}
        onModelChange={setUsageModel}
        onTimeRangeApply={setUsageTimeRange}
        onPageChange={(page) => void loadDetail({ usagePage: page })}
      />

      <CreditLedgerTable
        items={detail?.recentCreditLedger || []}
        loading={loading}
        page={ledgerPage}
        onPageChange={(page) => void loadDetail({ ledgerPage: page })}
      />

      <AssetsTable
        items={detail?.recentAssets || []}
        loading={loading}
        page={assetsPage}
        onPageChange={(page) => void loadDetail({ assetsPage: page })}
      />
    </>
  );
}

function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div style={{ fontSize: 11, color: "var(--app-text-muted)" }}>{label}</div>
      <div style={{ marginTop: 4, color: "var(--app-text-primary)", wordBreak: "break-all" }}>{value || "-"}</div>
    </div>
  );
}

function LoadingState() {
  return (
    <div style={{ padding: "40px 0", textAlign: "center", color: "var(--app-text-muted)", fontSize: 13 }}>
      <LoaderCircle className="size-4 animate-spin" style={{ display: "inline-block", marginRight: 6, verticalAlign: -3 }} />
      读取中
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <div style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)", fontSize: 13 }}>{text}</div>;
}

function UsageTable({ items, loading, page, model, timeRange, onModelChange, onTimeRangeApply, onPageChange }: {
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
    <div className="app-panel">
      <TableTitle title="最近使用记录" count={page.total} />
      <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) minmax(0, 2fr)", gap: 14, padding: 16, borderBottom: "1px solid var(--app-border)" }}>
        <label className="app-fld">
          <span className="fl">模型</span>
          <input className="app-input" value={model} onChange={(event) => onModelChange(event.target.value)} placeholder="全部模型" />
        </label>
        <div className="app-fld" style={{ minWidth: 0 }}>
          <span className="fl">时间范围</span>
          <TimeRangeFilter value={timeRange} onApply={onTimeRangeApply} />
        </div>
      </div>
      <div className="app-table-wrap">
        <table className="app-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>提示词</th>
              <th>模型</th>
              <th>图片</th>
              <th>耗时</th>
              <th>扣点</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {loading ? <TableLoading colSpan={7} /> : items.length === 0 ? <TableEmpty colSpan={7} text="暂无使用记录" /> : items.map((item) => (
              <tr key={item.id}>
                <td style={{ whiteSpace: "nowrap" }}>{formatDateTime(item.created_at)}</td>
                <td className="prompt" title={item.prompt}>{promptPreview(item.prompt)}</td>
                <td style={{ whiteSpace: "nowrap" }}>{item.model || "-"}</td>
                <td style={{ whiteSpace: "nowrap" }}>{numberText(item.count)}</td>
                <td style={{ whiteSpace: "nowrap" }}>{formatDuration(item.duration_ms)}</td>
                <td className="strong" style={{ whiteSpace: "nowrap" }}>{numberText(item.credits_used)}</td>
                <td style={{ whiteSpace: "nowrap" }}>
                  <span className={`app-badge ${generationStatusBadge(item.status)}`}>{generationStatusText(item.status)}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <TablePager page={page} loading={loading} onPageChange={onPageChange} />
    </div>
  );
}

function CreditLedgerTable({ items, loading, page, onPageChange }: { items: BusinessCreditLedgerEntry[]; loading: boolean; page: PaginationMeta; onPageChange: (page: number) => void }) {
  return (
    <div className="app-panel">
      <TableTitle title="点数流水" count={page.total} />
      <div className="app-table-wrap">
        <table className="app-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>原因</th>
              <th>变化</th>
              <th>余额</th>
            </tr>
          </thead>
          <tbody>
            {loading ? <TableLoading colSpan={4} /> : items.length === 0 ? <TableEmpty colSpan={4} text="暂无点数流水" /> : items.map((item) => (
              <tr key={item.id}>
                <td style={{ whiteSpace: "nowrap" }}>{formatDateTime(item.created_at)}</td>
                <td style={{ whiteSpace: "nowrap" }}>{reasonText(item.reason)}</td>
                <td style={{ whiteSpace: "nowrap" }}>
                  <span className={`app-badge ${item.delta >= 0 ? "ok" : "fail"}`}>{item.delta >= 0 ? "+" : ""}{numberText(item.delta)}</span>
                </td>
                <td className="strong" style={{ whiteSpace: "nowrap" }}>{numberText(item.balance_after)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <TablePager page={page} loading={loading} onPageChange={onPageChange} />
    </div>
  );
}

function AssetsTable({ items, loading, page, onPageChange }: { items: BusinessUserDetail["recentAssets"]; loading: boolean; page: PaginationMeta; onPageChange: (page: number) => void }) {
  return (
    <div className="app-panel">
      <TableTitle title="最近图片资产" count={page.total} />
      <div className="app-table-wrap">
        <table className="app-table">
          <thead>
            <tr>
              <th>文件</th>
              <th>生成记录</th>
              <th>会话</th>
              <th>大小</th>
              <th>类型</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            {loading ? <TableLoading colSpan={6} /> : items.length === 0 ? <TableEmpty colSpan={6} text="暂无图片资产" /> : items.map((item) => (
              <tr key={item.id}>
                <td style={{ minWidth: 240 }}>
                  <div className="app-user-meta">
                    <b style={{ maxWidth: 320, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={item.file_name}>{item.file_name}</b>
                    <small style={{ display: "block", maxWidth: 320, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={item.url}>{item.url}</small>
                  </div>
                </td>
                <td style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: "var(--font-mono)", fontSize: 11 }} title={item.generation_id}>{item.generation_id}</td>
                <td style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: "var(--font-mono)", fontSize: 11 }} title={item.conversation_id}>{item.conversation_id}</td>
                <td style={{ whiteSpace: "nowrap" }}>{formatBytes(item.size_bytes)}</td>
                <td style={{ whiteSpace: "nowrap" }}>{item.mime_type || "-"}</td>
                <td style={{ whiteSpace: "nowrap" }}>{formatDateTime(item.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <TablePager page={page} loading={loading} onPageChange={onPageChange} />
    </div>
  );
}

function TableTitle({ title, count }: { title: string; count: number }) {
  return (
    <div className="panel-title">
      <h3>{title}</h3>
      <span className="app-badge off">{numberText(count)}</span>
    </div>
  );
}

function TablePager({ page, loading, onPageChange }: { page: PaginationMeta; loading: boolean; onPageChange: (page: number) => void }) {
  return (
    <div className="app-pager">
      <span className="info">{paginationText(page)}</span>
      <div className="pages">
        <button type="button" disabled={loading || page.page <= 1} onClick={() => onPageChange(page.page - 1)}>上一页</button>
        <button type="button" disabled={loading || page.page * page.pageSize >= page.total} onClick={() => onPageChange(page.page + 1)}>下一页</button>
      </div>
    </div>
  );
}

function TableLoading({ colSpan }: { colSpan: number }) {
  return (
    <tr>
      <td colSpan={colSpan} style={{ padding: "40px 0", textAlign: "center", color: "var(--app-text-muted)" }}>
        <LoaderCircle className="size-4 animate-spin" style={{ display: "inline-block", marginRight: 6, verticalAlign: -3 }} />
        读取中
      </td>
    </tr>
  );
}

function TableEmpty({ colSpan, text }: { colSpan: number; text: string }) {
  return (
    <tr>
      <td colSpan={colSpan} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>{text}</td>
    </tr>
  );
}
