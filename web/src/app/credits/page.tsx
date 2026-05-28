"use client";

import { useEffect, useState, type CSSProperties } from "react";
import { ArrowRight, CalendarDays, CheckCircle2, Coins, Copy, CreditCard, Gift, LoaderCircle, RefreshCw, Send, Sparkles, TicketCheck, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { AppModal, AppSelect } from "@/components/app-controls";
import { BUSINESS_CREDIT_CHANGED_EVENT } from "@/components/app-shell-nav";
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

const subPanelStyle: CSSProperties = {
  padding: "12px 16px",
  borderRadius: 10,
  border: "1px solid var(--app-border)",
  background: "var(--app-bg-surface)",
};

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

function daysUntil(value?: string) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) return 0;
  return Math.max(0, Math.ceil((date.getTime() - Date.now()) / 86400000));
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
        icon={Coins}
        actions={
          <button className="app-btn" type="button" onClick={() => void loadData()} disabled={loading || redeeming}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        }
      />

      <div style={{ display: "grid", gap: 18, gridTemplateColumns: "minmax(340px, 0.75fr) minmax(0, 1.25fr)" }}>
        <div style={{ display: "grid", gap: 18, alignContent: "start" }}>
          <AdminPanel>
            <div style={{ padding: 18 }}>
              <div style={{ display: "flex", alignItems: "flex-start", gap: 14, marginBottom: 14 }}>
                <span
                  className="app-stat-ic amber"
                  style={{ width: 44, height: 44, flexShrink: 0 }}
                >
                  <Sparkles className="size-5" />
                </span>
                <div style={{ minWidth: 0, flex: 1 }}>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 10, alignItems: "flex-start", justifyContent: "space-between" }}>
                    <div>
                      <h2 style={{ fontSize: 16, fontWeight: 700, color: "var(--app-text-primary)" }}>账户概览</h2>
                      <p style={{ marginTop: 4, fontSize: 12.5, color: "var(--app-text-muted)" }}>生成图片会优先使用订阅点数，不足部分再使用普通余额。</p>
                    </div>
                    {subscription?.active ? (
                      <button className="app-btn-primary" type="button" onClick={requestRenewSubscription} disabled={!primaryRenewPackage || creatingOrderId === primaryRenewPackage?.id}>
                        {creatingOrderId === primaryRenewPackage?.id ? <LoaderCircle className="size-4 animate-spin" /> : <ArrowRight className="size-4" />}
                        续订
                      </button>
                    ) : null}
                  </div>
                </div>
              </div>
              <div style={{ display: "grid", gap: 10 }}>
                <div style={subPanelStyle}>
                  <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>普通余额</div>
                  <div style={{ marginTop: 8, fontSize: 32, fontWeight: 600, color: "#22d3ee" }}>
                    {loading ? "-" : numberText(credit?.balance)}
                  </div>
                </div>
                <div style={subPanelStyle}>
                  <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>订阅点数</div>
                  <div style={{ marginTop: 8, fontSize: 26, fontWeight: 600, color: "#34d399" }}>
                    {loading ? "-" : numberText(subscription?.active ? subscription.creditsLeft : 0)}
                  </div>
                  <div style={{ marginTop: 4, fontSize: 11, color: "var(--app-text-muted)" }}>
                    {subscription?.active ? `当前周期可用 ${numberText(subscription.creditsLeft)} 点` : "未订阅"}
                  </div>
                </div>
                <div style={subPanelStyle}>
                  <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>累计消耗</div>
                  <div style={{ marginTop: 8, fontSize: 22, fontWeight: 600, color: "var(--app-text-primary)" }}>
                    {loading ? "-" : numberText(credit?.spent)}
                  </div>
                </div>
              </div>
            </div>
          </AdminPanel>

          <AdminPanel>
            <div className="panel-title">
              <h3>
                <TicketCheck className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "#34d399" }} />
                订阅状态
              </h3>
            </div>
            <div className="cr-sub">
              <div className="cr-sub-top">
                <b>{subscription?.packageName || "未订阅"}</b>
                <span className={`app-badge ${subscription?.active ? "ok" : "off"}`}>
                  {subscriptionStatusText(subscription)}
                </span>
              </div>
              {subscription?.active ? (
                <>
                  <div className="cr-sub-days">剩余 <b>{daysUntil(subscription.expiresAt)}</b> 天</div>
                  <dl className="cr-sub-list">
                    <div><dt>生效时间</dt><dd>{formatDateTime(subscription.startsAt)}</dd></div>
                    <div><dt>到期时间</dt><dd>{formatDateTime(subscription.expiresAt)}</dd></div>
                    <div><dt>订阅点数</dt><dd>{numberText(subscription.creditsLeft)} / {numberText(subscription.creditsTotal)}</dd></div>
                    {subscriptionCoverageExpiresAt(subscription) && subscriptionCoverageExpiresAt(subscription) !== subscription.expiresAt ? (
                      <div><dt>订阅至</dt><dd>{formatDateTime(subscriptionCoverageExpiresAt(subscription))}</dd></div>
                    ) : null}
                  </dl>
                  <p className="cr-note">订阅期内优先消耗订阅点数，到期后未用完不结转，以实际为准。</p>
                </>
              ) : (
                <p className="cr-note">订阅套餐获得独立订阅点数，扣点时先用订阅点数再用普通余额。</p>
              )}
            </div>
          </AdminPanel>

          <AdminPanel>
            <div className="panel-title">
              <h3>
                <Gift className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "#22d3ee" }} />
                兑换码
              </h3>
            </div>
            <div style={{ padding: 18, display: "flex", flexDirection: "column", gap: 10 }}>
              <input
                className="app-input"
                value={redeemCode}
                onChange={(event) => setRedeemCode(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    requestRedeem();
                  }
                }}
                placeholder="输入兑换码"
              />
              <button className="app-btn-primary" type="button" onClick={requestRedeem} disabled={redeeming}>
                {redeeming ? <LoaderCircle className="size-4 animate-spin" /> : <Send className="size-4" />}
                兑换
              </button>
            </div>
          </AdminPanel>
        </div>

        <div style={{ display: "grid", gap: 18, alignContent: "start" }}>
          <AdminPanel>
            <div className="panel-title">
              <h3>
                <CreditCard className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "#34d399" }} />
                充值与订阅
              </h3>
              {subscription?.active ? (
                <span className="app-badge ok">订阅至 {formatDateTime(subscriptionCoverageExpiresAt(subscription))}</span>
              ) : null}
            </div>
            <div style={{ padding: 18 }}>
              {packages.length === 0 ? (
                <div style={{ ...subPanelStyle, fontSize: 13, color: "var(--app-text-muted)" }}>暂无可用套餐</div>
              ) : (
                <div style={{ display: "grid", gap: 14 }}>
                  {subscriptionPackages.length > 0 ? (
                    <div>
                      <div style={{ marginBottom: 8, fontSize: 11.5, fontWeight: 500, color: "var(--app-text-muted)" }}>订阅套餐</div>
                      <div style={{ display: "grid", gap: 10, gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))" }}>
                        {subscriptionPackages.map((item) => {
                          const action = packageActionText(item, currentSubscriptionPackage, subscription);
                          return (
                            <div key={item.id} style={subPanelStyle}>
                              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                                <div style={{ minWidth: 0 }}>
                                  <div style={{ fontSize: 13.5, fontWeight: 600, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.name}</div>
                                  <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{item.description || "人工确认后生效"} · {numberText(item.durationDays || 30)} 天</div>
                                </div>
                                <div style={{ flexShrink: 0, textAlign: "right" }}>
                                  <div style={{ fontSize: 13, fontWeight: 600, color: "#34d399" }}>{numberText(item.credits)} 订阅点</div>
                                  <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{moneyText(item.amountCents, item.currency)}</div>
                                </div>
                              </div>
                              <div style={{ marginTop: 10, minHeight: 18, fontSize: 11.5, color: "var(--app-text-muted)" }}>{action.hint}</div>
                              <button
                                className="app-btn-primary"
                                type="button"
                                style={{ marginTop: 10, width: "100%", justifyContent: "center" }}
                                onClick={() => requestCreateOrder(item, action.label)}
                                disabled={action.disabled || creatingOrderId === item.id}
                              >
                                {creatingOrderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <CreditCard className="size-4" />}
                                {action.label}
                              </button>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
                  {balancePackages.length > 0 ? (
                    <div>
                      <div style={{ marginBottom: 8, fontSize: 11.5, fontWeight: 500, color: "var(--app-text-muted)" }}>余额充值</div>
                      <div style={{ display: "grid", gap: 10, gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))" }}>
                        {balancePackages.map((item) => (
                          <div key={item.id} style={subPanelStyle}>
                            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                              <div style={{ minWidth: 0 }}>
                                <div style={{ fontSize: 13.5, fontWeight: 600, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.name}</div>
                                <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{item.description || "人工确认后到账"}</div>
                              </div>
                              <div style={{ flexShrink: 0, textAlign: "right" }}>
                                <div style={{ fontSize: 13, fontWeight: 600, color: "#22d3ee" }}>+{numberText(item.credits)}</div>
                                <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{moneyText(item.amountCents, item.currency)}</div>
                              </div>
                            </div>
                            <button
                              className="app-btn-primary"
                              type="button"
                              style={{ marginTop: 12, width: "100%", justifyContent: "center" }}
                              onClick={() => requestCreateOrder(item)}
                              disabled={creatingOrderId === item.id}
                            >
                              {creatingOrderId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <CreditCard className="size-4" />}
                              创建订单
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : null}
                </div>
              )}
              <div style={{ marginTop: 16, borderTop: "1px solid var(--app-border)", paddingTop: 14 }}>
                <div style={{ marginBottom: 10, fontSize: 11.5, fontWeight: 500, color: "var(--app-text-muted)" }}>最近订单</div>
                <div style={{ display: "grid", gap: 8 }}>
                  {orders.length === 0 ? (
                    <div style={{ ...subPanelStyle, fontSize: 13, color: "var(--app-text-muted)" }}>暂无充值订单</div>
                  ) : (
                    orders.map((item) => (
                      <div key={item.id} style={{ ...subPanelStyle, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                        <div style={{ minWidth: 0 }}>
                          <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.outTradeNo}</div>
                          <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{formatDateTime(item.createdAt)}</div>
                          <div style={{ marginTop: 2, fontSize: 11.5, color: "var(--app-text-muted)" }}>{orderBillingText(item)}</div>
                        </div>
                        <div style={{ flexShrink: 0, textAlign: "right" }}>
                          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--app-text-primary)" }}>
                            {isSubscriptionPackage(item) ? numberText(item.credits) : `+${numberText(item.credits)}`}
                          </div>
                          <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{moneyText(item.amountCents, item.currency)} · {orderStatusText(item.status)}</div>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </AdminPanel>

          {affiliate?.enabled ? (
            <AdminPanel>
              <div className="panel-title">
                <h3>
                  <UsersRound className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "#34d399" }} />
                  邀请返利
                </h3>
              </div>
              <div style={{ padding: 18 }}>
                <div style={{ display: "grid", gap: 10, gridTemplateColumns: "minmax(0, 1fr) auto", alignItems: "center" }}>
                  <div style={subPanelStyle}>
                    <div style={{ fontSize: 11.5, color: "var(--app-text-muted)" }}>我的邀请码</div>
                    <div style={{ marginTop: 4, fontSize: 17, fontWeight: 600, color: "var(--app-text-primary)", wordBreak: "break-all" }}>
                      {affiliate.profile?.codePreview || "-"}
                    </div>
                    <div style={{ marginTop: 8, fontSize: 11.5, color: "var(--app-text-muted)" }}>
                      已邀请 {numberText(affiliate.referralCount)} 人。
                      {affiliate.registrationRewardEnabled && affiliate.registrationRewardCredits > 0
                        ? `每成功邀请 1 人注册奖励 ${numberText(affiliate.registrationRewardCredits)} 点。`
                        : "当前未开启邀请注册奖励。"}
                    </div>
                  </div>
                  <button className="app-btn" type="button" onClick={() => void copyAffiliateLink()}>
                    <Copy className="size-4" />
                    复制链接
                  </button>
                </div>
                <div style={{ marginTop: 16, borderTop: "1px solid var(--app-border)", paddingTop: 14 }}>
                  <div style={{ marginBottom: 10, fontSize: 11.5, fontWeight: 500, color: "var(--app-text-muted)" }}>最近邀请</div>
                  <div style={{ display: "grid", gap: 8 }}>
                    {(affiliate.recentReferrals || []).length === 0 ? (
                      <div style={{ ...subPanelStyle, fontSize: 13, color: "var(--app-text-muted)" }}>暂无邀请记录</div>
                    ) : (
                      (affiliate.recentReferrals || []).map((item) => (
                        <div key={item.id} style={{ ...subPanelStyle, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                          <div style={{ minWidth: 0 }}>
                            <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                              {maskedUserText(item.referredUsername, item.referredEmail, item.referredUid)}
                            </div>
                            <div style={{ marginTop: 4, fontSize: 11.5, color: "var(--app-text-muted)" }}>{formatDateTime(item.createdAt)}</div>
                          </div>
                          <div style={{ flexShrink: 0, fontSize: 13, fontWeight: 600, color: item.rewardCredits > 0 ? "#34d399" : "var(--app-text-muted)" }}>
                            {item.rewardCredits > 0 ? `+${numberText(item.rewardCredits)}` : "未奖励"}
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              </div>
            </AdminPanel>
          ) : null}

          <AdminPanel>
            <div className="panel-title">
              <h3>
                <CalendarDays className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "var(--app-text-muted)" }} />
                积分明细
              </h3>
              <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, color: "var(--app-text-muted)" }}>
                <span>{paginationText(ledgerPage)}</span>
                <button className="app-btn" type="button" style={{ height: 28, padding: "0 10px", fontSize: 11.5 }} disabled={loading || ledgerPage.page <= 1} onClick={() => void loadData(ledgerPage.page - 1)}>上一页</button>
                <button className="app-btn" type="button" style={{ height: 28, padding: "0 10px", fontSize: 11.5 }} disabled={loading || ledgerPage.page * ledgerPage.pageSize >= ledgerPage.total} onClick={() => void loadData(ledgerPage.page + 1)}>下一页</button>
              </div>
            </div>
            <div className="app-table-wrap">
              <table className="app-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>账户</th>
                    <th>类型</th>
                    <th>变化</th>
                    <th>剩余</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={5} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                        <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
                        <div>读取中</div>
                      </td>
                    </tr>
                  ) : ledgerItems.length === 0 ? (
                    <tr>
                      <td colSpan={5} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无积分明细</td>
                    </tr>
                  ) : (
                    ledgerItems.map((item) => (
                      <tr key={item.id}>
                        <td style={{ whiteSpace: "nowrap" }}>{formatDateTime(item.created_at)}</td>
                        <td style={{ whiteSpace: "nowrap" }}>{ledgerAccountText(item)}</td>
                        <td style={{ whiteSpace: "nowrap", color: "var(--app-text-primary)" }}>{ledgerReasonText(item.reason)}</td>
                        <td style={{ whiteSpace: "nowrap", fontWeight: 600, color: item.delta >= 0 ? "#34d399" : "#fb7185" }}>
                          {item.delta >= 0 ? "+" : ""}
                          {numberText(item.delta)}
                        </td>
                        <td style={{ whiteSpace: "nowrap" }}>{ledgerBalanceLabel(item)} {numberText(item.balance_after)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </AdminPanel>
        </div>
      </div>

      <AppModal
        open={Boolean(confirmAction)}
        onClose={() => {
          if (!confirming) setConfirmAction(null);
        }}
        title={confirmAction?.title || "确认操作"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setConfirmAction(null)} disabled={confirming}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void runConfirmedAction()} disabled={confirming}>
              {confirming ? <LoaderCircle className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
              {confirmAction?.confirmText || "确认"}
            </button>
          </>
        }
      >
        <div style={{ display: "grid", gap: 14 }}>
          <p style={{ fontSize: 13, lineHeight: 1.6, color: "var(--app-text-secondary)", margin: 0 }}>
            {confirmAction?.description || "确认继续执行该操作？"}
          </p>
          {confirmAction?.payment ? (
            <label className="app-fld">
              <span className="fl">支付方式</span>
              <AppSelect
                value={selectedPaymentMethod}
                onChange={setSelectedPaymentMethod}
                options={paymentMethods.map((item) => ({ value: item.key, label: item.label }))}
              />
            </label>
          ) : null}
        </div>
      </AppModal>
    </AdminPage>
  );
}
