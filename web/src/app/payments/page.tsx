"use client";

import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, CreditCard, Eye, LoaderCircle, Pencil, Plus, RefreshCw, RotateCcw, Save, Search, Trash2, WalletCards, XCircle } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import { adminInputClass, adminSubPanelClass, adminTableBodyClass, adminTableClass, adminTableHeadClass, adminTableRowClass } from "@/components/admin-styles";
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
import {
  completeAdminBusinessPaymentOrder,
  cancelAdminBusinessPaymentOrder,
  createAdminBusinessPaymentProvider,
  createAdminBusinessPaymentPackage,
  deleteAdminBusinessPaymentProvider,
  deleteAdminBusinessPaymentPackage,
  fetchAdminBusinessPaymentOrders,
  fetchAdminBusinessPaymentOrderAuditLogs,
  fetchAdminBusinessPaymentPackages,
  fetchAdminBusinessPaymentProviders,
  fetchAdminBusinessSubscriptions,
  refundAdminBusinessPaymentOrder,
  updateAdminBusinessPaymentProvider,
  updateAdminBusinessPaymentPackage,
  type BusinessPaymentAuditLog,
  type BusinessPaymentOrder,
  type BusinessPaymentOrderStatus,
  type BusinessPaymentPackage,
  type BusinessPaymentProvider,
  type BusinessPaymentProviderInput,
  type BusinessSubscription,
  type PaginationMeta,
} from "@/lib/api";
import { cn } from "@/lib/utils";

type PaymentPackageType = "balance" | "subscription";
type OrderAction = "complete" | "cancel" | "refund";
type OrderStatusFilter = BusinessPaymentOrderStatus | "all";
type OrderKindFilter = "all" | "balance" | "subscription" | "renewal" | "upgrade";
type SubscriptionStatusFilter = "all" | "active" | "expired" | "cancelled" | "upgraded";
type SubscriptionWindowFilter = "all" | "current" | "future" | "history";

function numberText(value?: number | null) {
  return Number(value || 0).toLocaleString();
}

function defaultSubscriptionPage(): PaginationMeta {
  return { page: 1, pageSize: 10, total: 0 };
}

function defaultOrderPage(): PaginationMeta {
  return { page: 1, pageSize: 20, total: 0 };
}

function paginationText(page: PaginationMeta) {
  if (!page.total) {
    return "0 / 0";
  }
  const start = (page.page - 1) * page.pageSize + 1;
  const end = Math.min(page.page * page.pageSize, page.total);
  return `${start}-${end} / ${page.total}`;
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

function subscriptionStatusText(status?: string) {
  if (status === "active") return "订阅中";
  if (status === "expired") return "已过期";
  if (status === "cancelled") return "已取消";
  if (status === "upgraded") return "已升级";
  return status || "-";
}

function subscriptionRowStatusText(item: BusinessSubscription) {
  if (item.status === "active") {
    const startsAt = new Date(item.startsAt || "");
    if (!Number.isNaN(startsAt.getTime()) && startsAt.getTime() > Date.now()) {
      return "待生效";
    }
  }
  return subscriptionStatusText(item.status);
}

function subscriptionRowStatusVariant(item: BusinessSubscription): "success" | "secondary" | "warning" | "danger" | "info" {
  if (item.status === "active") {
    const startsAt = new Date(item.startsAt || "");
    if (!Number.isNaN(startsAt.getTime()) && startsAt.getTime() > Date.now()) {
      return "warning";
    }
  }
  return subscriptionStatusVariant(item.status);
}

function subscriptionCoverageExpiresAt(item: BusinessSubscription) {
  return item.coverageExpiresAt || item.expiresAt;
}

function subscriptionStatusVariant(status?: string): "success" | "secondary" | "warning" | "danger" | "info" {
  if (status === "active") return "success";
  if (status === "expired") return "warning";
  if (status === "cancelled") return "secondary";
  if (status === "upgraded") return "info";
  return "secondary";
}

function packageTypeText(type?: string) {
  if (type === "subscription" || type === "monthly") return "订阅";
  return "余额";
}

function billingActionText(order: BusinessPaymentOrder) {
  if (order.billingAction === "upgrade") return "升级补差价";
  if (order.billingAction === "renewal") return "续订";
  if (order.packageType === "subscription" || order.packageType === "monthly") return "开通订阅";
  return "余额充值";
}

function auditActionText(action: string) {
  if (action === "order_created") return "创建订单";
  if (action === "credit_granted") return "余额到账";
  if (action === "subscription_activated") return "订阅激活";
  if (action === "affiliate_commission_created") return "返利结算";
  if (action === "affiliate_commission_reversed") return "返利退回";
  if (action === "order_completed") return "确认到账";
  if (action === "order_cancelled") return "取消订单";
  if (action === "refunded") return "退款";
  if (action === "subscription_cancelled") return "订阅取消";
  return action || "-";
}

function actionText(action: OrderAction) {
  if (action === "complete") return "确认到账";
  if (action === "cancel") return "取消订单";
  return "退款";
}

function detailText(value: unknown) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  try {
    return JSON.stringify(value);
  } catch {
    return "";
  }
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

const emptyProviderForm = {
  id: "",
  providerKey: "easypay",
  name: "EasyPay",
  enabled: true,
  methods: ["alipay", "wxpay"],
  apiBase: "",
  pid: "",
  pkey: "",
  paymentMode: "qrcode",
  cidAlipay: "",
  cidWxpay: "",
  sortOrder: "100",
};

function paymentMethodText(method: string) {
  if (method === "manual") return "人工确认";
  if (method === "alipay") return "支付宝";
  if (method === "wxpay") return "微信支付";
  return method || "-";
}

export default function PaymentsPage() {
  const [packages, setPackages] = useState<BusinessPaymentPackage[]>([]);
  const [providers, setProviders] = useState<BusinessPaymentProvider[]>([]);
  const [orders, setOrders] = useState<BusinessPaymentOrder[]>([]);
  const [orderPage, setOrderPage] = useState<PaginationMeta>(defaultOrderPage());
  const [subscriptions, setSubscriptions] = useState<BusinessSubscription[]>([]);
  const [subscriptionPage, setSubscriptionPage] = useState<PaginationMeta>(defaultSubscriptionPage());
  const [loading, setLoading] = useState(true);
  const [savingPackage, setSavingPackage] = useState(false);
  const [savingProvider, setSavingProvider] = useState(false);
  const [packageDialogOpen, setPackageDialogOpen] = useState(false);
  const [providerDialogOpen, setProviderDialogOpen] = useState(false);
  const [actingOrderId, setActingOrderId] = useState("");
  const [deletingPackageId, setDeletingPackageId] = useState("");
  const [deletingProviderId, setDeletingProviderId] = useState("");
  const [commissionRateBps, setCommissionRateBps] = useState("0");
  const [packageForm, setPackageForm] = useState(emptyPackageForm);
  const [providerForm, setProviderForm] = useState(emptyProviderForm);
  const [orderStatusFilter, setOrderStatusFilter] = useState<OrderStatusFilter>("all");
  const [orderKindFilter, setOrderKindFilter] = useState<OrderKindFilter>("all");
  const [orderSearch, setOrderSearch] = useState("");
  const [subscriptionStatusFilter, setSubscriptionStatusFilter] = useState<SubscriptionStatusFilter>("all");
  const [subscriptionWindowFilter, setSubscriptionWindowFilter] = useState<SubscriptionWindowFilter>("current");
  const [subscriptionSearch, setSubscriptionSearch] = useState("");
  const [selectedOrder, setSelectedOrder] = useState<BusinessPaymentOrder | null>(null);
  const [auditLogs, setAuditLogs] = useState<BusinessPaymentAuditLog[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [confirmAction, setConfirmAction] = useState<{ order: BusinessPaymentOrder; action: OrderAction } | null>(null);

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

  const orderTotalPages = Math.max(1, Math.ceil((orderPage.total || 0) / (orderPage.pageSize || 20)));
  const subscriptionTotalPages = Math.max(1, Math.ceil((subscriptionPage.total || 0) / (subscriptionPage.pageSize || 10)));

  const loadData = async (nextOrderPage = orderPage.page, nextSubscriptionPage = subscriptionPage.page) => {
    setLoading(true);
    try {
      const [packagePayload, providerPayload, orderPayload, subscriptionPayload] = await Promise.all([
        fetchAdminBusinessPaymentPackages(),
        fetchAdminBusinessPaymentProviders(),
        fetchAdminBusinessPaymentOrders({
          page: nextOrderPage,
          pageSize: orderPage.pageSize,
          status: orderStatusFilter,
          kind: orderKindFilter,
          search: orderSearch.trim() || undefined,
        }),
        fetchAdminBusinessSubscriptions({
          page: nextSubscriptionPage,
          pageSize: subscriptionPage.pageSize,
          status: subscriptionStatusFilter,
          activeWindow: subscriptionWindowFilter,
          search: subscriptionSearch.trim() || undefined,
        }),
      ]);
      setPackages(packagePayload.items || []);
      setProviders(providerPayload.items || []);
      setOrders(orderPayload.items || []);
      setOrderPage(orderPayload.page || { ...orderPage, page: nextOrderPage });
      setSubscriptions(subscriptionPayload.items || []);
      setSubscriptionPage(subscriptionPayload.page || { ...subscriptionPage, page: nextSubscriptionPage });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取支付数据失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData(1, 1);
  }, [orderStatusFilter, orderKindFilter, subscriptionStatusFilter, subscriptionWindowFilter]);

  const openCreatePackage = () => {
    setPackageForm(emptyPackageForm);
    setPackageDialogOpen(true);
  };

  const openCreateProvider = () => {
    setProviderForm(emptyProviderForm);
    setProviderDialogOpen(true);
  };

  const openEditProvider = (item: BusinessPaymentProvider) => {
    setProviderForm({
      id: item.id,
      providerKey: item.providerKey || "easypay",
      name: item.name || "EasyPay",
      enabled: item.enabled,
      methods: item.supportedMethods?.length ? item.supportedMethods : ["alipay", "wxpay"],
      apiBase: item.config?.apiBase || "",
      pid: item.config?.pid || "",
      pkey: "",
      paymentMode: item.config?.paymentMode || "qrcode",
      cidAlipay: item.config?.cidAlipay || "",
      cidWxpay: item.config?.cidWxpay || "",
      sortOrder: String(item.sortOrder || 0),
    });
    setProviderDialogOpen(true);
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

  const openOrderDetail = async (order: BusinessPaymentOrder) => {
    setSelectedOrder(order);
    setAuditLogs([]);
    setAuditLoading(true);
    try {
      const payload = await fetchAdminBusinessPaymentOrderAuditLogs(order.id);
      setAuditLogs(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取订单日志失败");
    } finally {
      setAuditLoading(false);
    }
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

  const toggleProviderMethod = (method: string, checked: boolean) => {
    setProviderForm((current) => {
      const methods = new Set(current.methods);
      if (checked) {
        methods.add(method);
      } else {
        methods.delete(method);
      }
      return { ...current, methods: Array.from(methods) };
    });
  };

  const providerPayload = (): BusinessPaymentProviderInput => ({
    providerKey: providerForm.providerKey,
    name: providerForm.name,
    enabled: providerForm.enabled,
    supportedMethods: providerForm.methods,
    sortOrder: Math.floor(Number(providerForm.sortOrder) || 0),
    config: {
      apiBase: providerForm.apiBase,
      pid: providerForm.pid,
      pkey: providerForm.pkey,
      paymentMode: providerForm.paymentMode,
      cidAlipay: providerForm.cidAlipay,
      cidWxpay: providerForm.cidWxpay,
    },
  });

  const saveProvider = async () => {
    if (!providerForm.methods.length) {
      toast.error("至少选择一种支付方式");
      return;
    }
    setSavingProvider(true);
    try {
      if (providerForm.id) {
        await updateAdminBusinessPaymentProvider(providerForm.id, providerPayload());
      } else {
        await createAdminBusinessPaymentProvider(providerPayload());
      }
      setProviderForm(emptyProviderForm);
      setProviderDialogOpen(false);
      await loadData();
      toast.success("支付渠道已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存支付渠道失败");
    } finally {
      setSavingProvider(false);
    }
  };

  const removeProvider = async (item: BusinessPaymentProvider) => {
    if (item.id === "manual") {
      return;
    }
    if (!window.confirm(`确认删除支付渠道「${item.name}」？待处理订单会阻止删除。`)) {
      return;
    }
    setDeletingProviderId(item.id);
    try {
      await deleteAdminBusinessPaymentProvider(item.id);
      await loadData();
      toast.success("支付渠道已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除支付渠道失败");
    } finally {
      setDeletingProviderId("");
    }
  };

  const actOrder = async (order: BusinessPaymentOrder, action: OrderAction) => {
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
      setSelectedOrder((current) => current && current.id === order.id ? null : current);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订单操作失败");
    } finally {
      setActingOrderId("");
      setConfirmAction(null);
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
        <div className="flex items-center justify-between gap-3 border-b border-[var(--app-border)] px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">支付渠道</h2>
            <p className="mt-1 text-xs text-[var(--app-text-muted)]">配置用户下单时可选的支付方式。</p>
          </div>
          <Button type="button" size="sm" onClick={openCreateProvider}>
            <Plus className="size-4" />
            新增渠道
          </Button>
        </div>
        <div className="grid gap-3 p-5 md:grid-cols-2 xl:grid-cols-3">
          {providers.length === 0 ? (
            <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无支付渠道</div>
          ) : providers.map((item) => (
            <div key={item.id} className={cn(adminSubPanelClass, "p-4")}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-semibold text-[var(--app-text-primary)]">
                    <WalletCards className="size-4 text-[var(--app-accent-cyan)]" />
                    {item.name}
                  </div>
                  <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.providerKey}</div>
                </div>
                <Badge variant={item.enabled ? "success" : "secondary"}>{item.enabled ? "启用" : "停用"}</Badge>
              </div>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {(item.supportedMethods || []).map((method) => (
                  <Badge key={method} variant="secondary">{paymentMethodText(method)}</Badge>
                ))}
              </div>
              {item.providerKey === "easypay" ? (
                <div className="mt-3 grid gap-1 text-xs leading-5 text-[var(--app-text-muted)]">
                  <div>API：{item.config?.apiBase || "-"}</div>
                  <div>商户 ID：{item.config?.pid || "-"}</div>
                  <div>模式：{item.config?.paymentMode === "popup" ? "跳转收银台" : "二维码接口"}</div>
                </div>
              ) : (
                <div className="mt-3 text-xs leading-5 text-[var(--app-text-muted)]">内置人工确认渠道，不需要配置密钥。</div>
              )}
              {item.id !== "manual" ? (
                <div className="mt-4 flex justify-end gap-1.5">
                  <Button type="button" variant="ghost" size="icon" onClick={() => openEditProvider(item)} aria-label="编辑渠道">
                    <Pencil className="size-4" />
                  </Button>
                  <Button type="button" variant="ghost" size="icon" onClick={() => void removeProvider(item)} disabled={deletingProviderId === item.id} aria-label="删除渠道">
                    {deletingProviderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
                  </Button>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </AdminPanel>

      <AdminPanel className="overflow-hidden">
        <div className="flex flex-col gap-4 border-b border-[var(--app-border)] px-5 py-4">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
            <div>
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">订阅列表</h2>
              <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                当前显示 {paginationText(subscriptionPage)}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || subscriptionPage.page <= 1}
                onClick={() => void loadData(orderPage.page, Math.max(1, subscriptionPage.page - 1))}
              >
                上一页
              </Button>
              <div className="min-w-20 text-center text-xs text-[var(--app-text-muted)]">
                {subscriptionPage.page} / {subscriptionTotalPages}
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || subscriptionPage.page >= subscriptionTotalPages}
                onClick={() => void loadData(orderPage.page, Math.min(subscriptionTotalPages, subscriptionPage.page + 1))}
              >
                下一页
              </Button>
            </div>
          </div>
          <div className="grid gap-3 xl:grid-cols-[160px_180px_minmax(220px,1fr)_auto]">
            <Select value={subscriptionStatusFilter} onValueChange={(value) => setSubscriptionStatusFilter(value as SubscriptionStatusFilter)}>
              <SelectTrigger className={adminInputClass}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="active">有效</SelectItem>
                <SelectItem value="upgraded">已升级</SelectItem>
                <SelectItem value="expired">已过期</SelectItem>
                <SelectItem value="cancelled">已取消</SelectItem>
              </SelectContent>
            </Select>
            <Select value={subscriptionWindowFilter} onValueChange={(value) => setSubscriptionWindowFilter(value as SubscriptionWindowFilter)}>
              <SelectTrigger className={adminInputClass}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部有效期</SelectItem>
                <SelectItem value="current">当前可用</SelectItem>
                <SelectItem value="future">未来生效</SelectItem>
                <SelectItem value="history">历史记录</SelectItem>
              </SelectContent>
            </Select>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
              <Input
                value={subscriptionSearch}
                onChange={(event) => setSubscriptionSearch(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void loadData(orderPage.page, 1);
                  }
                }}
                className={cn(adminInputClass, "pl-9")}
                placeholder="搜索用户、邮箱、套餐或订单"
              />
            </div>
            <Button type="button" variant="outline" onClick={() => void loadData(orderPage.page, 1)} disabled={loading}>
              <Search className="size-4" />
              筛选
            </Button>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className={cn(adminTableClass, "w-full min-w-[980px] table-fixed")}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th className="w-[20%] px-4 py-3">用户</th>
                <th className="w-[20%] px-4 py-3">订阅</th>
                <th className="w-[16%] px-4 py-3">额度</th>
                <th className="w-[12%] px-4 py-3">状态</th>
                <th className="w-[16%] px-4 py-3">有效期</th>
                <th className="w-[16%] px-4 py-3">来源订单</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {loading ? (
                <tr><td colSpan={6} className="px-4 py-10 text-center text-[var(--app-text-muted)]"><LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />读取中</td></tr>
              ) : subscriptions.length === 0 ? (
                <tr><td colSpan={6} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无匹配订阅</td></tr>
              ) : subscriptions.map((item) => (
                <tr key={item.id} className={adminTableRowClass}>
                  <td className="min-w-0 px-4 py-3">
                    <div className="truncate font-medium text-[var(--app-text-primary)]">{item.username || "-"}</div>
                    <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]" title={item.userEmail || ""}>{item.userEmail || item.userId || "-"}</div>
                  </td>
                  <td className="min-w-0 px-4 py-3">
                    <div className="truncate font-medium text-[var(--app-text-primary)]">{item.packageName || "订阅"}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">{numberText(item.durationDays || 0)} 天</div>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <div className="font-semibold text-[var(--app-text-primary)]">{numberText(item.creditsLeft)} / {numberText(item.creditsTotal)}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">已用 {numberText(item.creditsUsed)}</div>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <Badge variant={subscriptionRowStatusVariant(item)}>{subscriptionRowStatusText(item)}</Badge>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">
                    <div>{formatDateTime(item.startsAt)}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">本周期至 {formatDateTime(item.expiresAt)}</div>
                    {item.coverageExpiresAt && item.coverageExpiresAt !== item.expiresAt ? (
                      <div className="mt-1 text-xs text-[var(--app-text-muted)]">续订至 {formatDateTime(subscriptionCoverageExpiresAt(item))}</div>
                    ) : null}
                  </td>
                  <td className="min-w-0 px-4 py-3">
                    <div className="truncate text-xs text-[var(--app-text-secondary)]" title={item.orderId || ""}>{item.orderId || "-"}</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </AdminPanel>

      <AdminPanel className="overflow-hidden">
        <div className="flex flex-col gap-4 border-b border-[var(--app-border)] px-5 py-4">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
            <div>
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">订单列表</h2>
              <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                当前显示 {paginationText(orderPage)}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <label className="flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
                订单返利比例
                <input type="number" min="0" max="10000" value={commissionRateBps} onChange={(event) => setCommissionRateBps(event.target.value)} className={cn(adminInputClass, "h-9 w-24 rounded-md px-3")} />
                bps
              </label>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || orderPage.page <= 1}
                onClick={() => void loadData(Math.max(1, orderPage.page - 1), subscriptionPage.page)}
              >
                上一页
              </Button>
              <div className="min-w-20 text-center text-xs text-[var(--app-text-muted)]">
                {orderPage.page} / {orderTotalPages}
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || orderPage.page >= orderTotalPages}
                onClick={() => void loadData(Math.min(orderTotalPages, orderPage.page + 1), subscriptionPage.page)}
              >
                下一页
              </Button>
            </div>
          </div>
          <div className="grid gap-3 xl:grid-cols-[160px_180px_minmax(220px,1fr)_auto]">
            <Select value={orderStatusFilter} onValueChange={(value) => setOrderStatusFilter(value as OrderStatusFilter)}>
              <SelectTrigger className={adminInputClass}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="pending">待确认</SelectItem>
                <SelectItem value="paid">已支付</SelectItem>
                <SelectItem value="completed">已到账</SelectItem>
                <SelectItem value="cancelled">已取消</SelectItem>
                <SelectItem value="refunded">已退款</SelectItem>
                <SelectItem value="expired">已过期</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
              </SelectContent>
            </Select>
            <Select value={orderKindFilter} onValueChange={(value) => setOrderKindFilter(value as OrderKindFilter)}>
              <SelectTrigger className={adminInputClass}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部类型</SelectItem>
                <SelectItem value="balance">余额充值</SelectItem>
                <SelectItem value="subscription">开通订阅</SelectItem>
                <SelectItem value="renewal">续订</SelectItem>
                <SelectItem value="upgrade">升级补差价</SelectItem>
              </SelectContent>
            </Select>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
              <Input
                value={orderSearch}
                onChange={(event) => setOrderSearch(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void loadData(1, subscriptionPage.page);
                  }
                }}
                className={cn(adminInputClass, "pl-9")}
                placeholder="搜索用户、邮箱、订单号"
              />
            </div>
            <Button type="button" variant="outline" onClick={() => void loadData(1, subscriptionPage.page)} disabled={loading}>
              <Search className="size-4" />
              筛选
            </Button>
          </div>
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
                <tr><td colSpan={7} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无匹配订单</td></tr>
              ) : orders.map((item) => {
                const acting = actingOrderId === item.id;
                return (
                  <tr key={item.id} className={adminTableRowClass}>
                    <td className="min-w-0 px-4 py-3">
                      <div className="truncate font-medium text-[var(--app-text-primary)]" title={item.outTradeNo}>{item.outTradeNo}</div>
                      <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.id}</div>
                      <div className="mt-1 text-xs text-[var(--app-text-muted)]">{billingActionText(item)}</div>
                    </td>
                    <td className="min-w-0 px-4 py-3">
                      <div className="truncate font-medium text-[var(--app-text-primary)]">{item.username || "-"}</div>
                      <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]" title={item.userEmail || ""}>{item.userEmail || "-"}</div>
                    </td>
                    <td className="whitespace-nowrap px-4 py-3">
                      <div>{moneyText(item.amountCents, item.currency)}</div>
                      {item.billingAction === "upgrade" && Number(item.upgradeCreditCents || 0) > 0 ? (
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">已抵扣 {moneyText(item.upgradeCreditCents, item.currency)}</div>
                      ) : null}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3">{numberText(item.credits)}</td>
                    <td className="whitespace-nowrap px-4 py-3"><Badge variant={statusVariant(item.status)}>{statusText(item.status)}</Badge></td>
                    <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.createdAt)}</td>
                    <td className="min-w-0 px-4 py-3">
                      <div className="flex flex-wrap gap-2">
                        <Button type="button" size="sm" variant="outline" onClick={() => void openOrderDetail(item)}>
                          <Eye className="size-4" />
                          详情
                        </Button>
                        <Button type="button" size="sm" disabled={acting || (item.status !== "pending" && item.status !== "paid")} onClick={() => setConfirmAction({ order: item, action: "complete" })}>
                          {acting ? <LoaderCircle className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
                          确认
                        </Button>
                        <Button type="button" size="sm" variant="outline" disabled={acting || item.status !== "pending"} onClick={() => setConfirmAction({ order: item, action: "cancel" })}>
                          <XCircle className="size-4" />
                          取消
                        </Button>
                        <Button type="button" size="sm" variant="outline" disabled={acting || item.status !== "completed"} onClick={() => setConfirmAction({ order: item, action: "refund" })}>
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

      <Dialog open={providerDialogOpen} onOpenChange={(open) => {
        if (!open && !savingProvider) {
          setProviderDialogOpen(false);
          setProviderForm(emptyProviderForm);
        }
      }}>
        <DialogContent className="max-h-[90vh] w-[min(92vw,760px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{providerForm.id ? "编辑支付渠道" : "新增支付渠道"}</DialogTitle>
            <DialogDescription>当前先支持 EasyPay。密钥保存后不会明文回显，编辑时留空表示不修改。</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">渠道类型</div>
              <Select value={providerForm.providerKey} disabled={Boolean(providerForm.id)} onValueChange={(value) => setProviderForm((current) => ({ ...current, providerKey: value }))}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="easypay">EasyPay</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">名称</div>
              <Input
                value={providerForm.name}
                onChange={(event) => setProviderForm((current) => ({ ...current, name: event.target.value }))}
                className={adminInputClass}
                placeholder="例如 易支付"
              />
            </label>
            <label className="space-y-2 md:col-span-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">API 地址</div>
              <Input
                value={providerForm.apiBase}
                onChange={(event) => setProviderForm((current) => ({ ...current, apiBase: event.target.value }))}
                className={adminInputClass}
                placeholder="https://pay.example.com"
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">商户 ID</div>
              <Input
                value={providerForm.pid}
                onChange={(event) => setProviderForm((current) => ({ ...current, pid: event.target.value }))}
                className={adminInputClass}
                placeholder="pid"
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">商户密钥</div>
              <Input
                value={providerForm.pkey}
                onChange={(event) => setProviderForm((current) => ({ ...current, pkey: event.target.value }))}
                className={adminInputClass}
                placeholder={providerForm.id ? "留空保持不变" : "pkey"}
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">支付模式</div>
              <Select value={providerForm.paymentMode} onValueChange={(value) => setProviderForm((current) => ({ ...current, paymentMode: value }))}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="qrcode">二维码接口</SelectItem>
                  <SelectItem value="popup">跳转收银台</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">排序</div>
              <Input
                type="number"
                value={providerForm.sortOrder}
                onChange={(event) => setProviderForm((current) => ({ ...current, sortOrder: event.target.value }))}
                className={adminInputClass}
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">支付宝通道 ID</div>
              <Input
                value={providerForm.cidAlipay}
                onChange={(event) => setProviderForm((current) => ({ ...current, cidAlipay: event.target.value }))}
                className={adminInputClass}
                placeholder="可选"
              />
            </label>
            <label className="space-y-2">
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">微信通道 ID</div>
              <Input
                value={providerForm.cidWxpay}
                onChange={(event) => setProviderForm((current) => ({ ...current, cidWxpay: event.target.value }))}
                className={adminInputClass}
                placeholder="可选"
              />
            </label>
            <div className={cn(adminSubPanelClass, "grid gap-3 p-4 md:col-span-2")}>
              <div className="text-sm font-medium text-[var(--app-text-secondary)]">可用支付方式</div>
              <div className="flex flex-wrap gap-4">
                {[
                  { key: "alipay", label: "支付宝" },
                  { key: "wxpay", label: "微信支付" },
                ].map((item) => (
                  <label key={item.key} className="flex items-center gap-2 text-sm text-[var(--app-text-secondary)]">
                    <Checkbox
                      checked={providerForm.methods.includes(item.key)}
                      onCheckedChange={(checked) => toggleProviderMethod(item.key, Boolean(checked))}
                    />
                    {item.label}
                  </label>
                ))}
              </div>
            </div>
            <label className={cn(adminSubPanelClass, "flex items-center justify-between gap-3 px-4 py-3 text-sm text-[var(--app-text-secondary)] md:col-span-2")}>
              <span>启用渠道</span>
              <Checkbox checked={providerForm.enabled} onCheckedChange={(checked) => setProviderForm((current) => ({ ...current, enabled: Boolean(checked) }))} />
            </label>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setProviderDialogOpen(false)} disabled={savingProvider}>取消</Button>
            <Button type="button" onClick={() => void saveProvider()} disabled={savingProvider}>
              {savingProvider ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(selectedOrder)} onOpenChange={(open) => {
        if (!open) {
          setSelectedOrder(null);
          setAuditLogs([]);
        }
      }}>
        <DialogContent className="w-[min(94vw,860px)] max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>订单详情</DialogTitle>
            <DialogDescription>{selectedOrder ? `${billingActionText(selectedOrder)} · ${statusText(selectedOrder.status)}` : ""}</DialogDescription>
          </DialogHeader>

          {selectedOrder ? (
            <div className="grid gap-4">
              <div className="grid gap-3 md:grid-cols-2">
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">订单</div>
                  <div className="mt-2 break-all text-sm font-semibold text-[var(--app-text-primary)]">{selectedOrder.outTradeNo}</div>
                  <div className="mt-1 break-all text-xs text-[var(--app-text-muted)]">{selectedOrder.id}</div>
                </div>
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">用户</div>
                  <div className="mt-2 text-sm font-semibold text-[var(--app-text-primary)]">{selectedOrder.username || "-"}</div>
                  <div className="mt-1 break-all text-xs text-[var(--app-text-muted)]">{selectedOrder.userEmail || selectedOrder.userId || "-"}</div>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-3">
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">套餐原价</div>
                  <div className="mt-2 text-lg font-semibold text-[var(--app-text-primary)]">{moneyText(selectedOrder.originalAmountCents || selectedOrder.amountCents, selectedOrder.currency)}</div>
                </div>
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">升级抵扣</div>
                  <div className="mt-2 text-lg font-semibold text-emerald-500 dark:text-emerald-300">{moneyText(selectedOrder.upgradeCreditCents || 0, selectedOrder.currency)}</div>
                </div>
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">实付金额</div>
                  <div className="mt-2 text-lg font-semibold text-[var(--app-accent-cyan)]">{moneyText(selectedOrder.amountCents, selectedOrder.currency)}</div>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-3">
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">点数</div>
                  <div className="mt-2 text-sm font-semibold text-[var(--app-text-primary)]">{numberText(selectedOrder.credits)}{selectedOrder.packageType === "subscription" ? " 订阅点" : ""}</div>
                </div>
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">订阅天数</div>
                  <div className="mt-2 text-sm font-semibold text-[var(--app-text-primary)]">{selectedOrder.packageType === "subscription" ? `${numberText(selectedOrder.durationDays || 0)} 天` : "-"}</div>
                </div>
                <div className={cn(adminSubPanelClass, "p-4")}>
                  <div className="text-xs text-[var(--app-text-muted)]">状态</div>
                  <div className="mt-2"><Badge variant={statusVariant(selectedOrder.status)}>{statusText(selectedOrder.status)}</Badge></div>
                </div>
              </div>

              <div className={cn(adminSubPanelClass, "grid gap-2 p-4 text-xs text-[var(--app-text-muted)] md:grid-cols-2")}>
                <div>创建时间：{formatDateTime(selectedOrder.createdAt)}</div>
                <div>过期时间：{formatDateTime(selectedOrder.expiresAt)}</div>
                <div>支付时间：{formatDateTime(selectedOrder.paidAt)}</div>
                <div>到账时间：{formatDateTime(selectedOrder.completedAt)}</div>
                <div>退款时间：{formatDateTime(selectedOrder.refundedAt)}</div>
                <div>渠道流水：{selectedOrder.providerTradeNo || "-"}</div>
              </div>

              <div>
                <div className="mb-2 text-sm font-semibold text-[var(--app-text-primary)]">审计日志</div>
                <div className="grid gap-2">
                  {auditLoading ? (
                    <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}><LoaderCircle className="mr-2 inline size-4 animate-spin" />读取中</div>
                  ) : auditLogs.length === 0 ? (
                    <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无日志</div>
                  ) : auditLogs.map((item) => (
                    <div key={item.id} className={cn(adminSubPanelClass, "grid gap-1 px-4 py-3 text-sm")}>
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="font-medium text-[var(--app-text-primary)]">{auditActionText(item.action)}</span>
                        <span className="text-xs text-[var(--app-text-muted)]">{formatDateTime(item.createdAt)}</span>
                      </div>
                      <div className="text-xs text-[var(--app-text-muted)]">操作人：{item.operator || "-"}</div>
                      {detailText(item.detail) ? (
                        <div className="break-all text-xs text-[var(--app-text-muted)]">{detailText(item.detail)}</div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(confirmAction)} onOpenChange={(open) => {
        if (!open) setConfirmAction(null);
      }}>
        <DialogContent className="w-[min(92vw,520px)]">
          <DialogHeader>
            <DialogTitle>{confirmAction ? actionText(confirmAction.action) : "确认操作"}</DialogTitle>
            <DialogDescription>
              {confirmAction ? `订单 ${confirmAction.order.outTradeNo}` : ""}
            </DialogDescription>
          </DialogHeader>
          {confirmAction ? (
            <div className={cn(adminSubPanelClass, "grid gap-2 p-4 text-sm text-[var(--app-text-secondary)]")}>
              <div>类型：{billingActionText(confirmAction.order)}</div>
              <div>用户：{confirmAction.order.username || confirmAction.order.userEmail || "-"}</div>
              <div>实付金额：{moneyText(confirmAction.order.amountCents, confirmAction.order.currency)}</div>
              {confirmAction.order.billingAction === "upgrade" && confirmAction.action === "complete" ? (
                <div className="text-amber-400">确认后会结束当前订阅，并从现在开始新订阅周期。</div>
              ) : null}
              {confirmAction.action === "refund" ? (
                <div className="text-amber-400">退款会回滚对应余额或订阅状态；升级订单的复杂回滚规则后续单独处理。</div>
              ) : null}
            </div>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirmAction(null)}>取消</Button>
            <Button type="button" onClick={() => confirmAction && void actOrder(confirmAction.order, confirmAction.action)} disabled={!confirmAction || actingOrderId === confirmAction.order.id}>
              {confirmAction && actingOrderId === confirmAction.order.id ? <LoaderCircle className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
              确认
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
