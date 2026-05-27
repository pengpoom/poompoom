"use client";

import { useEffect, useState } from "react";
import { ArrowRight, CalendarDays, CheckCircle2, Coins, Copy, CreditCard, Gift, LoaderCircle, RefreshCw, Send, Sparkles, TicketCheck, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { adminInputClass, adminSubPanelClass, adminTableBodyClass, adminTableClass, adminTableHeadClass, adminTableRowClass } from "@/components/admin-styles";
import { BUSINESS_CREDIT_CHANGED_EVENT } from "@/components/app-shell-nav";
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
import {
  fetchBusinessAffiliateSummary,
  fetchBusinessCredit,
  fetchBusinessCreditLedger,
  fetchBusinessPaymentMethods,
  fetchBusinessPaymentOrders,
  fetchBusinessPaymentPackages,
  fetchBusinessSubscription,
  createBusinessPaymentOrder,
  redeemBusinessCode,
  type BusinessAffiliateSummary,
  type BusinessCreditLedgerEntry,
  type BusinessCreditSummary,
  type BusinessPaymentOrder,
  type BusinessPaymentPackage,
  type BusinessPaymentMethod,
  type BusinessSubscription,
  type PaginationMeta,
} from "@/lib/api";
import { cn } from "@/lib/utils";

function numberText(value?: number | null) {
  return Number(value || 0).toLocaleString();
}

function formatDateTime(value?: string) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return date.toLocaleString();
}

function ledgerReasonText(reason: string) {
  if (reason === "registration_default") return "注册送点";
  if (reason === "registration_promo") return "优惠码赠送";
  if (reason === "affiliate_registration") return "邀请注册奖励";
  if (reason === "payment_recharge") return "充值到账";
  if (reason === "payment_refund") return "充值退款";
  if (reason === "affiliate_order_commission") return "订单邀请返利";
  if (reason === "affiliate_order_commission_reversal") return "订单返利退回";
  if (reason === "subscription_credit_reserve") return "订阅生图扣点";
  if (reason === "subscription_credit_refund") return "订阅失败退回";
  if (reason === "redeem_code") return "兑换码到账";
  if (reason === "admin_recharge") return "管理员充值";
  if (reason === "admin_refund") return "管理员扣减";
  if (reason === "admin_adjustment") return "管理员调整";
  if (reason === "new_user_default") return "新用户默认余额";
  return reason || "-";
}

function ledgerAccountText(item: BusinessCreditLedgerEntry) {
  if (item.source_type === "subscription" || item.reason === "subscription_credit_reserve" || item.reason === "subscription_credit_refund") {
    return "订阅点数";
  }
  return "普通余额";
}

function ledgerBalanceLabel(item: BusinessCreditLedgerEntry) {
  return ledgerAccountText(item) === "订阅点数" ? "订阅剩余" : "余额";
}

function maskedUserText(username?: string, email?: string, uid?: number) {
  const name = String(username || "").trim();
  if (name) {
    return name;
  }
  const rawEmail = String(email || "").trim();
  const at = rawEmail.indexOf("@");
  if (at > 1) {
    return `${rawEmail.slice(0, 2)}***${rawEmail.slice(at)}`;
  }
  if (Number(uid || 0) > 0) {
    return `用户 ${uid}`;
  }
  return "-";
}

function defaultLedgerPage(): PaginationMeta {
  return { page: 1, pageSize: 10, total: 0 };
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

function orderStatusText(status: string) {
  if (status === "pending") return "待确认";
  if (status === "paid") return "已支付";
  if (status === "completed") return "已到账";
  if (status === "cancelled") return "已取消";
  if (status === "refunded") return "已退款";
  if (status === "expired") return "已过期";
  if (status === "failed") return "失败";
  return status || "-";
}

function orderBillingText(order: BusinessPaymentOrder) {
  if (order.billingAction === "upgrade") return "升级补差价";
  if (order.billingAction === "renewal") return "续订";
  if (isSubscriptionPackage(order)) return "开通订阅";
  return "余额充值";
}

function packageTypeText(type?: string) {
  if (type === "subscription" || type === "monthly") return "订阅";
  return "余额";
}

function subscriptionStatusText(subscription?: BusinessSubscription | null) {
  if (!subscription?.id) return "未订阅";
  if (subscription.active) return "订阅中";
  if (subscription.status === "cancelled") return "已取消";
  if (subscription.status === "expired") return "已过期";
  if (subscription.status === "upgraded") return "已升级";
  return "未订阅";
}

function isSubscriptionPackage(item: Pick<BusinessPaymentPackage, "packageType">) {
  return item.packageType === "subscription" || item.packageType === "monthly";
}

function subscriptionCoverageExpiresAt(subscription?: BusinessSubscription | null) {
  return subscription?.coverageExpiresAt || subscription?.expiresAt;
}

function packageActionText(item: BusinessPaymentPackage, currentPackage?: BusinessPaymentPackage, subscription?: BusinessSubscription | null) {
  if (!subscription?.active) {
    return { label: "开通订阅", disabled: false, hint: "人工确认后生效。" };
  }
  if (!currentPackage) {
    return { label: "创建订阅订单", disabled: false, hint: "当前套餐不可见，最终价格以后端校验为准。" };
  }
  if (item.id === currentPackage.id) {
    return { label: "续订", disabled: false, hint: "确认后接在当前订阅到期后生效。" };
  }
  if (item.amountCents > currentPackage.amountCents) {
    return { label: "升级补差价", disabled: false, hint: "按剩余时间和剩余订阅点抵扣旧套餐价值。" };
  }
  return { label: "本周期不可降级", disabled: true, hint: "更低套餐需要等当前订阅到期后再开通。" };
}

type ConfirmAction = {
  title: string;
  description: string;
  confirmText?: string;
  payment?: boolean;
  run: (paymentMethod?: string) => void | Promise<void>;
};

export default function CreditsPage() {
  const [credit, setCredit] = useState<BusinessCreditSummary | null>(null);
  const [affiliate, setAffiliate] = useState<BusinessAffiliateSummary | null>(null);
  const [subscription, setSubscription] = useState<BusinessSubscription | null>(null);
  const [packages, setPackages] = useState<BusinessPaymentPackage[]>([]);
  const [paymentMethods, setPaymentMethods] = useState<BusinessPaymentMethod[]>([]);
  const [orders, setOrders] = useState<BusinessPaymentOrder[]>([]);
  const [ledgerItems, setLedgerItems] = useState<BusinessCreditLedgerEntry[]>([]);
  const [ledgerPage, setLedgerPage] = useState<PaginationMeta>(defaultLedgerPage());
  const [redeemCode, setRedeemCode] = useState("");
  const [loading, setLoading] = useState(true);
  const [redeeming, setRedeeming] = useState(false);
  const [creatingOrderId, setCreatingOrderId] = useState("");
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null);
  const [selectedPaymentMethod, setSelectedPaymentMethod] = useState("manual");
  const [confirming, setConfirming] = useState(false);
  const subscriptionPackages = packages.filter(isSubscriptionPackage);
  const balancePackages = packages.filter((item) => !isSubscriptionPackage(item));
  const currentSubscriptionPackage = subscription?.active ? subscriptionPackages.find((item) => item.id === subscription.packageId) : undefined;
  const primaryRenewPackage = currentSubscriptionPackage || (!subscription?.active ? subscriptionPackages[0] : undefined);

  const loadData = async (nextLedgerPage = ledgerPage.page) => {
    setLoading(true);
    try {
      const [creditPayload, affiliatePayload, subscriptionPayload, packagePayload, methodPayload, orderPayload, ledgerPayload] = await Promise.all([
        fetchBusinessCredit(),
        fetchBusinessAffiliateSummary().catch(() => null),
        fetchBusinessSubscription().catch(() => ({ subscription: {} })),
        fetchBusinessPaymentPackages().catch(() => ({ items: [] })),
        fetchBusinessPaymentMethods().catch(() => ({ items: [{ key: "manual", label: "人工确认", providerKey: "manual" }] })),
        fetchBusinessPaymentOrders({ limit: 10 }).catch(() => ({ items: [] })),
        fetchBusinessCreditLedger({ page: nextLedgerPage, pageSize: ledgerPage.pageSize }),
      ]);
      setCredit(creditPayload);
      setAffiliate(affiliatePayload);
      setSubscription(subscriptionPayload.subscription || null);
      setPackages(packagePayload.items || []);
      setPaymentMethods(methodPayload.items?.length ? methodPayload.items : [{ key: "manual", label: "人工确认", providerKey: "manual" }]);
      setOrders(orderPayload.items || []);
      setLedgerItems(ledgerPayload.items || []);
      setLedgerPage(ledgerPayload.page || { ...ledgerPage, page: nextLedgerPage });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取积分失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const requestConfirm = (action: ConfirmAction) => {
    if (action.payment) {
      setSelectedPaymentMethod(paymentMethods.find((item) => item.key !== "manual")?.key || paymentMethods[0]?.key || "manual");
    }
    setConfirmAction(action);
  };

  const runConfirmedAction = async () => {
    if (!confirmAction) {
      return;
    }
    setConfirming(true);
    try {
      await confirmAction.run(confirmAction.payment ? selectedPaymentMethod : undefined);
      setConfirmAction(null);
    } finally {
      setConfirming(false);
    }
  };

  const submitRedeem = async () => {
    const code = redeemCode.trim();
    if (!code) {
      toast.error("请输入兑换码");
      return;
    }
    setRedeeming(true);
    try {
      const payload = await redeemBusinessCode(code);
      setCredit(payload.credit);
      window.dispatchEvent(new CustomEvent(BUSINESS_CREDIT_CHANGED_EVENT, { detail: { balance: payload.credit.balance } }));
      setRedeemCode("");
      void loadData(1);
      toast.success(`兑换成功，到账 ${numberText(payload.result.creditsGranted)} 点`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "兑换失败");
    } finally {
      setRedeeming(false);
    }
  };

  const copyAffiliateLink = async () => {
    const code = affiliate?.profile?.codePreview;
    if (!code) {
      return;
    }
    const url = new URL(window.location.origin + "/login");
    url.searchParams.set("aff", code);
    await navigator.clipboard.writeText(url.toString());
    toast.success("邀请链接已复制");
  };

  const createOrder = async (packageId: string, paymentMethod?: string) => {
    setCreatingOrderId(packageId);
    try {
      const selectedPackage = packages.find((item) => item.id === packageId);
      const payload = await createBusinessPaymentOrder(packageId, paymentMethod);
      setOrders((current) => [payload.order, ...current.filter((item) => item.id !== payload.order.id)].slice(0, 10));
      if (payload.order.payUrl) {
        window.open(payload.order.payUrl, "_blank", "noopener,noreferrer");
        toast.success("订单已创建，请在打开的支付页完成支付");
      } else {
        toast.success(selectedPackage?.packageType === "subscription" || selectedPackage?.packageType === "monthly" ? "订阅订单已创建，等待管理员确认" : "充值订单已创建，等待管理员确认");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建订单失败");
    } finally {
      setCreatingOrderId("");
    }
  };

  const requestRedeem = () => {
    const code = redeemCode.trim();
    if (!code) {
      toast.error("请输入兑换码");
      return;
    }
    requestConfirm({
      title: "确认兑换",
      description: `确认使用兑换码「${code}」？兑换成功后会立即到账。`,
      confirmText: "兑换",
      run: () => submitRedeem(),
    });
  };

  const requestCreateOrder = (item: BusinessPaymentPackage, actionLabel?: string) => {
    const isSubscription = isSubscriptionPackage(item);
    const detail = isSubscription
      ? `${numberText(item.credits)} 订阅点，${numberText(item.durationDays || 30)} 天，${moneyText(item.amountCents, item.currency)}`
      : `${numberText(item.credits)} 普通余额点，${moneyText(item.amountCents, item.currency)}`;
    requestConfirm({
      title: actionLabel || (isSubscription ? "确认订阅" : "确认充值"),
      description: `确认创建「${item.name}」订单？${detail}。`,
      confirmText: "创建订单",
      payment: true,
      run: (paymentMethod) => createOrder(item.id, paymentMethod),
    });
  };

  const requestRenewSubscription = () => {
    if (!primaryRenewPackage) {
      toast.error("暂无可续订套餐");
      return;
    }
    requestCreateOrder(primaryRenewPackage, "确认续订");
  };

  return (
    <AdminPage>
      <AdminHeader
        title="积分中心"
        description="查看当前余额，使用兑换码，并管理邀请入口。"
        actions={
          <Button type="button" variant="outline" onClick={() => void loadData()} disabled={loading || redeeming}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </Button>
        }
      >
        <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
          <Coins className="size-5" />
        </div>
      </AdminHeader>

      <div className="grid gap-5 xl:grid-cols-[minmax(340px,0.75fr)_minmax(0,1.25fr)]">
        <div className="grid content-start gap-5">
          <AdminPanel className="p-5">
            <div className="flex items-start gap-4">
              <div className="grid size-12 shrink-0 place-items-center rounded-[16px] bg-[linear-gradient(135deg,rgba(255,214,92,0.28),rgba(40,214,255,0.16))] text-amber-200">
                <Sparkles className="size-5" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <h2 className="text-lg font-bold text-[var(--app-text-primary)]">账户概览</h2>
                    <p className="mt-1 text-sm text-[var(--app-text-muted)]">生成图片会优先使用订阅点数，不足部分再使用普通余额。</p>
                  </div>
                  {subscription?.active ? (
                    <Button type="button" size="sm" onClick={requestRenewSubscription} disabled={!primaryRenewPackage || creatingOrderId === primaryRenewPackage?.id}>
                      {creatingOrderId === primaryRenewPackage?.id ? <LoaderCircle className="size-4 animate-spin" /> : <ArrowRight className="size-4" />}
                      续订
                    </Button>
                  ) : null}
                </div>
                <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                  <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                    <div className="text-xs text-[var(--app-text-muted)]">普通余额</div>
                    <div className="mt-2 text-4xl font-semibold text-[var(--app-accent-cyan)]">
                      {loading ? "-" : numberText(credit?.balance)}
                    </div>
                  </div>
                  <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                    <div className="text-xs text-[var(--app-text-muted)]">订阅点数</div>
                    <div className="mt-2 text-3xl font-semibold text-emerald-500 dark:text-emerald-300">
                      {loading ? "-" : numberText(subscription?.active ? subscription.creditsLeft : 0)}
                    </div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                      {subscription?.active ? `当前周期可用 ${numberText(subscription.creditsLeft)} 点` : "未订阅"}
                    </div>
                  </div>
                  <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                    <div className="text-xs text-[var(--app-text-muted)]">累计消耗</div>
                    <div className="mt-2 text-2xl font-semibold text-[var(--app-text-primary)]">
                      {loading ? "-" : numberText(credit?.spent)}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </AdminPanel>

          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <TicketCheck className="size-4 text-emerald-300" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">订阅状态</h2>
            </div>
            <div className={cn(adminSubPanelClass, "p-4")}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-base font-semibold text-[var(--app-text-primary)]">{subscription?.packageName || "未订阅"}</div>
                  <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                    {subscription?.active ? `到期后未用订阅点数自动失效` : "订阅套餐会获得独立订阅点数，不会混入普通余额。"}
                  </div>
                </div>
                <span className={cn("inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold", subscription?.active ? "bg-emerald-400/10 text-emerald-300" : "bg-[var(--app-bg-surface)] text-[var(--app-text-muted)]")}>
                  <CheckCircle2 className="size-3.5" />
                  {subscriptionStatusText(subscription)}
                </span>
              </div>
              <div className="mt-4 grid gap-2 text-xs leading-5 text-[var(--app-text-muted)]">
                <div>生效时间：{subscription?.active ? formatDateTime(subscription.startsAt) : "-"}</div>
                <div>当前周期到期：{subscription?.active ? formatDateTime(subscription.expiresAt) : "-"}</div>
                <div>订阅至：{subscription?.active ? formatDateTime(subscriptionCoverageExpiresAt(subscription)) : "-"}</div>
                <div>当前周期点数：剩余 {numberText(subscription?.active ? subscription.creditsLeft : 0)} / {numberText(subscription?.active ? subscription.creditsTotal : 0)}</div>
                <div>扣点顺序：先用订阅点数，再用普通余额。</div>
              </div>
            </div>
          </AdminPanel>

          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <Gift className="size-4 text-[var(--app-accent-cyan)]" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">兑换码</h2>
            </div>
            <div className="flex flex-col gap-3">
              <Input
                value={redeemCode}
                onChange={(event) => setRedeemCode(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    requestRedeem();
                  }
                }}
                placeholder="输入兑换码"
                className={adminInputClass}
              />
              <Button type="button" onClick={requestRedeem} disabled={redeeming}>
                {redeeming ? <LoaderCircle className="size-4 animate-spin" /> : <Send className="size-4" />}
                兑换
              </Button>
            </div>
          </AdminPanel>
        </div>

        <div className="grid gap-5">
          <AdminPanel className="p-5">
            <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2">
                <CreditCard className="size-4 text-emerald-300" />
                <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">充值与订阅</h2>
              </div>
              {subscription?.active ? (
                <span className="rounded-full bg-emerald-400/10 px-2.5 py-1 text-xs font-semibold text-emerald-300">
                  订阅至 {formatDateTime(subscriptionCoverageExpiresAt(subscription))}
                </span>
              ) : null}
            </div>
            {packages.length === 0 ? (
              <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无可用套餐</div>
            ) : (
              <div className="grid gap-4">
                {subscriptionPackages.length > 0 ? (
                  <div>
                    <div className="mb-2 text-xs font-medium text-[var(--app-text-muted)]">订阅套餐</div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {subscriptionPackages.map((item) => {
                        const action = packageActionText(item, currentSubscriptionPackage, subscription);
                        return (
                          <div key={item.id} className={cn(adminSubPanelClass, "p-4")}>
                            <div className="flex items-start justify-between gap-3">
                              <div className="min-w-0">
                                <div className="truncate text-sm font-semibold text-[var(--app-text-primary)]">{item.name}</div>
                                <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.description || "人工确认后生效"} · {numberText(item.durationDays || 30)} 天</div>
                              </div>
                              <div className="shrink-0 text-right">
                                <div className="text-sm font-semibold text-emerald-500 dark:text-emerald-300">{numberText(item.credits)} 订阅点</div>
                                <div className="mt-1 text-xs text-[var(--app-text-muted)]">{moneyText(item.amountCents, item.currency)}</div>
                              </div>
                            </div>
                            <div className="mt-3 min-h-5 text-xs text-[var(--app-text-muted)]">{action.hint}</div>
                            <Button type="button" className="mt-3 w-full" onClick={() => requestCreateOrder(item, action.label)} disabled={action.disabled || creatingOrderId === item.id}>
                              {creatingOrderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <CreditCard className="size-4" />}
                              {action.label}
                            </Button>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ) : null}
                {balancePackages.length > 0 ? (
                  <div>
                    <div className="mb-2 text-xs font-medium text-[var(--app-text-muted)]">余额充值</div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {balancePackages.map((item) => (
                        <div key={item.id} className={cn(adminSubPanelClass, "p-4")}>
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="truncate text-sm font-semibold text-[var(--app-text-primary)]">{item.name}</div>
                              <div className="mt-1 text-xs text-[var(--app-text-muted)]">{item.description || "人工确认后到账"}</div>
                            </div>
                            <div className="shrink-0 text-right">
                              <div className="text-sm font-semibold text-[var(--app-accent-cyan)]">+{numberText(item.credits)}</div>
                              <div className="mt-1 text-xs text-[var(--app-text-muted)]">{moneyText(item.amountCents, item.currency)}</div>
                            </div>
                          </div>
                          <Button type="button" className="mt-4 w-full" onClick={() => requestCreateOrder(item)} disabled={creatingOrderId === item.id}>
                            {creatingOrderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <CreditCard className="size-4" />}
                            创建订单
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            )}
            <div className="mt-4 border-t border-[var(--app-border)] pt-4">
              <div className="mb-3 text-xs font-medium text-[var(--app-text-muted)]">最近订单</div>
              <div className="grid gap-2">
                {orders.length === 0 ? (
                  <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无充值订单</div>
                ) : (
                  orders.map((item) => (
                    <div key={item.id} className={cn(adminSubPanelClass, "flex items-center justify-between gap-3 px-4 py-3")}>
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium text-[var(--app-text-primary)]">{item.outTradeNo}</div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{formatDateTime(item.createdAt)}</div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{orderBillingText(item)}</div>
                      </div>
                      <div className="shrink-0 text-right">
                        <div className="text-sm font-semibold text-[var(--app-text-primary)]">
                          {isSubscriptionPackage(item) ? numberText(item.credits) : `+${numberText(item.credits)}`}
                        </div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{moneyText(item.amountCents, item.currency)} · {orderStatusText(item.status)}</div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </AdminPanel>

          {affiliate?.enabled ? (
            <AdminPanel className="p-5">
              <div className="mb-4 flex items-center gap-2">
                <UsersRound className="size-4 text-emerald-300" />
                <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">邀请返利</h2>
              </div>
              <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                  <div className="text-xs text-[var(--app-text-muted)]">我的邀请码</div>
                  <div className="mt-1 break-all text-lg font-semibold text-[var(--app-text-primary)]">
                    {affiliate.profile?.codePreview || "-"}
                  </div>
                  <div className="mt-2 text-xs text-[var(--app-text-muted)]">
                    已邀请 {numberText(affiliate.referralCount)} 人。
                    {affiliate.registrationRewardEnabled && affiliate.registrationRewardCredits > 0
                      ? `每成功邀请 1 人注册奖励 ${numberText(affiliate.registrationRewardCredits)} 点。`
                      : "当前未开启邀请注册奖励。"}
                  </div>
                </div>
                <Button type="button" variant="outline" onClick={() => void copyAffiliateLink()}>
                  <Copy className="size-4" />
                  复制链接
                </Button>
              </div>
              <div className="mt-4 border-t border-[var(--app-border)] pt-4">
                <div className="mb-3 text-xs font-medium text-[var(--app-text-muted)]">最近邀请</div>
                <div className="grid gap-2">
                  {(affiliate.recentReferrals || []).length === 0 ? (
                    <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无邀请记录</div>
                  ) : (
                    (affiliate.recentReferrals || []).map((item) => (
                      <div key={item.id} className={cn(adminSubPanelClass, "flex items-center justify-between gap-3 px-4 py-3")}>
                        <div className="min-w-0">
                          <div className="truncate text-sm font-medium text-[var(--app-text-primary)]">
                            {maskedUserText(item.referredUsername, item.referredEmail, item.referredUid)}
                          </div>
                          <div className="mt-1 text-xs text-[var(--app-text-muted)]">{formatDateTime(item.createdAt)}</div>
                        </div>
                        <div className={cn("shrink-0 text-sm font-semibold", item.rewardCredits > 0 ? "text-emerald-600 dark:text-emerald-300" : "text-[var(--app-text-muted)]")}>
                          {item.rewardCredits > 0 ? `+${numberText(item.rewardCredits)}` : "未奖励"}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </AdminPanel>
          ) : null}

          <AdminPanel className="p-5">
            <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2">
                <CalendarDays className="size-4 text-[var(--app-text-muted)]" />
                <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">积分明细</h2>
              </div>
              <div className="flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
                <span>{paginationText(ledgerPage)}</span>
                <Button type="button" variant="outline" size="sm" disabled={loading || ledgerPage.page <= 1} onClick={() => void loadData(ledgerPage.page - 1)}>上一页</Button>
                <Button type="button" variant="outline" size="sm" disabled={loading || ledgerPage.page * ledgerPage.pageSize >= ledgerPage.total} onClick={() => void loadData(ledgerPage.page + 1)}>下一页</Button>
              </div>
            </div>
            <div className="overflow-x-auto">
              <table className={adminTableClass}>
                <thead className={adminTableHeadClass}>
                  <tr>
                    <th className="px-4 py-3">时间</th>
                    <th className="px-4 py-3">账户</th>
                    <th className="px-4 py-3">类型</th>
                    <th className="px-4 py-3">变化</th>
                    <th className="px-4 py-3">剩余</th>
                  </tr>
                </thead>
                <tbody className={adminTableBodyClass}>
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
                        <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                        读取中
                      </td>
                    </tr>
                  ) : ledgerItems.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无积分明细</td>
                    </tr>
                  ) : (
                    ledgerItems.map((item) => (
                      <tr key={item.id} className={adminTableRowClass}>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{ledgerAccountText(item)}</td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-primary)]">{ledgerReasonText(item.reason)}</td>
                        <td className={cn("whitespace-nowrap px-4 py-3 font-semibold", item.delta >= 0 ? "text-emerald-600 dark:text-emerald-300" : "text-rose-600 dark:text-rose-300")}>
                          {item.delta >= 0 ? "+" : ""}
                          {numberText(item.delta)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{ledgerBalanceLabel(item)} {numberText(item.balance_after)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </AdminPanel>
        </div>
      </div>
      <Dialog open={Boolean(confirmAction)} onOpenChange={(open) => {
        if (!open && !confirming) {
          setConfirmAction(null);
        }
      }}>
        <DialogContent className="w-[min(92vw,460px)]">
          <DialogHeader>
            <DialogTitle>{confirmAction?.title || "确认操作"}</DialogTitle>
            <DialogDescription>{confirmAction?.description || "确认继续执行该操作？"}</DialogDescription>
          </DialogHeader>
          {confirmAction?.payment ? (
            <div className="grid gap-2">
              <div className="text-sm font-medium text-[var(--app-text-primary)]">支付方式</div>
              <Select value={selectedPaymentMethod} onValueChange={setSelectedPaymentMethod}>
                <SelectTrigger className={adminInputClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {paymentMethods.map((item) => (
                    <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirmAction(null)} disabled={confirming}>取消</Button>
            <Button type="button" onClick={() => void runConfirmedAction()} disabled={confirming}>
              {confirming ? <LoaderCircle className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
              {confirmAction?.confirmText || "确认"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
