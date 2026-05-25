"use client";

import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, CreditCard, LoaderCircle, Pencil, Plus, RefreshCw, RotateCcw, Save, Trash2, XCircle } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import { adminInputClass, adminSubPanelClass, adminTableBodyClass, adminTableClass, adminTableHeadClass, adminTableRowClass } from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
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
import {
  completeAdminBusinessPaymentOrder,
  cancelAdminBusinessPaymentOrder,
  createAdminBusinessPaymentPackage,
  deleteAdminBusinessPaymentPackage,
  fetchAdminBusinessPaymentOrders,
  fetchAdminBusinessPaymentPackages,
  fetchAdminBusinessPaymentProviders,
  refundAdminBusinessPaymentOrder,
  updateAdminBusinessPaymentPackage,
  type BusinessPaymentOrder,
  type BusinessPaymentPackage,
  type BusinessPaymentProvider,
} from "@/lib/api";
import { cn } from "@/lib/utils";

type PaymentPackageType = "balance" | "subscription";

function numberText(value?: number | null) {
  return Number(value || 0).toLocaleString();
}

function moneyText(amountCents?: number | null, currency = "CNY") {
  const amount = Number(amountCents || 0) / 100;
  const prefix = currency === "CNY" ? "¥" : `${currency} `;
  return `${prefix}${amount.toFixed(2)}`;
}

function formatDateTime(value?: string) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return date.toLocaleString();
}

function statusText(status: string) {
  if (status === "pending") return "待确认";
  if (status === "paid") return "已支付";
  if (status === "completed") return "已到账";
  if (status === "cancelled") return "已取消";
  if (status === "refunded") return "已退款";
  if (status === "expired") return "已过期";
  if (status === "failed") return "失败";
  return status || "-";
}

function statusVariant(status: string): "success" | "secondary" | "warning" | "danger" {
  if (status === "completed") return "success";
  if (status === "pending" || status === "paid") return "warning";
  if (status === "failed" || status === "refunded") return "danger";
  return "secondary";
}

function packageTypeText(type?: string) {
  if (type === "subscription" || type === "monthly") return "订阅";
  return "余额";
}

const emptyPackageForm = {
  id: "",
  packageType: "balance" as PaymentPackageType,
  name: "",
  description: "",
  amountCents: "990",
  credits: "100",
  durationDays: "30",
  currency: "CNY",
  enabled: true,
  sortOrder: "100",
};

export default function PaymentsPage() {
  const [packages, setPackages] = useState<BusinessPaymentPackage[]>([]);
  const [providers, setProviders] = useState<BusinessPaymentProvider[]>([]);
  const [orders, setOrders] = useState<BusinessPaymentOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingPackage, setSavingPackage] = useState(false);
  const [packageDialogOpen, setPackageDialogOpen] = useState(false);
  const [actingOrderId, setActingOrderId] = useState("");
  const [deletingPackageId, setDeletingPackageId] = useState("");
  const [commissionRateBps, setCommissionRateBps] = useState("0");
  const [packageForm, setPackageForm] = useState(emptyPackageForm);

  const stats = useMemo(() => {
    return orders.reduce(
      (acc, item) => {
        acc.total += 1;
        if (item.status === "pending") acc.pending += 1;
        if (item.status === "completed") {
          acc.completed += 1;
          acc.amountCents += item.amountCents;
          acc.credits += item.credits;
        }
        return acc;
      },
      { total: 0, pending: 0, completed: 0, amountCents: 0, credits: 0 },
    );
  }, [orders]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [packagePayload, providerPayload, orderPayload] = await Promise.all([
        fetchAdminBusinessPaymentPackages(),
        fetchAdminBusinessPaymentProviders(),
        fetchAdminBusinessPaymentOrders({ limit: 100 }),
      ]);
      setPackages(packagePayload.items || []);
      setProviders(providerPayload.items || []);
      setOrders(orderPayload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取支付数据失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const openCreatePackage = () => {
    setPackageForm(emptyPackageForm);
    setPackageDialogOpen(true);
  };

  const openEditPackage = (item: BusinessPaymentPackage) => {
    setPackageForm({
      id: item.id,
      packageType: item.packageType === "monthly" ? "subscription" : item.packageType || "balance",
      name: item.name,
      description: item.description || "",
      amountCents: String(item.amountCents || 0),
      credits: String(item.credits || 0),
      durationDays: String(item.durationDays || 30),
      currency: item.currency || "CNY",
      enabled: item.enabled,
      sortOrder: String(item.sortOrder || 0),
    });
    setPackageDialogOpen(true);
  };

  const savePackage = async () => {
    const payload = {
      packageType: packageForm.packageType,
      name: packageForm.name,
      description: packageForm.description,
      amountCents: Math.max(0, Math.floor(Number(packageForm.amountCents) || 0)),
      credits: Math.max(0, Math.floor(Number(packageForm.credits) || 0)),
      durationDays: packageForm.packageType === "subscription" ? Math.max(1, Math.floor(Number(packageForm.durationDays) || 30)) : 0,
      currency: packageForm.currency || "CNY",
      enabled: packageForm.enabled,
      sortOrder: Math.floor(Number(packageForm.sortOrder) || 0),
    };
    setSavingPackage(true);
    try {
      if (packageForm.id) {
        await updateAdminBusinessPaymentPackage(packageForm.id, payload);
      } else {
        await createAdminBusinessPaymentPackage(payload);
      }
      setPackageForm(emptyPackageForm);
      setPackageDialogOpen(false);
      await loadData();
      toast.success("套餐已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存套餐失败");
    } finally {
      setSavingPackage(false);
    }
  };

  const removePackage = async (item: BusinessPaymentPackage) => {
    if (!window.confirm(`确认删除套餐「${item.name}」？已有订单的套餐不能删除，可以改为停用。`)) {
      return;
    }
    setDeletingPackageId(item.id);
    try {
      await deleteAdminBusinessPaymentPackage(item.id);
      await loadData();
      toast.success("套餐已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除套餐失败");
    } finally {
      setDeletingPackageId("");
    }
  };

  const actOrder = async (order: BusinessPaymentOrder, action: "complete" | "cancel" | "refund") => {
    setActingOrderId(order.id);
    try {
      if (action === "complete") {
        await completeAdminBusinessPaymentOrder(order.id, {
          commissionRateBps: Math.max(0, Math.floor(Number(commissionRateBps) || 0)),
        });
        toast.success("订单已确认到账");
      } else if (action === "cancel") {
        await cancelAdminBusinessPaymentOrder(order.id);
        toast.success("订单已取消");
      } else {
        await refundAdminBusinessPaymentOrder(order.id);
        toast.success("订单已退款");
      }
      await loadData();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订单操作失败");
    } finally {
      setActingOrderId("");
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title="支付订单"
        description="管理充值套餐、人工确认订单和订单返利结算。"
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => void loadData()} disabled={loading || savingPackage}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
            <Button type="button" onClick={openCreatePackage}>
              <Plus className="size-4" />
              新增套餐
            </Button>
          </>
        }
      >
        <div className="mb-3 inline-flex size-10 items-center justify-center rounded-[var(--app-radius-md)] border border-white/10 bg-white/[0.045] text-[var(--app-text-primary)]">
          <CreditCard className="size-5" />
        </div>
      </AdminHeader>

      <div className="grid gap-4 md:grid-cols-4">
        <AdminStatCard label="订单数" value={numberText(stats.total)} sub={`待确认 ${numberText(stats.pending)}`} icon={CreditCard} color="text-cyan-300" />
        <AdminStatCard label="已完成" value={numberText(stats.completed)} sub="人工确认到账" icon={CheckCircle2} color="text-emerald-300" />
        <AdminStatCard label="完成金额" value={moneyText(stats.amountCents)} sub="按订单金额统计" icon={CreditCard} color="text-amber-300" />
        <AdminStatCard label="发放点数" value={numberText(stats.credits)} sub="含充值到账" icon={Plus} color="text-violet-300" />
      </div>

      <AdminPanel className="overflow-hidden">
        <AdminSectionTitle title="套餐列表" />
        <div className="overflow-x-auto">
          <table className={adminTableClass}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th className="px-4 py-3">套餐</th>
                <th className="px-4 py-3">类型</th>
                <th className="px-4 py-3">金额</th>
                <th className="px-4 py-3">点数</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {packages.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无套餐</td>
                </tr>
              ) : packages.map((item) => (
                <tr key={item.id} className={adminTableRowClass}>
                  <td className="px-4 py-3">
                    <div className="font-medium text-[var(--app-text-primary)]">{item.name}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                      {item.description || "-"}
                      {item.packageType === "subscription" || item.packageType === "monthly" ? ` · ${Number(item.durationDays || 30).toLocaleString()} 天` : ""}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <Badge variant={item.packageType === "subscription" || item.packageType === "monthly" ? "info" : "secondary"}>{packageTypeText(item.packageType)}</Badge>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">{moneyText(item.amountCents, item.currency)}</td>
                  <td className="whitespace-nowrap px-4 py-3">{numberText(item.credits)}{item.packageType === "subscription" || item.packageType === "monthly" ? " 订阅点" : ""}</td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <Badge variant={item.enabled ? "success" : "secondary"}>{item.enabled ? "启用" : "停用"}</Badge>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <div className="flex justify-end gap-1.5">
                      <Button type="button" variant="ghost" size="icon" onClick={() => openEditPackage(item)} aria-label="编辑套餐">
                        <Pencil className="size-4" />
                      </Button>
                      <Button type="button" variant="ghost" size="icon" onClick={() => void removePackage(item)} disabled={deletingPackageId === item.id} aria-label="删除套餐">
                        {deletingPackageId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </AdminPanel>

      <AdminPanel className="overflow-hidden">
        <AdminSectionTitle title="支付渠道" />
        <div className="grid gap-3 p-5 md:grid-cols-2 xl:grid-cols-3">
          {providers.length === 0 ? (
            <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无支付渠道</div>
          ) : providers.map((item) => (
            <div key={item.id} className={cn(adminSubPanelClass, "p-4")}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-[var(--app-text-primary)]">{item.name}</div>
                  <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.providerKey}</div>
                </div>
                <Badge variant={item.enabled ? "success" : "secondary"}>{item.enabled ? "启用" : "停用"}</Badge>
              </div>
              <div className="mt-3 text-xs leading-5 text-[var(--app-text-muted)]">
                当前内置人工确认渠道。真实支付接入后，会在这里配置渠道实例和密钥。
              </div>
            </div>
          ))}
        </div>
      </AdminPanel>

      <AdminPanel className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-[var(--app-border)] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">订单列表</h2>
          <label className="flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
            订单返利比例
            <input type="number" min="0" max="10000" value={commissionRateBps} onChange={(event) => setCommissionRateBps(event.target.value)} className={cn(adminInputClass, "h-9 w-24 rounded-md px-3")} />
            bps
          </label>
        </div>
        <div className="overflow-x-auto">
          <table className={cn(adminTableClass, "w-full min-w-[980px] table-fixed")}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th className="w-[26%] px-4 py-3">订单</th>
                <th className="w-[17%] px-4 py-3">用户</th>
                <th className="w-[9%] px-4 py-3">金额</th>
                <th className="w-[8%] px-4 py-3">点数</th>
                <th className="w-[9%] px-4 py-3">状态</th>
                <th className="w-[14%] px-4 py-3">时间</th>
                <th className="w-[17%] px-4 py-3">操作</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {loading ? (
                <tr><td colSpan={7} className="px-4 py-10 text-center text-[var(--app-text-muted)]"><LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />读取中</td></tr>
              ) : orders.length === 0 ? (
                <tr><td colSpan={7} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无订单</td></tr>
              ) : orders.map((item) => {
                const acting = actingOrderId === item.id;
                return (
                  <tr key={item.id} className={adminTableRowClass}>
                    <td className="min-w-0 px-4 py-3">
                      <div className="truncate font-medium text-[var(--app-text-primary)]" title={item.outTradeNo}>{item.outTradeNo}</div>
                      <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.id}</div>
                    </td>
                    <td className="min-w-0 px-4 py-3">
                      <div className="truncate font-medium text-[var(--app-text-primary)]">{item.username || "-"}</div>
                      <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]" title={item.userEmail || ""}>{item.userEmail || "-"}</div>
                    </td>
                    <td className="whitespace-nowrap px-4 py-3">{moneyText(item.amountCents, item.currency)}</td>
                    <td className="whitespace-nowrap px-4 py-3">{numberText(item.credits)}</td>
                    <td className="whitespace-nowrap px-4 py-3"><Badge variant={statusVariant(item.status)}>{statusText(item.status)}</Badge></td>
                    <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.createdAt)}</td>
                    <td className="min-w-0 px-4 py-3">
                      <div className="flex flex-wrap gap-2">
                        <Button type="button" size="sm" disabled={acting || (item.status !== "pending" && item.status !== "paid")} onClick={() => void actOrder(item, "complete")}>
                          {acting ? <LoaderCircle className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
                          确认
                        </Button>
                        <Button type="button" size="sm" variant="outline" disabled={acting || item.status !== "pending"} onClick={() => void actOrder(item, "cancel")}>
                          <XCircle className="size-4" />
                          取消
                        </Button>
                        <Button type="button" size="sm" variant="outline" disabled={acting || item.status !== "completed"} onClick={() => void actOrder(item, "refund")}>
                          <RotateCcw className="size-4" />
                          退款
                        </Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </AdminPanel>

      <Dialog open={packageDialogOpen} onOpenChange={(open) => {
        setPackageDialogOpen(open);
        if (!open) setPackageForm(emptyPackageForm);
      }}>
        <DialogContent className="max-h-[90vh] w-[min(92vw,720px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{packageForm.id ? "编辑套餐" : "新增套餐"}</DialogTitle>
          </DialogHeader>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">套餐类型</div>
              <Select value={packageForm.packageType} onValueChange={(value) => setPackageForm((current) => ({ ...current, packageType: value as PaymentPackageType }))}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="balance">余额</SelectItem>
                  <SelectItem value="subscription">订阅</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">名称</div>
              <Input
                value={packageForm.name}
                onChange={(event) => setPackageForm((current) => ({ ...current, name: event.target.value }))}
                className={adminInputClass}
                placeholder={packageForm.packageType === "subscription" ? "例如 月度会员" : "例如 100 点"}
              />
            </label>
            <label className="space-y-2 md:col-span-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">说明</div>
              <Input
                value={packageForm.description}
                onChange={(event) => setPackageForm((current) => ({ ...current, description: event.target.value }))}
                className={adminInputClass}
                placeholder={packageForm.packageType === "subscription" ? "订阅套餐，人工确认后生效" : "人工确认后到账"}
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">金额（分）</div>
              <Input
                type="number"
                min="1"
                value={packageForm.amountCents}
                onChange={(event) => setPackageForm((current) => ({ ...current, amountCents: event.target.value }))}
                className={adminInputClass}
              />
            </label>
            {packageForm.packageType === "subscription" ? (
              <label className="space-y-2">
                <div className="text-sm font-medium text-[var(--app-text-secondary)]">订阅天数</div>
                <Input
                  type="number"
                  min="1"
                  value={packageForm.durationDays}
                  onChange={(event) => setPackageForm((current) => ({ ...current, durationDays: event.target.value }))}
                  className={adminInputClass}
                />
              </label>
            ) : null}
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">点数</div>
              <Input
                type="number"
                min="1"
                value={packageForm.credits}
                onChange={(event) => setPackageForm((current) => ({ ...current, credits: event.target.value }))}
                className={adminInputClass}
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">币种</div>
              <Input
                value={packageForm.currency}
                onChange={(event) => setPackageForm((current) => ({ ...current, currency: event.target.value }))}
                className={adminInputClass}
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">排序</div>
              <Input
                type="number"
                value={packageForm.sortOrder}
                onChange={(event) => setPackageForm((current) => ({ ...current, sortOrder: event.target.value }))}
                className={adminInputClass}
              />
            </label>
            <label className={cn(adminSubPanelClass, "flex items-center justify-between gap-3 px-4 py-3 text-sm text-[var(--app-text-secondary)] md:col-span-2")}>
              <span>启用套餐</span>
              <input type="checkbox" checked={packageForm.enabled} onChange={(event) => setPackageForm((current) => ({ ...current, enabled: event.target.checked }))} />
            </label>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setPackageDialogOpen(false)}>取消</Button>
            <Button type="button" onClick={() => void savePackage()} disabled={savingPackage}>
              {savingPackage ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
