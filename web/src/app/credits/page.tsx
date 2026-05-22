"use client";

import { CalendarDays, Coins, Gift, Sparkles } from "lucide-react";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { Button } from "@/components/ui/button";

export default function CreditsPage() {
  return (
    <AdminPage>
      <AdminHeader
        title="积分中心"
        description="充值、活动赠送和积分明细会在这里统一管理。"
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
            <div className="min-w-0">
              <h2 className="text-lg font-bold text-[var(--app-text-primary)]">充值入口筹备中</h2>
              <p className="mt-2 max-w-xl text-sm leading-6 text-[var(--app-text-secondary)]">
                这里后续会接入套餐购买、订单状态和余额到账记录。当前先作为积分入口占位，方便侧栏积分按钮有明确去向。
              </p>
              <div className="mt-5 flex flex-wrap gap-2">
                <Button type="button" disabled>
                  <Coins className="size-4" />
                  立即充值
                </Button>
                <Button type="button" variant="outline" disabled>
                  查看订单
                </Button>
              </div>
            </div>
          </div>
        </AdminPanel>

        <div className="grid gap-5">
          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <Gift className="size-4 text-[var(--app-accent-cyan)]" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">活动入口</h2>
            </div>
            <div className="rounded-[16px] border border-white/10 bg-white/[0.04] p-4 text-sm leading-6 text-[var(--app-text-secondary)]">
              邀请奖励、签到赠送、新用户任务等活动后续可以放在这里。
            </div>
          </AdminPanel>

          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <CalendarDays className="size-4 text-[var(--app-text-muted)]" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">积分明细</h2>
            </div>
            <div className="rounded-[16px] border border-dashed border-white/12 bg-white/[0.025] p-6 text-center text-sm text-[var(--app-text-muted)]">
              暂无明细展示，后续接入充值和消费流水。
            </div>
          </AdminPanel>
        </div>
      </div>
    </AdminPage>
  );
}
