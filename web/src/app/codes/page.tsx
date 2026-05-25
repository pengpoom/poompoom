"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Eye, Gift, KeyRound, LoaderCircle, Pencil, Plus, RefreshCw, Search, Ticket, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard, AdminToolbar } from "@/components/admin-layout";
import { adminInputClass, adminInputPillClass, adminTableBodyClass, adminTableClass, adminTableHeadClass, adminTableRowClass } from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
  createBusinessCode,
  deleteBusinessCode,
  fetchBusinessCodeUsages,
  fetchBusinessCodes,
  updateBusinessCodeStatusBatch,
  updateBusinessCode,
  type BusinessCode,
  type BusinessCodeStatus,
  type BusinessCodeType,
  type BusinessCodeUsage,
} from "@/lib/api";
import { cn } from "@/lib/utils";

type TypeFilter = "all" | BusinessCodeType;
type StatusFilter = "all" | BusinessCodeStatus;
type CodeView = "registration" | "redeem";

const typeOptions: Array<{ value: BusinessCodeType; label: string; hint: string }> = [
  { value: "invite", label: "邀请码", hint: "只提供注册资格，不送点数。" },
  { value: "promo", label: "优惠码", hint: "提供注册资格，并在注册时赠送点数。" },
  { value: "redeem", label: "兑换码", hint: "已登录用户在积分中心兑换点数。" },
];

const statusOptions: Array<{ value: BusinessCodeStatus; label: string }> = [
  { value: "active", label: "启用" },
  { value: "disabled", label: "停用" },
  { value: "expired", label: "过期" },
];

const filterTypeOptions: Array<{ value: TypeFilter; label: string }> = [
  { value: "all", label: "全部类型" },
  ...typeOptions.map((item) => ({ value: item.value, label: item.label })),
];

const filterStatusOptions: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "全部状态" },
  ...statusOptions,
];

function typeLabel(value: string) {
  return typeOptions.find((item) => item.value === value)?.label || value;
}

function statusLabel(value: string) {
  return statusOptions.find((item) => item.value === value)?.label || value;
}

function statusVariant(value: string): "success" | "secondary" | "warning" {
  if (value === "active") {
    return "success";
  }
  if (value === "expired") {
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

function isLegacyPreviewCode(value?: string) {
  return Boolean(value && value.includes("..."));
}

function contextLabel(value: string) {
  if (value === "registration") return "注册";
  if (value === "recharge_center") return "兑换";
  return value || "-";
}

export default function CodesPage() {
  const [items, setItems] = useState<BusinessCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [view, setView] = useState<CodeView>("registration");
  const [typeFilter, setTypeFilter] = useState<TypeFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<BusinessCode | null>(null);
  const [createdCode, setCreatedCode] = useState("");
  const [codeType, setCodeType] = useState<BusinessCodeType>("invite");
  const [rawCode, setRawCode] = useState("");
  const [title, setTitle] = useState("");
  const [credits, setCredits] = useState("0");
  const [maxUses, setMaxUses] = useState("1");
  const [status, setStatus] = useState<BusinessCodeStatus>("active");
  const [startsAt, setStartsAt] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [note, setNote] = useState("");
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [usageDialogCode, setUsageDialogCode] = useState<BusinessCode | null>(null);
  const [usageItems, setUsageItems] = useState<BusinessCodeUsage[]>([]);
  const [usageLoading, setUsageLoading] = useState(false);

  const loadItems = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessCodes({
        type: view === "redeem" ? "redeem" : typeFilter === "all" ? "registration" : typeFilter === "redeem" ? "registration" : typeFilter,
        status: statusFilter,
        search: search.trim() || undefined,
      });
      const nextItems = payload.items || [];
      setItems(nextItems);
      setSelectedIDs((current) => current.filter((id) => nextItems.some((item) => item.id === id)));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取码列表失败");
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, typeFilter, view]);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const stats = useMemo(() => {
    return items.reduce(
      (acc, item) => {
        acc.total += 1;
        if (item.type === "redeem") acc.redeem += 1;
        if (item.type === "promo") acc.promo += 1;
        if (item.type === "invite") acc.invite += 1;
        acc.used += Number(item.usedCount || 0);
        return acc;
      },
      { total: 0, redeem: 0, promo: 0, invite: 0, used: 0 },
    );
  }, [items]);
  const dialogTypeOptions = view === "redeem"
    ? typeOptions.filter((item) => item.value === "redeem")
    : typeOptions.filter((item) => item.value !== "redeem");
  const allSelected = items.length > 0 && selectedIDs.length === items.length;
  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  const resetForm = () => {
    setEditingItem(null);
    setCreatedCode("");
    setCodeType("invite");
    setRawCode("");
    setTitle("");
    setCredits("0");
    setMaxUses("1");
    setStatus("active");
    setStartsAt("");
    setExpiresAt("");
    setNote("");
  };

  const openCreateDialog = (type: BusinessCodeType = "invite") => {
    resetForm();
    setCodeType(type);
    setCredits(type === "redeem" ? "10" : "0");
    setDialogOpen(true);
  };

  const switchView = (nextView: CodeView) => {
    setView(nextView);
    setTypeFilter(nextView === "redeem" ? "redeem" : "all");
    setSelectedIDs([]);
  };

  const openEditDialog = (item: BusinessCode) => {
    setEditingItem(item);
    setCreatedCode("");
    setCodeType(item.type);
    setRawCode("");
    setTitle(item.title);
    setCredits(String(item.credits || 0));
    setMaxUses(String(item.maxUses || 1));
    setStatus(item.status);
    setStartsAt(toDateTimeLocal(item.startsAt));
    setExpiresAt(toDateTimeLocal(item.expiresAt));
    setNote(item.note || "");
    setDialogOpen(true);
  };

  const saveCode = async () => {
    const normalizedCredits = Math.max(0, Math.floor(Number(credits) || 0));
    const normalizedMaxUses = Math.max(1, Math.floor(Number(maxUses) || 1));
    if (codeType === "invite" && normalizedCredits > 0) {
      toast.error("邀请码不赠送点数");
      return;
    }
    if (codeType === "redeem" && normalizedCredits <= 0) {
      toast.error("兑换码点数必须大于 0");
      return;
    }
    setSaving(true);
    try {
      const payload = {
        title: title.trim() || typeLabel(codeType),
        credits: normalizedCredits,
        maxUses: normalizedMaxUses,
        status,
        startsAt: fromDateTimeLocal(startsAt),
        expiresAt: fromDateTimeLocal(expiresAt),
        note: note.trim(),
      };
      if (editingItem) {
        const response = await updateBusinessCode(editingItem.id, payload);
        setItems((current) => current.map((item) => (item.id === editingItem.id ? response.item : item)));
        setDialogOpen(false);
        toast.success("码已更新");
      } else {
        const response = await createBusinessCode({
          ...payload,
          type: codeType,
          code: rawCode.trim() || undefined,
        });
        setItems((current) => [response.item, ...current]);
        setCreatedCode(response.code || "");
        toast.success("码已创建");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const removeCode = async (id: string) => {
    setSaving(true);
    try {
      await deleteBusinessCode(id);
      setItems((current) => current.filter((item) => item.id !== id));
      toast.success("已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    } finally {
      setSaving(false);
    }
  };

  const copyCode = async (code: string) => {
    if (!code) {
      return;
    }
    await navigator.clipboard.writeText(code);
    toast.success("已复制");
  };

  const toggleSelected = (id: string) => {
    setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  };

  const toggleSelectAll = () => {
    setSelectedIDs(allSelected ? [] : items.map((item) => item.id));
  };

  const batchUpdateStatus = async (nextStatus: BusinessCodeStatus) => {
    if (selectedIDs.length === 0) {
      toast.error("请先选择码");
      return;
    }
    setSaving(true);
    try {
      const payload = await updateBusinessCodeStatusBatch(selectedIDs, nextStatus);
      toast.success(`已更新 ${Number(payload.updated || 0).toLocaleString()} 个码`);
      setSelectedIDs([]);
      await loadItems();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "批量更新失败");
    } finally {
      setSaving(false);
    }
  };

  const openUsageDialog = async (item: BusinessCode) => {
    setUsageDialogCode(item);
    setUsageItems([]);
    setUsageLoading(true);
    try {
      const payload = await fetchBusinessCodeUsages(item.id);
      setUsageItems(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取使用记录失败");
    } finally {
      setUsageLoading(false);
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title={view === "redeem" ? "兑换码管理" : "注册码管理"}
        description={view === "redeem" ? "管理用户在积分中心兑换余额的兑换码。" : "管理注册页可用的邀请码和优惠码。"}
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => void loadItems()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
            <Button type="button" onClick={() => openCreateDialog(view === "redeem" ? "redeem" : "invite")}>
              <Plus className="size-4" />
              新建
            </Button>
          </>
        }
      >
        <div className="mb-3 inline-flex size-10 items-center justify-center rounded-[var(--app-radius-md)] border border-white/10 bg-white/[0.045] text-[var(--app-text-primary)]">
          <Ticket className="size-5" />
        </div>
      </AdminHeader>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <AdminStatCard label="总数" value={stats.total.toLocaleString()} sub={`已使用 ${stats.used.toLocaleString()} 次`} icon={Ticket} color="text-[var(--app-accent-cyan)]" />
        <AdminStatCard label="邀请码" value={stats.invite.toLocaleString()} sub="注册资格" icon={KeyRound} color="text-violet-300" />
        <AdminStatCard label="优惠码" value={stats.promo.toLocaleString()} sub="注册资格 + 赠点" icon={Gift} color="text-emerald-300" />
        <AdminStatCard label="兑换码" value={stats.redeem.toLocaleString()} sub="用户余额兑换" icon={Ticket} color="text-amber-300" />
      </div>

      <AdminToolbar className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={view === "registration" ? "default" : "outline"}
            className="h-9 px-3 text-[13px]"
            onClick={() => switchView("registration")}
          >
            <KeyRound className="size-4" />
            注册码
          </Button>
          <Button
            type="button"
            variant={view === "redeem" ? "default" : "outline"}
            className="h-9 px-3 text-[13px]"
            onClick={() => switchView("redeem")}
          >
            <Ticket className="size-4" />
            兑换码
          </Button>
          <Select value={typeFilter} onValueChange={(value) => setTypeFilter(value as TypeFilter)}>
            <SelectTrigger className={cn(adminInputPillClass, "w-[150px]")} disabled={view === "redeem"}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(view === "redeem" ? filterTypeOptions.filter((item) => item.value === "redeem") : filterTypeOptions.filter((item) => item.value !== "redeem")).map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as StatusFilter)}>
            <SelectTrigger className={cn(adminInputPillClass, "w-[150px]")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {filterStatusOptions.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <label className="relative min-w-0 flex-1 lg:max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="搜索标题、备注或预览码"
            className={cn(adminInputPillClass, "pl-9")}
          />
        </label>
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle
          title="码列表"
          action={
            selectedIDs.length > 0 ? (
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">已选 {selectedIDs.length}</Badge>
                <Button type="button" size="sm" variant="outline" onClick={() => void batchUpdateStatus("active")} disabled={saving}>批量启用</Button>
                <Button type="button" size="sm" variant="outline" onClick={() => void batchUpdateStatus("disabled")} disabled={saving}>批量停用</Button>
              </div>
            ) : null
          }
        />
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-[var(--app-text-muted)]">
            <LoaderCircle className="size-4 animate-spin" />
            正在读取
          </div>
        ) : items.length === 0 ? (
          <div className="p-8 text-center text-sm text-[var(--app-text-muted)]">暂无记录</div>
        ) : (
          <div className="overflow-x-auto">
            <table className={cn(adminTableClass, "w-full min-w-[1120px]")}>
              <thead className={adminTableHeadClass}>
                <tr>
                  <th className="w-10 px-4 py-3">
                    <Checkbox checked={allSelected} onCheckedChange={toggleSelectAll} aria-label="全选码" />
                  </th>
                  <th className="px-4 py-3">码</th>
                  <th className="px-4 py-3">类型</th>
                  <th className="px-4 py-3">点数</th>
                  <th className="px-4 py-3">使用</th>
                  <th className="px-4 py-3">有效期</th>
                  <th className="px-4 py-3">备注</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className={adminTableBodyClass}>
                {items.map((item) => {
                  const legacyPreview = isLegacyPreviewCode(item.codePreview);
                  return (
                    <tr key={item.id} className={adminTableRowClass}>
                      <td className="px-4 py-3">
                        <Checkbox checked={selectedSet.has(item.id)} onCheckedChange={() => toggleSelected(item.id)} aria-label={`选择 ${item.title}`} />
                      </td>
                      <td className="min-w-[220px] px-4 py-3">
                        <div className="flex items-center gap-2">
                          <code className="max-w-[220px] truncate rounded-[var(--app-radius-sm)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-2 py-1 text-xs text-[var(--app-text-primary)]">
                            {item.codePreview}
                          </code>
                          {legacyPreview ? <Badge variant="secondary">旧预览</Badge> : null}
                          <Button type="button" size="icon" variant="ghost" onClick={() => void copyCode(item.codePreview)} disabled={legacyPreview} aria-label="复制码">
                            <Copy className="size-4" />
                          </Button>
                        </div>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Badge variant={item.type === "redeem" ? "warning" : item.type === "promo" ? "success" : "info"}>{typeLabel(item.type)}</Badge>
                          <Badge variant={statusVariant(item.status)}>{statusLabel(item.status)}</Badge>
                        </div>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{Number(item.credits || 0).toLocaleString()}</td>
                      <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">
                        {item.usedCount.toLocaleString()} / {item.maxUses.toLocaleString()}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-xs leading-5 text-[var(--app-text-muted)]">
                        <div>{formatTime(item.startsAt)}</div>
                        <div>{formatTime(item.expiresAt)}</div>
                      </td>
                      <td className="max-w-[180px] truncate px-4 py-3 text-[var(--app-text-muted)]" title={item.note || ""}>{item.note || "-"}</td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-1.5">
                          <Button type="button" size="icon" variant="ghost" onClick={() => void openUsageDialog(item)} aria-label="使用记录">
                            <Eye className="size-4" />
                          </Button>
                          <Button type="button" size="icon" variant="ghost" onClick={() => openEditDialog(item)} aria-label="编辑">
                            <Pencil className="size-4" />
                          </Button>
                          <Button type="button" size="icon" variant="ghost" onClick={() => void removeCode(item.id)} disabled={saving} aria-label="删除">
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </AdminPanel>

      <Dialog open={dialogOpen} onOpenChange={(open) => {
        setDialogOpen(open);
        if (!open) resetForm();
      }}>
        <DialogContent className="max-h-[90vh] w-[min(92vw,720px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingItem ? "编辑码" : "新建码"}</DialogTitle>
            <DialogDescription>
              邀请码和优惠码用于注册页；兑换码用于用户积分中心。
            </DialogDescription>
          </DialogHeader>

          {createdCode ? (
            <div className="rounded-[var(--app-radius-md)] border border-emerald-400/25 bg-emerald-400/10 p-4">
              <div className="text-sm font-semibold text-emerald-100">新码已创建</div>
              <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                <Input value={createdCode} readOnly className={adminInputClass} />
                <Button type="button" onClick={() => void copyCode(createdCode)}>
                  <Copy className="size-4" />
                  复制
                </Button>
              </div>
            </div>
          ) : null}

          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">类型</div>
              <Select value={codeType} onValueChange={(value) => {
                const nextType = value as BusinessCodeType;
                setCodeType(nextType);
                if (nextType === "invite") setCredits("0");
                if (nextType === "redeem" && Number(credits || 0) <= 0) setCredits("10");
              }} disabled={Boolean(editingItem) || view === "redeem"}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {dialogTypeOptions.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="text-xs leading-5 text-[var(--app-text-muted)]">
                {typeOptions.find((item) => item.value === codeType)?.hint}
              </div>
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">状态</div>
              <Select value={status} onValueChange={(value) => setStatus(value as BusinessCodeStatus)}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {statusOptions.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">标题</div>
              <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder={typeLabel(codeType)} className={adminInputClass} />
            </label>
            {!editingItem ? (
              <label className="space-y-2">
                <div className="text-sm font-medium text-[var(--app-text-secondary)]">自定义码</div>
                <Input value={rawCode} onChange={(event) => setRawCode(event.target.value)} placeholder="留空自动生成" className={adminInputClass} />
              </label>
            ) : null}
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">赠送/兑换点数</div>
              <Input type="number" min="0" step="1" value={credits} onChange={(event) => setCredits(event.target.value)} disabled={codeType === "invite"} className={adminInputClass} />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">最大使用次数</div>
              <Input type="number" min="1" step="1" value={maxUses} onChange={(event) => setMaxUses(event.target.value)} className={adminInputClass} />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">开始时间</div>
              <Input type="datetime-local" value={startsAt} onChange={(event) => setStartsAt(event.target.value)} className={adminInputClass} />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">结束时间</div>
              <Input type="datetime-local" value={expiresAt} onChange={(event) => setExpiresAt(event.target.value)} className={adminInputClass} />
            </label>
            <label className="space-y-2 md:col-span-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">备注</div>
              <Textarea value={note} onChange={(event) => setNote(event.target.value)} rows={3} className={adminInputClass} />
            </label>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>关闭</Button>
            <Button type="button" onClick={() => void saveCode()} disabled={saving}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(usageDialogCode)} onOpenChange={(open) => {
        if (!open) {
          setUsageDialogCode(null);
          setUsageItems([]);
        }
      }}>
        <DialogContent className="max-h-[88vh] w-[min(94vw,900px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>使用记录</DialogTitle>
            <DialogDescription>
              {usageDialogCode?.title || "-"} · {usageDialogCode?.codePreview || "-"}
            </DialogDescription>
          </DialogHeader>
          <div className="overflow-x-auto">
            <table className={cn(adminTableClass, "w-full min-w-[760px]")}>
              <thead className={adminTableHeadClass}>
                <tr>
                  <th className="px-4 py-3">用户</th>
                  <th className="px-4 py-3">场景</th>
                  <th className="px-4 py-3">到账</th>
                  <th className="px-4 py-3">使用时间</th>
                </tr>
              </thead>
              <tbody className={adminTableBodyClass}>
                {usageLoading ? (
                  <tr>
                    <td colSpan={4} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
                      <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                      读取中
                    </td>
                  </tr>
                ) : usageItems.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无使用记录</td>
                  </tr>
                ) : (
                  usageItems.map((item) => (
                    <tr key={item.id} className={adminTableRowClass}>
                      <td className="px-4 py-3">
                        <div className="font-medium text-[var(--app-text-primary)]">{item.username || "-"}</div>
                        <div className="mt-0.5 text-xs text-[var(--app-text-muted)]">UID {item.uid || "-"} · {item.email || item.userId}</div>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{contextLabel(item.context)}</td>
                      <td className="whitespace-nowrap px-4 py-3 font-medium text-[var(--app-text-primary)]">{Number(item.creditsGranted || 0).toLocaleString()}</td>
                      <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatTime(item.createdAt)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setUsageDialogCode(null)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
