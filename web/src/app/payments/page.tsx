"use client";

import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, CreditCard, Eye, LoaderCircle, Pencil, RefreshCw, RotateCcw, Trash2, WalletCards, XCircle } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import { AppModal, AppSelect } from "@/components/app-controls";
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
  fetchBusinessSystemSettings,
  refundAdminBusinessPaymentOrder,
  updateAdminBusinessPaymentProvider,
  updateAdminBusinessPaymentPackage,
  type BusinessBillingLevel,
  type BusinessPaymentAuditLog,
  type BusinessPaymentOrder,
  type BusinessPaymentOrderStatus,
  type BusinessPaymentPackage,
  type BusinessPaymentProvider,
  type BusinessPaymentProviderInput,
  type BusinessSubscription,
  type PaginationMeta,
} from "@/lib/api";

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
  if (!page.total) return "0 / 0";
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
  if (Number.isNaN(date.getTime())) return value || "-";
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

function statusBadgeClass(status: string) {
  if (status === "completed") return "ok";
  if (status === "pending" || status === "paid") return "warn";
  if (status === "failed" || status === "refunded") return "fail";
  return "off";
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

function subscriptionStatusBadgeClass(status?: string) {
  if (status === "active") return "ok";
  if (status === "expired") return "warn";
  if (status === "cancelled") return "off";
  if (status === "upgraded") return "run";
  return "off";
}

function subscriptionRowStatusBadgeClass(item: BusinessSubscription) {
  if (item.status === "active") {
    const startsAt = new Date(item.startsAt || "");
    if (!Number.isNaN(startsAt.getTime()) && startsAt.getTime() > Date.now()) {
      return "warn";
    }
  }
  return subscriptionStatusBadgeClass(item.status);
}

function subscriptionCoverageExpiresAt(item: BusinessSubscription) {
  return item.coverageExpiresAt || item.expiresAt;
}

function packageTypeText(type?: string) {
  if (type === "subscription" || type === "monthly") return "订阅";
  return "余额";
}

function packageTypeBadgeClass(type?: string) {
  if (type === "subscription" || type === "monthly") return "run";
  return "off";
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
  levelTag: "",
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
  const [subscriptionLevels, setSubscriptionLevels] = useState<BusinessBillingLevel[]>([]);
  const [walletLevels, setWalletLevels] = useState<BusinessBillingLevel[]>([]);
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
  const [deletePackageTarget, setDeletePackageTarget] = useState<BusinessPaymentPackage | null>(null);
  const [deleteProviderTarget, setDeleteProviderTarget] = useState<BusinessPaymentProvider | null>(null);

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
      const [packagePayload, providerPayload, orderPayload, subscriptionPayload, settingsPayload] = await Promise.all([
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
        fetchBusinessSystemSettings(),
      ]);
      setPackages(packagePayload.items || []);
      setProviders(providerPayload.items || []);
      setOrders(orderPayload.items || []);
      setOrderPage(orderPayload.page || { ...orderPage, page: nextOrderPage });
      setSubscriptions(subscriptionPayload.items || []);
      setSubscriptionPage(subscriptionPayload.page || { ...subscriptionPage, page: nextSubscriptionPage });
      setSubscriptionLevels((settingsPayload.settings.billing.subscriptionLevels || []).filter((level) => level.enabled));
      setWalletLevels((settingsPayload.settings.billing.walletLevels || []).filter((level) => level.enabled));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取支付数据失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData(1, 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
      levelTag: item.levelTag || "",
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
      levelTag: packageForm.levelTag,
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

  const removePackage = async () => {
    if (!deletePackageTarget) return;
    setDeletingPackageId(deletePackageTarget.id);
    try {
      await deleteAdminBusinessPaymentPackage(deletePackageTarget.id);
      await loadData();
      toast.success("套餐已删除");
      setDeletePackageTarget(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除套餐失败");
    } finally {
      setDeletingPackageId("");
    }
  };

  const toggleProviderMethod = (method: string, checked: boolean) => {
    setProviderForm((current) => {
      const methods = new Set(current.methods);
      if (checked) methods.add(method);
      else methods.delete(method);
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

  const removeProvider = async () => {
    if (!deleteProviderTarget) return;
    setDeletingProviderId(deleteProviderTarget.id);
    try {
      await deleteAdminBusinessPaymentProvider(deleteProviderTarget.id);
      await loadData();
      toast.success("支付渠道已删除");
      setDeleteProviderTarget(null);
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
        icon={CreditCard}
        actions={
          <>
            <button className="app-btn" type="button" onClick={() => void loadData()} disabled={loading || savingPackage}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button className="app-btn-primary" type="button" onClick={openCreatePackage}>
              + 新增套餐
            </button>
          </>
        }
      />

      <section className="app-stats">
        <AdminStatCard label="订单数" value={numberText(stats.total)} sub={`待确认 ${numberText(stats.pending)}`} icon={CreditCard} color="text-cyan-300" />
        <AdminStatCard label="已完成" value={numberText(stats.completed)} sub="人工确认到账" icon={CheckCircle2} color="text-emerald-300" />
        <AdminStatCard label="完成金额" value={moneyText(stats.amountCents)} sub="按订单金额统计" icon={CreditCard} color="text-amber-300" />
        <AdminStatCard label="发放点数" value={numberText(stats.credits)} sub="含充值到账" icon={CheckCircle2} color="text-violet-300" />
      </section>

      <AdminPanel>
        <AdminSectionTitle title="套餐列表" action={<span className="panel-count">{packages.length}</span>} />
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                <th>套餐</th>
                <th>类型</th>
                <th>金额</th>
                <th>点数</th>
                <th>状态</th>
                <th style={{ textAlign: "right" }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {packages.length === 0 ? (
                <tr><td colSpan={6} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无套餐</td></tr>
              ) : packages.map((item) => (
                <tr key={item.id}>
                  <td>
                    <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.name}</div>
                    <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>
                      {item.description || "-"}
                      {item.packageType === "subscription" || item.packageType === "monthly" ? ` · ${Number(item.durationDays || 30).toLocaleString()} 天` : ""}
                      {item.levelTag ? ` · ${item.levelTag}` : ""}
                    </div>
                  </td>
                  <td><span className={`app-badge ${packageTypeBadgeClass(item.packageType)}`}>{packageTypeText(item.packageType)}</span></td>
                  <td>{moneyText(item.amountCents, item.currency)}</td>
                  <td>{numberText(item.credits)}{item.packageType === "subscription" || item.packageType === "monthly" ? " 订阅点" : ""}</td>
                  <td><span className={`app-badge ${item.enabled ? "ok" : "off"}`}>{item.enabled ? "启用" : "停用"}</span></td>
                  <td>
                    <div style={{ display: "flex", justifyContent: "flex-end" }}>
                      <div className="app-act">
                        <button type="button" onClick={() => openEditPackage(item)} aria-label="编辑套餐">
                          <Pencil className="size-3.5" />
                        </button>
                        <button type="button" className="danger" onClick={() => setDeletePackageTarget(item)} disabled={deletingPackageId === item.id} aria-label="删除套餐">
                          {deletingPackageId === item.id ? <LoaderCircle className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </AdminPanel>

      <AdminPanel>
        <AdminSectionTitle
          title="支付渠道"
          action={
            <button className="app-btn-primary" type="button" onClick={openCreateProvider} style={{ height: 30, padding: "0 12px", fontSize: 12 }}>
              + 新增渠道
            </button>
          }
        />
        <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", padding: 16 }}>
          {providers.length === 0 ? (
            <div style={{ padding: "12px 16px", fontSize: 13, color: "var(--app-text-muted)" }}>暂无支付渠道</div>
          ) : providers.map((item) => (
            <div key={item.id} style={{
              padding: 16,
              borderRadius: 12,
              border: "1px solid var(--app-border)",
              background: "var(--app-bg-surface)",
            }}>
              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14, fontWeight: 600, color: "var(--app-text-primary)" }}>
                    <WalletCards className="size-4" style={{ color: "var(--app-accent-cyan)" }} />
                    {item.name}
                  </div>
                  <div style={{ marginTop: 4, fontSize: 11, color: "var(--app-text-muted)" }}>{item.providerKey}</div>
                </div>
                <span className={`app-badge ${item.enabled ? "ok" : "off"}`}>{item.enabled ? "启用" : "停用"}</span>
              </div>
              <div style={{ marginTop: 12, display: "flex", flexWrap: "wrap", gap: 6 }}>
                {(item.supportedMethods || []).map((method) => (
                  <span key={method} className="app-badge off">{paymentMethodText(method)}</span>
                ))}
              </div>
              {item.providerKey === "easypay" ? (
                <div style={{ marginTop: 12, fontSize: 11, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                  <div>API：{item.config?.apiBase || "-"}</div>
                  <div>商户 ID：{item.config?.pid || "-"}</div>
                  <div>模式：{item.config?.paymentMode === "popup" ? "跳转收银台" : "二维码接口"}</div>
                </div>
              ) : (
                <div style={{ marginTop: 12, fontSize: 11, lineHeight: 1.6, color: "var(--app-text-muted)" }}>内置人工确认渠道，不需要配置密钥。</div>
              )}
              {item.id !== "manual" ? (
                <div style={{ marginTop: 14, display: "flex", justifyContent: "flex-end" }}>
                  <div className="app-act">
                    <button type="button" onClick={() => openEditProvider(item)} aria-label="编辑渠道">
                      <Pencil className="size-3.5" />
                    </button>
                    <button type="button" className="danger" onClick={() => setDeleteProviderTarget(item)} disabled={deletingProviderId === item.id} aria-label="删除渠道">
                      {deletingProviderId === item.id ? <LoaderCircle className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </AdminPanel>

      <AdminPanel>
        <AdminSectionTitle
          title="订阅列表"
          action={<span className="panel-count">{paginationText(subscriptionPage)}</span>}
        />
        <div className="app-toolbar" style={{ border: 0, borderRadius: 0, boxShadow: "none", borderBottom: "1px solid var(--app-border)", backdropFilter: "none" }}>
          <AppSelect
            value={subscriptionStatusFilter}
            onChange={(v) => setSubscriptionStatusFilter(v as SubscriptionStatusFilter)}
            options={[
              { value: "all", label: "全部状态" },
              { value: "active", label: "有效" },
              { value: "upgraded", label: "已升级" },
              { value: "expired", label: "已过期" },
              { value: "cancelled", label: "已取消" },
            ]}
          />
          <AppSelect
            value={subscriptionWindowFilter}
            onChange={(v) => setSubscriptionWindowFilter(v as SubscriptionWindowFilter)}
            options={[
              { value: "all", label: "全部有效期" },
              { value: "current", label: "当前可用" },
              { value: "future", label: "未来生效" },
              { value: "history", label: "历史记录" },
            ]}
          />
          <input
            className="app-input"
            type="search"
            value={subscriptionSearch}
            onChange={(event) => setSubscriptionSearch(event.target.value)}
            onKeyDown={(event) => { if (event.key === "Enter") void loadData(orderPage.page, 1); }}
            placeholder="搜索用户、邮箱、套餐或订单"
            style={{ flex: 1, minWidth: 200 }}
          />
          <button className="app-btn" type="button" onClick={() => void loadData(orderPage.page, 1)} disabled={loading}>
            筛选
          </button>
        </div>
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>订阅</th>
                <th>额度</th>
                <th>状态</th>
                <th>有效期</th>
                <th>来源订单</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={6} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                  <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
                  <div>读取中</div>
                </td></tr>
              ) : subscriptions.length === 0 ? (
                <tr><td colSpan={6} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无匹配订阅</td></tr>
              ) : subscriptions.map((item) => (
                <tr key={item.id}>
                  <td style={{ whiteSpace: "normal" }}>
                    <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.username || "-"}</div>
                    <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }} title={item.userEmail || ""}>{item.userEmail || item.userId || "-"}</div>
                  </td>
                  <td style={{ whiteSpace: "normal" }}>
                    <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.packageName || "订阅"}</div>
                    <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>{numberText(item.durationDays || 0)} 天</div>
                  </td>
                  <td>
                    <div style={{ fontWeight: 600, color: "var(--app-text-primary)" }}>{numberText(item.creditsLeft)} / {numberText(item.creditsTotal)}</div>
                    <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>已用 {numberText(item.creditsUsed)}</div>
                  </td>
                  <td><span className={`app-badge ${subscriptionRowStatusBadgeClass(item)}`}>{subscriptionRowStatusText(item)}</span></td>
                  <td>
                    <div>{formatDateTime(item.startsAt)}</div>
                    <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>本周期至 {formatDateTime(item.expiresAt)}</div>
                    {item.coverageExpiresAt && item.coverageExpiresAt !== item.expiresAt ? (
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>续订至 {formatDateTime(subscriptionCoverageExpiresAt(item))}</div>
                    ) : null}
                  </td>
                  <td style={{ fontSize: 11, color: "var(--app-text-secondary)", maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis" }} title={item.orderId || ""}>{item.orderId || "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="app-pager">
          <span className="info">第 {subscriptionPage.page} / {subscriptionTotalPages} 页</span>
          <div className="pages">
            <button type="button" disabled={loading || subscriptionPage.page <= 1} onClick={() => void loadData(orderPage.page, Math.max(1, subscriptionPage.page - 1))}>上一页</button>
            <button type="button" disabled={loading || subscriptionPage.page >= subscriptionTotalPages} onClick={() => void loadData(orderPage.page, Math.min(subscriptionTotalPages, subscriptionPage.page + 1))}>下一页</button>
          </div>
        </div>
      </AdminPanel>

      <AdminPanel>
        <AdminSectionTitle
          title="订单列表"
          action={
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <label style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, color: "var(--app-text-muted)" }}>
                订单返利比例
                <input
                  className="app-input"
                  type="number"
                  min={0}
                  max={10000}
                  value={commissionRateBps}
                  onChange={(event) => setCommissionRateBps(event.target.value)}
                  style={{ width: 80, height: 28, padding: "0 8px", fontSize: 12 }}
                />
                bps
              </label>
              <span className="panel-count">{paginationText(orderPage)}</span>
            </div>
          }
        />
        <div className="app-toolbar" style={{ border: 0, borderRadius: 0, boxShadow: "none", borderBottom: "1px solid var(--app-border)", backdropFilter: "none" }}>
          <AppSelect
            value={orderStatusFilter}
            onChange={(v) => setOrderStatusFilter(v as OrderStatusFilter)}
            options={[
              { value: "all", label: "全部状态" },
              { value: "pending", label: "待确认" },
              { value: "paid", label: "已支付" },
              { value: "completed", label: "已到账" },
              { value: "cancelled", label: "已取消" },
              { value: "refunded", label: "已退款" },
              { value: "expired", label: "已过期" },
              { value: "failed", label: "失败" },
            ]}
          />
          <AppSelect
            value={orderKindFilter}
            onChange={(v) => setOrderKindFilter(v as OrderKindFilter)}
            options={[
              { value: "all", label: "全部类型" },
              { value: "balance", label: "余额充值" },
              { value: "subscription", label: "开通订阅" },
              { value: "renewal", label: "续订" },
              { value: "upgrade", label: "升级补差价" },
            ]}
          />
          <input
            className="app-input"
            type="search"
            value={orderSearch}
            onChange={(event) => setOrderSearch(event.target.value)}
            onKeyDown={(event) => { if (event.key === "Enter") void loadData(1, subscriptionPage.page); }}
            placeholder="搜索用户、邮箱、订单号"
            style={{ flex: 1, minWidth: 200 }}
          />
          <button className="app-btn" type="button" onClick={() => void loadData(1, subscriptionPage.page)} disabled={loading}>
            筛选
          </button>
        </div>
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                <th>订单</th>
                <th>用户</th>
                <th>金额</th>
                <th>点数</th>
                <th>状态</th>
                <th>时间</th>
                <th style={{ textAlign: "right" }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={7} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                  <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
                  <div>读取中</div>
                </td></tr>
              ) : orders.length === 0 ? (
                <tr><td colSpan={7} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无匹配订单</td></tr>
              ) : orders.map((item) => {
                const acting = actingOrderId === item.id;
                return (
                  <tr key={item.id}>
                    <td style={{ whiteSpace: "normal" }}>
                      <div style={{ fontWeight: 500, color: "var(--app-text-primary)", maxWidth: 240, overflow: "hidden", textOverflow: "ellipsis" }} title={item.outTradeNo}>{item.outTradeNo}</div>
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>{item.id}</div>
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>{billingActionText(item)}</div>
                    </td>
                    <td style={{ whiteSpace: "normal" }}>
                      <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.username || "-"}</div>
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }} title={item.userEmail || ""}>{item.userEmail || "-"}</div>
                    </td>
                    <td>
                      <div>{moneyText(item.amountCents, item.currency)}</div>
                      {item.billingAction === "upgrade" && Number(item.upgradeCreditCents || 0) > 0 ? (
                        <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>已抵扣 {moneyText(item.upgradeCreditCents, item.currency)}</div>
                      ) : null}
                    </td>
                    <td>{numberText(item.credits)}</td>
                    <td><span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusText(item.status)}</span></td>
                    <td style={{ fontSize: 12 }}>{formatDateTime(item.createdAt)}</td>
                    <td>
                      <div style={{ display: "flex", justifyContent: "flex-end" }}>
                        <div className="app-act">
                          <button type="button" onClick={() => void openOrderDetail(item)} aria-label="详情">
                            <Eye className="size-3.5" />
                            详情
                          </button>
                          <button type="button" disabled={acting || (item.status !== "pending" && item.status !== "paid")} onClick={() => setConfirmAction({ order: item, action: "complete" })}>
                            {acting ? <LoaderCircle className="size-3.5 animate-spin" /> : <CheckCircle2 className="size-3.5" />}
                            确认
                          </button>
                          <button type="button" className="warn" disabled={acting || item.status !== "pending"} onClick={() => setConfirmAction({ order: item, action: "cancel" })}>
                            <XCircle className="size-3.5" />
                            取消
                          </button>
                          <button type="button" className="danger" disabled={acting || item.status !== "completed"} onClick={() => setConfirmAction({ order: item, action: "refund" })}>
                            <RotateCcw className="size-3.5" />
                            退款
                          </button>
                        </div>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="app-pager">
          <span className="info">第 {orderPage.page} / {orderTotalPages} 页</span>
          <div className="pages">
            <button type="button" disabled={loading || orderPage.page <= 1} onClick={() => void loadData(Math.max(1, orderPage.page - 1), subscriptionPage.page)}>上一页</button>
            <button type="button" disabled={loading || orderPage.page >= orderTotalPages} onClick={() => void loadData(Math.min(orderTotalPages, orderPage.page + 1), subscriptionPage.page)}>下一页</button>
          </div>
        </div>
      </AdminPanel>

      <AppModal
        open={packageDialogOpen}
        onClose={() => { setPackageDialogOpen(false); setPackageForm(emptyPackageForm); }}
        title={packageForm.id ? "编辑套餐" : "新增套餐"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setPackageDialogOpen(false)}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void savePackage()} disabled={savingPackage}>
              {savingPackage ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </button>
          </>
        }
      >
        <div className="app-form-grid">
          <div className="app-fld">
            <span className="fl">套餐类型</span>
            <AppSelect
              value={packageForm.packageType}
              onChange={(v) => setPackageForm((current) => ({ ...current, packageType: v as PaymentPackageType }))}
              options={[{ value: "balance", label: "余额" }, { value: "subscription", label: "订阅" }]}
            />
          </div>
          <div className="app-fld">
            <span className="fl">名称</span>
            <input
              className="app-input"
              value={packageForm.name}
              onChange={(event) => setPackageForm((current) => ({ ...current, name: event.target.value }))}
              placeholder={packageForm.packageType === "subscription" ? "例如 月度会员" : "例如 100 点"}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">说明</span>
            <input
              className="app-input"
              value={packageForm.description}
              onChange={(event) => setPackageForm((current) => ({ ...current, description: event.target.value }))}
              placeholder={packageForm.packageType === "subscription" ? "订阅套餐，人工确认后生效" : "人工确认后到账"}
            />
          </div>
          <div className="app-fld">
            <span className="fl">金额（分）</span>
            <input
              className="app-input"
              type="number"
              min={1}
              value={packageForm.amountCents}
              onChange={(event) => setPackageForm((current) => ({ ...current, amountCents: event.target.value }))}
            />
          </div>
          {packageForm.packageType === "subscription" ? (
            <div className="app-fld">
              <span className="fl">订阅天数</span>
              <input
                className="app-input"
                type="number"
                min={1}
                value={packageForm.durationDays}
                onChange={(event) => setPackageForm((current) => ({ ...current, durationDays: event.target.value }))}
              />
            </div>
          ) : null}
          <div className="app-fld">
            <span className="fl">{packageForm.packageType === "subscription" ? "订阅等级" : "充值等级"}</span>
            <AppSelect
              value={packageForm.levelTag}
              onChange={(value) => setPackageForm((current) => ({ ...current, levelTag: value }))}
              options={[
                { value: "", label: "不绑定等级" },
                ...(packageForm.packageType === "subscription" ? subscriptionLevels : walletLevels).map((level) => ({
                  value: level.tag,
                  label: `${level.name} · ${level.tag}`,
                })),
              ]}
            />
          </div>
          <div className="app-fld">
            <span className="fl">点数</span>
            <input
              className="app-input"
              type="number"
              min={1}
              value={packageForm.credits}
              onChange={(event) => setPackageForm((current) => ({ ...current, credits: event.target.value }))}
            />
          </div>
          <div className="app-fld">
            <span className="fl">币种</span>
            <input
              className="app-input"
              value={packageForm.currency}
              onChange={(event) => setPackageForm((current) => ({ ...current, currency: event.target.value }))}
            />
          </div>
          <div className="app-fld">
            <span className="fl">排序</span>
            <input
              className="app-input"
              type="number"
              value={packageForm.sortOrder}
              onChange={(event) => setPackageForm((current) => ({ ...current, sortOrder: event.target.value }))}
            />
          </div>
          <div className="app-fld full switch-row">
            <div className="fl-wrap">
              <span className="fl">启用套餐</span>
              <span className="fd">关闭后用户不可见</span>
            </div>
            <button
              type="button"
              className={`app-switch ${packageForm.enabled ? "on" : ""}`}
              aria-label="启用套餐"
              onClick={() => setPackageForm((current) => ({ ...current, enabled: !current.enabled }))}
            />
          </div>
        </div>
      </AppModal>

      <AppModal
        open={providerDialogOpen}
        onClose={() => { if (!savingProvider) { setProviderDialogOpen(false); setProviderForm(emptyProviderForm); } }}
        title={providerForm.id ? "编辑支付渠道" : "新增支付渠道"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setProviderDialogOpen(false)} disabled={savingProvider}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void saveProvider()} disabled={savingProvider}>
              {savingProvider ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </button>
          </>
        }
      >
        <div style={{ padding: "0 18px 12px", fontSize: 12, color: "var(--app-text-muted)" }}>
          当前先支持 EasyPay。密钥保存后不会明文回显，编辑时留空表示不修改。
        </div>
        <div className="app-form-grid">
          <div className="app-fld">
            <span className="fl">渠道类型</span>
            <AppSelect
              value={providerForm.providerKey}
              onChange={(v) => setProviderForm((current) => ({ ...current, providerKey: v }))}
              options={[{ value: "easypay", label: "EasyPay" }]}
            />
          </div>
          <div className="app-fld">
            <span className="fl">名称</span>
            <input className="app-input" value={providerForm.name} onChange={(event) => setProviderForm((current) => ({ ...current, name: event.target.value }))} placeholder="例如 易支付" />
          </div>
          <div className="app-fld full">
            <span className="fl">API 地址</span>
            <input className="app-input" value={providerForm.apiBase} onChange={(event) => setProviderForm((current) => ({ ...current, apiBase: event.target.value }))} placeholder="https://pay.example.com" />
          </div>
          <div className="app-fld">
            <span className="fl">商户 ID</span>
            <input className="app-input" value={providerForm.pid} onChange={(event) => setProviderForm((current) => ({ ...current, pid: event.target.value }))} placeholder="pid" />
          </div>
          <div className="app-fld">
            <span className="fl">商户密钥</span>
            <input className="app-input" value={providerForm.pkey} onChange={(event) => setProviderForm((current) => ({ ...current, pkey: event.target.value }))} placeholder={providerForm.id ? "留空保持不变" : "pkey"} />
          </div>
          <div className="app-fld">
            <span className="fl">支付模式</span>
            <AppSelect
              value={providerForm.paymentMode}
              onChange={(v) => setProviderForm((current) => ({ ...current, paymentMode: v }))}
              options={[{ value: "qrcode", label: "二维码接口" }, { value: "popup", label: "跳转收银台" }]}
            />
          </div>
          <div className="app-fld">
            <span className="fl">排序</span>
            <input className="app-input" type="number" value={providerForm.sortOrder} onChange={(event) => setProviderForm((current) => ({ ...current, sortOrder: event.target.value }))} />
          </div>
          <div className="app-fld">
            <span className="fl">支付宝通道 ID</span>
            <input className="app-input" value={providerForm.cidAlipay} onChange={(event) => setProviderForm((current) => ({ ...current, cidAlipay: event.target.value }))} placeholder="可选" />
          </div>
          <div className="app-fld">
            <span className="fl">微信通道 ID</span>
            <input className="app-input" value={providerForm.cidWxpay} onChange={(event) => setProviderForm((current) => ({ ...current, cidWxpay: event.target.value }))} placeholder="可选" />
          </div>
          <div className="app-fld full">
            <span className="fl">可用支付方式</span>
            <div style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
              {[
                { key: "alipay", label: "支付宝" },
                { key: "wxpay", label: "微信支付" },
              ].map((item) => (
                <label key={item.key} style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--app-text-secondary)", cursor: "pointer" }}>
                  <input
                    type="checkbox"
                    checked={providerForm.methods.includes(item.key)}
                    onChange={(event) => toggleProviderMethod(item.key, event.target.checked)}
                  />
                  {item.label}
                </label>
              ))}
            </div>
          </div>
          <div className="app-fld full switch-row">
            <div className="fl-wrap">
              <span className="fl">启用渠道</span>
              <span className="fd">关闭后不再用于新订单</span>
            </div>
            <button
              type="button"
              className={`app-switch ${providerForm.enabled ? "on" : ""}`}
              aria-label="启用渠道"
              onClick={() => setProviderForm((current) => ({ ...current, enabled: !current.enabled }))}
            />
          </div>
        </div>
      </AppModal>

      <AppModal
        open={!!selectedOrder}
        onClose={() => { setSelectedOrder(null); setAuditLogs([]); }}
        title="订单详情"
        footer={
          <button className="app-btn" type="button" onClick={() => { setSelectedOrder(null); setAuditLogs([]); }}>关闭</button>
        }
      >
        {selectedOrder ? (
          <div style={{ padding: "0 18px 16px", display: "grid", gap: 14 }}>
            <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>
              {billingActionText(selectedOrder)} · {statusText(selectedOrder.status)}
            </div>

            <div style={{ display: "grid", gap: 10, gridTemplateColumns: "1fr 1fr" }}>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>订单</small>
                  <b style={{ fontSize: 13, wordBreak: "break-all", display: "block", marginTop: 4 }}>{selectedOrder.outTradeNo}</b>
                  <small style={{ display: "block", marginTop: 2, wordBreak: "break-all" }}>{selectedOrder.id}</small>
                </div>
              </div>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>用户</small>
                  <b style={{ fontSize: 13, display: "block", marginTop: 4 }}>{selectedOrder.username || "-"}</b>
                  <small style={{ display: "block", marginTop: 2, wordBreak: "break-all" }}>{selectedOrder.userEmail || selectedOrder.userId || "-"}</small>
                </div>
              </div>
            </div>

            <div style={{ display: "grid", gap: 10, gridTemplateColumns: "1fr 1fr 1fr" }}>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>套餐原价</small>
                  <b style={{ fontSize: 16 }}>{moneyText(selectedOrder.originalAmountCents || selectedOrder.amountCents, selectedOrder.currency)}</b>
                </div>
              </div>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>升级抵扣</small>
                  <b style={{ fontSize: 16, color: "#6ee7b7" }}>{moneyText(selectedOrder.upgradeCreditCents || 0, selectedOrder.currency)}</b>
                </div>
              </div>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>实付金额</small>
                  <b style={{ fontSize: 16, color: "var(--app-accent-cyan)" }}>{moneyText(selectedOrder.amountCents, selectedOrder.currency)}</b>
                </div>
              </div>
            </div>

            <div style={{ display: "grid", gap: 10, gridTemplateColumns: "1fr 1fr 1fr" }}>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>点数</small>
                  <b style={{ fontSize: 13 }}>{numberText(selectedOrder.credits)}{selectedOrder.packageType === "subscription" ? " 订阅点" : ""}</b>
                </div>
              </div>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>订阅天数</small>
                  <b style={{ fontSize: 13 }}>{selectedOrder.packageType === "subscription" ? `${numberText(selectedOrder.durationDays || 0)} 天` : "-"}</b>
                </div>
              </div>
              <div className="app-stat" style={{ padding: 14 }}>
                <div>
                  <small>状态</small>
                  <b style={{ display: "block", marginTop: 4 }}>
                    <span className={`app-badge ${statusBadgeClass(selectedOrder.status)}`}>{statusText(selectedOrder.status)}</span>
                  </b>
                </div>
              </div>
            </div>

            <div style={{
              padding: 14,
              borderRadius: 10,
              border: "1px solid var(--app-border)",
              background: "var(--app-bg-surface)",
              display: "grid",
              gap: 6,
              gridTemplateColumns: "1fr 1fr",
              fontSize: 11.5,
              color: "var(--app-text-muted)",
            }}>
              <div>创建时间：{formatDateTime(selectedOrder.createdAt)}</div>
              <div>过期时间：{formatDateTime(selectedOrder.expiresAt)}</div>
              <div>支付时间：{formatDateTime(selectedOrder.paidAt)}</div>
              <div>到账时间：{formatDateTime(selectedOrder.completedAt)}</div>
              <div>退款时间：{formatDateTime(selectedOrder.refundedAt)}</div>
              <div>渠道流水：{selectedOrder.providerTradeNo || "-"}</div>
            </div>

            <div>
              <div style={{ fontSize: 13, fontWeight: 600, color: "var(--app-text-primary)", marginBottom: 8 }}>审计日志</div>
              <div style={{ display: "grid", gap: 8 }}>
                {auditLoading ? (
                  <div style={{ padding: "12px 14px", borderRadius: 10, border: "1px solid var(--app-border)", background: "var(--app-bg-surface)", fontSize: 13, color: "var(--app-text-muted)" }}>
                    <LoaderCircle className="size-4 animate-spin" style={{ display: "inline-block", verticalAlign: "middle", marginRight: 6 }} />
                    读取中
                  </div>
                ) : auditLogs.length === 0 ? (
                  <div style={{ padding: "12px 14px", borderRadius: 10, border: "1px solid var(--app-border)", background: "var(--app-bg-surface)", fontSize: 13, color: "var(--app-text-muted)" }}>暂无日志</div>
                ) : auditLogs.map((item) => (
                  <div key={item.id} style={{ padding: "10px 14px", borderRadius: 10, border: "1px solid var(--app-border)", background: "var(--app-bg-surface)", display: "grid", gap: 4, fontSize: 13 }}>
                    <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
                      <span style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{auditActionText(item.action)}</span>
                      <span style={{ fontSize: 11, color: "var(--app-text-muted)" }}>{formatDateTime(item.createdAt)}</span>
                    </div>
                    <div style={{ fontSize: 11, color: "var(--app-text-muted)" }}>操作人：{item.operator || "-"}</div>
                    {detailText(item.detail) ? (
                      <div style={{ fontSize: 11, color: "var(--app-text-muted)", wordBreak: "break-all" }}>{detailText(item.detail)}</div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : null}
      </AppModal>

      <AppModal
        open={!!confirmAction}
        onClose={() => setConfirmAction(null)}
        title={confirmAction ? actionText(confirmAction.action) : "确认操作"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setConfirmAction(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => confirmAction && void actOrder(confirmAction.order, confirmAction.action)}
              disabled={!confirmAction || actingOrderId === confirmAction?.order.id}
              style={confirmAction?.action === "refund" ? { background: "linear-gradient(135deg, #ef4444, #dc2626)" } : undefined}
            >
              {confirmAction && actingOrderId === confirmAction.order.id ? <LoaderCircle className="size-4 animate-spin" /> : null}
              确认
            </button>
          </>
        }
      >
        {confirmAction ? (
          <div style={{ padding: "12px 18px 18px", display: "grid", gap: 8, fontSize: 13, color: "var(--app-text-secondary)" }}>
            <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>订单 {confirmAction.order.outTradeNo}</div>
            <div style={{
              padding: 14,
              borderRadius: 10,
              border: "1px solid var(--app-border)",
              background: "var(--app-bg-surface)",
              display: "grid",
              gap: 6,
            }}>
              <div>类型：{billingActionText(confirmAction.order)}</div>
              <div>用户：{confirmAction.order.username || confirmAction.order.userEmail || "-"}</div>
              <div>实付金额：{moneyText(confirmAction.order.amountCents, confirmAction.order.currency)}</div>
              {confirmAction.order.billingAction === "upgrade" && confirmAction.action === "complete" ? (
                <div style={{ color: "#fcd34d" }}>确认后会结束当前订阅，并从现在开始新订阅周期。</div>
              ) : null}
              {confirmAction.action === "refund" ? (
                <div style={{ color: "#fcd34d" }}>退款会回滚对应余额或订阅状态；升级订单的复杂回滚规则后续单独处理。</div>
              ) : null}
            </div>
          </div>
        ) : null}
      </AppModal>

      <AppModal
        open={!!deletePackageTarget}
        onClose={() => setDeletePackageTarget(null)}
        title="删除套餐"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeletePackageTarget(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void removePackage()}
              style={{ background: "linear-gradient(135deg, #ef4444, #dc2626)" }}
              disabled={!!deletingPackageId}
            >
              确认删除
            </button>
          </>
        }
      >
        <div style={{ padding: "16px 18px", fontSize: 13.5, color: "var(--app-text-secondary)", lineHeight: 1.7 }}>
          确认删除套餐 <b style={{ color: "var(--app-text-primary)" }}>「{deletePackageTarget?.name}」</b> 吗？已有订单的套餐不能删除，可改为停用。
        </div>
      </AppModal>

      <AppModal
        open={!!deleteProviderTarget}
        onClose={() => setDeleteProviderTarget(null)}
        title="删除支付渠道"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeleteProviderTarget(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void removeProvider()}
              style={{ background: "linear-gradient(135deg, #ef4444, #dc2626)" }}
              disabled={!!deletingProviderId}
            >
              确认删除
            </button>
          </>
        }
      >
        <div style={{ padding: "16px 18px", fontSize: 13.5, color: "var(--app-text-secondary)", lineHeight: 1.7 }}>
          确认删除支付渠道 <b style={{ color: "var(--app-text-primary)" }}>「{deleteProviderTarget?.name}」</b> 吗？待处理订单会阻止删除。
        </div>
      </AppModal>
    </AdminPage>
  );
}
