"use client";

import { useEffect, useState } from "react";
import { CalendarDays, CheckCircle2, LoaderCircle, RefreshCw, Save, Share2, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle } from "@/components/admin-layout";
import {
  fetchAdminBusinessAffiliateReferrals,
  fetchBusinessSystemSettings,
  updateBusinessAffiliateSettings,
  type BusinessAffiliateReferral,
  type BusinessSystemSettings,
} from "@/lib/api";

function formatDateTime(value?: string) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return date.toLocaleString();
}

function numberText(value?: number | null) {
  return Number(value || 0).toLocaleString();
}

function userText(username: string, email: string, uid: number) {
  const name = username || email || "-";
  return uid > 0 ? `${name} · UID ${uid}` : name;
}

export default function AffiliatePage() {
  const [settings, setSettings] = useState<BusinessSystemSettings | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [registrationRewardEnabled, setRegistrationRewardEnabled] = useState(false);
  const [registrationRewardCredits, setRegistrationRewardCredits] = useState(0);
  const [referrals, setReferrals] = useState<BusinessAffiliateReferral[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const [payload, referralsPayload] = await Promise.all([
        fetchBusinessSystemSettings(),
        fetchAdminBusinessAffiliateReferrals(100),
      ]);
      setSettings(payload.settings);
      setEnabled(Boolean(payload.settings.affiliate?.enabled));
      setRegistrationRewardEnabled(Boolean(payload.settings.affiliate?.registrationRewardEnabled));
      setRegistrationRewardCredits(Number(payload.settings.affiliate?.registrationRewardCredits || 0));
      setReferrals(referralsPayload.items || []);
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
        icon={Share2}
        actions={
          <>
            <button className="app-btn" type="button" onClick={() => void loadSettings()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              重新读取
            </button>
            <button className="app-btn-primary" type="button" onClick={() => void saveSettings()} disabled={!isDirty || loading || saving}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </button>
          </>
        }
      />

      <div style={{ display: "grid", gap: 18, gridTemplateColumns: "minmax(0, 0.9fr) minmax(0, 1.1fr)" }}>
        <AdminPanel>
          <div style={{ padding: 18 }}>
            <div style={{ display: "flex", alignItems: "flex-start", gap: 12, marginBottom: 14 }}>
              <span className="app-stat-ic mono" style={{ width: 36, height: 36, flexShrink: 0 }}>
                <UsersRound className="size-4" />
              </span>
              <div style={{ minWidth: 0 }}>
                <h2 style={{ fontSize: 15, fontWeight: 600, color: "var(--app-text-primary)" }}>用户入口</h2>
                <p style={{ marginTop: 4, fontSize: 12.5, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                  开启后，用户积分中心展示邀请码、邀请链接和邀请人数。
                </p>
              </div>
            </div>

            <div style={{
              padding: 14,
              borderRadius: 12,
              border: "1px solid var(--app-border)",
              background: "var(--app-bg-surface)",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 14,
            }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-secondary)" }}>开启邀请返利入口</div>
                <div style={{ marginTop: 4, fontSize: 11.5, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                  关闭后，用户积分中心不会展示邀请返利卡片。
                </div>
              </div>
              <button
                type="button"
                className={`app-switch ${enabled ? "on" : ""}`}
                aria-label="开启邀请返利入口"
                onClick={() => setEnabled((v) => !v)}
                disabled={loading || saving}
              />
            </div>

            <div style={{
              marginTop: 10,
              padding: 14,
              borderRadius: 12,
              border: "1px solid var(--app-border)",
              background: "var(--app-bg-surface)",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 14,
              opacity: enabled ? 1 : 0.5,
            }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-secondary)" }}>邀请注册奖励</div>
                <div style={{ marginTop: 4, fontSize: 11.5, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                  被邀请用户注册成功后，给邀请人增加固定点数。
                </div>
              </div>
              <button
                type="button"
                className={`app-switch ${enabled && registrationRewardEnabled ? "on" : ""}`}
                aria-label="邀请注册奖励"
                onClick={() => setRegistrationRewardEnabled((v) => !v)}
                disabled={!enabled || loading || saving}
              />
            </div>

            <div style={{ marginTop: 14 }}>
              <label style={{ display: "block", fontSize: 13, fontWeight: 500, color: "var(--app-text-secondary)", marginBottom: 6 }}>
                奖励点数
              </label>
              <input
                className="app-input"
                type="number"
                min={0}
                step={1}
                value={registrationRewardCredits}
                disabled={!enabled || !registrationRewardEnabled || loading || saving}
                onChange={(event) => setRegistrationRewardCredits(Number(event.target.value))}
                style={{ width: "100%" }}
              />
            </div>
          </div>
        </AdminPanel>

        <AdminPanel>
          <div style={{ padding: 18 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, color: "var(--app-text-primary)" }}>当前策略</h2>
            <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
              <div style={{
                padding: 14,
                borderRadius: 12,
                border: "1px solid var(--app-border)",
                background: "var(--app-bg-surface)",
              }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)" }}>现阶段</div>
                <div style={{ marginTop: 4, fontSize: 12, lineHeight: 1.7, color: "var(--app-text-muted)" }}>
                  生成用户邀请码和邀请关系；开启注册奖励后，邀请成功会自动给邀请人加点。
                </div>
              </div>
              <div style={{
                padding: 14,
                borderRadius: 12,
                border: "1px solid var(--app-border)",
                background: "var(--app-bg-surface)",
              }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)" }}>订单返利</div>
                <div style={{ marginTop: 4, fontSize: 12, lineHeight: 1.7, color: "var(--app-text-muted)" }}>
                  后续充值返利仍应绑定真实支付订单，避免注册赠送或兑换码到账触发商业返利。
                </div>
              </div>
            </div>
          </div>
        </AdminPanel>
      </div>

      <AdminPanel>
        <AdminSectionTitle
          title="邀请记录"
          action={
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <CalendarDays className="size-4" style={{ color: "var(--app-text-muted)" }} />
              <span className="panel-count">{numberText(referrals.length)}</span>
            </div>
          }
        />
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                <th>邀请人</th>
                <th>被邀请人</th>
                <th>注册时间</th>
                <th>奖励状态</th>
                <th>奖励点数</th>
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
              ) : referrals.length === 0 ? (
                <tr>
                  <td colSpan={5} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无邀请记录</td>
                </tr>
              ) : (
                referrals.map((item) => (
                  <tr key={item.id}>
                    <td style={{ whiteSpace: "normal" }}>
                      <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{userText(item.referrerUsername, item.referrerEmail, item.referrerUid)}</div>
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>{item.referrerEmail || "-"}</div>
                    </td>
                    <td style={{ whiteSpace: "normal" }}>
                      <div style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{userText(item.referredUsername, item.referredEmail, item.referredUid)}</div>
                      <div style={{ marginTop: 2, fontSize: 11, color: "var(--app-text-muted)" }}>{item.referredEmail || "-"}</div>
                    </td>
                    <td>{formatDateTime(item.createdAt)}</td>
                    <td>
                      {item.rewardCredits > 0 ? (
                        <span className="app-badge ok">
                          <CheckCircle2 className="size-3" />
                          已发放
                        </span>
                      ) : (
                        <span className="app-badge off">未发放</span>
                      )}
                    </td>
                    <td style={{ fontWeight: 600, color: "var(--app-text-primary)" }}>{numberText(item.rewardCredits)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </AdminPanel>
    </AdminPage>
  );
}
