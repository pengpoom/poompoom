"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Archive, Bell, Eye, LoaderCircle, RefreshCw, Send } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard, AdminToolbar } from "@/components/admin-layout";
import { AppDatePicker, AppModal, AppSelect } from "@/components/app-controls";
import {
  createBusinessNotification,
  deleteBusinessNotification,
  fetchAdminBusinessNotifications,
  updateBusinessNotification,
  type BusinessNotification,
  type BusinessNotificationLevel,
  type BusinessNotificationNotifyMode,
  type BusinessNotificationStatus,
  type BusinessNotificationTargeting,
} from "@/lib/api";
import "./notifications.css";

type StatusFilter = "all" | BusinessNotificationStatus;
type TargetMode = NonNullable<BusinessNotificationTargeting>["mode"];
type BalanceOperator = NonNullable<NonNullable<BusinessNotificationTargeting>["balance"]>["operator"];

const levelOptions: Array<{ value: BusinessNotificationLevel; label: string }> = [
  { value: "info", label: "普通" },
  { value: "success", label: "成功" },
  { value: "warning", label: "重要" },
];

const statusOptions: Array<{ value: BusinessNotificationStatus; label: string }> = [
  { value: "draft", label: "草稿" },
  { value: "published", label: "生效" },
  { value: "archived", label: "归档" },
];

const statusFilterOptions: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "全部状态" },
  ...statusOptions,
];

const notifyModeOptions: Array<{ value: BusinessNotificationNotifyMode; label: string }> = [
  { value: "silent", label: "静默" },
  { value: "popup", label: "弹窗" },
];

const targetModeOptions: Array<{ value: TargetMode; label: string }> = [
  { value: "all", label: "全体用户" },
  { value: "balance", label: "按余额条件" },
];

const balanceOperatorOptions: Array<{ value: BalanceOperator; label: string }> = [
  { value: ">=", label: "大于等于" },
  { value: ">", label: "大于" },
  { value: "<=", label: "小于等于" },
  { value: "<", label: "小于" },
  { value: "=", label: "等于" },
];

function levelLabel(value: string) {
  return levelOptions.find((item) => item.value === value)?.label || "普通";
}

function statusLabel(value: string) {
  return statusOptions.find((item) => item.value === value)?.label || value;
}

function notifyModeLabel(value?: string) {
  return notifyModeOptions.find((item) => item.value === value)?.label || "静默";
}

function levelBadgeClass(value: string) {
  if (value === "success") return "ok";
  if (value === "warning") return "warn";
  return "run";
}

function statusBadgeClass(value: string) {
  if (value === "published") return "ok";
  if (value === "archived") return "warn";
  return "off";
}

function formatTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function readPercent(item: BusinessNotification) {
  const total = Number(item.audienceCount || 0);
  if (total <= 0) return 0;
  return Math.min(100, Math.round((Number(item.readCount || 0) / total) * 100));
}

function readSummary(item: BusinessNotification) {
  const read = Number(item.readCount || 0);
  const total = Number(item.audienceCount || 0);
  return `${read.toLocaleString()} / ${total.toLocaleString()} (${readPercent(item)}%)`;
}

function toDateTimeLocal(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const offsetMs = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16);
}

function fromDateTimeLocal(value: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString();
}

function windowSummary(item: BusinessNotification) {
  const starts = item.startsAt ? formatTime(item.startsAt) : "立即";
  const ends = item.endsAt ? formatTime(item.endsAt) : "长期";
  return `${starts} ~ ${ends}`;
}

function windowState(item: BusinessNotification) {
  if (item.status !== "published") return statusLabel(item.status);
  const now = Date.now();
  const starts = item.startsAt ? new Date(item.startsAt).getTime() : 0;
  const ends = item.endsAt ? new Date(item.endsAt).getTime() : 0;
  if (starts && !Number.isNaN(starts) && starts > now) return "未开始";
  if (ends && !Number.isNaN(ends) && ends <= now) return "已结束";
  return "展示中";
}

function windowStateBadgeClass(state: string) {
  if (state === "展示中") return "ok";
  if (state === "未开始") return "run";
  return "off";
}

function targetSummary(item: BusinessNotification) {
  const targeting = item.targeting;
  if (!targeting || targeting.mode !== "balance" || !targeting.balance) return "全体用户";
  return `余额 ${targeting.balance.operator} ${Number(targeting.balance.value || 0).toLocaleString()}`;
}

function buildTargeting(mode: TargetMode, operator: BalanceOperator, value: string): BusinessNotificationTargeting {
  if (mode !== "balance") return { mode: "all" };
  const threshold = Number(value);
  return {
    mode: "balance",
    balance: {
      operator,
      value: Number.isFinite(threshold) ? Math.max(0, Math.floor(threshold)) : 0,
    },
  };
}

export default function NotificationsPage() {
  const [items, setItems] = useState<BusinessNotification[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<BusinessNotification | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BusinessNotification | null>(null);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [level, setLevel] = useState<BusinessNotificationLevel>("info");
  const [status, setStatus] = useState<BusinessNotificationStatus>("published");
  const [notifyMode, setNotifyMode] = useState<BusinessNotificationNotifyMode>("silent");
  const [startsAt, setStartsAt] = useState("");
  const [endsAt, setEndsAt] = useState("");
  const [targetMode, setTargetMode] = useState<TargetMode>("all");
  const [balanceOperator, setBalanceOperator] = useState<BalanceOperator>(">=");
  const [balanceValue, setBalanceValue] = useState("0");

  const publishedCount = useMemo(() => items.filter((item) => item.status === "published").length, [items]);
  const archivedCount = useMemo(() => items.filter((item) => item.status === "archived").length, [items]);
  const activeWindowCount = useMemo(() => items.filter((item) => windowState(item) === "展示中").length, [items]);

  const loadItems = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await fetchAdminBusinessNotifications({
        status: statusFilter,
        search: search.trim(),
        limit: 100,
      });
      setItems(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取公告失败");
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadItems();
    }, 180);
    return () => window.clearTimeout(timer);
  }, [loadItems]);

  const resetForm = () => {
    setEditingItem(null);
    setTitle("");
    setBody("");
    setLevel("info");
    setStatus("published");
    setNotifyMode("silent");
    setStartsAt("");
    setEndsAt("");
    setTargetMode("all");
    setBalanceOperator(">=");
    setBalanceValue("0");
  };

  const openCreateDialog = () => {
    resetForm();
    setDialogOpen(true);
  };

  const openEditDialog = (item: BusinessNotification) => {
    setEditingItem(item);
    setTitle(item.title);
    setBody(item.body);
    setLevel(item.level);
    setStatus(item.status);
    setNotifyMode(item.notifyMode || "silent");
    setStartsAt(toDateTimeLocal(item.startsAt));
    setEndsAt(toDateTimeLocal(item.endsAt));
    setTargetMode(item.targeting?.mode || "all");
    setBalanceOperator(item.targeting?.balance?.operator || ">=");
    setBalanceValue(String(item.targeting?.balance?.value ?? 0));
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    resetForm();
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = {
        title,
        body,
        level,
        status,
        audience: "all" as const,
        notifyMode,
        startsAt: fromDateTimeLocal(startsAt),
        endsAt: fromDateTimeLocal(endsAt),
        targeting: buildTargeting(targetMode, balanceOperator, balanceValue),
      };
      if (editingItem) {
        const response = await updateBusinessNotification(editingItem.id, payload);
        setItems((current) => current.map((item) => (item.id === response.item.id ? response.item : item)));
        toast.success("公告已更新");
      } else {
        const response = await createBusinessNotification(payload);
        setItems((current) => [response.item, ...current]);
        toast.success(status === "published" ? "公告已生效" : "公告已保存");
      }
      closeDialog();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存公告失败");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteBusinessNotification(deleteTarget.id);
      setItems((current) => current.filter((item) => item.id !== deleteTarget.id));
      toast.success("公告已删除");
      setDeleteTarget(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除公告失败");
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title="通知公告"
        description="创建公告并投放给全体用户，用户会在左侧消息通知里查看。"
        icon={Bell}
        actions={
          <>
            <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button className="app-btn-primary" type="button" onClick={openCreateDialog}>
              + 创建公告
            </button>
          </>
        }
      />

      <section className="app-stats">
        <AdminStatCard label="公告总数" value={items.length.toLocaleString()} icon={Bell} color="text-cyan-300" />
        <AdminStatCard label="已发布" value={publishedCount.toLocaleString()} icon={Send} color="text-emerald-300" />
        <AdminStatCard label="展示中" value={activeWindowCount.toLocaleString()} icon={Eye} color="text-sky-300" />
        <AdminStatCard label="已归档" value={archivedCount.toLocaleString()} icon={Archive} color="text-amber-300" />
      </section>

      <AdminToolbar>
        <AppSelect
          value={statusFilter}
          onChange={(value) => setStatusFilter(value as StatusFilter)}
          options={statusFilterOptions}
        />
        <input
          className="app-input"
          type="search"
          placeholder="搜索标题或内容…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          style={{ flex: 1 }}
        />
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle title="公告列表" action={<span className="panel-count">{items.length}</span>} />
        {loading ? (
          <div style={{ padding: "48px 16px", textAlign: "center", fontSize: 13, color: "var(--app-text-muted)" }}>
            <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
            <div>读取中</div>
          </div>
        ) : items.length === 0 ? (
          <div style={{ padding: "48px 16px", textAlign: "center", fontSize: 13, color: "var(--app-text-muted)" }}>暂无公告</div>
        ) : (
          <div className="notify-list">
            {items.map((item) => {
              const percent = readPercent(item);
              const state = windowState(item);
              return (
                <div key={item.id} className="notify-card">
                  <div className="notify-top">
                    <span className="badges">
                      <span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusLabel(item.status)}</span>
                      <span className={`app-badge ${levelBadgeClass(item.level)}`}>{levelLabel(item.level)}</span>
                      <span className={`app-badge ${item.notifyMode === "popup" ? "warn" : "off"}`}>{notifyModeLabel(item.notifyMode)}</span>
                      <span className={`app-badge ${windowStateBadgeClass(state)}`}>{state}</span>
                    </span>
                    <span className="title">{item.title}</span>
                    <div className="app-act">
                      <button type="button" onClick={() => openEditDialog(item)}>编辑</button>
                      <button type="button" className="danger" onClick={() => setDeleteTarget(item)}>删除</button>
                    </div>
                  </div>
                  <div className="notify-meta">
                    <span>发布 <b>{formatTime(item.publishedAt || item.createdAt)}</b></span>
                    <span>窗口 <b>{windowSummary(item)}</b></span>
                    <span>条件 <b>{targetSummary(item)}</b></span>
                    <span>已读 <b>{readSummary(item)}</b></span>
                  </div>
                  <div className="notify-progress"><i style={{ width: `${percent}%` }} /></div>
                  <div className="notify-preview">{item.body}</div>
                </div>
              );
            })}
          </div>
        )}
      </AdminPanel>

      <AppModal
        open={dialogOpen}
        onClose={closeDialog}
        title={editingItem ? "编辑公告" : "创建公告"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={closeDialog}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void handleSave()}
              disabled={saving || !title.trim() || !body.trim()}
            >
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              {editingItem ? "保存" : "创建"}
            </button>
          </>
        }
      >
        <div className="app-form-grid">
          <div className="app-fld full">
            <span className="fl">公告标题</span>
            <input
              className="app-input"
              placeholder="公告标题"
              maxLength={80}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">公告内容</span>
            <textarea
              className="app-textarea"
              placeholder="公告内容"
              maxLength={1200}
              value={body}
              onChange={(event) => setBody(event.target.value)}
            />
            <span className="fd" style={{ textAlign: "right" }}>{body.length}/1200</span>
          </div>
          <div className="app-fld">
            <span className="fl">级别</span>
            <AppSelect value={level} onChange={(v) => setLevel(v as BusinessNotificationLevel)} options={levelOptions} />
          </div>
          <div className="app-fld">
            <span className="fl">状态</span>
            <AppSelect value={status} onChange={(v) => setStatus(v as BusinessNotificationStatus)} options={statusOptions} />
          </div>
          <div className="app-fld">
            <span className="fl">通知方式</span>
            <AppSelect value={notifyMode} onChange={(v) => setNotifyMode(v as BusinessNotificationNotifyMode)} options={notifyModeOptions} />
          </div>
          <div className="app-fld">
            <span className="fl">展示条件</span>
            <AppSelect value={targetMode} onChange={(v) => setTargetMode(v as TargetMode)} options={targetModeOptions} />
          </div>
          {targetMode === "balance" ? (
            <>
              <div className="app-fld">
                <span className="fl">余额关系</span>
                <AppSelect value={balanceOperator} onChange={(v) => setBalanceOperator(v as BalanceOperator)} options={balanceOperatorOptions} />
              </div>
              <div className="app-fld">
                <span className="fl">余额阈值</span>
                <input
                  className="app-input"
                  type="number"
                  min={0}
                  step={1}
                  value={balanceValue}
                  onChange={(event) => setBalanceValue(event.target.value)}
                />
              </div>
            </>
          ) : null}
          <div className="app-fld">
            <span className="fl">开始时间</span>
            <AppDatePicker value={startsAt} onChange={setStartsAt} placeholder="选择开始时间" withTime />
            <span className="fd">留空表示立即生效</span>
          </div>
          <div className="app-fld">
            <span className="fl">结束时间</span>
            <AppDatePicker value={endsAt} onChange={setEndsAt} placeholder="选择结束时间" withTime />
            <span className="fd">留空表示长期有效</span>
          </div>
        </div>
      </AppModal>

      <AppModal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title="删除公告"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeleteTarget(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void handleDelete()}
              style={{ background: "linear-gradient(135deg, #ef4444, #dc2626)" }}
            >
              确认删除
            </button>
          </>
        }
      >
        <div style={{ padding: "16px 18px", fontSize: 13.5, color: "var(--app-text-secondary)", lineHeight: 1.7 }}>
          确认删除公告 <b style={{ color: "var(--app-text-primary)" }}>「{deleteTarget?.title}」</b> 吗？此操作不可恢复。
        </div>
      </AppModal>
    </AdminPage>
  );
}
