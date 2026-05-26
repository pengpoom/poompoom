"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { Coins, KeyRound, LoaderCircle, RefreshCw, ShieldCheck, Upload, UserRound } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminStatCard } from "@/components/admin-layout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
    { label: "角色", value: roleText(me?.user.role), icon: ShieldCheck, color: "text-[var(--app-text-primary)]" },
    { label: "余额", value: numberText(me?.credit.balance), icon: Coins, color: "text-violet-600 dark:text-violet-300" },
    { label: "已消耗", value: numberText(me?.credit.spent), icon: Coins, color: "text-amber-600 dark:text-amber-300" },
  ];

  return (
    <AdminPage>
        <AdminHeader
          title="账号信息"
          description="查看当前账号、点数余额和安全设置。"
          actions={
            <Button type="button" variant="outline" className="h-10 w-full px-4 sm:w-auto" onClick={() => void loadMe()} disabled={loading || submitting}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
          }
        >
          <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            <UserRound className="size-5" />
          </div>
        </AdminHeader>

        <section className="grid gap-3 sm:grid-cols-3">
          {summaryItems.map((item) => {
            return <AdminStatCard key={item.label} {...item} value={loading ? "-" : item.value} />;
          })}
        </section>

        <div className="grid gap-5 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center justify-between gap-3">
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">基本信息</h2>
              <Badge variant={me?.user.status === "active" ? "success" : "warning"}>{me?.user.status === "active" ? "启用" : "禁用"}</Badge>
            </div>
            {loading ? (
              <div className="py-10 text-center text-[var(--app-text-muted)]">
                <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                读取中
              </div>
            ) : (
              <div className="space-y-4 text-sm">
                <div className="flex items-center gap-4 rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-4">
                  <div className="grid size-16 shrink-0 place-items-center overflow-hidden rounded-full border border-[var(--app-border-strong)] bg-[var(--app-bg-muted)] text-[var(--app-text-muted)]">
                    {me?.user.avatarUrl ? (
                      <img src={me.user.avatarUrl} alt="当前头像" className="size-full object-cover" />
                    ) : (
                      <UserRound className="size-7" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-semibold text-[var(--app-text-primary)]">{me?.user.username || "-"}</div>
                    <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]">{me?.user.email || "-"}</div>
                  </div>
                  <input
                    ref={avatarInputRef}
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif"
                    className="hidden"
                    onChange={(event) => void handleAvatarChange(event)}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => avatarInputRef.current?.click()}
                    disabled={avatarUploading}
                  >
                    {avatarUploading ? <LoaderCircle className="size-4 animate-spin" /> : <Upload className="size-4" />}
                    上传
                  </Button>
                </div>
                <div>
                  <div className="text-xs text-[var(--app-text-muted)]">用户名</div>
                  <div className="mt-1 font-medium text-[var(--app-text-primary)]">{me?.user.username || "-"}</div>
                </div>
                <div>
                  <div className="text-xs text-[var(--app-text-muted)]">UID</div>
                  <div className="mt-1 font-mono text-sm text-[var(--app-text-secondary)]">{me?.user.uid || "-"}</div>
                </div>
                <div>
                  <div className="text-xs text-[var(--app-text-muted)]">创建时间</div>
                  <div className="mt-1 text-[var(--app-text-secondary)]">{formatDateTime(me?.user.created_at)}</div>
                </div>
              </div>
            )}
          </AdminPanel>

          <AdminPanel className="p-5">
            <div className="mb-4 flex items-center gap-2">
              <KeyRound className="size-4 text-[var(--app-text-muted)]" />
              <h2 className="text-sm font-semibold text-[var(--app-text-primary)]">修改密码</h2>
            </div>
            <div className="grid gap-4">
              <label className="grid gap-1.5 text-sm">
                <span className="text-[var(--app-text-secondary)]">当前密码</span>
                <Input type="password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} disabled={submitting} autoComplete="current-password" />
              </label>
              <label className="grid gap-1.5 text-sm">
                <span className="text-[var(--app-text-secondary)]">新密码</span>
                <Input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} disabled={submitting} autoComplete="new-password" />
              </label>
              <label className="grid gap-1.5 text-sm">
                <span className="text-[var(--app-text-secondary)]">确认新密码</span>
                <Input type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} disabled={submitting} autoComplete="new-password" />
              </label>
              <div className="flex justify-end">
                <Button type="button" onClick={() => void handleChangePassword()} disabled={submitting || loading}>
                  {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
                  更新密码
                </Button>
              </div>
            </div>
          </AdminPanel>
        </div>
    </AdminPage>
  );
}
