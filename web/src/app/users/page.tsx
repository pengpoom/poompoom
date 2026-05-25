"use client";

import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, Coins, Eye, KeyRound, LoaderCircle, Plus, RefreshCw, RotateCcw, Search, Trash2, UserRound, UsersRound } from "lucide-react";
import { Link } from "react-router-dom";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminToolbar,
  AdminStatCard,
} from "@/components/admin-layout";
import {
  adminInputPillClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
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
  adjustBusinessUserCredit,
  clearBusinessUserData,
  createBusinessUser,
  deleteBusinessUser,
  fetchBusinessUsers,
  purgeBusinessUser,
  restoreBusinessUser,
  updateBusinessUser,
  updateBusinessUserStatus,
  type BusinessUser,
  type BusinessUserRole,
  type BusinessUserStatus,
} from "@/lib/api";
import { cn } from "@/lib/utils";

const roleLabel: Record<BusinessUserRole, string> = {
  admin: "管理员",
  user: "测试用户",
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
  const [users, setUsers] = useState<BusinessUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<BusinessUser | null>(null);
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

  const loadUsers = async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessUsers({ includeDeleted });
      setUsers(payload.items);
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
          actions={
            <>
            <Button type="button" variant="outline" onClick={() => void loadUsers()} disabled={loading || submitting}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </Button>
            <Button type="button" onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              创建用户
            </Button>
            </>
          }
        >
          <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            <UsersRound className="size-5" />
          </div>
        </AdminHeader>

        <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { label: "用户总数", value: stats.total, icon: UserRound, color: "text-[var(--app-text-primary)]" },
            { label: "启用", value: stats.active, icon: CheckCircle2, color: "text-emerald-300" },
            { label: "余额", value: usageNumber(stats.balance), icon: Coins, color: "text-violet-300" },
            { label: "已消耗", value: usageNumber(stats.spent), icon: Coins, color: "text-[var(--app-text-muted)]" },
          ].map((item) => {
            return <AdminStatCard key={item.label} {...item} />;
          })}
        </section>

        <AdminToolbar className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_180px]">
          <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
            搜索
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
              <Input
                value={searchText}
                onChange={(event) => setSearchText(event.target.value)}
                className={`${adminInputPillClass} pl-9`}
                placeholder="按 UID / 邮箱 / 用户名搜索"
              />
            </div>
          </label>
          <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
            状态
            <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as UserStatusFilter)}>
              <SelectTrigger className={`${adminInputPillClass} w-full`}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(["all", "active", "disabled", "deleted"] as UserStatusFilter[]).map((value) => (
                  <SelectItem key={value} value={value}>{statusFilterLabel[value]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
        </AdminToolbar>

        <AdminPanel>
          <div className="overflow-x-auto">
            <table className={adminTableClass}>
              <thead className={adminTableHeadClass}>
                <tr>
                  <th className="px-4 py-3">用户</th>
                  <th className="px-4 py-3">角色</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">点数</th>
                  <th className="px-4 py-3">生成</th>
                  <th className="px-4 py-3">图片</th>
                  <th className="px-4 py-3">存储</th>
                  <th className="px-4 py-3">最近生成</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className={adminTableBodyClass}>
                {loading ? (
                  <tr>
                    <td colSpan={9} className="px-4 py-12 text-center text-[var(--app-text-muted)]">
                      <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                      读取中
                    </td>
                  </tr>
                ) : filteredUsers.length === 0 ? (
                  <tr>
                    <td colSpan={9} className="px-4 py-12 text-center text-[var(--app-text-muted)]">
                      暂无匹配用户
                    </td>
                  </tr>
                ) : (
                  filteredUsers.map((user) => (
                    <tr key={user.id} className={adminTableRowClass}>
                      <td className="px-4 py-3">
                        <div className="font-medium text-[var(--app-text-primary)]">{user.username}</div>
                        <div className="mt-0.5 font-mono text-xs text-[var(--app-text-muted)]">UID {user.uid || "-"}</div>
                        <div className="mt-0.5 max-w-[240px] truncate text-xs text-[var(--app-text-muted)]">{user.email || "-"}</div>
                        {user.status === "deleted" ? (
                          <div className="mt-0.5 text-xs text-rose-600 dark:text-rose-300">删除时间 {formatDateTime(user.deleted_at || "")}</div>
                        ) : null}
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant={userRoleVariant(user.role)}>{roleLabel[user.role]}</Badge>
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant={userStatusVariant(user.status)}>{statusLabel[user.status]}</Badge>
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-medium text-[var(--app-text-primary)]">{usageNumber(user.credit?.balance)}</div>
                        <div className="mt-0.5 whitespace-nowrap text-xs text-[var(--app-text-muted)]">
                          已消耗 {usageNumber(user.credit?.spent)}
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-medium text-[var(--app-text-primary)]">{usageNumber(user.usage?.generation_count)}</div>
                        <div className="mt-0.5 whitespace-nowrap text-xs text-[var(--app-text-muted)]">
                          成功 {usageNumber(user.usage?.success_count)} / 失败 {usageNumber(user.usage?.failed_count)}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-[var(--app-text-secondary)]">{usageNumber(user.usage?.image_count)}</td>
                      <td className="px-4 py-3 whitespace-nowrap text-[var(--app-text-secondary)]">{formatBytes(user.usage?.storage_bytes)}</td>
                      <td className="px-4 py-3 whitespace-nowrap text-[var(--app-text-secondary)]">{formatDateTime(user.usage?.last_generated_at || "")}</td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-2">
                          <Button type="button" size="sm" variant="outline" asChild>
                            <Link to={`/users/${encodeURIComponent(user.id)}`}>
                              <Eye className="size-4" />
                              详情
                            </Link>
                          </Button>
                          {user.status === "deleted" ? (
                            <>
                              <Button type="button" size="sm" variant="outline" onClick={() => void handleRestoreUser(user)} disabled={submitting}>
                                <RotateCcw className="size-4" />
                                恢复
                              </Button>
                              <Button type="button" size="sm" variant="destructive" onClick={() => setPurgeTarget(user)} disabled={submitting}>
                                <Trash2 className="size-4" />
                                删除
                              </Button>
                            </>
                          ) : (
                            <>
                              <Button type="button" size="sm" variant="outline" onClick={() => openEditDialog(user)} disabled={submitting}>
                                <KeyRound className="size-4" />
                                编辑
                              </Button>
                              <Button type="button" size="sm" variant="outline" onClick={() => openCreditDialog(user)} disabled={submitting}>
                                <Coins className="size-4" />
                                调整余额
                              </Button>
                              <Button type="button" size="sm" variant="outline" onClick={() => setClearDataTarget(user)} disabled={submitting}>
                                <Trash2 className="size-4" />
                                清空数据
                              </Button>
                              <Button type="button" size="sm" variant={user.status === "active" ? "secondary" : "outline"} onClick={() => void handleToggleStatus(user)} disabled={submitting}>
                                {user.status === "active" ? "禁用" : "启用"}
                              </Button>
                              <Button type="button" size="sm" variant="destructive" onClick={() => setDeleteTarget(user)} disabled={submitting}>
                                <Trash2 className="size-4" />
                                删除
                              </Button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </AdminPanel>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>创建用户</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <label className="grid gap-2 text-sm font-medium">
              邮箱
              <Input id="create-email" name="email" type="email" autoComplete="email" value={newUser.email} onChange={(event) => setNewUser((current) => ({ ...current, email: event.target.value }))} placeholder="user@example.com" />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              用户名
              <Input id="create-username" name="username" autoComplete="username" value={newUser.username} onChange={(event) => setNewUser((current) => ({ ...current, username: event.target.value }))} placeholder="请输入用户名(选填)" />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              密码
              <Input id="create-password" name="password" type="password" autoComplete="new-password" value={newUser.password} onChange={(event) => setNewUser((current) => ({ ...current, password: event.target.value }))} placeholder="至少输入一个临时密码" />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              初始余额
              <Input id="create-balance" name="balance" type="number" min="0" step="1" value={newUser.balance} onChange={(event) => setNewUser((current) => ({ ...current, balance: event.target.value }))} placeholder="留空使用默认余额" />
            </label>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)} disabled={submitting}>取消</Button>
            <Button type="button" onClick={() => void handleCreateUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(editTarget)} onOpenChange={(open) => {
        if (!open) {
          setEditTarget(null);
          setEditUser({ username: "", password: "" });
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑用户</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="text-sm text-[var(--app-text-muted)]">{editTarget?.email || "-"}</div>
            <label className="grid gap-2 text-sm font-medium">
              用户名
              <Input id="edit-username" name="username" value={editUser.username} onChange={(event) => setEditUser((current) => ({ ...current, username: event.target.value }))} placeholder="显示用户名" />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              新密码
              <Input id="edit-password" name="password" type="password" autoComplete="new-password" value={editUser.password} onChange={(event) => setEditUser((current) => ({ ...current, password: event.target.value }))} placeholder="留空则不修改密码" />
            </label>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setEditTarget(null)} disabled={submitting}>取消</Button>
            <Button type="button" onClick={() => void handleUpdateUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(creditTarget)} onOpenChange={(open) => {
        if (!open) {
          setCreditTarget(null);
          setCreditAmount("");
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>充值 / 退款</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="text-sm text-[var(--app-text-muted)]">
              {creditTarget?.username ?? "-"} 当前余额 {usageNumber(creditTarget?.credit?.balance)}
            </div>
            <label className="grid gap-2 text-sm font-medium">
              类型
              <Select value={creditOperation} onValueChange={(value) => setCreditOperation(value as "recharge" | "refund")}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="recharge">充值</SelectItem>
                  <SelectItem value="refund">退款</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="grid gap-2 text-sm font-medium">
              点数
              <Input id="credit-amount" name="amount" type="number" min="1" step="1" value={creditAmount} onChange={(event) => setCreditAmount(event.target.value)} placeholder="输入点数" />
            </label>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => {
              setCreditTarget(null);
              setCreditAmount("");
            }} disabled={submitting}>取消</Button>
            <Button type="button" onClick={() => void handleAdjustCredit()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Coins className="size-4" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => {
        if (!open) {
          setDeleteTarget(null);
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>删除用户</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 text-sm text-[var(--app-text-secondary)]">
            <p>确认删除用户 {deleteTarget?.username ?? "-"}？</p>
            <p className="text-[var(--app-text-muted)]">这会立即禁止登录并保留账号数据 7 天；7 天内可恢复，之后会自动永久清理。</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleteTarget(null)} disabled={submitting}>取消</Button>
            <Button type="button" variant="destructive" onClick={() => void handleDeleteUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(clearDataTarget)} onOpenChange={(open) => {
        if (!open) {
          setClearDataTarget(null);
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>清空用户数据</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 text-sm text-[var(--app-text-secondary)]">
            <p>确认清空用户 {clearDataTarget?.username ?? "-"} 的业务数据？</p>
            <p className="text-[var(--app-text-muted)]">账号、邮箱、用户名、密码和登录状态会保留；图片历史、图片资产、使用记录、余额和账本会被删除，操作后无法恢复。</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setClearDataTarget(null)} disabled={submitting}>取消</Button>
            <Button type="button" variant="destructive" onClick={() => void handleClearUserData()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              清空数据
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(purgeTarget)} onOpenChange={(open) => {
        if (!open) {
          setPurgeTarget(null);
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>永久删除用户</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 text-sm text-[var(--app-text-secondary)]">
            <p>确认永久删除用户 {purgeTarget?.username ?? "-"}？</p>
            <p className="text-[var(--app-text-muted)]">这会立即删除该用户账号、余额账本、使用记录和图片资产，删除后无法恢复。</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setPurgeTarget(null)} disabled={submitting}>取消</Button>
            <Button type="button" variant="destructive" onClick={() => void handlePurgeUser()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}
