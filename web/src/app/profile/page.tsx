"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { Coins, KeyRound, LoaderCircle, RefreshCw, ShieldCheck, Upload, UserRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminStatCard } from "@/components/admin-layout";
import { changeBusinessMePassword, fetchBusinessMe, uploadBusinessMeAvatar, type BusinessMe } from "@/lib/api";
import { setStoredAuthAvatarUrl } from "@/store/auth";

function roleText(value: string | undefined) {
  return value === "admin" ? "管理员" : "普通用户";
}

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function formatDateTime(value: string | undefined) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export default function ProfilePage() {
  const [me, setMe] = useState<BusinessMe | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const avatarInputRef = useRef<HTMLInputElement | null>(null);

  const loadMe = async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessMe();
      setMe(payload);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取账号信息失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadMe();
  }, []);

  const handleChangePassword = async () => {
    const current = currentPassword.trim();
    const next = newPassword.trim();
    if (!current) {
      toast.error("请输入当前密码");
      return;
    }
    if (next.length < 6) {
      toast.error("新密码至少 6 位");
      return;
    }
    if (next !== confirmPassword.trim()) {
      toast.error("两次输入的新密码不一致");
      return;
    }
    setSubmitting(true);
    try {
      const payload = await changeBusinessMePassword({ currentPassword: current, newPassword: next });
      setMe((currentMe) => currentMe ? { ...currentMe, user: payload.user } : currentMe);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      toast.success("密码已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "修改密码失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleAvatarChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) {
      return;
    }
    if (!["image/png", "image/jpeg", "image/webp", "image/gif"].includes(file.type)) {
      toast.error("头像只支持 PNG、JPG、WebP 或 GIF");
      return;
    }
    if (file.size > 4 * 1024 * 1024) {
      toast.error("头像不能超过 4MB");
      return;
    }
    setAvatarUploading(true);
    try {
      const payload = await uploadBusinessMeAvatar(file);
      setMe((currentMe) => currentMe ? { ...currentMe, user: payload.user } : currentMe);
      await setStoredAuthAvatarUrl(payload.user.avatarUrl || null);
      toast.success("头像已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "上传头像失败");
    } finally {
      setAvatarUploading(false);
    }
  };

  const summaryItems = [
    { label: "角色", value: roleText(me?.user.role), icon: ShieldCheck, color: "text-cyan-300" },
    { label: "余额", value: numberText(me?.credit.balance), icon: Coins, color: "text-violet-300" },
    { label: "已消耗", value: numberText(me?.credit.spent), icon: Coins, color: "text-amber-300" },
  ];

  return (
    <AdminPage>
      <AdminHeader
        title="账号信息"
        description="查看当前账号、点数余额和安全设置。"
        icon={UserRound}
        actions={
          <button className="app-btn" type="button" onClick={() => void loadMe()} disabled={loading || submitting}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        }
      />

      <section className="app-stats">
        {summaryItems.map((item) => (
          <AdminStatCard key={item.label} {...item} value={loading ? "-" : item.value} />
        ))}
      </section>

      <div style={{ display: "grid", gap: 18, gridTemplateColumns: "minmax(0, 0.95fr) minmax(0, 1.05fr)" }}>
        <AdminPanel>
          <div className="panel-title">
            <h3>基本信息</h3>
            <span className={`app-badge ${me?.user.status === "active" ? "ok" : "warn"}`}>
              {me?.user.status === "active" ? "启用" : "禁用"}
            </span>
          </div>
          {loading ? (
            <div style={{ padding: "40px 0", textAlign: "center", color: "var(--app-text-muted)" }}>
              <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
              <div>读取中</div>
            </div>
          ) : (
            <div style={{ padding: 18, display: "grid", gap: 16, fontSize: 13 }}>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 16,
                  padding: 16,
                  borderRadius: 12,
                  border: "1px solid var(--app-border)",
                  background: "var(--app-bg-surface)",
                }}
              >
                <div
                  style={{
                    display: "grid",
                    placeItems: "center",
                    width: 64,
                    height: 64,
                    flexShrink: 0,
                    borderRadius: "50%",
                    overflow: "hidden",
                    border: "1px solid var(--app-border)",
                    background: "var(--app-bg-surface)",
                    color: "var(--app-text-muted)",
                  }}
                >
                  {me?.user.avatarUrl ? (
                    <img src={me.user.avatarUrl} alt="当前头像" style={{ width: "100%", height: "100%", objectFit: "cover" }} />
                  ) : (
                    <UserRound className="size-7" />
                  )}
                </div>
                <div style={{ minWidth: 0, flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 600, color: "var(--app-text-primary)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {me?.user.username || "-"}
                  </div>
                  <div style={{ marginTop: 4, fontSize: 12, color: "var(--app-text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {me?.user.email || "-"}
                  </div>
                </div>
                <input
                  ref={avatarInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/webp,image/gif"
                  style={{ display: "none" }}
                  onChange={(event) => void handleAvatarChange(event)}
                />
                <button
                  className="app-btn"
                  type="button"
                  onClick={() => avatarInputRef.current?.click()}
                  disabled={avatarUploading}
                >
                  {avatarUploading ? <LoaderCircle className="size-4 animate-spin" /> : <Upload className="size-4" />}
                  上传
                </button>
              </div>
              <div>
                <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>用户名</div>
                <div style={{ marginTop: 4, fontWeight: 500, color: "var(--app-text-primary)" }}>{me?.user.username || "-"}</div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>UID</div>
                <div style={{ marginTop: 4, fontFamily: "ui-monospace, monospace", fontSize: 13, color: "var(--app-text-secondary)" }}>{me?.user.uid || "-"}</div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>创建时间</div>
                <div style={{ marginTop: 4, color: "var(--app-text-secondary)" }}>{formatDateTime(me?.user.created_at)}</div>
              </div>
            </div>
          )}
        </AdminPanel>

        <AdminPanel>
          <div className="panel-title">
            <h3>
              <KeyRound className="size-4" style={{ display: "inline-block", verticalAlign: "-2px", marginRight: 6, color: "var(--app-text-muted)" }} />
              修改密码
            </h3>
          </div>
          <div className="app-form-grid">
            <label className="app-fld full">
              <span className="fl">当前密码</span>
              <input className="app-input" type="password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} disabled={submitting} autoComplete="current-password" />
            </label>
            <label className="app-fld full">
              <span className="fl">新密码</span>
              <input className="app-input" type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} disabled={submitting} autoComplete="new-password" />
            </label>
            <label className="app-fld full">
              <span className="fl">确认新密码</span>
              <input className="app-input" type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} disabled={submitting} autoComplete="new-password" />
            </label>
            <div className="full" style={{ gridColumn: "1 / -1", display: "flex", justifyContent: "flex-end" }}>
              <button className="app-btn-primary" type="button" onClick={() => void handleChangePassword()} disabled={submitting || loading}>
                {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
                更新密码
              </button>
            </div>
          </div>
        </AdminPanel>
      </div>
    </AdminPage>
  );
}
