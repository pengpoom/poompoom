"use client";

import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  LoaderCircle,
  Pencil,
  Plus,
  Power,
  RefreshCw,
  Search,
  Send,
  Star,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
  createBusinessAPIProvider,
  deleteBusinessAPIProvider,
  fetchBusinessAPIProviders,
  setDefaultBusinessAPIProvider,
  testBusinessAPIProvider,
  updateBusinessAPIProvider,
  type APIAccessPlatform,
  type BusinessAPIProvider,
  type BusinessAPIProviderInput,
} from "@/lib/api";
import { cn } from "@/lib/utils";

import { ConfigSection, Field, TooltipDetails } from "./shared";
import {
  settingsCounterClass,
  settingsInputClass,
  settingsSelectClass,
  settingsSmallButtonClass,
  settingsTableWrapClass,
} from "./styles";

const allFilterValue = "all";

const platformOptions: Array<{ label: string; value: APIAccessPlatform }> = [
  { label: "gpt-image", value: "gpt-image" },
  { label: "gemini-banana", value: "gemini-banana" },
];

const geminiBananaModelOptions = [
  "gemini-3.1-flash-image-preview",
  "gemini-3-pro-image-preview",
  "gemini-2.5-flash-image",
  "gemini-2.5-flash-image-preview",
];

type EnabledFilter = "all" | "enabled" | "disabled";
type ProviderTestResult = {
  status: "running" | "succeeded" | "failed";
  message: string;
  durationMs?: number;
  imageCount?: number;
  testedAt: string;
};

function defaultModelForPlatform(platform: APIAccessPlatform) {
  return platform === "gemini-banana" ? "gemini-2.5-flash-image" : "gpt-image-2";
}

function createEmptyDraft(): BusinessAPIProviderInput {
  return {
    name: "",
    platform: "gpt-image",
    baseUrl: "",
    apiKey: "",
    defaultModel: "gpt-image-2",
    enabled: true,
    isDefault: false,
  };
}

function providerToDraft(provider: BusinessAPIProvider): BusinessAPIProviderInput {
  return {
    name: provider.name,
    platform: provider.platform,
    baseUrl: provider.baseUrl,
    apiKey: provider.apiKey,
    defaultModel: provider.defaultModel,
    enabled: provider.enabled,
    isDefault: provider.isDefault,
  };
}

function maskSecret(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "未填写";
  }
  if (trimmed.length <= 10) {
    return "已填写";
  }
  return `${trimmed.slice(0, 4)}...${trimmed.slice(-4)}`;
}

function formatProviderTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function platformLabel(value: APIAccessPlatform) {
  return platformOptions.find((item) => item.value === value)?.label ?? value;
}

function providerTestResultText(result: ProviderTestResult) {
  if (result.status === "running") {
    return "测试中：正在请求上游图片接口";
  }
  if (result.status === "succeeded") {
    return `测试成功：返回 ${result.imageCount ?? 0} 张图片，用时 ${result.durationMs ?? 0}ms`;
  }
  return `测试失败：${result.message}`;
}

export function APIAccessSection() {
  const [items, setItems] = useState<BusinessAPIProvider[]>([]);
  const [draft, setDraft] = useState<BusinessAPIProviderInput>(createEmptyDraft);
  const [editingId, setEditingId] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [platformFilter, setPlatformFilter] = useState<typeof allFilterValue | APIAccessPlatform>(allFilterValue);
  const [enabledFilter, setEnabledFilter] = useState<EnabledFilter>("all");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [busyId, setBusyId] = useState("");
  const [testingId, setTestingId] = useState("");
  const [testResults, setTestResults] = useState<Record<string, ProviderTestResult>>({});

  const filteredItems = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return items.filter((item) => {
      if (platformFilter !== allFilterValue && item.platform !== platformFilter) {
        return false;
      }
      if (enabledFilter === "enabled" && !item.enabled) {
        return false;
      }
      if (enabledFilter === "disabled" && item.enabled) {
        return false;
      }
      if (!keyword) {
        return true;
      }
      return [
        item.name,
        item.platform,
        item.baseUrl,
        item.defaultModel,
      ].some((value) => value.toLowerCase().includes(keyword));
    });
  }, [enabledFilter, items, platformFilter, searchText]);

  const geminiModelOptionsForDraft = useMemo(() => {
    const currentModel = draft.defaultModel.trim();
    if (!currentModel || geminiBananaModelOptions.includes(currentModel)) {
      return geminiBananaModelOptions;
    }
    return [currentModel, ...geminiBananaModelOptions];
  }, [draft.defaultModel]);

  const enabledCount = useMemo(
    () => items.filter((item) => item.enabled).length,
    [items],
  );

  const loadProviders = async () => {
    setIsLoading(true);
    try {
      const result = await fetchBusinessAPIProviders();
      setItems(result.items);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取 API 接入失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadProviders();
  }, []);

  const openCreateDialog = () => {
    setEditingId("");
    setDraft(createEmptyDraft());
    setDialogOpen(true);
  };

  const openEditDialog = (provider: BusinessAPIProvider) => {
    setEditingId(provider.id);
    setDraft(providerToDraft(provider));
    setDialogOpen(true);
  };

  const resetDraft = () => {
    setEditingId("");
    setDraft(createEmptyDraft());
  };

  const closeDialog = () => {
    setDialogOpen(false);
    resetDraft();
  };

  const updateDraft = <K extends keyof BusinessAPIProviderInput>(
    key: K,
    value: BusinessAPIProviderInput[K],
  ) => {
    setDraft((current) => {
      const next = {
        ...current,
        [key]: value,
      };
      if (
        key === "platform" &&
        (!current.defaultModel ||
          current.defaultModel === defaultModelForPlatform(current.platform))
      ) {
        next.defaultModel = defaultModelForPlatform(value as APIAccessPlatform);
      }
      return next;
    });
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      if (editingId) {
        await updateBusinessAPIProvider(editingId, draft);
        toast.success("API 接入已更新");
      } else {
        await createBusinessAPIProvider(draft);
        toast.success("API 接入已添加");
      }
      closeDialog();
      await loadProviders();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存 API 接入失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleSetDefault = async (provider: BusinessAPIProvider) => {
    setBusyId(provider.id);
    try {
      await setDefaultBusinessAPIProvider(provider.id);
      toast.success("默认 API 接入已切换");
      await loadProviders();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "切换默认接入失败");
    } finally {
      setBusyId("");
    }
  };

  const handleToggleEnabled = async (provider: BusinessAPIProvider) => {
    setBusyId(provider.id);
    try {
      await updateBusinessAPIProvider(provider.id, {
        ...providerToDraft(provider),
        enabled: !provider.enabled,
      });
      toast.success(provider.enabled ? "API 接入已禁用" : "API 接入已启用");
      await loadProviders();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新启用状态失败");
    } finally {
      setBusyId("");
    }
  };

  const handleDelete = async (provider: BusinessAPIProvider) => {
    if (!window.confirm(`确认删除 API 接入「${provider.name}」？`)) {
      return;
    }
    setBusyId(provider.id);
    try {
      await deleteBusinessAPIProvider(provider.id);
      if (editingId === provider.id) {
        closeDialog();
      }
      toast.success("API 接入已删除");
      await loadProviders();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除 API 接入失败");
    } finally {
      setBusyId("");
    }
  };

  const handleTestProvider = async (provider: BusinessAPIProvider) => {
    setBusyId(provider.id);
    setTestingId(provider.id);
    setTestResults((current) => ({
      ...current,
      [provider.id]: {
        status: "running",
        message: "正在请求上游图片接口",
        testedAt: new Date().toISOString(),
      },
    }));
    try {
      const result = await testBusinessAPIProvider(provider.id);
      const nextResult: ProviderTestResult = {
        status: result.ok ? "succeeded" : "failed",
        message: result.message || (result.ok ? "测试成功" : "测试失败"),
        durationMs: result.durationMs,
        imageCount: result.imageCount,
        testedAt: new Date().toISOString(),
      };
      setTestResults((current) => ({
        ...current,
        [provider.id]: nextResult,
      }));
      if (result.ok) {
        toast.success(`测试成功：返回 ${result.imageCount} 张图片，用时 ${result.durationMs}ms`);
        return;
      }
      toast.error(result.message || "测试失败");
    } catch (error) {
      const message = error instanceof Error ? error.message : "测试 API 接入失败";
      setTestResults((current) => ({
        ...current,
        [provider.id]: {
          status: "failed",
          message,
          testedAt: new Date().toISOString(),
        },
      }));
      toast.error(message);
    } finally {
      setTestingId("");
      setBusyId("");
    }
  };

  return (
    <ConfigSection
      title="API 接入"
      description="维护图片生成平台的接口地址和密钥。工作台选择平台后，后端会使用该平台当前启用的默认接入。"
      actions={
        <>
          <Button
            type="button"
            variant="outline"
            className={settingsSmallButtonClass}
            onClick={() => void loadProviders()}
            disabled={isLoading || isSaving}
          >
            {isLoading ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              <RefreshCw className="size-4" />
            )}
            刷新
          </Button>
          <Button
            type="button"
            className={settingsSmallButtonClass}
            onClick={openCreateDialog}
            disabled={isLoading || isSaving}
          >
            <Plus className="size-4" />
            添加账号
          </Button>
        </>
      }
    >
      <div className="md:col-span-2">
        <div className="mb-3 grid gap-2 lg:grid-cols-[minmax(220px,1fr)_180px_160px]">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
            <Input
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
              placeholder="搜索名称、平台、Base URL、模型..."
              className={cn(settingsInputClass, "pl-9")}
            />
          </div>
          <Select
            value={platformFilter}
            onValueChange={(value) =>
              setPlatformFilter(value as typeof allFilterValue | APIAccessPlatform)
            }
          >
            <SelectTrigger className={settingsSelectClass}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={allFilterValue}>全部平台</SelectItem>
              {platformOptions.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={enabledFilter}
            onValueChange={(value) => setEnabledFilter(value as EnabledFilter)}
          >
            <SelectTrigger className={settingsSelectClass}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="enabled">已启用</SelectItem>
              <SelectItem value="disabled">已禁用</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="mb-3 flex flex-wrap gap-2 text-xs text-[var(--app-text-muted)]">
          <span className={settingsCounterClass}>总计 {items.length}</span>
          <span className="rounded-full bg-emerald-950/30 px-2.5 py-1 text-emerald-200">启用 {enabledCount}</span>
          <span className={settingsCounterClass}>当前显示 {filteredItems.length}</span>
        </div>

        <div className={settingsTableWrapClass}>
          <div className="grid min-w-[1120px] grid-cols-[minmax(190px,1.1fr)_140px_minmax(220px,1.4fr)_minmax(160px,1fr)_120px_280px] border-b border-[var(--app-border)] bg-[var(--app-bg-surface)] px-4 py-3 text-xs font-medium text-[var(--app-text-muted)]">
            <div>名称</div>
            <div>平台</div>
            <div>Base URL</div>
            <div>模型</div>
            <div>状态</div>
            <div>操作</div>
          </div>
          {isLoading ? (
            <div className="flex min-w-[1120px] items-center justify-center gap-2 px-4 py-10 text-sm text-[var(--app-text-muted)]">
              <LoaderCircle className="size-4 animate-spin" />
              正在读取 API 接入
            </div>
          ) : filteredItems.length === 0 ? (
            <div className="min-w-[1120px] px-4 py-10 text-center text-sm text-[var(--app-text-muted)]">
              没有匹配的 API 接入
            </div>
          ) : (
            filteredItems.map((provider) => {
              const testResult = testResults[provider.id];
              return (
              <div
                key={provider.id}
                className="border-b border-[var(--app-border)] last:border-b-0"
              >
                <div className="grid min-w-[1120px] grid-cols-[minmax(190px,1.1fr)_140px_minmax(220px,1.4fr)_minmax(160px,1fr)_120px_280px] items-center px-4 py-3 text-sm">
                  <div className="min-w-0">
                    <div className="truncate font-medium text-[var(--app-text-primary)]">{provider.name}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">
                      更新 {formatProviderTime(provider.updatedAt)}
                    </div>
                  </div>
                  <div>
                    <span className="rounded-full bg-[var(--app-bg-surface)] px-2.5 py-1 text-xs font-medium text-[var(--app-text-secondary)]">
                      {platformLabel(provider.platform)}
                    </span>
                  </div>
                  <div className="min-w-0 break-all pr-4 text-xs leading-5 text-[var(--app-text-muted)]">
                    {provider.baseUrl}
                  </div>
                  <div className="min-w-0 pr-4">
                    <div className="truncate text-xs text-[var(--app-text-secondary)]">{provider.defaultModel}</div>
                    <div className="mt-1 text-xs text-[var(--app-text-muted)]">Key {maskSecret(provider.apiKey)}</div>
                  </div>
                  <div className="space-y-1">
                    <span
                      className={cn(
                        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-medium",
                        provider.enabled
                          ? "bg-emerald-950/30 text-emerald-200"
                          : "bg-[var(--app-bg-surface)] text-[var(--app-text-muted)]",
                      )}
                    >
                      {provider.enabled ? <CheckCircle2 className="size-3" /> : <Power className="size-3" />}
                      {provider.enabled ? "启用" : "禁用"}
                    </span>
                    {provider.isDefault ? (
                      <div className="text-xs font-medium text-amber-600">默认</div>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    <Button
                      type="button"
                      variant="outline"
                      className="h-8 px-2.5 text-xs"
                      onClick={() => void handleTestProvider(provider)}
                      disabled={busyId === provider.id}
                    >
                      {busyId === provider.id ? (
                        <LoaderCircle className="size-3.5 animate-spin" />
                      ) : (
                        <Send className="size-3.5" />
                      )}
                      {testingId === provider.id ? "测试中" : "测试"}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      className="h-8 px-2.5 text-xs"
                      onClick={() => openEditDialog(provider)}
                      disabled={busyId === provider.id}
                    >
                      <Pencil className="size-3.5" />
                      编辑
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      className="h-8 px-2.5 text-xs"
                      onClick={() => void handleSetDefault(provider)}
                      disabled={provider.isDefault || busyId === provider.id}
                    >
                      <Star className="size-3.5" />
                      默认
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      className="h-8 px-2.5 text-xs"
                      onClick={() => void handleToggleEnabled(provider)}
                      disabled={busyId === provider.id}
                    >
                      {provider.enabled ? "禁用" : "启用"}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      className="h-8 px-2.5 text-xs"
                      onClick={() => void handleDelete(provider)}
                      disabled={busyId === provider.id}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </div>
                {testResult ? (
                  <div className="px-4 pb-3">
                    <div
                      className={cn(
                        "flex items-start gap-2 rounded-[var(--app-radius-md)] border px-3 py-2 text-xs leading-5",
                        testResult.status === "running" &&
                          "border-sky-500/30 bg-sky-950/30 text-sky-200",
                        testResult.status === "succeeded" &&
                          "border-emerald-500/30 bg-emerald-950/30 text-emerald-200",
                        testResult.status === "failed" &&
                          "border-rose-500/30 bg-rose-950/30 text-rose-200",
                      )}
                    >
                      {testResult.status === "running" ? (
                        <LoaderCircle className="mt-0.5 size-3.5 shrink-0 animate-spin" />
                      ) : testResult.status === "succeeded" ? (
                        <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
                      ) : (
                        <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                      )}
                      <div className="min-w-0">
                        <div className="break-words font-medium">
                          {providerTestResultText(testResult)}
                        </div>
                        <div className="mt-0.5 text-[11px] opacity-70">
                          {formatProviderTime(testResult.testedAt)}
                        </div>
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>
              );
            })
          )}
        </div>
      </div>

      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (isSaving) {
            return;
          }
          if (open) {
            setDialogOpen(true);
            return;
          }
          closeDialog();
        }}
      >
        <DialogContent className="max-h-[90vh] w-[min(92vw,760px)] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingId ? "编辑 API 接入" : "添加 API 接入"}</DialogTitle>
            <DialogDescription>
              配置业务后台请求上游图片模型时使用的 Base URL、Key 和请求模型。
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 md:grid-cols-2">
            <Field
              label="名称"
              hint="用于区分不同 API 接入。"
            >
              <Input
                value={draft.name}
                onChange={(event) => updateDraft("name", event.target.value)}
                placeholder="默认 API 接入"
                className={settingsInputClass}
              />
            </Field>

            <Field
              label="平台"
              hint="选择这个接入对应的图片平台。"
              tooltip={
                <TooltipDetails
                  items={[
                    {
                      title: "gpt-image",
                      body: <>OpenAI 兼容图片接口。</>,
                    },
                    {
                      title: "gemini-banana",
                      body: <>Gemini 图片接口。</>,
                    },
                  ]}
                />
              }
            >
              <Select
                value={draft.platform}
                onValueChange={(value) => updateDraft("platform", value as APIAccessPlatform)}
              >
                <SelectTrigger className={settingsSelectClass}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {platformOptions.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>

            <Field
              label="Base URL 地址"
              hint="填写上游服务地址。"
              fullWidth
            >
              <Input
                value={draft.baseUrl}
                onChange={(event) => updateDraft("baseUrl", event.target.value)}
                placeholder={
                  draft.platform === "gemini-banana"
                    ? "https://generativelanguage.googleapis.com"
                    : "http://127.0.0.1:8080"
                }
                className={settingsInputClass}
              />
            </Field>

            <Field
              label={draft.platform === "gemini-banana" ? "请求模型" : "默认模型"}
              hint={
                draft.platform === "gemini-banana"
                  ? "选择 Gemini 图片生成请求使用的模型。"
                  : "上游请求的默认模型。"
              }
            >
              {draft.platform === "gemini-banana" ? (
                <Select
                  value={draft.defaultModel || defaultModelForPlatform(draft.platform)}
                  onValueChange={(value) => updateDraft("defaultModel", value)}
                >
                  <SelectTrigger className={settingsSelectClass}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {geminiModelOptionsForDraft.map((model) => (
                      <SelectItem key={model} value={model}>
                        {model}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  value={draft.defaultModel}
                  onChange={(event) => updateDraft("defaultModel", event.target.value)}
                  placeholder={defaultModelForPlatform(draft.platform)}
                  className={settingsInputClass}
                />
              )}
            </Field>

            <Field
              label="API Key"
              hint="后端请求上游时使用。"
            >
              <Input
                type="password"
                value={draft.apiKey}
                onChange={(event) => updateDraft("apiKey", event.target.value)}
                placeholder={draft.platform === "gemini-banana" ? "AIza..." : "sk-..."}
                className={settingsInputClass}
              />
            </Field>

            <div className="flex flex-wrap items-center gap-2 md:col-span-2">
              <Button
                type="button"
                variant={draft.enabled ? "default" : "outline"}
                className={settingsSmallButtonClass}
                onClick={() => updateDraft("enabled", !draft.enabled)}
              >
                {draft.enabled ? "已启用" : "已禁用"}
              </Button>
              <Button
                type="button"
                variant={draft.isDefault ? "default" : "outline"}
                className={settingsSmallButtonClass}
                onClick={() => updateDraft("isDefault", !draft.isDefault)}
              >
                <Star className="size-3.5" />
                {draft.isDefault ? "默认接入" : "设为默认"}
              </Button>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="px-4"
              onClick={closeDialog}
              disabled={isSaving}
            >
              取消
            </Button>
            <Button
              type="button"
              className="px-4"
              onClick={() => void handleSave()}
              disabled={isSaving}
            >
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              {editingId ? "更新" : "添加"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfigSection>
  );
}
