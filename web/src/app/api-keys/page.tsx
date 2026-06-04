"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, KeyRound, LoaderCircle, Power, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminToolbar } from "@/components/admin-layout";
import { AppModal, AppSelect } from "@/components/app-controls";
import {
  createAdminAPIKey,
  fetchAdminAPIKeys,
  revokeAdminAPIKey,
  updateAdminAPIKey,
  type BusinessAPIKey,
} from "@/lib/api";

function formatTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function statusLabel(value: string) {
  if (value === "active") return "启用";
  if (value === "disabled") return "停用";
  return "已撤销";
}

function statusBadgeClass(value: string) {
  if (value === "active") return "ok";
  if (value === "disabled") return "off";
  return "warn";
}

function maskedKey(item: BusinessAPIKey) {
  return `${item.keyPrefix}_…${item.keyLast4}`;
}

export default function APIKeysPage() {
  const [items, setItems] = useState<BusinessAPIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [searchUserId, setSearchUserId] = useState("");

  const [dialogOpen, setDialogOpen] = useState(false);
  const [createdSecret, setCreatedSecret] = useState("");
  const [formUserId, setFormUserId] = useState("");
  const [formName, setFormName] = useState("");
  const [formEnv, setFormEnv] = useState<"live" | "test">("live");
  const [formCreditLimit, setFormCreditLimit] = useState("0");
  const [formRateLimit, setFormRateLimit] = useState("0");
  const [formConcurrency, setFormConcurrency] = useState("0");

  const [revokeTarget, setRevokeTarget] = useState<BusinessAPIKey | null>(null);

  const loadItems = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await fetchAdminAPIKeys(searchUserId.trim() || undefined);
      setItems(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取 API Key 失败");
    } finally {
      setLoading(false);
    }
  }, [searchUserId]);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const stats = useMemo(() => {
    return items.reduce(
      (acc, item) => {
        acc.total += 1;
        if (item.status === "active") acc.active += 1;
        return acc;
      },
      { total: 0, active: 0 },
    );
  }, [items]);

  const openCreateDialog = () => {
    setCreatedSecret("");
    setFormUserId(searchUserId.trim());
    setFormName("");
    setFormEnv("live");
    setFormCreditLimit("0");
    setFormRateLimit("0");
    setFormConcurrency("0");
    setDialogOpen(true);
  };

  const createKey = async () => {
    if (!formUserId.trim()) {
      toast.error("请输入用户 ID");
      return;
    }
    if (!formName.trim()) {
      toast.error("请输入 Key 名称");
      return;
    }
    setSaving(true);
    try {
      const response = await createAdminAPIKey({
        userId: formUserId.trim(),
        name: formName.trim(),
        env: formEnv,
        creditLimit: Math.max(0, Math.floor(Number(formCreditLimit) || 0)),
        rateLimitPerMinute: Math.max(0, Math.floor(Number(formRateLimit) || 0)),
        concurrencyLimit: Math.max(0, Math.floor(Number(formConcurrency) || 0)),
      });
      setItems((current) => [response.item, ...current]);
      setCreatedSecret(response.secret || "");
      toast.success("API Key 已创建");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建失败");
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (item: BusinessAPIKey) => {
    const next = item.status === "active" ? "disabled" : "active";
    setSaving(true);
    try {
      const response = await updateAdminAPIKey(item.id, { status: next });
      setItems((current) => current.map((it) => (it.id === item.id ? response.item : it)));
      toast.success(next === "active" ? "已启用" : "已停用");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "操作失败");
    } finally {
      setSaving(false);
    }
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    setSaving(true);
    try {
      const response = await revokeAdminAPIKey(revokeTarget.id);
      setItems((current) => current.map((it) => (it.id === revokeTarget.id ? response.item : it)));
      toast.success("已撤销");
      setRevokeTarget(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "撤销失败");
    } finally {
      setSaving(false);
    }
  };

  const copyText = async (text: string) => {
    if (!text) return;
    await navigator.clipboard.writeText(text);
    toast.success("已复制");
  };

  return (
    <AdminPage>
      <AdminHeader
        title="API Key 分发管理"
        description="为用户分发、管理对外 API 密钥；用户接入开关在「用户管理」页每行控制。"
        icon={KeyRound}
        actions={
          <>
            <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button className="app-btn-primary" type="button" onClick={openCreateDialog}>
              + 代发 Key
            </button>
          </>
        }
      />

      <AdminToolbar>
        <input
          className="app-input"
          type="search"
          placeholder="按用户 ID 过滤 Key…"
          value={searchUserId}
          onChange={(event) => setSearchUserId(event.target.value)}
          style={{ flex: 1, minWidth: 200 }}
        />
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle title="API Key 列表" action={<span className="panel-count">{stats.total}（启用 {stats.active}）</span>} />
        {loading ? (
          <div style={{ padding: "48px 16px", textAlign: "center", fontSize: 13, color: "var(--app-text-muted)" }}>
            <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
            <div>读取中</div>
          </div>
        ) : items.length === 0 ? (
          <div style={{ padding: "48px 16px", textAlign: "center", fontSize: 13, color: "var(--app-text-muted)" }}>暂无记录</div>
        ) : (
          <div className="app-table-wrap">
            <table className="app-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>Key</th>
                  <th>用户</th>
                  <th>状态</th>
                  <th>用量 / 上限</th>
                  <th>最后使用</th>
                  <th style={{ textAlign: "right" }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <code style={{
                          padding: "3px 8px",
                          borderRadius: 6,
                          border: "1px solid var(--app-border)",
                          background: "var(--app-bg-surface)",
                          fontSize: 12,
                        }}>
                          {maskedKey(item)}
                        </code>
                      </div>
                    </td>
                    <td style={{ fontFamily: "ui-monospace, monospace", fontSize: 12, color: "var(--app-text-secondary)" }}>{item.userId}</td>
                    <td><span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusLabel(item.status)}</span></td>
                    <td>{Number(item.usedCredits || 0).toLocaleString()} / {item.creditLimit > 0 ? Number(item.creditLimit).toLocaleString() : "∞"}</td>
                    <td style={{ fontSize: 12, color: "var(--app-text-muted)" }}>{formatTime(item.lastUsedAt)}</td>
                    <td>
                      <div style={{ display: "flex", justifyContent: "flex-end" }}>
                        <div className="app-act">
                          <button
                            type="button"
                            onClick={() => void toggleStatus(item)}
                            disabled={saving || item.status === "revoked"}
                            aria-label={item.status === "active" ? "停用" : "启用"}
                            title={item.status === "active" ? "停用" : "启用"}
                          >
                            <Power className="size-3.5" />
                          </button>
                          <button
                            type="button"
                            className="danger"
                            onClick={() => setRevokeTarget(item)}
                            disabled={saving || item.status === "revoked"}
                            aria-label="撤销"
                            title="撤销"
                          >
                            <Trash2 className="size-3.5" />
                          </button>
                        </div>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </AdminPanel>

      <AppModal
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title="代发 API Key"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDialogOpen(false)}>关闭</button>
            {!createdSecret ? (
              <button className="app-btn-primary" type="button" onClick={() => void createKey()} disabled={saving}>
                {saving ? <LoaderCircle className="size-4 animate-spin" /> : null}
                创建
              </button>
            ) : null}
          </>
        }
      >
        {createdSecret ? (
          <div style={{
            margin: "0 18px 16px",
            padding: 14,
            borderRadius: 12,
            border: "1px solid rgba(110, 231, 183, 0.32)",
            background: "rgba(110, 231, 183, 0.08)",
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: "#6ee7b7", marginBottom: 8 }}>明文 Key 已生成，仅显示这一次</div>
            <div style={{ display: "flex", gap: 8 }}>
              <input className="app-input" value={createdSecret} readOnly style={{ flex: 1 }} />
              <button className="app-btn-primary" type="button" onClick={() => void copyText(createdSecret)}>
                <Copy className="size-4" />
                复制
              </button>
            </div>
          </div>
        ) : (
          <div className="app-form-grid">
            <div className="app-fld">
              <span className="fl">用户 ID *</span>
              <input className="app-input" value={formUserId} onChange={(event) => setFormUserId(event.target.value)} placeholder="key 归属的用户 ID" />
            </div>
            <div className="app-fld">
              <span className="fl">名称 *</span>
              <input className="app-input" value={formName} onChange={(event) => setFormName(event.target.value)} placeholder="如：客户A-生产" />
            </div>
            <div className="app-fld">
              <span className="fl">环境</span>
              <AppSelect
                value={formEnv}
                onChange={(v) => setFormEnv(v as "live" | "test")}
                options={[{ value: "live", label: "生产 (live)" }, { value: "test", label: "测试 (test)" }]}
              />
            </div>
            <div className="app-fld">
              <span className="fl">额度上限（0=不限）</span>
              <input className="app-input" type="number" min={0} step={1} value={formCreditLimit} onChange={(event) => setFormCreditLimit(event.target.value)} />
            </div>
            <div className="app-fld">
              <span className="fl">每分钟限速（0=不限）</span>
              <input className="app-input" type="number" min={0} step={1} value={formRateLimit} onChange={(event) => setFormRateLimit(event.target.value)} />
            </div>
            <div className="app-fld">
              <span className="fl">并发上限（0=不限）</span>
              <input className="app-input" type="number" min={0} step={1} value={formConcurrency} onChange={(event) => setFormConcurrency(event.target.value)} />
            </div>
          </div>
        )}
      </AppModal>

      <AppModal
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        title="撤销 API Key"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setRevokeTarget(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void handleRevoke()}
              style={{ background: "linear-gradient(135deg, #ef4444, #dc2626)" }}
              disabled={saving}
            >
              确认撤销
            </button>
          </>
        }
      >
        <div style={{ padding: "16px 18px", fontSize: 13.5, color: "var(--app-text-secondary)", lineHeight: 1.7 }}>
          确认撤销 <b style={{ color: "var(--app-text-primary)" }}>「{revokeTarget?.name}」</b>（{revokeTarget ? maskedKey(revokeTarget) : ""}）吗？该 key 将立即失效，不可恢复。
        </div>
      </AppModal>
    </AdminPage>
  );
}
