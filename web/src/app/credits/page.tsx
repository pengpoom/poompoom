"use client";

import { useEffect, useState } from "react";
import { CalendarDays, CheckCircle2, Coins, Copy, CreditCard, Gift, LoaderCircle, RefreshCw, Send, Sparkles, TicketCheck, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { adminInputClass, adminSubPanelClass, adminTableBodyClass, adminTableClass, adminTableHeadClass, adminTableRowClass } from "@/components/admin-styles";
import { BUSINESS_CREDIT_CHANGED_EVENT } from "@/components/app-shell-nav";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  fetchBusinessAffiliateSummary,
  fetchBusinessCredit,
  fetchBusinessCreditLedger,
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
  if (reason === "redeem_code") return "兑换码到账";
  if (reason === "admin_recharge") return "管理员充值";
  if (reason === "admin_refund") return "管理员扣减";
  if (reason === "admin_adjustment") return "管理员调整";
  if (reason === "new_user_default") return "新用户默认余额";
  return reason || "-";
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

function packageTypeText(type?: string) {
  if (type === "subscription" || type === "monthly") return "订阅";
  return "余额";
}

function subscriptionStatusText(subscription?: BusinessSubscription | null) {
  if (!subscription?.id) return "未订阅";
  if (subscription.active) return "订阅中";
  if (subscription.status === "cancelled") return "已取消";
  if (subscription.status === "expired") return "已过期";
  return "未订阅";
}

function daysLeft(value?: string) {
  const expiresAt = new Date(value || "");
  if (Number.isNaN(expiresAt.getTime())) {
    return 0;
  }
  return Math.max(0, Math.ceil((expiresAt.getTime() - Date.now()) / 86400000));
}

export default function CreditsPage() {
  const [credit, setCredit] = useState<BusinessCreditSummary | null>(null);
  const [affiliate, setAffiliate] = useState<BusinessAffiliateSummary | null>(null);
  const [subscription, setSubscription] = useState<BusinessSubscription | null>(null);
  const [packages, setPackages] = useState<BusinessPaymentPackage[]>([]);
  const [orders, setOrders] = useState<BusinessPaymentOrder[]>([]);
  const [ledgerItems, setLedgerItems] = useState<BusinessCreditLedgerEntry[]>([]);
  const [ledgerPage, setLedgerPage] = useState<PaginationMeta>(defaultLedgerPage());
  const [redeemCode, setRedeemCode] = useState("");
  const [loading, setLoading] = useState(true);
  const [redeeming, setRedeeming] = useState(false);
  const [creatingOrderId, setCreatingOrderId] = useState("");

  const loadData = async (nextLedgerPage = ledgerPage.page) => {
    setLoading(true);
    try {
      const [creditPayload, affiliatePayload, subscriptionPayload, packagePayload, orderPayload, ledgerPayload] = await Promise.all([
        fetchBusinessCredit(),
        fetchBusinessAffiliateSummary().catch(() => null),
        fetchBusinessSubscription().catch(() => ({ subscription: {} })),
        fetchBusinessPaymentPackages().catch(() => ({ items: [] })),
        fetchBusinessPaymentOrders({ limit: 10 }).catch(() => ({ items: [] })),
        fetchBusinessCreditLedger({ page: nextLedgerPage, pageSize: ledgerPage.pageSize }),
      ]);
      setCredit(creditPayload);
      setAffiliate(affiliatePayload);
      setSubscription(subscriptionPayload.subscription || null);
      setPackages(packagePayload.items || []);
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

  const createOrder = async (packageId: string) => {
    setCreatingOrderId(packageId);
    try {
      const selectedPackage = packages.find((item) => item.id === packageId);
      const payload = await createBusinessPaymentOrder(packageId);
      setOrders((current) => [payload.order, ...current.filter((item) => item.id !== payload.order.id)].slice(0, 10));
      toast.success(selectedPackage?.packageType === "subscription" || selectedPackage?.packageType === "monthly" ? "订阅订单已创建，等待管理员确认" : "充值订单已创建，等待管理员确认");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建订单失败");
    } finally {
      setCreatingOrderId("");
    }
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
                <h2 className="text-lg font-bold text-[var(--app-text-primary)]">账户概览</h2>
                <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                  <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                    <div className="text-xs text-[var(--app-text-muted)]">普通余额</div>
                    <div className="mt-2 text-4xl font-semibold text-[var(--app-accent-cyan)]">
                      {loading ? "-" : numberText(credit?.balance)}
                    </div>
                  </div>
                  <div className={cn(adminSubPanelClass, "px-4 py-3")}>
                    <div className="text-xs text-[var(--app-text-muted)]">累计消耗</div>
                    <div className="mt-2 text-2xl font-semibold text-[var(--app-text-primary)]">
                      {loading ? "-" : numberText(credit?.spent)}
                    </div>
                  </div>
                </div>
                <p className="mt-3 text-sm leading-6 text-[var(--app-text-secondary)]">
                  余额用于图片生成扣点；充值和兑换成功后会自动刷新。
                </p>
              </div>
            </div>
          </AdminPanel>

          {subscription?.active ? (
            <AdminPanel className="p-5">
              <div className="mb-4 flex items-center gap-2">
                <TicketCheck className="size-4 text-emerald-300" />
                <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">订阅状态</h2>
              </div>
              <div className={cn(adminSubPanelClass, "p-4")}>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-base font-semibold text-[var(--app-text-primary)]">{subscription.packageName || "订阅"}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                      {subscription.active ? `剩余 ${daysLeft(subscription.expiresAt)} 天` : subscriptionStatusText(subscription)}
                    </div>
                  </div>
                  <span className={cn("inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold", subscription.active ? "bg-emerald-400/10 text-emerald-300" : "bg-[var(--app-bg-surface)] text-[var(--app-text-muted)]")}>
                    <CheckCircle2 className="size-3.5" />
                    {subscriptionStatusText(subscription)}
                  </span>
                </div>
                <div className="mt-4 grid gap-2 text-xs leading-5 text-[var(--app-text-muted)]">
                  <div>生效时间：{formatDateTime(subscription.startsAt)}</div>
                  <div>到期时间：{formatDateTime(subscription.expiresAt)}</div>
                  <div>订阅点数：剩余 {numberText(subscription.creditsLeft)} / {numberText(subscription.creditsTotal)}</div>
                  <div>会员权益：订阅期内优先消耗订阅点数；到期后未用订阅点数自动失效。</div>
                </div>
              </div>
            </AdminPanel>
          ) : null}

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
                    void submitRedeem();
                  }
                }}
                placeholder="输入兑换码"
                className={adminInputClass}
              />
              <Button type="button" onClick={() => void submitRedeem()} disabled={redeeming}>
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
                  订阅至 {formatDateTime(subscription.expiresAt)}
                </span>
              ) : null}
            </div>
            {packages.length === 0 ? (
              <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm text-[var(--app-text-muted)]")}>暂无可用套餐</div>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                {packages.map((item) => (
                  <div key={item.id} className={cn(adminSubPanelClass, "p-4")}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-semibold text-[var(--app-text-primary)]">{item.name}</div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{packageTypeText(item.packageType)} · {item.description || "人工确认后到账"}</div>
                      </div>
                      <div className="shrink-0 text-right">
                        <div className="text-sm font-semibold text-emerald-500 dark:text-emerald-300">
                          {item.packageType === "subscription" || item.packageType === "monthly" ? `${numberText(item.credits)} 订阅点` : `+${numberText(item.credits)}`}
                        </div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{moneyText(item.amountCents, item.currency)}</div>
                      </div>
                    </div>
                    <Button type="button" className="mt-4 w-full" onClick={() => void createOrder(item.id)} disabled={creatingOrderId === item.id}>
                      {creatingOrderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <CreditCard className="size-4" />}
                      {item.packageType === "subscription" || item.packageType === "monthly" ? "创建订阅订单" : "创建订单"}
                    </Button>
                  </div>
                ))}
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
                      </div>
                      <div className="shrink-0 text-right">
                        <div className="text-sm font-semibold text-[var(--app-text-primary)]">+{numberText(item.credits)}</div>
                        <div className="mt-1 text-xs text-[var(--app-text-muted)]">{orderStatusText(item.status)}</div>
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
                    <th className="px-4 py-3">类型</th>
                    <th className="px-4 py-3">变化</th>
                    <th className="px-4 py-3">余额</th>
                  </tr>
                </thead>
                <tbody className={adminTableBodyClass}>
                  {loading ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
                        <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                        读取中
                      </td>
                    </tr>
                  ) : ledgerItems.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-10 text-center text-[var(--app-text-muted)]">暂无积分明细</td>
                    </tr>
                  ) : (
                    ledgerItems.map((item) => (
                      <tr key={item.id} className={adminTableRowClass}>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-secondary)]">{formatDateTime(item.created_at)}</td>
                        <td className="whitespace-nowrap px-4 py-3 text-[var(--app-text-primary)]">{ledgerReasonText(item.reason)}</td>
                        <td className={cn("whitespace-nowrap px-4 py-3 font-semibold", item.delta >= 0 ? "text-emerald-600 dark:text-emerald-300" : "text-rose-600 dark:text-rose-300")}>
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
        </div>
      </div>
    </AdminPage>
  );
}
