"use client";

import { useEffect, useState } from "react";
import { CalendarDays, Coins, Copy, Gift, LoaderCircle, RefreshCw, Send, Sparkles, UsersRound } from "lucide-react";
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
  redeemBusinessCode,
  type BusinessAffiliateSummary,
  type BusinessCreditLedgerEntry,
  type BusinessCreditSummary,
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
  if (reason === "redeem_code") return "兑换码到账";
  if (reason === "admin_recharge") return "管理员充值";
  if (reason === "admin_refund") return "管理员扣减";
  if (reason === "admin_adjustment") return "管理员调整";
  if (reason === "new_user_default") return "新用户默认余额";
  return reason || "-";
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

export default function CreditsPage() {
  const [credit, setCredit] = useState<BusinessCreditSummary | null>(null);
  const [affiliate, setAffiliate] = useState<BusinessAffiliateSummary | null>(null);
  const [ledgerItems, setLedgerItems] = useState<BusinessCreditLedgerEntry[]>([]);
  const [ledgerPage, setLedgerPage] = useState<PaginationMeta>(defaultLedgerPage());
  const [redeemCode, setRedeemCode] = useState("");
  const [loading, setLoading] = useState(true);
  const [redeeming, setRedeeming] = useState(false);

  const loadData = async (nextLedgerPage = ledgerPage.page) => {
    setLoading(true);
    try {
      const [creditPayload, affiliatePayload, ledgerPayload] = await Promise.all([
        fetchBusinessCredit(),
        fetchBusinessAffiliateSummary().catch(() => null),
        fetchBusinessCreditLedger({ page: nextLedgerPage, pageSize: ledgerPage.pageSize }),
      ]);
      setCredit(creditPayload);
      setAffiliate(affiliatePayload);
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

      <div className="grid gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <AdminPanel className="p-6">
          <div className="flex items-start gap-4">
            <div className="grid size-12 shrink-0 place-items-center rounded-[16px] bg-[linear-gradient(135deg,rgba(255,214,92,0.28),rgba(40,214,255,0.16))] text-amber-200">
              <Sparkles className="size-5" />
            </div>
            <div className="min-w-0 flex-1">
              <h2 className="text-lg font-bold text-[var(--app-text-primary)]">当前余额</h2>
              <div className="mt-3 text-4xl font-semibold text-[var(--app-accent-cyan)]">
                {loading ? "-" : numberText(credit?.balance)}
              </div>
              <p className="mt-2 text-sm leading-6 text-[var(--app-text-secondary)]">
                已消耗 {loading ? "-" : numberText(credit?.spent)} 点。
              </p>
            </div>
          </div>
        </AdminPanel>

        <div className="grid gap-5">
          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <Gift className="size-4 text-[var(--app-accent-cyan)]" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">兑换码</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
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
