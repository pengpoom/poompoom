"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Eye, Gift, KeyRound, LoaderCircle, Pencil, RefreshCw, Ticket, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard, AdminToolbar } from "@/components/admin-layout";
import { AppDatePicker, AppModal, AppSelect } from "@/components/app-controls";
import {
  createBusinessCode,
  deleteBusinessCode,
  fetchBusinessCodeUsages,
  fetchBusinessCodes,
  updateBusinessCodeStatusBatch,
  updateBusinessCode,
  type BusinessCode,
  type BusinessCodeStatus,
  type BusinessCodeType,
  type BusinessCodeUsage,
} from "@/lib/api";

type TypeFilter = "all" | BusinessCodeType;
type StatusFilter = "all" | BusinessCodeStatus;
type CodeView = "registration" | "redeem";

const typeOptions: Array<{ value: BusinessCodeType; label: string; hint: string }> = [
  { value: "invite", label: "邀请码", hint: "只提供注册资格，不送点数。" },
  { value: "promo", label: "优惠码", hint: "提供注册资格，并在注册时赠送点数。" },
  { value: "redeem", label: "兑换码", hint: "已登录用户在积分中心兑换点数。" },
];

const statusOptions: Array<{ value: BusinessCodeStatus; label: string }> = [
  { value: "active", label: "启用" },
  { value: "disabled", label: "停用" },
  { value: "expired", label: "过期" },
];

const filterTypeOptions: Array<{ value: TypeFilter; label: string }> = [
  { value: "all", label: "全部类型" },
  ...typeOptions.map((item) => ({ value: item.value, label: item.label })),
];

const filterStatusOptions: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "全部状态" },
  ...statusOptions,
];

function typeLabel(value: string) {
  return typeOptions.find((item) => item.value === value)?.label || value;
}

function statusLabel(value: string) {
  return statusOptions.find((item) => item.value === value)?.label || value;
}

function typeBadgeClass(value: string) {
  if (value === "redeem") return "warn";
  if (value === "promo") return "ok";
  return "run";
}

function statusBadgeClass(value: string) {
  if (value === "active") return "ok";
  if (value === "expired") return "warn";
  return "off";
}

function formatTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function toDateTimeLocal(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const offsetMs = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16);
}

function fromDateTimeLocal(value: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString();
}

function isLegacyPreviewCode(value?: string) {
  return Boolean(value && value.includes("..."));
}

function contextLabel(value: string) {
  if (value === "registration") return "注册";
  if (value === "recharge_center") return "兑换";
  return value || "-";
}

export default function CodesPage() {
  const [items, setItems] = useState<BusinessCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [view, setView] = useState<CodeView>("registration");
  const [typeFilter, setTypeFilter] = useState<TypeFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<BusinessCode | null>(null);
  const [createdCode, setCreatedCode] = useState("");
  const [codeType, setCodeType] = useState<BusinessCodeType>("invite");
  const [rawCode, setRawCode] = useState("");
  const [title, setTitle] = useState("");
  const [credits, setCredits] = useState("0");
  const [maxUses, setMaxUses] = useState("1");
  const [status, setStatus] = useState<BusinessCodeStatus>("active");
  const [startsAt, setStartsAt] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [note, setNote] = useState("");
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [usageDialogCode, setUsageDialogCode] = useState<BusinessCode | null>(null);
  const [usageItems, setUsageItems] = useState<BusinessCodeUsage[]>([]);
  const [usageLoading, setUsageLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<BusinessCode | null>(null);

  const loadItems = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessCodes({
        type: view === "redeem" ? "redeem" : typeFilter === "all" ? "registration" : typeFilter === "redeem" ? "registration" : typeFilter,
        status: statusFilter,
        search: search.trim() || undefined,
      });
      const nextItems = payload.items || [];
      setItems(nextItems);
      setSelectedIDs((current) => current.filter((id) => nextItems.some((item) => item.id === id)));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取码列表失败");
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, typeFilter, view]);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const stats = useMemo(() => {
    return items.reduce(
      (acc, item) => {
        acc.total += 1;
        if (item.type === "redeem") acc.redeem += 1;
        if (item.type === "promo") acc.promo += 1;
        if (item.type === "invite") acc.invite += 1;
        acc.used += Number(item.usedCount || 0);
        return acc;
      },
      { total: 0, redeem: 0, promo: 0, invite: 0, used: 0 },
    );
  }, [items]);

  const dialogTypeOptions = view === "redeem"
    ? typeOptions.filter((item) => item.value === "redeem")
    : typeOptions.filter((item) => item.value !== "redeem");
  const allSelected = items.length > 0 && selectedIDs.length === items.length;
  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  const resetForm = () => {
    setEditingItem(null);
    setCreatedCode("");
    setCodeType("invite");
    setRawCode("");
    setTitle("");
    setCredits("0");
    setMaxUses("1");
    setStatus("active");
    setStartsAt("");
    setExpiresAt("");
    setNote("");
  };

  const openCreateDialog = (type: BusinessCodeType = "invite") => {
    resetForm();
    setCodeType(type);
    setCredits(type === "redeem" ? "10" : "0");
    setDialogOpen(true);
  };

  const switchView = (nextView: CodeView) => {
    setView(nextView);
    setTypeFilter(nextView === "redeem" ? "redeem" : "all");
    setSelectedIDs([]);
  };

  const openEditDialog = (item: BusinessCode) => {
    setEditingItem(item);
    setCreatedCode("");
    setCodeType(item.type);
    setRawCode("");
    setTitle(item.title);
    setCredits(String(item.credits || 0));
    setMaxUses(String(item.maxUses || 1));
    setStatus(item.status);
    setStartsAt(toDateTimeLocal(item.startsAt));
    setExpiresAt(toDateTimeLocal(item.expiresAt));
    setNote(item.note || "");
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    resetForm();
  };

  const saveCode = async () => {
    const normalizedCredits = Math.max(0, Math.floor(Number(credits) || 0));
    const normalizedMaxUses = Math.max(1, Math.floor(Number(maxUses) || 1));
    if (codeType === "invite" && normalizedCredits > 0) {
      toast.error("邀请码不赠送点数");
      return;
    }
    if (codeType === "redeem" && normalizedCredits <= 0) {
      toast.error("兑换码点数必须大于 0");
      return;
    }
    setSaving(true);
    try {
      const payload = {
        title: title.trim() || typeLabel(codeType),
        credits: normalizedCredits,
        maxUses: normalizedMaxUses,
        status,
        startsAt: fromDateTimeLocal(startsAt),
        expiresAt: fromDateTimeLocal(expiresAt),
        note: note.trim(),
      };
      if (editingItem) {
        const response = await updateBusinessCode(editingItem.id, payload);
        setItems((current) => current.map((item) => (item.id === editingItem.id ? response.item : item)));
        setDialogOpen(false);
        toast.success("码已更新");
      } else {
        const response = await createBusinessCode({
          ...payload,
          type: codeType,
          code: rawCode.trim() || undefined,
        });
        setItems((current) => [response.item, ...current]);
        setCreatedCode(response.code || "");
        toast.success("码已创建");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setSaving(true);
    try {
      await deleteBusinessCode(deleteTarget.id);
      setItems((current) => current.filter((item) => item.id !== deleteTarget.id));
      toast.success("已删除");
      setDeleteTarget(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    } finally {
      setSaving(false);
    }
  };

  const copyCode = async (code: string) => {
    if (!code) return;
    await navigator.clipboard.writeText(code);
    toast.success("已复制");
  };

  const toggleSelected = (id: string) => {
    setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  };

  const toggleSelectAll = () => {
    setSelectedIDs(allSelected ? [] : items.map((item) => item.id));
  };

  const batchUpdateStatus = async (nextStatus: BusinessCodeStatus) => {
    if (selectedIDs.length === 0) {
      toast.error("请先选择码");
      return;
    }
    setSaving(true);
    try {
      const payload = await updateBusinessCodeStatusBatch(selectedIDs, nextStatus);
      toast.success(`已更新 ${Number(payload.updated || 0).toLocaleString()} 个码`);
      setSelectedIDs([]);
      await loadItems();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "批量更新失败");
    } finally {
      setSaving(false);
    }
  };

  const openUsageDialog = async (item: BusinessCode) => {
    setUsageDialogCode(item);
    setUsageItems([]);
    setUsageLoading(true);
    try {
      const payload = await fetchBusinessCodeUsages(item.id);
      setUsageItems(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取使用记录失败");
    } finally {
      setUsageLoading(false);
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title={view === "redeem" ? "兑换码管理" : "注册码管理"}
        description={view === "redeem" ? "管理用户在积分中心兑换余额的兑换码。" : "管理注册页可用的邀请码和优惠码。"}
        icon={Ticket}
        actions={
          <>
            <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading || saving}>
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button className="app-btn-primary" type="button" onClick={() => openCreateDialog(view === "redeem" ? "redeem" : "invite")}>
              + 新建
            </button>
          </>
        }
      />

      <section className="app-stats">
        <AdminStatCard label="总数" value={stats.total.toLocaleString()} sub={`已使用 ${stats.used.toLocaleString()} 次`} icon={Ticket} color="text-cyan-300" />
        <AdminStatCard label="邀请码" value={stats.invite.toLocaleString()} sub="注册资格" icon={KeyRound} color="text-violet-300" />
        <AdminStatCard label="优惠码" value={stats.promo.toLocaleString()} sub="注册资格 + 赠点" icon={Gift} color="text-emerald-300" />
        <AdminStatCard label="兑换码" value={stats.redeem.toLocaleString()} sub="用户余额兑换" icon={Ticket} color="text-amber-300" />
      </section>

      <AdminToolbar>
        <div className="app-seg">
          <button type="button" className={view === "registration" ? "on" : ""} onClick={() => switchView("registration")}>注册码</button>
          <button type="button" className={view === "redeem" ? "on" : ""} onClick={() => switchView("redeem")}>兑换码</button>
        </div>
        {view !== "redeem" ? (
          <AppSelect
            value={typeFilter}
            onChange={(v) => setTypeFilter(v as TypeFilter)}
            options={filterTypeOptions.filter((item) => item.value !== "redeem")}
          />
        ) : null}
        <AppSelect
          value={statusFilter}
          onChange={(v) => setStatusFilter(v as StatusFilter)}
          options={filterStatusOptions}
        />
        <input
          className="app-input"
          type="search"
          placeholder="搜索标题、备注或预览码…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          style={{ flex: 1, minWidth: 180 }}
        />
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle
          title="码列表"
          action={
            selectedIDs.length > 0 ? (
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span className="panel-count">已选 {selectedIDs.length}</span>
                <div className="app-act">
                  <button type="button" onClick={() => void batchUpdateStatus("active")} disabled={saving}>批量启用</button>
                  <button type="button" className="warn" onClick={() => void batchUpdateStatus("disabled")} disabled={saving}>批量停用</button>
                </div>
              </div>
            ) : (
              <span className="panel-count">{items.length}</span>
            )
          }
        />
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
                  <th style={{ width: 40 }}>
                    <input
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAll}
                      aria-label="全选码"
                    />
                  </th>
                  <th>码</th>
                  <th>类型</th>
                  <th>点数</th>
                  <th>使用</th>
                  <th>有效期</th>
                  <th>备注</th>
                  <th style={{ textAlign: "right" }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => {
                  const legacyPreview = isLegacyPreviewCode(item.codePreview);
                  return (
                    <tr key={item.id}>
                      <td>
                        <input
                          type="checkbox"
                          checked={selectedSet.has(item.id)}
                          onChange={() => toggleSelected(item.id)}
                          aria-label={`选择 ${item.title}`}
                        />
                      </td>
                      <td>
                        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                          <code style={{
                            maxWidth: 220,
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                            padding: "3px 8px",
                            borderRadius: 6,
                            border: "1px solid var(--app-border)",
                            background: "var(--app-bg-surface)",
                            fontSize: 12,
                            color: "var(--app-text-primary)",
                          }}>
                            {item.codePreview}
                          </code>
                          {legacyPreview ? <span className="app-badge off">旧预览</span> : null}
                          <div className="app-act">
                            <button type="button" onClick={() => void copyCode(item.codePreview)} disabled={legacyPreview} aria-label="复制码">
                              <Copy className="size-3.5" />
                            </button>
                          </div>
                        </div>
                      </td>
                      <td>
                        <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                          <span className={`app-badge ${typeBadgeClass(item.type)}`}>{typeLabel(item.type)}</span>
                          <span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusLabel(item.status)}</span>
                        </div>
                      </td>
                      <td>{Number(item.credits || 0).toLocaleString()}</td>
                      <td>{item.usedCount.toLocaleString()} / {item.maxUses.toLocaleString()}</td>
                      <td style={{ fontSize: 12, color: "var(--app-text-muted)", lineHeight: 1.5 }}>
                        <div>{formatTime(item.startsAt)}</div>
                        <div>{formatTime(item.expiresAt)}</div>
                      </td>
                      <td style={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", color: "var(--app-text-muted)" }} title={item.note || ""}>{item.note || "-"}</td>
                      <td>
                        <div style={{ display: "flex", justifyContent: "flex-end" }}>
                          <div className="app-act">
                            <button type="button" onClick={() => void openUsageDialog(item)} aria-label="使用记录">
                              <Eye className="size-3.5" />
                            </button>
                            <button type="button" onClick={() => openEditDialog(item)} aria-label="编辑">
                              <Pencil className="size-3.5" />
                            </button>
                            <button type="button" className="danger" onClick={() => setDeleteTarget(item)} disabled={saving} aria-label="删除">
                              <Trash2 className="size-3.5" />
                            </button>
                          </div>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </AdminPanel>

      <AppModal
        open={dialogOpen}
        onClose={closeDialog}
        title={editingItem ? "编辑码" : "新建码"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={closeDialog}>关闭</button>
            <button className="app-btn-primary" type="button" onClick={() => void saveCode()} disabled={saving}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </button>
          </>
        }
      >
        {createdCode ? (
          <div style={{
            margin: "0 18px 16px",
            padding: 14,
            borderRadius: 12,
            border: "1px solid rgba(110, 231, 183, 0.32)",
            background: "rgba(110, 231, 183, 0.08)",
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: "#6ee7b7", marginBottom: 8 }}>新码已创建</div>
            <div style={{ display: "flex", gap: 8 }}>
              <input className="app-input" value={createdCode} readOnly style={{ flex: 1 }} />
              <button className="app-btn-primary" type="button" onClick={() => void copyCode(createdCode)}>
                <Copy className="size-4" />
                复制
              </button>
            </div>
          </div>
        ) : null}

        <div className="app-form-grid">
          <div className="app-fld">
            <span className="fl">类型</span>
            <AppSelect
              value={codeType}
              onChange={(v) => {
                const nextType = v as BusinessCodeType;
                setCodeType(nextType);
                if (nextType === "invite") setCredits("0");
                if (nextType === "redeem" && Number(credits || 0) <= 0) setCredits("10");
              }}
              options={dialogTypeOptions.map((item) => ({ value: item.value, label: item.label }))}
            />
            <span className="fd">{typeOptions.find((item) => item.value === codeType)?.hint}</span>
          </div>
          <div className="app-fld">
            <span className="fl">状态</span>
            <AppSelect value={status} onChange={(v) => setStatus(v as BusinessCodeStatus)} options={statusOptions} />
          </div>
          <div className="app-fld">
            <span className="fl">标题</span>
            <input className="app-input" value={title} onChange={(event) => setTitle(event.target.value)} placeholder={typeLabel(codeType)} />
          </div>
          {!editingItem ? (
            <div className="app-fld">
              <span className="fl">自定义码</span>
              <input className="app-input" value={rawCode} onChange={(event) => setRawCode(event.target.value)} placeholder="留空自动生成" />
            </div>
          ) : null}
          <div className="app-fld">
            <span className="fl">赠送/兑换点数</span>
            <input
              className="app-input"
              type="number"
              min={0}
              step={1}
              value={credits}
              onChange={(event) => setCredits(event.target.value)}
              disabled={codeType === "invite"}
            />
          </div>
          <div className="app-fld">
            <span className="fl">最大使用次数</span>
            <input className="app-input" type="number" min={1} step={1} value={maxUses} onChange={(event) => setMaxUses(event.target.value)} />
          </div>
          <div className="app-fld">
            <span className="fl">开始时间</span>
            <AppDatePicker value={startsAt} onChange={setStartsAt} placeholder="选择开始时间" withTime />
          </div>
          <div className="app-fld">
            <span className="fl">结束时间</span>
            <AppDatePicker value={expiresAt} onChange={setExpiresAt} placeholder="选择结束时间" withTime />
          </div>
          <div className="app-fld full">
            <span className="fl">备注</span>
            <textarea className="app-textarea" value={note} onChange={(event) => setNote(event.target.value)} rows={3} />
          </div>
        </div>
      </AppModal>

      <AppModal
        open={!!usageDialogCode}
        onClose={() => { setUsageDialogCode(null); setUsageItems([]); }}
        title="使用记录"
        footer={
          <button className="app-btn" type="button" onClick={() => { setUsageDialogCode(null); setUsageItems([]); }}>关闭</button>
        }
      >
        <div style={{ padding: "0 18px 8px", fontSize: 12.5, color: "var(--app-text-muted)" }}>
          {usageDialogCode?.title || "-"} · {usageDialogCode?.codePreview || "-"}
        </div>
        <div className="app-table-wrap">
          <table className="app-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>场景</th>
                <th>到账</th>
                <th>使用时间</th>
              </tr>
            </thead>
            <tbody>
              {usageLoading ? (
                <tr>
                  <td colSpan={4} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                    <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
                    <div>读取中</div>
                  </td>
                </tr>
              ) : usageItems.length === 0 ? (
                <tr>
                  <td colSpan={4} style={{ padding: "40px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>暂无使用记录</td>
                </tr>
              ) : (
                usageItems.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <div className="app-user">
                        <div className="app-user-meta">
                          <b>{item.username || "-"}</b>
                          <small>UID {item.uid || "-"} · {item.email || item.userId}</small>
                        </div>
                      </div>
                    </td>
                    <td>{contextLabel(item.context)}</td>
                    <td style={{ color: "var(--app-text-primary)", fontWeight: 500 }}>{Number(item.creditsGranted || 0).toLocaleString()}</td>
                    <td>{formatTime(item.createdAt)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </AppModal>

      <AppModal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title="删除码"
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
          确认删除码 <b style={{ color: "var(--app-text-primary)" }}>「{deleteTarget?.title}」</b>（{deleteTarget?.codePreview}）吗？此操作不可恢复。
        </div>
      </AppModal>
    </AdminPage>
  );
}
