"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Archive, Bell, CheckCircle2, Eye, LoaderCircle, Pencil, Plus, RefreshCw, Search, Send, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard, AdminToolbar } from "@/components/admin-layout";
import { adminInputClass, adminInputPillClass, adminSubPanelClass } from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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
import { cn } from "@/lib/utils";

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

function levelVariant(value: string): "info" | "success" | "warning" {
  if (value === "success") {
    return "success";
  }
  if (value === "warning") {
    return "warning";
  }
  return "info";
}

function statusVariant(value: string): "success" | "secondary" | "warning" {
  if (value === "published") {
    return "success";
  }
  if (value === "archived") {
    return "warning";
  }
  return "secondary";
}

function formatTime(value?: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function readPercent(item: BusinessNotification) {
  const total = Number(item.audienceCount || 0);
  if (total <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((Number(item.readCount || 0) / total) * 100));
}

function readSummary(item: BusinessNotification) {
  const read = Number(item.readCount || 0);
  const total = Number(item.audienceCount || 0);
  return `已读 ${read.toLocaleString()} / 目标用户 ${total.toLocaleString()} (${readPercent(item)}%)`;
}

function toDateTimeLocal(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const offsetMs = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16);
}

function fromDateTimeLocal(value: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toISOString();
}

function windowSummary(item: BusinessNotification) {
  const starts = item.startsAt ? formatTime(item.startsAt) : "立即";
  const ends = item.endsAt ? formatTime(item.endsAt) : "长期";
  return `${starts} 至 ${ends}`;
}

function windowState(item: BusinessNotification) {
  if (item.status !== "published") {
    return statusLabel(item.status);
  }
  const now = Date.now();
  const starts = item.startsAt ? new Date(item.startsAt).getTime() : 0;
  const ends = item.endsAt ? new Date(item.endsAt).getTime() : 0;
  if (starts && !Number.isNaN(starts) && starts > now) {
    return "未开始";
  }
  if (ends && !Number.isNaN(ends) && ends <= now) {
    return "已结束";
  }
  return "展示中";
}

function targetSummary(item: BusinessNotification) {
  const targeting = item.targeting;
  if (!targeting || targeting.mode !== "balance" || !targeting.balance) {
    return "全体用户";
  }
  return `余额 ${targeting.balance.operator} ${Number(targeting.balance.value || 0).toLocaleString()}`;
}

function buildTargeting(mode: TargetMode, operator: BalanceOperator, value: string): BusinessNotificationTargeting {
  if (mode !== "balance") {
    return { mode: "all" };
  }
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
      setDialogOpen(false);
      resetForm();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存公告失败");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm("确定删除这条公告吗？")) {
      return;
    }
    try {
      await deleteBusinessNotification(id);
      setItems((current) => current.filter((item) => item.id !== id));
      toast.success("公告已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除公告失败");
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title="通知公告"
        description="创建公告并投放给全体用户，用户会在左侧消息通知里查看。"
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => void loadItems()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
            <Button type="button" onClick={openCreateDialog}>
              <Plus className="size-4" />
              创建公告
            </Button>
          </>
        }
      />

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <AdminStatCard label="公告总数" value={items.length.toLocaleString()} icon={Bell} color="text-[var(--app-text-primary)]" />
        <AdminStatCard label="已发布" value={publishedCount.toLocaleString()} icon={Send} color="text-emerald-300" />
        <AdminStatCard label="展示中" value={activeWindowCount.toLocaleString()} icon={Eye} color="text-sky-300" />
        <AdminStatCard label="已归档" value={archivedCount.toLocaleString()} icon={Archive} color="text-amber-300" />
      </section>

      <AdminToolbar>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="搜索标题或内容"
              className={cn(adminInputPillClass, "pl-9")}
            />
          </div>
          <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as StatusFilter)}>
            <SelectTrigger className={cn(adminInputPillClass, "w-full lg:w-40")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {statusFilterOptions.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle title="公告列表" action={<Badge variant="secondary">{items.length}</Badge>} />
        {loading ? (
          <div className="px-4 py-12 text-center text-sm text-[var(--app-text-muted)]">
            <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
            读取中
          </div>
        ) : items.length === 0 ? (
          <div className="px-4 py-12 text-center text-sm text-[var(--app-text-muted)]">暂无公告</div>
        ) : (
          <div className="divide-y divide-[var(--app-border)]">
            {items.map((item) => {
              const percent = readPercent(item);
              return (
                <article key={item.id} className="grid min-w-0 gap-3 px-4 py-4 sm:px-5">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant={statusVariant(item.status)}>{statusLabel(item.status)}</Badge>
                        <Badge variant={levelVariant(item.level)}>{levelLabel(item.level)}</Badge>
                        <Badge variant={item.notifyMode === "popup" ? "warning" : "outline"}>{notifyModeLabel(item.notifyMode)}</Badge>
                        <Badge variant={windowState(item) === "展示中" ? "success" : "outline"}>{windowState(item)}</Badge>
                        <h2 className="min-w-0 break-words text-base font-semibold text-[var(--app-text-primary)] [overflow-wrap:anywhere]">{item.title}</h2>
                      </div>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--app-text-muted)]">
                        <span>发布时间：{formatTime(item.publishedAt || item.createdAt)}</span>
                        <span className="text-[var(--app-border-strong)]">/</span>
                        <span>窗口：{windowSummary(item)}</span>
                        <span className="text-[var(--app-border-strong)]">/</span>
                        <span>条件：{targetSummary(item)}</span>
                        <span className="text-[var(--app-border-strong)]">/</span>
                        <span className="inline-flex items-center gap-1">
                          <Eye className="size-3.5" />
                          {readSummary(item)}
                        </span>
                      </div>
                    </div>
                    <div className="flex shrink-0 flex-wrap items-center gap-2">
                      <Button type="button" size="sm" variant="outline" onClick={() => openEditDialog(item)}>
                        <Pencil className="size-4" />
                        编辑
                      </Button>
                      <Button type="button" size="sm" variant="outline" onClick={() => void handleDelete(item.id)}>
                        <Trash2 className="size-4" />
                        删除
                      </Button>
                    </div>
                  </div>
                  <div className="grid gap-2">
                    <div className="h-1.5 overflow-hidden rounded-full bg-[var(--app-bg-surface-hover)]">
                      <div className="h-full rounded-full bg-[var(--app-accent-cyan)]" style={{ width: `${percent}%` }} />
                    </div>
                    <p className={cn(adminSubPanelClass, "max-h-32 min-w-0 overflow-y-auto overflow-x-hidden whitespace-pre-wrap break-words px-3 py-2 text-sm leading-6 text-[var(--app-text-secondary)] [overflow-wrap:anywhere]")}>{item.body}</p>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </AdminPanel>

      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            resetForm();
          }
        }}
      >
        <DialogContent className="max-h-[90vh] w-[min(92vw,760px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingItem ? "编辑公告" : "创建公告"}</DialogTitle>
            <DialogDescription>草稿不会出现在用户消息里；生效时间和展示条件会同时过滤。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <div>
              <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">标题</label>
              <Input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="公告标题"
                maxLength={80}
                className={cn(adminInputClass, "rounded-[var(--app-radius-md)]")}
              />
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">内容</label>
              <Textarea
                value={body}
                onChange={(event) => setBody(event.target.value)}
                placeholder="公告内容"
                maxLength={1200}
                className={cn(adminInputClass, "min-h-44 resize-y rounded-[var(--app-radius-md)] whitespace-pre-wrap break-words [overflow-wrap:anywhere]")}
              />
              <div className="mt-1 text-right text-xs text-[var(--app-text-muted)]">{body.length}/1200</div>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">类型</label>
                <Select value={level} onValueChange={(value) => setLevel(value as BusinessNotificationLevel)}>
                  <SelectTrigger className="h-11 rounded-[var(--app-radius-md)]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {levelOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">状态</label>
                <Select value={status} onValueChange={(value) => setStatus(value as BusinessNotificationStatus)}>
                  <SelectTrigger className="h-11 rounded-[var(--app-radius-md)]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {statusOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">通知方式</label>
                <Select value={notifyMode} onValueChange={(value) => setNotifyMode(value as BusinessNotificationNotifyMode)}>
                  <SelectTrigger className="h-11 rounded-[var(--app-radius-md)]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {notifyModeOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">开始时间</label>
                <Input
                  type="datetime-local"
                  value={startsAt}
                  onChange={(event) => setStartsAt(event.target.value)}
                  className={cn(adminInputClass, "rounded-[var(--app-radius-md)]")}
                />
                <p className="mt-1 text-xs text-[var(--app-text-muted)]">留空表示立即生效。</p>
              </div>
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">结束时间</label>
                <Input
                  type="datetime-local"
                  value={endsAt}
                  onChange={(event) => setEndsAt(event.target.value)}
                  className={cn(adminInputClass, "rounded-[var(--app-radius-md)]")}
                />
                <p className="mt-1 text-xs text-[var(--app-text-muted)]">留空表示长期有效。</p>
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">展示条件</label>
                <Select value={targetMode} onValueChange={(value) => setTargetMode(value as TargetMode)}>
                  <SelectTrigger className="h-11 rounded-[var(--app-radius-md)]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {targetModeOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {targetMode === "balance" ? (
                <>
                  <div>
                    <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">余额关系</label>
                    <Select value={balanceOperator} onValueChange={(value) => setBalanceOperator(value as BalanceOperator)}>
                      <SelectTrigger className="h-11 rounded-[var(--app-radius-md)]">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {balanceOperatorOptions.map((item) => (
                          <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <label className="mb-2 block text-sm font-medium text-[var(--app-text-secondary)]">余额阈值</label>
                    <Input
                      type="number"
                      min={0}
                      step={1}
                      value={balanceValue}
                      onChange={(event) => setBalanceValue(event.target.value)}
                      className={cn(adminInputClass, "rounded-[var(--app-radius-md)]")}
                    />
                  </div>
                </>
              ) : null}
              <div className="md:col-span-3">
                <p className="text-xs text-[var(--app-text-muted)]">
                  当前支持全体用户或按余额阈值筛选，后续可以在同一结构里继续扩展更多条件。
                </p>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button type="button" onClick={() => void handleSave()} disabled={saving || !title.trim() || !body.trim()}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : editingItem ? <CheckCircle2 className="size-4" /> : <Send className="size-4" />}
              {editingItem ? "保存" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
