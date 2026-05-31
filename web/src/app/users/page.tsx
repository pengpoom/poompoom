"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { CheckCircle2, Coins, KeyRound, LoaderCircle, Plus, RefreshCw, RotateCcw, Trash2, UserRound } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminStatCard,
} from "@/components/admin-layout";
import { AppModal, AppSelect } from "@/components/app-controls";
import { AppDrawer } from "@/components/app-drawer";
import { UserDetailDrawerContent, UserDetailDrawerTitle } from "@/app/users/detail/page";
import {
  adjustBusinessUserCredit,
  clearBusinessUserData,
  createBusinessUser,
  deleteBusinessUser,
  fetchBusinessSystemSettings,
  fetchBusinessUsers,
  purgeBusinessUser,
  restoreBusinessUser,
  updateBusinessUser,
  updateBusinessUserBillingLevels,
  updateBusinessUserStatus,
  type BusinessBillingLevel,
  type BusinessUser,
  type BusinessUserDetail,
  type BusinessUserRole,
  type BusinessUserStatus,
} from "@/lib/api";

const roleLabel: Record<BusinessUserRole, string> = {
  admin: "管理员",
  user: "用户",
};

const statusLabel: Record<BusinessUserStatus, string> = {
  active: "启用",
  disabled: "禁用",
  deleted: "已删除",
};

type UserStatusFilter = "all" | BusinessUserStatus;

const statusFilterLabel: Record<UserStatusFilter, string> = {
  all: "全部状态",
  active: "启用",
  disabled: "禁用",
  deleted: "已删除",
};

function formatDateTime(value: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const pad = (num: number) => String(num).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  const units = ["KB", "MB", "GB", "TB"];
  let amount = bytes / 1024;
  let unitIndex = 0;
  while (amount >= 1024 && unitIndex < units.length - 1) {
    amount /= 1024;
    unitIndex += 1;
  }
  return `${amount >= 10 ? amount.toFixed(1) : amount.toFixed(2)} ${units[unitIndex]}`;
}

function usageNumber(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function billingLevelName(level: { name?: string } | undefined, fallback: string) {
  const name = String(level?.name || "").trim();
  return name || fallback;
}

function subscriptionSummary(user: BusinessUser) {
  const subscription = user.billing?.subscription;
  if (subscription?.active && subscription.packageName) {
    return subscription.packageName;
  }
  if (subscription?.active) {
    return "订阅中";
  }
  return "无订阅";
}

function userStatusVariant(status: BusinessUserStatus) {
  if (status === "active") {
    return "success";
  }
  if (status === "deleted") {
    return "danger";
  }
  return "secondary";
}

function userRoleVariant(role: BusinessUserRole) {
  return role === "admin" ? "violet" : "info";
}

export default function UsersPage() {
  const navigate = useNavigate();
  const params = useParams<{ id?: string }>();
  const detailUserID = (params.id || "").trim();
  const [drawerDetail, setDrawerDetail] = useState<BusinessUserDetail | null>(null);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [drawerReload, setDrawerReload] = useState<(() => void) | null>(null);
  const handleDrawerLoaded = useCallback((d: BusinessUserDetail | null, l: boolean) => {
    setDrawerDetail(d);
    setDrawerLoading(l);
  }, []);
  const handleDrawerReady = useCallback((api: { reload: () => void }) => {
    setDrawerReload(() => api.reload);
  }, []);
  const closeDrawer = useCallback(() => {
    navigate("/users");
    setDrawerDetail(null);
    setDrawerReload(null);
  }, [navigate]);
  const [users, setUsers] = useState<BusinessUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<BusinessUser | null>(null);
  const [billingTarget, setBillingTarget] = useState<BusinessUser | null>(null);
  const [creditTarget, setCreditTarget] = useState<BusinessUser | null>(null);
  const [clearDataTarget, setClearDataTarget] = useState<BusinessUser | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BusinessUser | null>(null);
  const [purgeTarget, setPurgeTarget] = useState<BusinessUser | null>(null);
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState<UserStatusFilter>("all");
  const [newUser, setNewUser] = useState({
    email: "",
    username: "",
    password: "",
    balance: "",
  });
  const [editUser, setEditUser] = useState({ username: "", password: "" });
  const [billingDraft, setBillingDraft] = useState({ subscriptionLevelTag: "", walletLevelTag: "" });
  const [subscriptionLevels, setSubscriptionLevels] = useState<BusinessBillingLevel[]>([]);
  const [walletLevels, setWalletLevels] = useState<BusinessBillingLevel[]>([]);
  const [creditOperation, setCreditOperation] = useState<"recharge" | "refund">("recharge");
  const [creditAmount, setCreditAmount] = useState("");
  const includeDeleted = statusFilter === "all" || statusFilter === "deleted";

  const filteredUsers = useMemo(() => {
    const query = searchText.trim().toLowerCase();
    return users.filter((user) => {
      if (statusFilter !== "all" && user.status !== statusFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      return [String(user.uid || ""), user.email, user.username].some((value) => String(value || "").toLowerCase().includes(query));
    });
  }, [searchText, statusFilter, users]);

  const stats = useMemo(() => {
    return filteredUsers.reduce(
      (acc, user) => {
        acc.total += 1;
        if (user.status === "deleted") {
          acc.deleted += 1;
          return acc;
        }
        if (user.role === "admin") acc.admin += 1;
        if (user.status === "active") acc.active += 1;
        if (user.status === "disabled") acc.disabled += 1;
        acc.generations += user.usage?.generation_count || 0;
        acc.images += user.usage?.image_count || 0;
        acc.storageBytes += user.usage?.storage_bytes || 0;
        acc.balance += user.credit?.balance || 0;
        acc.spent += user.credit?.spent || 0;
        return acc;
      },
      { total: 0, admin: 0, active: 0, disabled: 0, deleted: 0, generations: 0, images: 0, storageBytes: 0, balance: 0, spent: 0 },
    );
  }, [filteredUsers]);

  const subscriptionLevelOptions = useMemo(
    () => [
      { value: "", label: "自动（按订阅套餐）" },
      ...subscriptionLevels.map((level) => ({
        value: level.tag,
        label: `${level.name || level.tag} · ${level.tag}`,
      })),
    ],
    [subscriptionLevels],
  );

  const walletLevelOptions = useMemo(
    () => [
      { value: "", label: "自动（按充值记录）" },
      ...walletLevels.map((level) => ({
        value: level.tag,
        label: `${level.name || level.tag} · ${level.tag}`,
      })),
    ],
    [walletLevels],
  );

  const loadUsers = async () => {
    setLoading(true);
    try {
      const [payload, settingsPayload] = await Promise.all([
        fetchBusinessUsers({ includeDeleted }),
        fetchBusinessSystemSettings(),
      ]);
      setUsers(payload.items);
      setSubscriptionLevels((settingsPayload.settings.billing.subscriptionLevels || []).filter((level) => level.enabled));
      setWalletLevels((settingsPayload.settings.billing.walletLevels || []).filter((level) => level.enabled));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取用户失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadUsers();
  }, [includeDeleted]);

  const handleCreateUser = async () => {
    const email = newUser.email.trim();
    const username = newUser.username.trim();
    const password = newUser.password.trim();
    const balanceText = newUser.balance.trim();
    if (!email || !password) {
      toast.error("邮箱和密码不能为空");
      return;
    }
    const balance = balanceText === "" ? undefined : Number(balanceText);
    if (balance !== undefined && (!Number.isInteger(balance) || balance < 0)) {
      toast.error("余额必须是大于等于 0 的整数");
      return;
    }
    setSubmitting(true);
    try {
      await createBusinessUser({
        email,
        ...(username ? { username } : {}),
        password,
        ...(balance !== undefined ? { balance } : {}),
      });
      await loadUsers();
      setCreateOpen(false);
      setNewUser({ email: "", username: "", password: "", balance: "" });
      toast.success("用户已创建");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建用户失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleStatus = async (user: BusinessUser) => {
    const nextStatus: BusinessUserStatus = user.status === "active" ? "disabled" : "active";
    setSubmitting(true);
    try {
      const payload = await updateBusinessUserStatus(user.id, nextStatus);
      setUsers((current) => current.map((item) => (item.id === user.id ? { ...payload.item, usage: item.usage, credit: item.credit } : item)));
      toast.success(nextStatus === "active" ? "用户已启用" : "用户已禁用");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新用户状态失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteUser = async () => {
    if (!deleteTarget) {
      return;
    }
    setSubmitting(true);
    try {
      const payload = await deleteBusinessUser(deleteTarget.id);
      setUsers((current) => {
        if (statusFilter !== "all" && statusFilter !== "deleted") {
          return current.filter((item) => item.id !== deleteTarget.id);
        }
        return current.map((item) => (
          item.id === deleteTarget.id
            ? { ...payload.item, usage: item.usage, credit: item.credit }
            : item
        ));
      });
      setDeleteTarget(null);
      toast.success("用户已删除，7 天内可恢复");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除用户失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleRestoreUser = async (user: BusinessUser) => {
    setSubmitting(true);
    try {
      const payload = await restoreBusinessUser(user.id);
      setUsers((current) => (
        statusFilter === "deleted"
          ? current.filter((item) => item.id !== user.id)
          : current.map((item) => (item.id === user.id ? { ...payload.item, usage: item.usage, credit: item.credit } : item))
      ));
      toast.success("用户已恢复");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "恢复用户失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handlePurgeUser = async () => {
    if (!purgeTarget) {
      return;
    }
    setSubmitting(true);
    try {
      await purgeBusinessUser(purgeTarget.id);
      setUsers((current) => current.filter((item) => item.id !== purgeTarget.id));
      setPurgeTarget(null);
      toast.success("用户已永久删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "永久删除用户失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleClearUserData = async () => {
    if (!clearDataTarget) {
      return;
    }
    setSubmitting(true);
    try {
      await clearBusinessUserData(clearDataTarget.id);
      setUsers((current) => current.map((item) => (
        item.id === clearDataTarget.id
          ? { ...item, usage: undefined, credit: undefined }
          : item
      )));
      setClearDataTarget(null);
      toast.success("用户数据已清空");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "清空用户数据失败");
    } finally {
      setSubmitting(false);
    }
  };

  const openEditDialog = (user: BusinessUser) => {
    setEditTarget(user);
    setEditUser({ username: user.username, password: "" });
  };

  const openBillingDialog = (user: BusinessUser) => {
    setBillingTarget(user);
    setBillingDraft({
      subscriptionLevelTag: user.subscriptionLevelTag || "",
      walletLevelTag: user.walletLevelTag || "",
    });
  };

  const handleUpdateUser = async () => {
    if (!editTarget) {
      return;
    }
    const username = editUser.username.trim();
    const password = editUser.password.trim();
    if (!username) {
      toast.error("用户名不能为空");
      return;
    }
    setSubmitting(true);
    try {
      const payload = await updateBusinessUser(editTarget.id, {
        username,
        ...(password ? { password } : {}),
      });
      setUsers((current) => current.map((item) => (item.id === editTarget.id ? { ...payload.item, usage: item.usage, credit: item.credit } : item)));
      setEditTarget(null);
      setEditUser({ username: "", password: "" });
      toast.success("用户已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新用户失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleUpdateBillingLevels = async () => {
    if (!billingTarget) {
      return;
    }
    setSubmitting(true);
    try {
      const payload = await updateBusinessUserBillingLevels(billingTarget.id, {
        subscriptionLevelTag: billingDraft.subscriptionLevelTag,
        walletLevelTag: billingDraft.walletLevelTag,
      });
      setUsers((current) => current.map((item) => (
        item.id === billingTarget.id
          ? { ...item, ...payload.item, usage: item.usage, credit: item.credit }
          : item
      )));
      setBillingTarget(null);
      setBillingDraft({ subscriptionLevelTag: "", walletLevelTag: "" });
      toast.success("等级已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新等级失败");
    } finally {
      setSubmitting(false);
    }
  };

  const openCreditDialog = (user: BusinessUser) => {
    setCreditTarget(user);
    setCreditOperation("recharge");
    setCreditAmount("");
  };

  const handleAdjustCredit = async () => {
    if (!creditTarget) {
      return;
    }
    const amount = Number(creditAmount);
    if (!Number.isInteger(amount) || amount <= 0) {
      toast.error("金额必须是大于 0 的整数");
      return;
    }
    setSubmitting(true);
    try {
      const payload = await adjustBusinessUserCredit(creditTarget.id, { operation: creditOperation, amount });
      setUsers((current) => current.map((item) => (item.id === creditTarget.id ? { ...item, credit: payload.credit } : item)));
      setCreditTarget(null);
      setCreditAmount("");
      toast.success(creditOperation === "recharge" ? "充值成功" : "退款成功");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新余额失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AdminPage>
        <AdminHeader
          title="用户管理"
          description="管理业务用户、账户状态、余额和生成数据。"
          icon={UserRound}
          actions={
            <>
              <button className="app-btn" type="button" onClick={() => void loadUsers()} disabled={loading || submitting}>
                {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                刷新
              </button>
              <button className="app-btn-primary" type="button" onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                创建用户
              </button>
            </>
          }
        />

        <div className="app-stats">
          <AdminStatCard label="用户总数" value={stats.total} icon={UserRound} color="text-[var(--app-text-primary)]" />
          <AdminStatCard label="启用" value={stats.active} icon={CheckCircle2} color="text-emerald-300" />
          <AdminStatCard label="余额" value={usageNumber(stats.balance)} icon={Coins} color="text-violet-300" />
          <AdminStatCard label="已消耗" value={usageNumber(stats.spent)} icon={Coins} color="text-[var(--app-text-muted)]" />
        </div>

        <div className="app-toolbar">
          <input className="app-input" type="search" value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="搜索 UID / 邮箱 / 用户名…" />
          <AppSelect
            value={statusFilter}
            onChange={(value) => setStatusFilter(value as UserStatusFilter)}
            options={[
              { value: "all", label: "全部状态" },
              { value: "active", label: "启用" },
              { value: "disabled", label: "禁用" },
              { value: "deleted", label: "已删除" },
            ]}
          />
          <span className="spacer" />
          <button className="app-btn" type="button" onClick={() => void loadUsers()} disabled={loading || submitting}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        </div>

        <div className="app-panel">
          <div className="app-table-wrap">
            <table className="app-table">
              <thead>
                <tr>
                  <th>用户</th>
                  <th>角色</th>
                  <th>状态</th>
                  <th>等级 / 订阅</th>
                  <th>点数</th>
                  <th>生成</th>
                  <th>图片</th>
                  <th>存储</th>
                  <th>最近生成</th>
                  <th style={{ textAlign: "right" }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <tr>
                    <td colSpan={10} style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                      <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" /> 读取中
                    </td>
                  </tr>
                ) : filteredUsers.length === 0 ? (
                  <tr>
                    <td colSpan={10} style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                      暂无匹配用户
                    </td>
                  </tr>
                ) : (
                  filteredUsers.map((user) => (
                    <tr key={user.id}>
                      <td>
                        <div className="app-user">
                          <div className="app-user-meta">
                            <b>{user.username}</b>
                            <small>UID {user.uid || "-"} · {user.email || "-"}</small>
                          </div>
                        </div>
                        {user.status === "deleted" ? (
                          <div style={{ marginTop: 4, fontSize: 11, color: "#fca5a5" }}>删除 {formatDateTime(user.deleted_at || "")}</div>
                        ) : null}
                      </td>
                      <td><span className={`app-badge ${user.role === "admin" ? "role-admin" : "role-user"}`}>{roleLabel[user.role]}</span></td>
                      <td><span className={user.status === "active" ? "app-badge ok" : user.status === "deleted" ? "app-badge fail" : "app-badge warn"}>{statusLabel[user.status]}</span></td>
                      <td>
                        <div className="strong" style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                          <span>{billingLevelName(user.billing?.subscriptionLevel, "Free")}</span>
                          {user.billing?.subscriptionLevelOverride ? <span className="app-badge warn">手动</span> : null}
                        </div>
                        <small style={{ color: "var(--app-text-muted)", fontSize: 11 }}>
                          {subscriptionSummary(user)} · 充值 {billingLevelName(user.billing?.walletLevel, "None")}
                          {user.billing?.walletLevelOverride ? " · 手动" : ""}
                        </small>
                      </td>
                      <td className="strong">{usageNumber(user.credit?.balance)}<br /><small style={{ color: "var(--app-text-muted)", fontSize: 11 }}>消耗 {usageNumber(user.credit?.spent)}</small></td>
                      <td className="strong">{usageNumber(user.usage?.generation_count)}<br /><small style={{ color: "var(--app-text-muted)", fontSize: 11 }}>成功 {usageNumber(user.usage?.success_count)}</small></td>
                      <td>{usageNumber(user.usage?.image_count)}</td>
                      <td>{formatBytes(user.usage?.storage_bytes)}</td>
                      <td>{formatDateTime(user.usage?.last_generated_at || "")}</td>
                      <td>
                        <div style={{ display: "flex", justifyContent: "flex-end" }}>
                          <div className="app-act">
                            <button type="button" onClick={() => navigate(`/users/${encodeURIComponent(user.id)}`)}>详情</button>
                            {user.status === "deleted" ? (
                              <>
                                <button type="button" onClick={() => void handleRestoreUser(user)} disabled={submitting}>恢复</button>
                                <button type="button" className="danger" onClick={() => setPurgeTarget(user)} disabled={submitting}>删除</button>
                              </>
                            ) : (
                              <>
                                <button type="button" onClick={() => openEditDialog(user)} disabled={submitting}>编辑</button>
                                <button type="button" onClick={() => openCreditDialog(user)} disabled={submitting}>余额</button>
                                <button type="button" onClick={() => openBillingDialog(user)} disabled={submitting}>等级</button>
                                <button type="button" className={user.status === "active" ? "warn" : ""} onClick={() => void handleToggleStatus(user)} disabled={submitting}>{user.status === "active" ? "禁用" : "启用"}</button>
                                <button type="button" className="danger" onClick={() => setDeleteTarget(user)} disabled={submitting}>删除</button>
                              </>
                            )}
                          </div>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

      <AppModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="创建用户"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setCreateOpen(false)} disabled={submitting}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void handleCreateUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}
              创建
            </button>
          </>
        }
      >
        <div className="app-form-grid" style={{ padding: 0 }}>
          <label className="app-fld full">
            <span className="fl">邮箱</span>
            <input className="app-input" id="create-email" name="email" type="email" autoComplete="email" value={newUser.email} onChange={(event) => setNewUser((current) => ({ ...current, email: event.target.value }))} placeholder="user@example.com" />
          </label>
          <label className="app-fld full">
            <span className="fl">用户名</span>
            <input className="app-input" id="create-username" name="username" autoComplete="username" value={newUser.username} onChange={(event) => setNewUser((current) => ({ ...current, username: event.target.value }))} placeholder="请输入用户名(选填)" />
          </label>
          <label className="app-fld full">
            <span className="fl">密码</span>
            <input className="app-input" id="create-password" name="password" type="password" autoComplete="new-password" value={newUser.password} onChange={(event) => setNewUser((current) => ({ ...current, password: event.target.value }))} placeholder="至少输入一个临时密码" />
          </label>
          <label className="app-fld full">
            <span className="fl">初始余额</span>
            <input className="app-input" id="create-balance" name="balance" type="number" min="0" step="1" value={newUser.balance} onChange={(event) => setNewUser((current) => ({ ...current, balance: event.target.value }))} placeholder="留空使用默认余额" />
          </label>
        </div>
      </AppModal>

      <AppModal
        open={Boolean(editTarget)}
        onClose={() => {
          setEditTarget(null);
          setEditUser({ username: "", password: "" });
        }}
        title="编辑用户"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setEditTarget(null)} disabled={submitting}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void handleUpdateUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
              保存
            </button>
          </>
        }
      >
        <div className="app-form-grid" style={{ padding: 0 }}>
          <div className="full" style={{ gridColumn: "1 / -1", fontSize: 13, color: "var(--app-text-muted)" }}>{editTarget?.email || "-"}</div>
          <label className="app-fld full">
            <span className="fl">用户名</span>
            <input className="app-input" id="edit-username" name="username" value={editUser.username} onChange={(event) => setEditUser((current) => ({ ...current, username: event.target.value }))} placeholder="显示用户名" />
          </label>
          <label className="app-fld full">
            <span className="fl">新密码</span>
            <input className="app-input" id="edit-password" name="password" type="password" autoComplete="new-password" value={editUser.password} onChange={(event) => setEditUser((current) => ({ ...current, password: event.target.value }))} placeholder="留空则不修改密码" />
          </label>
        </div>
      </AppModal>

      <AppModal
        open={Boolean(billingTarget)}
        onClose={() => {
          setBillingTarget(null);
          setBillingDraft({ subscriptionLevelTag: "", walletLevelTag: "" });
        }}
        title="编辑等级"
        footer={
          <>
            <button
              className="app-btn"
              type="button"
              onClick={() => {
                setBillingTarget(null);
                setBillingDraft({ subscriptionLevelTag: "", walletLevelTag: "" });
              }}
              disabled={submitting}
            >
              取消
            </button>
            <button className="app-btn-primary" type="button" onClick={() => void handleUpdateBillingLevels()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
              保存
            </button>
          </>
        }
      >
        <div className="app-form-grid" style={{ padding: 0 }}>
          <div className="full" style={{ gridColumn: "1 / -1", fontSize: 13, color: "var(--app-text-muted)" }}>
            {billingTarget?.username ?? "-"} · {billingTarget?.email || "-"}
          </div>
          <label className="app-fld full">
            <span className="fl">订阅等级</span>
            <AppSelect
              value={billingDraft.subscriptionLevelTag}
              onChange={(value) => setBillingDraft((current) => ({ ...current, subscriptionLevelTag: value }))}
              options={subscriptionLevelOptions}
            />
          </label>
          <label className="app-fld full">
            <span className="fl">充值等级</span>
            <AppSelect
              value={billingDraft.walletLevelTag}
              onChange={(value) => setBillingDraft((current) => ({ ...current, walletLevelTag: value }))}
              options={walletLevelOptions}
            />
          </label>
          <div className="full" style={{ gridColumn: "1 / -1", fontSize: 12, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
            选择“自动”会恢复为按真实订阅和充值记录计算；手动等级只影响展示和号池调度标签。
          </div>
        </div>
      </AppModal>

      <AppModal
        open={Boolean(creditTarget)}
        onClose={() => {
          setCreditTarget(null);
          setCreditAmount("");
        }}
        title="充值 / 退款"
        footer={
          <>
            <button
              className="app-btn"
              type="button"
              onClick={() => {
                setCreditTarget(null);
                setCreditAmount("");
              }}
              disabled={submitting}
            >
              取消
            </button>
            <button className="app-btn-primary" type="button" onClick={() => void handleAdjustCredit()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Coins className="size-4" />}
              保存
            </button>
          </>
        }
      >
        <div className="app-form-grid" style={{ padding: 0 }}>
          <div className="full" style={{ gridColumn: "1 / -1", fontSize: 13, color: "var(--app-text-muted)" }}>
            {creditTarget?.username ?? "-"} 当前余额 {usageNumber(creditTarget?.credit?.balance)}
          </div>
          <label className="app-fld full">
            <span className="fl">类型</span>
            <AppSelect
              value={creditOperation}
              onChange={(value) => setCreditOperation(value as "recharge" | "refund")}
              options={[
                { value: "recharge", label: "充值" },
                { value: "refund", label: "退款" },
              ]}
            />
          </label>
          <label className="app-fld full">
            <span className="fl">点数</span>
            <input className="app-input" id="credit-amount" name="amount" type="number" min="1" step="1" value={creditAmount} onChange={(event) => setCreditAmount(event.target.value)} placeholder="输入点数" />
          </label>
        </div>
      </AppModal>

      <AppModal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="删除用户"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeleteTarget(null)} disabled={submitting}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void handleDeleteUser()} disabled={submitting} style={{ background: "linear-gradient(135deg, rgba(252, 165, 165, 0.95), rgba(244, 63, 94, 0.85))", borderColor: "rgba(252, 165, 165, 0.5)" }}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              删除
            </button>
          </>
        }
      >
        <p style={{ fontSize: 14, color: "var(--app-text-primary)", lineHeight: 1.6 }}>确认删除用户 <b>{deleteTarget?.username ?? "-"}</b>？</p>
        <p style={{ marginTop: 8, fontSize: 13, color: "var(--app-text-muted)", lineHeight: 1.6 }}>这会立即禁止登录并保留账号数据 7 天；7 天内可恢复，之后会自动永久清理。</p>
      </AppModal>

      <AppModal
        open={Boolean(clearDataTarget)}
        onClose={() => setClearDataTarget(null)}
        title="清空用户数据"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setClearDataTarget(null)} disabled={submitting}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void handleClearUserData()}
              disabled={submitting}
              style={{ background: "linear-gradient(135deg, rgba(252, 165, 165, 0.95), rgba(244, 63, 94, 0.85))", borderColor: "rgba(252, 165, 165, 0.5)" }}
            >
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              清空数据
            </button>
          </>
        }
      >
        <p style={{ fontSize: 14, color: "var(--app-text-primary)", lineHeight: 1.6 }}>确认清空用户 <b>{clearDataTarget?.username ?? "-"}</b> 的业务数据？</p>
        <p style={{ marginTop: 8, fontSize: 13, color: "var(--app-text-muted)", lineHeight: 1.6 }}>账号、邮箱、用户名、密码和登录状态会保留；图片历史、图片资产、使用记录、余额和账本会被删除，操作后无法恢复。</p>
      </AppModal>

      <AppModal
        open={Boolean(purgeTarget)}
        onClose={() => setPurgeTarget(null)}
        title="永久删除用户"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setPurgeTarget(null)} disabled={submitting}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void handlePurgeUser()} disabled={submitting} style={{ background: "linear-gradient(135deg, rgba(252, 165, 165, 0.95), rgba(244, 63, 94, 0.85))", borderColor: "rgba(252, 165, 165, 0.5)" }}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              确认永久删除
            </button>
          </>
        }
      >
        <div style={{ display: "flex", alignItems: "flex-start", gap: 10, padding: 12, marginBottom: 12, borderRadius: 10, border: "1px solid rgba(252, 165, 165, 0.3)", background: "rgba(252, 165, 165, 0.08)", color: "#fca5a5", fontSize: 13, lineHeight: 1.5 }}>
          <Trash2 className="size-4" style={{ marginTop: 1, flexShrink: 0 }} />
          <span>此操作不可恢复，请仔细确认。</span>
        </div>
        <p style={{ fontSize: 14, color: "var(--app-text-primary)", lineHeight: 1.6 }}>确认永久删除用户 <b>{purgeTarget?.username ?? "-"}</b>？</p>
        <p style={{ marginTop: 8, fontSize: 13, color: "var(--app-text-muted)", lineHeight: 1.6 }}>这会立即清除该用户账号、余额账本、使用记录和图片资产，删除后无法恢复。</p>
      </AppModal>

      <AppDrawer
        open={Boolean(detailUserID)}
        onClose={closeDrawer}
        title={detailUserID ? (
          <UserDetailDrawerTitle userID={detailUserID} detail={drawerDetail} loading={drawerLoading} />
        ) : null}
        actions={detailUserID ? (
          <button className="app-btn" type="button" onClick={() => drawerReload?.()} disabled={drawerLoading || !drawerReload}>
            {drawerLoading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        ) : null}
      >
        {detailUserID ? (
          <UserDetailDrawerContent userID={detailUserID} onLoaded={handleDrawerLoaded} onReady={handleDrawerReady} />
        ) : null}
      </AppDrawer>
    </AdminPage>
  );
}
