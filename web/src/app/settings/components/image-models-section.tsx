"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  FlaskConical,
  LoaderCircle,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Star,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

import { AppModal, AppSelect } from "@/components/app-controls";
import {
  createBusinessImageModel,
  deleteBusinessImageModel,
  fetchAdminBusinessImageModels,
  setDefaultBusinessImageModel,
  testBusinessImageModel,
  updateBusinessImageModel,
  type APIAccessPlatform,
  type BusinessImageModel,
  type BusinessImageModelInput,
  type ImageModelCapabilities,
} from "@/lib/api";
import {
  providerPlatformLabel,
  providerPlatformMeta,
  providerPlatformOptions,
} from "@/lib/provider-platforms";
import { cn } from "@/lib/utils";

import { ConfigSection, Field } from "./shared";
import {
  settingsTableWrapClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "./styles";

const adapterOptions = [
  { label: "OpenAI Images", value: "openai-images" },
  { label: "Gemini", value: "gemini" },
];

const defaultCapabilities: ImageModelCapabilities = {
  generate: true,
  edit: true,
  referenceImage: true,
  mask: true,
  sizes: ["auto", "1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9", "21:9"],
  qualities: ["low", "medium", "high"],
  maxImages: 1,
  maxReferenceImages: 8,
};

type EnabledFilter = "all" | "enabled" | "disabled";
type DraftState = BusinessImageModelInput;
type ModelTestResult = {
  status: "running" | "succeeded" | "failed";
  message: string;
  durationMs?: number;
  imageCount?: number;
  testedAt: string;
};

function emptyDraft(): DraftState {
  return {
    id: "",
    vendor: "openai",
    vendorLabel: "OpenAI",
    displayName: "",
    adapter: "openai-images",
    platform: "gpt-image",
    upstreamModel: "",
    enabled: true,
    preview: false,
    compareEnabled: true,
    capabilities: defaultCapabilities,
    creditCost: 10,
    isDefault: false,
    sortOrder: 100,
  };
}

function modelToDraft(item: BusinessImageModel): DraftState {
  return {
    id: item.id,
    vendor: item.vendor,
    vendorLabel: item.vendorLabel,
    displayName: item.displayName,
    adapter: item.adapter,
    platform: item.platform as APIAccessPlatform,
    upstreamModel: item.upstreamModel,
    enabled: item.enabled,
    preview: Boolean(item.preview),
    compareEnabled: item.compareEnabled,
    capabilities: { ...defaultCapabilities, ...item.capabilities },
    creditCost: item.creditCost,
    isDefault: Boolean(item.isDefault),
    sortOrder: item.sortOrder,
  };
}

function platformLabel(value: string) {
  return providerPlatformLabel(value);
}

function normalizeDraft(input: DraftState): BusinessImageModelInput {
  return {
    ...input,
    id: input.id?.trim() || undefined,
    vendor: input.vendor.trim().toLowerCase(),
    vendorLabel: input.vendorLabel.trim(),
    displayName: input.displayName.trim(),
    upstreamModel: input.upstreamModel.trim(),
    adapter: input.adapter.trim(),
    platform: input.platform,
    creditCost: Math.max(0, Math.floor(Number(input.creditCost) || 0)),
    sortOrder: Math.max(0, Math.floor(Number(input.sortOrder) || 0)),
    capabilities: {
      ...input.capabilities,
      maxImages: Math.max(1, Math.floor(Number(input.capabilities.maxImages) || 1)),
      maxReferenceImages: Math.max(1, Math.floor(Number(input.capabilities.maxReferenceImages) || 8)),
    },
  };
}

function updatePlatformDefaults(draft: DraftState, platform: APIAccessPlatform): DraftState {
  const meta = providerPlatformMeta(platform);
  return {
    ...draft,
    platform,
    vendor: meta.vendor,
    vendorLabel: meta.vendorLabel,
    adapter: meta.adapter,
  };
}

export function ImageModelsSection() {
  const [items, setItems] = useState<BusinessImageModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [query, setQuery] = useState("");
  const [enabledFilter, setEnabledFilter] = useState<EnabledFilter>("all");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<DraftState>(emptyDraft);
  const [modalOpen, setModalOpen] = useState(false);
  const [testingModelId, setTestingModelId] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, ModelTestResult>>({});

  const loadItems = async () => {
    setLoading(true);
    try {
      const payload = await fetchAdminBusinessImageModels();
      setItems(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取模型目录失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadItems();
  }, []);

  const filteredItems = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return items.filter((item) => {
      if (enabledFilter === "enabled" && !item.enabled) return false;
      if (enabledFilter === "disabled" && item.enabled) return false;
      if (!keyword) return true;
      return [
        item.id,
        item.vendor,
        item.vendorLabel,
        item.displayName,
        item.platform,
        item.upstreamModel,
        item.adapter,
      ].some((value) => String(value || "").toLowerCase().includes(keyword));
    });
  }, [enabledFilter, items, query]);

  const openCreate = () => {
    setEditingId(null);
    setDraft(emptyDraft());
    setModalOpen(true);
  };

  const openEdit = (item: BusinessImageModel) => {
    setEditingId(item.id);
    setDraft(modelToDraft(item));
    setModalOpen(true);
  };

  const closeModal = () => {
    if (submitting) return;
    setModalOpen(false);
    setEditingId(null);
  };

  const handleSubmit = async () => {
    const payload = normalizeDraft(draft);
    if (!payload.vendor || !payload.vendorLabel || !payload.displayName || !payload.upstreamModel) {
      toast.error("请完整填写厂商、显示名和上游模型");
      return;
    }
    setSubmitting(true);
    try {
      if (editingId) {
        await updateBusinessImageModel(editingId, payload);
        toast.success("模型已更新");
      } else {
        await createBusinessImageModel(payload);
        toast.success("模型已添加");
      }
      closeModal();
      await loadItems();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存模型失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleSetDefault = async (item: BusinessImageModel) => {
    setSubmitting(true);
    try {
      await setDefaultBusinessImageModel(item.id);
      toast.success("默认模型已切换");
      await loadItems();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "切换默认模型失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (item: BusinessImageModel) => {
    if (!window.confirm(`确认删除模型「${item.displayName}」？`)) {
      return;
    }
    setSubmitting(true);
    try {
      await deleteBusinessImageModel(item.id);
      toast.success("模型已删除");
      await loadItems();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除模型失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleTest = async (item: BusinessImageModel) => {
    if (!window.confirm(`确认真实请求上游测试「${item.displayName}」？这可能消耗上游额度。`)) {
      return;
    }
    setTestingModelId(item.id);
    setTestResults((current) => ({
      ...current,
      [item.id]: {
        status: "running",
        message: "正在请求上游",
        testedAt: new Date().toISOString(),
      },
    }));
    try {
      const result = await testBusinessImageModel(item.id);
      setTestResults((current) => ({
        ...current,
        [item.id]: {
          status: result.ok ? "succeeded" : "failed",
          message: result.message || (result.ok ? "测试成功" : "测试失败"),
          durationMs: result.durationMs,
          imageCount: result.imageCount,
          testedAt: new Date().toISOString(),
        },
      }));
      if (result.ok) {
        toast.success("模型测试成功");
      } else {
        toast.error(result.message || "模型测试失败");
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "模型测试失败";
      setTestResults((current) => ({
        ...current,
        [item.id]: {
          status: "failed",
          message,
          testedAt: new Date().toISOString(),
        },
      }));
      toast.error(message);
    } finally {
      setTestingModelId(null);
    }
  };

  const updateDraft = <K extends keyof DraftState>(key: K, value: DraftState[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const updateCapabilities = <K extends keyof ImageModelCapabilities>(key: K, value: ImageModelCapabilities[K]) => {
    setDraft((current) => ({
      ...current,
      capabilities: { ...current.capabilities, [key]: value },
    }));
  };

  return (
    <ConfigSection
      title="模型目录"
      description="维护工作台可选模型、默认模型、对比状态和每张扣点。生图请求会按 modelId 路由并按模型扣点。"
      actions={
        <>
          <button className="app-btn" type="button" onClick={() => void loadItems()} disabled={loading}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
          <button className="app-btn-primary" type="button" onClick={openCreate}>
            <Plus className="size-4" />
            添加模型
          </button>
        </>
      }
    >
      <div className="md:col-span-2">
        <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
            <input
              className="app-input pl-9"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索模型、厂商、平台..."
            />
          </div>
          <div className="w-full sm:w-40">
            <AppSelect
              value={enabledFilter}
              onChange={(value) => setEnabledFilter(value as EnabledFilter)}
              options={[
                { value: "all", label: "全部状态" },
                { value: "enabled", label: "已启用" },
                { value: "disabled", label: "已停用" },
              ]}
            />
          </div>
        </div>

        <div className={settingsTableWrapClass}>
          <table className={adminTableClass}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th className="px-3 py-3 text-left font-medium">模型</th>
                <th className="px-3 py-3 text-left font-medium">平台</th>
                <th className="px-3 py-3 text-left font-medium">扣点</th>
                <th className="px-3 py-3 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {loading ? (
                <tr>
                  <td className="px-3 py-8 text-center text-[var(--app-text-muted)]" colSpan={4}>
                    <LoaderCircle className="mr-2 inline size-4 animate-spin" />
                    正在读取模型目录
                  </td>
                </tr>
              ) : filteredItems.length === 0 ? (
                <tr>
                  <td className="px-3 py-8 text-center text-[var(--app-text-muted)]" colSpan={4}>
                    暂无模型
                  </td>
                </tr>
              ) : (
                filteredItems.map((item) => (
                  <tr className={adminTableRowClass} key={item.id}>
                    <td className="px-3 py-3">
                      <div className="flex min-w-0 flex-col gap-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-semibold text-[var(--app-text-primary)]">{item.displayName}</span>
                          {item.isDefault ? (
                            <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 px-2 py-0.5 text-xs font-semibold text-amber-300">
                              <Star className="size-3" />
                              默认
                            </span>
                          ) : null}
                          {item.preview ? <span className="rounded-full bg-cyan-500/15 px-2 py-0.5 text-xs font-semibold text-cyan-300">Preview</span> : null}
                        </div>
                        <div className="text-xs text-[var(--app-text-muted)]">{item.id}</div>
                        <div className="text-xs text-[var(--app-text-muted)]">{item.upstreamModel}</div>
                        <div className="flex flex-wrap gap-1.5 pt-1">
                          <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", item.enabled ? "bg-emerald-500/15 text-emerald-300" : "bg-zinc-500/15 text-zinc-300")}>
                            {item.enabled ? "启用" : "停用"}
                          </span>
                          <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", item.compareEnabled ? "bg-blue-500/15 text-blue-300" : "bg-zinc-500/15 text-zinc-300")}>
                            {item.compareEnabled ? "可对比" : "不对比"}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3">
                      <div className="font-medium text-[var(--app-text-secondary)]">{item.vendorLabel}</div>
                      <div className="text-xs text-[var(--app-text-muted)]">{platformLabel(String(item.platform))} / {item.adapter}</div>
                    </td>
                    <td className="px-3 py-3 font-semibold text-[var(--app-text-primary)]">{item.creditCost}</td>
                    <td className="px-3 py-3">
                      <div className="flex justify-end gap-2">
                        <button className="app-icon-btn" type="button" title="设为默认" onClick={() => void handleSetDefault(item)} disabled={submitting || item.isDefault || !item.enabled}>
                          <CheckCircle2 className="size-4" />
                        </button>
                        <button className="app-icon-btn" type="button" title="真实测试模型" onClick={() => void handleTest(item)} disabled={submitting || testingModelId === item.id || !item.enabled || !item.availability?.available}>
                          {testingModelId === item.id ? <LoaderCircle className="size-4 animate-spin" /> : <FlaskConical className="size-4" />}
                        </button>
                        <button className="app-icon-btn" type="button" title="编辑" onClick={() => openEdit(item)} disabled={submitting}>
                          <Pencil className="size-4" />
                        </button>
                        <button className="app-icon-btn danger" type="button" title="删除" onClick={() => void handleDelete(item)} disabled={submitting}>
                          <Trash2 className="size-4" />
                        </button>
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
        open={modalOpen}
        onClose={closeModal}
        title={editingId ? "编辑模型" : "添加模型"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={closeModal} disabled={submitting}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void handleSubmit()} disabled={submitting}>
              {submitting ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </button>
          </>
        }
      >
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="模型 ID" hint="稳定主键，例如 openai/gpt-image-2；创建后不建议修改。">
            <input className="app-input" value={draft.id || ""} disabled={Boolean(editingId)} onChange={(event) => updateDraft("id", event.target.value)} placeholder="openai/gpt-image-2" />
          </Field>
          <Field label="显示名" hint="工作台展示给用户看的模型名称。">
            <input className="app-input" value={draft.displayName} onChange={(event) => updateDraft("displayName", event.target.value)} placeholder="GPT Image 2" />
          </Field>
          <Field label="平台" hint="决定使用哪类上游适配器和号池。">
            <AppSelect value={draft.platform} onChange={(value) => setDraft((current) => updatePlatformDefaults(current, value as APIAccessPlatform))} options={providerPlatformOptions} />
          </Field>
          <Field label="适配器" hint="OpenAI Images 表示 OpenAI 兼容图片接口；Gemini 表示 Google Gemini 图片接口。">
            <AppSelect value={draft.adapter} onChange={(value) => updateDraft("adapter", value)} options={adapterOptions} />
          </Field>
          <Field label="厂商标识" hint="英文标识，用于搜索和后续分组。">
            <input className="app-input" value={draft.vendor} onChange={(event) => updateDraft("vendor", event.target.value)} placeholder="openai" />
          </Field>
          <Field label="厂商显示名" hint="后台展示用。">
            <input className="app-input" value={draft.vendorLabel} onChange={(event) => updateDraft("vendorLabel", event.target.value)} placeholder="OpenAI" />
          </Field>
          <Field label="上游模型" hint="真正发送给上游 API 的 model 字段。">
            <input className="app-input" value={draft.upstreamModel} onChange={(event) => updateDraft("upstreamModel", event.target.value)} placeholder="gpt-image-2" />
          </Field>
          <Field label="每张扣点" hint="按模型独立扣点；请求多张时按张数相乘。">
            <input className="app-input" type="number" min="0" step="1" value={draft.creditCost} onChange={(event) => updateDraft("creditCost", Number(event.target.value))} />
          </Field>
          <Field label="排序" hint="数字越小越靠前。">
            <input className="app-input" type="number" min="0" step="1" value={draft.sortOrder} onChange={(event) => updateDraft("sortOrder", Number(event.target.value))} />
          </Field>
          <Field label="最大参考图" hint="前端能力描述，后续可用于限制上传数量。">
            <input className="app-input" type="number" min="1" step="1" value={draft.capabilities.maxReferenceImages} onChange={(event) => updateCapabilities("maxReferenceImages", Number(event.target.value))} />
          </Field>
          <div className="md:col-span-2 grid gap-2 sm:grid-cols-2">
            {[
              ["enabled", "启用"],
              ["preview", "Preview"],
              ["compareEnabled", "参与多模型对比"],
              ["isDefault", "设为默认模型"],
            ].map(([key, label]) => (
              <label key={key} className="flex items-center justify-between rounded-[var(--app-radius-md)] border border-[var(--app-border)] px-3 py-2 text-sm text-[var(--app-text-secondary)]">
                <span>{label}</span>
                <input
                  type="checkbox"
                  checked={Boolean(draft[key as keyof DraftState])}
                  onChange={(event) => updateDraft(key as keyof DraftState, event.target.checked as never)}
                />
              </label>
            ))}
          </div>
          <div className="md:col-span-2 grid gap-2 sm:grid-cols-4">
            {[
              ["generate", "生成"],
              ["edit", "编辑"],
              ["referenceImage", "参考图"],
              ["mask", "蒙版"],
            ].map(([key, label]) => (
              <label key={key} className="flex items-center justify-between rounded-[var(--app-radius-md)] border border-[var(--app-border)] px-3 py-2 text-sm text-[var(--app-text-secondary)]">
                <span>{label}</span>
                <input
                  type="checkbox"
                  checked={Boolean(draft.capabilities[key as keyof ImageModelCapabilities])}
                  onChange={(event) => updateCapabilities(key as keyof ImageModelCapabilities, event.target.checked as never)}
                />
              </label>
            ))}
          </div>
        </div>
      </AppModal>
    </ConfigSection>
  );
}
