"use client";

import { useEffect, useState } from "react";
import { Coins, Copy, Globe2, KeyRound, LoaderCircle, Power, RefreshCw, ShieldAlert, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminStatCard } from "@/components/admin-layout";
import { AppModal } from "@/components/app-controls";
import {
  createMyAPIKey,
  deleteMyAPIKey,
  fetchBusinessMe,
  fetchMyAPIKeys,
  updateMyAPIKeyStatus,
  type BusinessAPIKey,
  type BusinessMe,
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

export default function APIAccessPage() {
  const [me, setMe] = useState<BusinessMe | null>(null);
  const [keys, setKeys] = useState<BusinessAPIKey[]>([]);
  const [baseUrl, setBaseUrl] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [keyName, setKeyName] = useState("");
  const [createdSecret, setCreatedSecret] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<BusinessAPIKey | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const meData = await fetchBusinessMe();
      setMe(meData);
      if (meData.apiAccessEnabled) {
        const keysData = await fetchMyAPIKeys();
        setKeys(keysData.items || []);
        setBaseUrl(keysData.baseUrl || "");
      } else {
        setKeys([]);
        setBaseUrl("");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取数据失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const openCreateDialog = () => {
    setKeyName("");
    setCreatedSecret("");
    setDialogOpen(true);
  };

  const createKey = async () => {
    if (!keyName.trim()) {
      toast.error("请输入 Key 名称");
      return;
    }
    setSaving(true);
    try {
      const response = await createMyAPIKey(keyName.trim());
      setKeys((current) => [response.item, ...current]);
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
      const response = await updateMyAPIKeyStatus(item.id, next);
      setKeys((current) => current.map((it) => (it.id === item.id ? response.item : it)));
      toast.success(next === "active" ? "已启用" : "已停用");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "操作失败");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setSaving(true);
    try {
      await deleteMyAPIKey(deleteTarget.id);
      setKeys((current) => current.filter((it) => it.id !== deleteTarget.id));
      toast.success("已删除");
      setDeleteTarget(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    } finally {
      setSaving(false);
    }
  };

  const copyText = async (text: string) => {
    if (!text) return;
    await navigator.clipboard.writeText(text);
    toast.success("已复制");
  };

  const enabled = !!me?.apiAccessEnabled;

  return (
    <AdminPage>
      <AdminHeader
        title="API 接入"
        description="生成并管理你的 API 密钥，用于程序化接入生图能力。"
        icon={KeyRound}
        actions={
          <button className="app-btn" type="button" onClick={() => void load()} disabled={loading || saving}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        }
      />

      {loading ? (
        <AdminPanel>
          <div style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
            <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
            <div>读取中</div>
          </div>
        </AdminPanel>
      ) : !enabled ? (
        <AdminPanel>
          <div style={{ padding: "48px 24px", textAlign: "center", color: "var(--app-text-muted)" }}>
            <ShieldAlert className="size-12" style={{ margin: "0 auto 16px", opacity: 0.5 }} />
            <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: "var(--app-text-secondary)" }}>API 接入未开通</div>
            <div style={{ fontSize: 13 }}>当前账号尚未开通 API 接入权限，请联系管理员申请开通。</div>
          </div>
        </AdminPanel>
      ) : (
        <>
          <section className="app-stats">
            <AdminStatCard label="余额" value={Number(me?.credit.balance || 0).toLocaleString()} sub="可用额度" icon={Coins} color="text-violet-300" />
            <AdminStatCard label="已消耗" value={Number(me?.credit.spent || 0).toLocaleString()} sub="累计" icon={Coins} color="text-amber-300" />
            <AdminStatCard label="API Key" value={keys.filter((k) => k.status === "active").length.toLocaleString()} sub="启用中" icon={KeyRound} color="text-cyan-300" />
          </section>

          <AdminPanel>
            <div className="panel-title">
              <h3>我的 API Key</h3>
              <button className="app-btn-primary" type="button" onClick={openCreateDialog}>+ 新建</button>
            </div>
            {baseUrl ? (
              <div className="api-access-endpoint" role="group" aria-label="API 端点">
                <div className="api-access-endpoint-label">
                  <Globe2 className="size-4" aria-hidden="true" />
                  <span>API 端点</span>
                </div>
                <code className="api-access-endpoint-url" title={baseUrl}>
                  {baseUrl}
                </code>
                <button
                  type="button"
                  className="api-access-endpoint-copy"
                  onClick={() => void copyText(baseUrl)}
                  aria-label="复制 API 端点"
                  title="复制 API 端点"
                >
                  <Copy className="size-4" />
                </button>
              </div>
            ) : null}
            {keys.length === 0 ? (
              <div style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                还没有 API Key，点击右上角「新建」创建第一个。
              </div>
            ) : (
              <div className="app-table-wrap">
                <table className="app-table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>Key</th>
                      <th>状态</th>
                      <th>用量 / 上限</th>
                      <th>创建时间</th>
                      <th>最后使用</th>
                      <th style={{ textAlign: "right" }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {keys.map((item) => (
                      <tr key={item.id}>
                        <td>{item.name}</td>
                        <td>
                          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                            <code style={{
                              padding: "3px 8px",
                              borderRadius: 6,
                              border: "1px solid var(--app-border)",
                              background: "var(--app-bg-surface)",
                              fontSize: 12,
                            }}>
                              {maskedKey(item)}
                            </code>
                            <div className="app-act">
                              <button
                                type="button"
                                onClick={() => item.plaintext && void copyText(item.plaintext)}
                                disabled={!item.plaintext}
                                aria-label="复制完整 key"
                                title={item.plaintext ? "复制完整 key" : "无法获取明文"}
                              >
                                <Copy className="size-3.5" />
                              </button>
                            </div>
                          </div>
                        </td>
                        <td><span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusLabel(item.status)}</span></td>
                        <td>{Number(item.usedCredits || 0).toLocaleString()} / {item.creditLimit > 0 ? Number(item.creditLimit).toLocaleString() : "∞"}</td>
                        <td style={{ fontSize: 12, color: "var(--app-text-muted)" }}>{formatTime(item.createdAt)}</td>
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
                                onClick={() => setDeleteTarget(item)}
                                disabled={saving}
                                aria-label="删除"
                                title="删除"
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
        </>
      )}

      <AppModal
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title="新建 API Key"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDialogOpen(false)}>关闭</button>
            {!createdSecret ? (
              <button className="app-btn-primary" type="button" onClick={() => void createKey()} disabled={saving || !keyName.trim()}>
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
            <div style={{ fontSize: 13, fontWeight: 600, color: "#6ee7b7", marginBottom: 8 }}>API Key 已生成，仅显示这一次，请妥善保存</div>
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
            <div className="app-fld full">
              <span className="fl">Key 名称 *</span>
              <input className="app-input" value={keyName} onChange={(event) => setKeyName(event.target.value)} placeholder="如：我的应用" disabled={saving} />
            </div>
          </div>
        )}
      </AppModal>

      <AppModal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title="删除 API Key"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeleteTarget(null)}>取消</button>
            <button
              className="app-btn-primary"
              type="button"
              onClick={() => void handleDelete()}
              style={{ background: "linear-gradient(135deg, #ef4444, #dc2626)" }}
              disabled={saving}
            >
              确认删除
            </button>
          </>
        }
      >
        <div style={{ padding: "16px 18px", fontSize: 13.5, color: "var(--app-text-secondary)", lineHeight: 1.7 }}>
          确认删除 <b style={{ color: "var(--app-text-primary)" }}>「{deleteTarget?.name}」</b>（{deleteTarget ? maskedKey(deleteTarget) : ""}）吗？此操作不可恢复，删除后使用该 key 的请求将立即失效。
        </div>
      </AppModal>
    </AdminPage>
  );
}
