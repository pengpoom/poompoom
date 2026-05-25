"use client";

import { useEffect, useState } from "react";
import { LoaderCircle, RefreshCw, Save, Share2, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { adminInputClass, adminSubPanelClass } from "@/components/admin-styles";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  fetchBusinessSystemSettings,
  updateBusinessAffiliateSettings,
  type BusinessSystemSettings,
} from "@/lib/api";
import { cn } from "@/lib/utils";

export default function AffiliatePage() {
  const [settings, setSettings] = useState<BusinessSystemSettings | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [registrationRewardEnabled, setRegistrationRewardEnabled] = useState(false);
  const [registrationRewardCredits, setRegistrationRewardCredits] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessSystemSettings();
      setSettings(payload.settings);
      setEnabled(Boolean(payload.settings.affiliate?.enabled));
      setRegistrationRewardEnabled(Boolean(payload.settings.affiliate?.registrationRewardEnabled));
      setRegistrationRewardCredits(Number(payload.settings.affiliate?.registrationRewardCredits || 0));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取邀请返利设置失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSettings();
  }, []);

  const saveSettings = async () => {
    setSaving(true);
    try {
      const payload = await updateBusinessAffiliateSettings({
        enabled,
        registrationRewardEnabled: enabled && registrationRewardEnabled,
        registrationRewardCredits: Math.max(0, Math.floor(Number(registrationRewardCredits) || 0)),
      });
      setSettings(payload.settings);
      setEnabled(Boolean(payload.settings.affiliate?.enabled));
      setRegistrationRewardEnabled(Boolean(payload.settings.affiliate?.registrationRewardEnabled));
      setRegistrationRewardCredits(Number(payload.settings.affiliate?.registrationRewardCredits || 0));
      toast.success("邀请返利设置已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const normalizedRewardCredits = Math.max(0, Math.floor(Number(registrationRewardCredits) || 0));
  const isDirty = Boolean(
    settings &&
      (Boolean(settings.affiliate?.enabled) !== enabled ||
        Boolean(settings.affiliate?.registrationRewardEnabled) !== (enabled && registrationRewardEnabled) ||
        Number(settings.affiliate?.registrationRewardCredits || 0) !== normalizedRewardCredits),
  );

  return (
    <AdminPage>
      <AdminHeader
        title="邀请返利"
        description="控制用户侧邀请入口和邀请注册成功后的固定奖励。"
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => void loadSettings()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              重新读取
            </Button>
            <Button type="button" onClick={() => void saveSettings()} disabled={!isDirty || loading || saving}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </Button>
          </>
        }
      >
        <div className="mb-3 inline-flex size-10 items-center justify-center rounded-[var(--app-radius-md)] border border-white/10 bg-white/[0.045] text-[var(--app-text-primary)]">
          <Share2 className="size-5" />
        </div>
      </AdminHeader>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <AdminPanel className="p-5">
          <div className="mb-5 flex items-start gap-3">
            <div className="inline-flex size-10 shrink-0 items-center justify-center rounded-[var(--app-radius-md)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
              <UsersRound className="size-4" />
            </div>
            <div className="min-w-0">
              <h2 className="text-base font-semibold tracking-tight text-[var(--app-text-primary)]">用户入口</h2>
              <p className="mt-1 text-sm leading-6 text-[var(--app-text-muted)]">
                开启后，用户积分中心展示邀请码、邀请链接和邀请人数。
              </p>
            </div>
          </div>
          <div className={cn(adminSubPanelClass, "p-4")}>
            <div className="flex items-start gap-3">
              <Checkbox
                checked={enabled}
                disabled={loading || saving}
                onCheckedChange={(value) => setEnabled(Boolean(value))}
                className="mt-0.5"
              />
              <div className="min-w-0">
                <div className="text-sm font-medium text-[var(--app-text-secondary)]">开启邀请返利入口</div>
                <div className="mt-1 text-xs leading-5 text-[var(--app-text-muted)]">
                  关闭后，用户积分中心不会展示邀请返利卡片。
                </div>
              </div>
            </div>
          </div>
          <div className={cn(adminSubPanelClass, "mt-3 p-4")}>
            <div className="flex items-start gap-3">
              <Checkbox
                checked={enabled && registrationRewardEnabled}
                disabled={!enabled || loading || saving}
                onCheckedChange={(value) => setRegistrationRewardEnabled(Boolean(value))}
                className="mt-0.5"
              />
              <div className="min-w-0">
                <div className="text-sm font-medium text-[var(--app-text-secondary)]">邀请注册奖励</div>
                <div className="mt-1 text-xs leading-5 text-[var(--app-text-muted)]">
                  被邀请用户注册成功后，给邀请人增加固定点数。
                </div>
              </div>
            </div>
          </div>
          <label className="mt-4 block text-sm font-medium text-[var(--app-text-secondary)]">
            奖励点数
            <Input
              type="number"
              min="0"
              step="1"
              value={registrationRewardCredits}
              disabled={!enabled || !registrationRewardEnabled || loading || saving}
              onChange={(event) => setRegistrationRewardCredits(Number(event.target.value))}
              className={cn(adminInputClass, "mt-2")}
            />
          </label>
        </AdminPanel>

        <AdminPanel className="p-5">
          <h2 className="text-base font-semibold text-[var(--app-text-primary)]">当前策略</h2>
          <div className="mt-4 grid gap-3 text-sm leading-6 text-[var(--app-text-secondary)]">
            <div className={cn(adminSubPanelClass, "p-4")}>
              <div className="font-medium text-[var(--app-text-primary)]">现阶段</div>
              <div className="mt-1 text-[var(--app-text-muted)]">
                生成用户邀请码和邀请关系；开启注册奖励后，邀请成功会自动给邀请人加点。
              </div>
            </div>
            <div className={cn(adminSubPanelClass, "p-4")}>
              <div className="font-medium text-[var(--app-text-primary)]">订单返利</div>
              <div className="mt-1 text-[var(--app-text-muted)]">
                后续充值返利仍应绑定真实支付订单，避免注册赠送或兑换码到账触发商业返利。
              </div>
            </div>
          </div>
        </AdminPanel>
      </div>
    </AdminPage>
  );
}
