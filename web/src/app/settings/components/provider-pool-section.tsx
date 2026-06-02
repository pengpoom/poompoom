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

import { AppModal, AppSelect } from "@/components/app-controls";
import { cn } from "@/lib/utils";
import {
  createBusinessProviderGroup,
  createBusinessProviderMember,
  deleteBusinessProviderGroup,
  deleteBusinessProviderMember,
  fetchAdminBusinessImageModels,
  fetchBusinessProviderPools,
  fetchBusinessSystemSettings,
  previewBusinessProviderDispatch,
  recoverBusinessProviderMember,
  setDefaultBusinessProviderGroup,
  testBusinessProviderGroup,
  testBusinessProviderMember,
  updateBusinessProviderGroup,
  updateBusinessProviderMember,
  type APIAccessPlatform,
  type BusinessAPIProviderTestResponse,
  type BusinessBillingLevel,
  type BusinessImageModel,
  type BusinessProviderDispatchPreviewPool,
  type BusinessProviderDispatchPreviewInput,
  type BusinessProviderDispatchPreviewResponse,
  type BusinessProviderGroup,
  type BusinessProviderGroupInput,
  type BusinessProviderMember,
  type BusinessProviderMemberInput,
  type BusinessProviderMemberStatus,
  type BusinessProviderPool,
} from "@/lib/api";
import {
  defaultModelForPlatform,
  isGeminiPlatform,
  providerPlatformLabel,
  providerPlatformOptions,
} from "@/lib/provider-platforms";

import { ConfigSection, Field, TooltipDetails } from "./shared";
import { settingsTableWrapClass } from "./styles";

const allFilterValue = "all";

const statusOptions: Array<{ label: string; value: BusinessProviderMemberStatus }> = [
  { label: "active", value: "active" },
  { label: "limited", value: "limited" },
  { label: "unavailable", value: "unavailable" },
];

const matchModeOptions: Array<{ label: string; value: BusinessProviderGroupInput["matchMode"] }> = [
  { label: "兜底池", value: "fallback" },
  { label: "命中任一标签", value: "any" },
  { label: "命中全部标签", value: "all" },
];

const defaultSubscriptionLevelOptions: BusinessBillingLevel[] = [
  { name: "Free", tag: "tier:free", description: "默认等级", enabled: true, sortOrder: 0 },
  { name: "Lumen", tag: "tier:lumen", description: "入门订阅", enabled: true, sortOrder: 10 },
  { name: "Prism", tag: "tier:prism", description: "标准订阅", enabled: true, sortOrder: 20 },
  { name: "Atelier", tag: "tier:atelier", description: "创作订阅", enabled: true, sortOrder: 30 },
  { name: "Meridian", tag: "tier:meridian", description: "高阶订阅", enabled: true, sortOrder: 40 },
];

const defaultWalletLevelOptions: BusinessBillingLevel[] = [
  { name: "None", tag: "wallet:none", description: "未充值", enabled: true, sortOrder: 0 },
  { name: "Ember", tag: "wallet:ember", description: "小额充值", enabled: true, sortOrder: 10 },
  { name: "Glow", tag: "wallet:glow", description: "中等充值", enabled: true, sortOrder: 20 },
  { name: "Flare", tag: "wallet:flare", description: "高价值充值", enabled: true, sortOrder: 30 },
  { name: "Radiant", tag: "wallet:radiant", description: "重度充值", enabled: true, sortOrder: 40 },
  { name: "Zenith", tag: "wallet:zenith", description: "顶级充值用户", enabled: true, sortOrder: 50 },
];

type EnabledFilter = "all" | "enabled" | "disabled";
type DialogState =
  | { type: "none" }
  | { type: "group"; id?: string }
  | { type: "member"; id?: string };
type ProviderTestResult = {
  status: "running" | "succeeded" | "failed";
  message: string;
  durationMs?: number;
  imageCount?: number;
  testedAt: string;
  memberName?: string;
};

type PreviewDraft = BusinessProviderDispatchPreviewInput & {
  extraTagsText: string;
};

function createEmptyGroupDraft(): BusinessProviderGroupInput {
  return {
    name: "",
    platform: "gpt-image",
    description: "",
    tags: "",
    matchMode: "fallback",
    enabled: true,
    isDefault: false,
    priority: 50,
  };
}

function createEmptyMemberDraft(group?: BusinessProviderGroup): BusinessProviderMemberInput {
  const platform = group?.platform ?? "gpt-image";
  return {
    groupId: group?.id ?? "",
    name: "",
    platform,
    baseUrl: "",
    apiKey: "",
    defaultModel: defaultModelForPlatform(platform),
    enabled: true,
    priority: 50,
    weight: 1,
    maxConcurrent: 0,
    cooldownSeconds: 120,
    failureThreshold: 5,
    status: "active",
  };
}

function createDefaultPreviewDraft(): PreviewDraft {
  return {
    platform: "gpt-image",
    role: "user",
    subscriptionTag: "tier:free",
    walletTag: "wallet:none",
    mode: "generate",
    quality: "high",
    size: "1248x1248",
    model: defaultModelForPlatform("gpt-image"),
    extraTags: [],
    extraTagsText: "",
  };
}

function groupToDraft(group: BusinessProviderGroup): BusinessProviderGroupInput {
  return {
    name: group.name,
    platform: group.platform,
    description: group.description,
    tags: group.tags || "",
    matchMode: group.matchMode || "fallback",
    enabled: group.enabled,
    isDefault: group.isDefault,
    priority: group.priority,
  };
}

function memberToDraft(member: BusinessProviderMember): BusinessProviderMemberInput {
  return {
    groupId: member.groupId,
    name: member.name,
    platform: member.platform,
    baseUrl: member.baseUrl,
    apiKey: member.apiKey,
    defaultModel: member.defaultModel,
    enabled: member.enabled,
    priority: member.priority,
    weight: member.weight || 1,
    maxConcurrent: member.maxConcurrent || 0,
    cooldownSeconds: member.cooldownSeconds || 120,
    failureThreshold: member.failureThreshold || 5,
    status: member.status,
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
  return providerPlatformLabel(value);
}

function statusBadgeClass(status: BusinessProviderMemberStatus) {
  if (status === "active") {
    return "ok";
  }
  if (status === "limited") {
    return "warn";
  }
  return "fail";
}

function providerTestResultText(result: ProviderTestResult) {
  if (result.status === "running") {
    return "测试中：正在请求上游图片接口";
  }
  if (result.status === "succeeded") {
    return `测试成功：${result.memberName ? `${result.memberName} · ` : ""}返回 ${result.imageCount ?? 0} 张图片，用时 ${result.durationMs ?? 0}ms`;
  }
  return `测试失败：${result.memberName ? `${result.memberName} · ` : ""}${result.message}`;
}

function parseDispatchTags(value: string) {
  return value
    .split(/[\s,，]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function previewStrategyLabel(value?: string) {
  switch (value) {
    case "tagged_pool":
      return "标签池";
    case "tagged_pool_fallback":
      return "标签池回退默认池";
    case "fallback_pool":
      return "默认池";
    case "api_fallback":
      return "API 兜底";
    case "api_access":
      return "API 接入";
    default:
      return value || "-";
  }
}

function compactTagText(tags?: string[], limit = 5) {
  const items = (tags || []).map((tag) => tag.trim()).filter(Boolean);
  if (items.length === 0) {
    return "-";
  }
  const visible = items.slice(0, limit).join(" · ");
  return items.length > limit ? `${visible} +${items.length - limit}` : visible;
}

function previewPoolRoleLabel(role: string) {
  if (role === "tagged") return "标签池";
  if (role === "fallback") return "fallback";
  return "未参与";
}

function previewPoolBadgeClass(pool: BusinessProviderDispatchPreviewPool) {
  if (pool.role === "tagged" && pool.availableMembers > 0) return "ok";
  if (pool.role === "fallback" && pool.availableMembers > 0) return "warn";
  if (pool.considered) return "warn";
  return "off";
}

function billingLevelOptions(levels: BusinessBillingLevel[], fallback: BusinessBillingLevel[]) {
  const source = levels.length > 0 ? levels : fallback;
  return source
    .filter((level) => level.enabled && level.tag)
    .sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0))
    .map((level) => ({ value: level.tag, label: level.name || level.tag }));
}

function groupMatches(pool: BusinessProviderPool, keyword: string) {
  if (!keyword) {
    return true;
  }
  return [
    pool.name,
    pool.platform,
    pool.description,
    pool.tags,
    pool.matchMode,
    ...pool.members.flatMap((member) => [
      member.name,
      member.baseUrl,
      member.defaultModel,
      member.status,
    ]),
  ].some((value) => value.toLowerCase().includes(keyword));
}

function groupOptions(pools: BusinessProviderPool[]) {
  return pools.map((pool) => ({
    value: pool.id,
    label: `${pool.name || pool.platform} · ${platformLabel(pool.platform)}`,
  }));
}

function poolMemberCount(pools: BusinessProviderPool[]) {
  return pools.reduce((total, pool) => total + pool.members.length, 0);
}

function enabledMemberCount(pools: BusinessProviderPool[]) {
  return pools.reduce(
    (total, pool) => total + pool.members.filter((member) => member.enabled).length,
    0,
  );
}

function configuredModelsForPlatform(
  models: BusinessImageModel[],
  platform: APIAccessPlatform,
  currentModel: string,
) {
  const configured = models
    .filter((model) => model.enabled && model.platform === platform)
    .map((model) => model.upstreamModel.trim())
    .filter(Boolean);
  const fallback = defaultModelForPlatform(platform);
  const source = configured.length > 0 ? configured : [fallback];
  const seen = new Set<string>();
  const options = source.filter((model) => {
    const key = model.toLowerCase();
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
  const current = currentModel.trim();
  if (current && !seen.has(current.toLowerCase()) && current !== fallback) {
    return [current, ...options];
  }
  return options;
}

function testResultFromResponse(result: BusinessAPIProviderTestResponse): ProviderTestResult {
  return {
    status: result.ok ? "succeeded" : "failed",
    message: result.message || (result.ok ? "测试成功" : "测试失败"),
    durationMs: result.durationMs,
    imageCount: result.imageCount,
    memberName: result.member?.name,
    testedAt: new Date().toISOString(),
  };
}

export function ProviderPoolSection() {
  const [pools, setPools] = useState<BusinessProviderPool[]>([]);
  const [imageModels, setImageModels] = useState<BusinessImageModel[]>([]);
  const [groupDraft, setGroupDraft] = useState<BusinessProviderGroupInput>(createEmptyGroupDraft);
  const [memberDraft, setMemberDraft] = useState<BusinessProviderMemberInput>(createEmptyMemberDraft);
  const [dialog, setDialog] = useState<DialogState>({ type: "none" });
  const [searchText, setSearchText] = useState("");
  const [platformFilter, setPlatformFilter] = useState<typeof allFilterValue | APIAccessPlatform>(allFilterValue);
  const [enabledFilter, setEnabledFilter] = useState<EnabledFilter>("all");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [busyId, setBusyId] = useState("");
  const [testingId, setTestingId] = useState("");
  const [testResults, setTestResults] = useState<Record<string, ProviderTestResult>>({});
  const [subscriptionLevels, setSubscriptionLevels] = useState<BusinessBillingLevel[]>([]);
  const [walletLevels, setWalletLevels] = useState<BusinessBillingLevel[]>([]);
  const [previewDraft, setPreviewDraft] = useState<PreviewDraft>(createDefaultPreviewDraft);
  const [previewResult, setPreviewResult] = useState<BusinessProviderDispatchPreviewResponse | null>(null);
  const [isPreviewing, setIsPreviewing] = useState(false);

  const filteredPools = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return pools.filter((pool) => {
      if (platformFilter !== allFilterValue && pool.platform !== platformFilter) {
        return false;
      }
      if (enabledFilter === "enabled" && !pool.enabled) {
        return false;
      }
      if (enabledFilter === "disabled" && pool.enabled) {
        return false;
      }
      return groupMatches(pool, keyword);
    });
  }, [enabledFilter, platformFilter, pools, searchText]);

  const memberGroup = useMemo(
    () => pools.find((pool) => pool.id === memberDraft.groupId),
    [memberDraft.groupId, pools],
  );

  const memberPlatform = memberGroup?.platform ?? memberDraft.platform;

  const memberModelOptions = useMemo(
    () => configuredModelsForPlatform(imageModels, memberPlatform, memberDraft.defaultModel),
    [imageModels, memberDraft.defaultModel, memberPlatform],
  );

  const previewSubscriptionOptions = useMemo(
    () => billingLevelOptions(subscriptionLevels, defaultSubscriptionLevelOptions),
    [subscriptionLevels],
  );

  const previewWalletOptions = useMemo(
    () => billingLevelOptions(walletLevels, defaultWalletLevelOptions),
    [walletLevels],
  );

  const loadPools = async () => {
    setIsLoading(true);
    try {
      const result = await fetchBusinessProviderPools();
      setPools(result.items);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取 Provider 号池失败");
    } finally {
      setIsLoading(false);
    }
  };

  const loadBillingLevels = async () => {
    try {
      const payload = await fetchBusinessSystemSettings();
      const nextSubscriptions = (payload.settings.billing.subscriptionLevels || []).filter((level) => level.enabled);
      const nextWallets = (payload.settings.billing.walletLevels || []).filter((level) => level.enabled);
      setSubscriptionLevels(nextSubscriptions);
      setWalletLevels(nextWallets);
      setPreviewDraft((current) => ({
        ...current,
        subscriptionTag: nextSubscriptions.some((level) => level.tag === current.subscriptionTag)
          ? current.subscriptionTag
          : nextSubscriptions[0]?.tag || current.subscriptionTag,
        walletTag: nextWallets.some((level) => level.tag === current.walletTag)
          ? current.walletTag
          : nextWallets[0]?.tag || current.walletTag,
      }));
    } catch {
      setSubscriptionLevels(defaultSubscriptionLevelOptions);
      setWalletLevels(defaultWalletLevelOptions);
    }
  };

  const loadImageModels = async () => {
    try {
      const payload = await fetchAdminBusinessImageModels();
      setImageModels(payload.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取模型目录失败");
    }
  };

  useEffect(() => {
    void loadPools();
    void loadBillingLevels();
    void loadImageModels();
  }, []);

  const configuredDefaultModel = (platform: APIAccessPlatform) =>
    configuredModelsForPlatform(imageModels, platform, "")[0] || defaultModelForPlatform(platform);

  const closeDialog = () => {
    setDialog({ type: "none" });
    setGroupDraft(createEmptyGroupDraft());
    setMemberDraft(createEmptyMemberDraft());
  };

  const openCreateGroup = () => {
    setGroupDraft(createEmptyGroupDraft());
    setDialog({ type: "group" });
  };

  const openEditGroup = (pool: BusinessProviderPool) => {
    setGroupDraft(groupToDraft(pool));
    setDialog({ type: "group", id: pool.id });
  };

  const openCreateMember = (pool?: BusinessProviderPool) => {
    if (!pool && pools.length === 0) {
      toast.error("请先创建 Provider 池");
      return;
    }
    const draft = createEmptyMemberDraft(pool ?? pools[0]);
    draft.defaultModel = configuredDefaultModel(draft.platform);
    setMemberDraft(draft);
    setDialog({ type: "member" });
  };

  const openEditMember = (member: BusinessProviderMember) => {
    setMemberDraft(memberToDraft(member));
    setDialog({ type: "member", id: member.id });
  };

  const updateGroupDraft = <K extends keyof BusinessProviderGroupInput>(
    key: K,
    value: BusinessProviderGroupInput[K],
  ) => {
    setGroupDraft((current) => ({ ...current, [key]: value }));
  };

  const updateMemberDraft = <K extends keyof BusinessProviderMemberInput>(
    key: K,
    value: BusinessProviderMemberInput[K],
  ) => {
    setMemberDraft((current) => {
      const next = {
        ...current,
        [key]: value,
      };
      if (key === "groupId") {
        const group = pools.find((pool) => pool.id === value);
        if (group) {
          next.platform = group.platform;
          if (!current.defaultModel || current.defaultModel === defaultModelForPlatform(current.platform)) {
            next.defaultModel = configuredDefaultModel(group.platform);
          }
        }
      }
      return next;
    });
  };

  const updatePreviewDraft = <K extends keyof PreviewDraft>(key: K, value: PreviewDraft[K]) => {
    setPreviewDraft((current) => {
      const next = { ...current, [key]: value };
      if (key === "platform") {
        const platform = value as APIAccessPlatform;
        next.model = configuredDefaultModel(platform);
      }
      return next;
    });
  };

  const handlePreviewDispatch = async () => {
    setIsPreviewing(true);
    try {
      const result = await previewBusinessProviderDispatch({
        platform: previewDraft.platform,
        role: previewDraft.role,
        subscriptionTag: previewDraft.subscriptionTag,
        walletTag: previewDraft.walletTag,
        mode: previewDraft.mode,
        quality: previewDraft.quality,
        size: previewDraft.size,
        model: previewDraft.model,
        extraTags: parseDispatchTags(previewDraft.extraTagsText),
      });
      setPreviewResult(result);
      if (result.ok) {
        toast.success(result.message || "调度预览完成");
      } else {
        toast.error(result.message || "没有可用调度结果");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "预览调度失败");
    } finally {
      setIsPreviewing(false);
    }
  };

  const saveGroup = async () => {
    if (dialog.type !== "group") {
      return;
    }
    setIsSaving(true);
    try {
      if (dialog.id) {
        await updateBusinessProviderGroup(dialog.id, groupDraft);
        toast.success("Provider 池已更新");
      } else {
        await createBusinessProviderGroup(groupDraft);
        toast.success("Provider 池已创建");
      }
      closeDialog();
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存 Provider 池失败");
    } finally {
      setIsSaving(false);
    }
  };

  const saveMember = async () => {
    if (dialog.type !== "member") {
      return;
    }
    setIsSaving(true);
    try {
      if (dialog.id) {
        await updateBusinessProviderMember(dialog.id, memberDraft);
        toast.success("Provider 成员已更新");
      } else {
        await createBusinessProviderMember(memberDraft);
        toast.success("Provider 成员已添加");
      }
      closeDialog();
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存 Provider 成员失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleToggleGroup = async (pool: BusinessProviderPool) => {
    setBusyId(pool.id);
    try {
      await updateBusinessProviderGroup(pool.id, {
        ...groupToDraft(pool),
        enabled: !pool.enabled,
      });
      toast.success(pool.enabled ? "Provider 池已禁用" : "Provider 池已启用");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新 Provider 池失败");
    } finally {
      setBusyId("");
    }
  };

  const handleTestGroup = async (pool: BusinessProviderPool) => {
    setBusyId(pool.id);
    setTestingId(pool.id);
    setTestResults((current) => ({
      ...current,
      [pool.id]: {
        status: "running",
        message: "正在按调度策略选择成员并请求上游图片接口",
        testedAt: new Date().toISOString(),
      },
    }));
    try {
      const result = await testBusinessProviderGroup(pool.id);
      const nextResult = testResultFromResponse(result);
      setTestResults((current) => ({ ...current, [pool.id]: nextResult }));
      if (result.ok) {
        toast.success(`测试成功：${result.member?.name || "已选成员"} 返回 ${result.imageCount} 张图片`);
        return;
      }
      toast.error(result.message || "测试池失败");
    } catch (error) {
      const apiError = error as { code?: string; status?: number; message?: string };
      const message =
        apiError.code === "provider_empty_response"
          ? "上游接口返回成功，但没有返回可用图片数据"
          : apiError.message || "测试 Provider 池失败";
      setTestResults((current) => ({
        ...current,
        [pool.id]: {
          status: "failed",
          imageCount: 0,
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

  const handleSetDefaultGroup = async (pool: BusinessProviderPool) => {
    setBusyId(pool.id);
    try {
      await setDefaultBusinessProviderGroup(pool.id);
      toast.success("默认 Provider 池已切换");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "切换默认 Provider 池失败");
    } finally {
      setBusyId("");
    }
  };

  const handleDeleteGroup = async (pool: BusinessProviderPool) => {
    if (!window.confirm(`确认删除 Provider 池「${pool.name}」？池内成员也会一起删除。`)) {
      return;
    }
    setBusyId(pool.id);
    try {
      await deleteBusinessProviderGroup(pool.id);
      toast.success("Provider 池已删除");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除 Provider 池失败");
    } finally {
      setBusyId("");
    }
  };

  const handleToggleMember = async (member: BusinessProviderMember) => {
    setBusyId(member.id);
    try {
      await updateBusinessProviderMember(member.id, {
        ...memberToDraft(member),
        enabled: !member.enabled,
      });
      toast.success(member.enabled ? "Provider 成员已禁用" : "Provider 成员已启用");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新 Provider 成员失败");
    } finally {
      setBusyId("");
    }
  };

  const handleDeleteMember = async (member: BusinessProviderMember) => {
    if (!window.confirm(`确认删除 Provider 成员「${member.name}」？`)) {
      return;
    }
    setBusyId(member.id);
    try {
      await deleteBusinessProviderMember(member.id);
      toast.success("Provider 成员已删除");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除 Provider 成员失败");
    } finally {
      setBusyId("");
    }
  };

  const handleTestMember = async (member: BusinessProviderMember) => {
    setBusyId(member.id);
    setTestingId(member.id);
    setTestResults((current) => ({
      ...current,
      [member.id]: {
        status: "running",
        message: "正在请求上游图片接口",
        testedAt: new Date().toISOString(),
      },
    }));
    try {
      const result = await testBusinessProviderMember(member.id);
      const nextResult = testResultFromResponse(result);
      setTestResults((current) => ({ ...current, [member.id]: nextResult }));
      if (result.ok) {
        toast.success(`测试成功：返回 ${result.imageCount} 张图片，用时 ${result.durationMs}ms`);
        return;
      }
      toast.error(result.message || "测试失败");
    } catch (error) {
      const apiError = error as { code?: string; status?: number; message?: string };
      const message =
        apiError.code === "provider_empty_response"
          ? "上游接口返回成功，但没有返回可用图片数据"
          : apiError.message || "测试 Provider 成员失败";
      setTestResults((current) => ({
        ...current,
        [member.id]: {
          status: "failed",
          imageCount: 0,
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

  const handleRecoverMember = async (member: BusinessProviderMember) => {
    setBusyId(member.id);
    try {
      await recoverBusinessProviderMember(member.id);
      toast.success("Provider 成员已恢复");
      await loadPools();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "恢复 Provider 成员失败");
    } finally {
      setBusyId("");
    }
  };

  return (
    <ConfigSection
      title="Provider 号池"
      description="按平台维护多个上游成员。生图请求会优先选择对应平台的默认启用池，并按成员优先级和冷却状态调度。"
    >
      <div className="md:col-span-2">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <div className="relative w-[280px] shrink-0">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--app-text-muted)]" />
            <input
              className="app-input"
              style={{ paddingLeft: 36 }}
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
              placeholder="搜索池、成员、Base URL、模型..."
            />
          </div>
          <div className="w-[140px] shrink-0">
            <AppSelect
              value={platformFilter}
              onChange={(value) => setPlatformFilter(value as typeof allFilterValue | APIAccessPlatform)}
              options={[
                { value: allFilterValue, label: "全部平台" },
                ...providerPlatformOptions.map((item) => ({ value: item.value, label: item.label })),
              ]}
            />
          </div>
          <div className="w-[140px] shrink-0">
            <AppSelect
              value={enabledFilter}
              onChange={(value) => setEnabledFilter(value as EnabledFilter)}
              options={[
                { value: "all", label: "全部状态" },
                { value: "enabled", label: "已启用" },
                { value: "disabled", label: "已禁用" },
              ]}
            />
          </div>
          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              className="app-btn"
              onClick={() => void loadPools()}
              disabled={isLoading || isSaving}
            >
              {isLoading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button
              type="button"
              className="app-btn"
              onClick={() => openCreateMember()}
              disabled={isLoading || isSaving || pools.length === 0}
            >
              <Plus className="size-4" />
              添加成员
            </button>
            <button
              type="button"
              className="app-btn-primary"
              onClick={openCreateGroup}
              disabled={isLoading || isSaving}
            >
              <Plus className="size-4" />
              创建池
            </button>
          </div>
        </div>

        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="app-badge off">池 {pools.length}</span>
          <span className="app-badge ok">成员 {enabledMemberCount(pools)}/{poolMemberCount(pools)}</span>
          <span className="app-badge off">当前 {filteredPools.length}</span>
        </div>

        <div className="mb-4 rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-4">
          <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-semibold text-[var(--app-text-primary)]">预览调度</div>
              <div className="mt-1 text-xs text-[var(--app-text-muted)]">按真实标签选择逻辑预览命中的池和成员，不会占用并发或请求上游。</div>
            </div>
            <button className="app-btn-primary" type="button" onClick={() => void handlePreviewDispatch()} disabled={isPreviewing}>
              {isPreviewing ? <LoaderCircle className="size-4 animate-spin" /> : <Send className="size-4" />}
              预览
            </button>
          </div>
          <div className="grid gap-3 md:grid-cols-4 xl:grid-cols-8">
            <label className="app-fld">
              <span className="fl">平台</span>
              <AppSelect
                value={previewDraft.platform}
                onChange={(value) => updatePreviewDraft("platform", value as APIAccessPlatform)}
                options={providerPlatformOptions}
              />
            </label>
            <label className="app-fld">
              <span className="fl">订阅等级</span>
              <AppSelect
                value={previewDraft.subscriptionTag}
                onChange={(value) => updatePreviewDraft("subscriptionTag", value)}
                options={previewSubscriptionOptions}
              />
            </label>
            <label className="app-fld">
              <span className="fl">充值等级</span>
              <AppSelect
                value={previewDraft.walletTag}
                onChange={(value) => updatePreviewDraft("walletTag", value)}
                options={previewWalletOptions}
              />
            </label>
            <label className="app-fld">
              <span className="fl">角色</span>
              <AppSelect
                value={previewDraft.role}
                onChange={(value) => updatePreviewDraft("role", value)}
                options={[
                  { value: "user", label: "用户" },
                  { value: "admin", label: "管理员" },
                ]}
              />
            </label>
            <label className="app-fld">
              <span className="fl">模式</span>
              <AppSelect
                value={previewDraft.mode}
                onChange={(value) => updatePreviewDraft("mode", value)}
                options={[
                  { value: "generate", label: "生成" },
                  { value: "edit", label: "编辑" },
                ]}
              />
            </label>
            <label className="app-fld">
              <span className="fl">清晰度</span>
              <AppSelect
                value={previewDraft.quality}
                onChange={(value) => updatePreviewDraft("quality", value)}
                options={[
                  { value: "low", label: "Low" },
                  { value: "medium", label: "Medium" },
                  { value: "high", label: "High" },
                ]}
              />
            </label>
            <label className="app-fld">
              <span className="fl">尺寸</span>
              <input className="app-input" value={previewDraft.size} onChange={(event) => updatePreviewDraft("size", event.target.value)} />
            </label>
            <label className="app-fld">
              <span className="fl">模型</span>
              <input className="app-input" value={previewDraft.model} onChange={(event) => updatePreviewDraft("model", event.target.value)} />
            </label>
          </div>
          <label className="app-fld mt-3">
            <span className="fl">额外请求标签</span>
            <input
              className="app-input"
              value={previewDraft.extraTagsText}
              onChange={(event) => updatePreviewDraft("extraTagsText", event.target.value)}
              placeholder="例如 scene:portrait source:web"
            />
          </label>
          {previewResult ? (
            <div
              className={cn(
                "mt-3 rounded-[var(--app-radius-md)] border px-3 py-2 text-xs leading-5",
                previewResult.ok ? "border-emerald-500/25 bg-emerald-950/20 text-emerald-100" : "border-rose-500/25 bg-rose-950/20 text-rose-100",
              )}
            >
              <div className="mb-1 flex flex-wrap items-center gap-2">
                <span className={cn("app-badge", previewResult.ok ? "ok" : "fail")}>{previewResult.ok ? "可调度" : "不可调度"}</span>
                <span className="font-medium">{previewResult.message}</span>
                <span className="app-badge off">{previewStrategyLabel(previewResult.strategy)}</span>
              </div>
              <div className="grid gap-x-4 gap-y-1 md:grid-cols-2">
                <span>池：{previewResult.group?.name || "-"}</span>
                <span>成员：{previewResult.member?.name || "-"}</span>
                <span>API 兜底：{previewResult.fallbackAvailable ? previewResult.fallbackName || previewResult.fallbackSource || "可用" : "未使用"}</span>
                <span title={(previewResult.userTags || []).join(", ")}>用户标签：{compactTagText(previewResult.userTags, 4)}</span>
                <span className="md:col-span-2" title={(previewResult.dispatchTags || []).join(", ")}>最终标签：{compactTagText(previewResult.dispatchTags, 8)}</span>
              </div>
              {(previewResult.trace || []).length > 0 ? (
                <div className="mt-3 rounded-[var(--app-radius-md)] border border-white/10 bg-black/10 px-3 py-2">
                  <div className="mb-1 font-medium text-[var(--app-text-primary)]">调度链路</div>
                  <div className="flex flex-wrap gap-1.5">
                    {(previewResult.trace || []).map((item, index) => (
                      <span key={`${item}-${index}`} className="app-badge off">{index + 1}. {item}</span>
                    ))}
                  </div>
                </div>
              ) : null}
              {(previewResult.issues || []).length > 0 ? (
                <div className="mt-3 grid gap-2 md:grid-cols-3">
                  {(previewResult.issues || []).map((issue) => (
                    <div key={issue.code} className="rounded-[var(--app-radius-md)] border border-white/10 bg-black/10 px-3 py-2">
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-medium text-[var(--app-text-primary)]">{issue.label}</span>
                        <span className="app-badge off">{issue.count}</span>
                      </div>
                      {issue.detail ? <div className="mt-1 text-[11px] text-[var(--app-text-muted)]">{issue.detail}</div> : null}
                    </div>
                  ))}
                </div>
              ) : null}
              {(previewResult.pools || []).length > 0 ? (
                <div className="mt-3 rounded-[var(--app-radius-md)] border border-white/10 bg-black/10">
                  <div className="border-b border-white/10 px-3 py-2 font-medium text-[var(--app-text-primary)]">候选池明细</div>
                  <div className="divide-y divide-white/10">
                    {(previewResult.pools || []).map((pool) => (
                      <div key={pool.id} className="grid gap-2 px-3 py-2 md:grid-cols-[minmax(0,1.2fr)_minmax(0,2fr)]">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="truncate font-medium text-[var(--app-text-primary)]">{pool.name || pool.id}</span>
                            <span className={cn("app-badge", previewPoolBadgeClass(pool))}>{previewPoolRoleLabel(pool.role)}</span>
                            {pool.isDefault ? <span className="app-badge warn">默认</span> : null}
                          </div>
                          <div className="mt-1 text-[11px] text-[var(--app-text-muted)]">
                            {pool.reason} · 成员 {pool.availableMembers}/{pool.memberTotal} 可用
                          </div>
                          <div className="mt-1 text-[11px] text-[var(--app-text-muted)]" title={(pool.tags || []).join(", ")}>
                            标签 {compactTagText(pool.tags, 4)}
                          </div>
                        </div>
                        <div className="grid gap-1 text-[11px] text-[var(--app-text-muted)] sm:grid-cols-3">
                          <span>禁用 {pool.disabledMembers}</span>
                          <span>冷却 {pool.coolingMembers}</span>
                          <span>并发满 {pool.concurrencyFullMembers}</span>
                          <span>unavailable {pool.unavailableMembers}</span>
                          <span>limited {pool.limitedMembers}</span>
                          <span>平台不符 {pool.platformMismatchMembers}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>

        <div className="space-y-4">
          {isLoading ? (
            <div className="flex items-center justify-center gap-2 rounded-[var(--app-radius-lg)] border border-[var(--app-border)] px-4 py-10 text-sm text-[var(--app-text-muted)]">
              <LoaderCircle className="size-4 animate-spin" />
              正在读取 Provider 号池
            </div>
          ) : filteredPools.length === 0 ? (
            <div className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)] px-4 py-10 text-center text-sm text-[var(--app-text-muted)]">
              没有匹配的 Provider 池
            </div>
          ) : (
            filteredPools.map((pool) => (
              <div key={pool.id} className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)]">
                <div className="flex flex-col gap-3 border-b border-[var(--app-border)] bg-[var(--app-bg-surface)] px-4 py-3 lg:flex-row lg:items-center">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-semibold text-[var(--app-text-primary)]">{pool.name}</span>
                      <span className="app-badge off">{platformLabel(pool.platform)}</span>
                      <span className={cn("app-badge", pool.enabled ? "ok" : "off")}>
                        {pool.enabled ? "启用" : "禁用"}
                      </span>
                      {pool.isDefault ? (
                        <span className="app-badge warn">
                          <Star className="size-3" />
                          默认池
                        </span>
                      ) : null}
                    </div>
                    <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--app-text-muted)]">
                      <span>优先级 {pool.priority}</span>
                      <span>{matchModeOptions.find((item) => item.value === (pool.matchMode || "fallback"))?.label || "兜底池"}</span>
                      {pool.tags ? <span className="min-w-0 truncate">标签 {pool.tags}</span> : null}
                      <span>成员 {pool.members.filter((member) => member.enabled).length}/{pool.members.length}</span>
                      <span>更新 {formatProviderTime(pool.updatedAt)}</span>
                      {pool.description ? <span className="min-w-0 truncate">{pool.description}</span> : null}
                    </div>
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center gap-1">
                    <button type="button" className="app-btn" title={testingId === pool.id ? "测试中" : "测试整个池"} onClick={() => void handleTestGroup(pool)} disabled={busyId === pool.id}>
                      {busyId === pool.id ? <LoaderCircle className="size-3.5 animate-spin" /> : <Send className="size-3.5" />}
                    </button>
                    <button type="button" className="app-btn" onClick={() => openCreateMember(pool)} disabled={busyId === pool.id}>
                      <Plus className="size-3.5" />
                    </button>
                    <button type="button" className="app-btn" onClick={() => openEditGroup(pool)} disabled={busyId === pool.id}>
                      <Pencil className="size-3.5" />
                    </button>
                    <button
                      type="button"
                      className="app-btn"
                      title={pool.isDefault ? "已是默认池" : "设为默认池"}
                      onClick={() => void handleSetDefaultGroup(pool)}
                      disabled={pool.isDefault || busyId === pool.id}
                    >
                      <Star className="size-3.5" />
                    </button>
                    <button type="button" className="app-btn" onClick={() => void handleToggleGroup(pool)} disabled={busyId === pool.id}>
                      <Power className="size-3.5" />
                    </button>
                    <button
                      type="button"
                      className="app-btn"
                      style={{ color: "#fca5a5", borderColor: "rgba(252,165,165,0.32)" }}
                      onClick={() => void handleDeleteGroup(pool)}
                      disabled={busyId === pool.id}
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </div>
                </div>

                {testResults[pool.id] ? (
                  <div className="border-b border-[var(--app-border)] px-4 py-3">
                    <div
                      className={cn(
                        "flex items-start gap-2 rounded-[var(--app-radius-md)] border px-3 py-2 text-xs leading-5",
                        testResults[pool.id].status === "running" && "border-sky-500/30 bg-sky-950/30 text-sky-200",
                        testResults[pool.id].status === "succeeded" && "border-emerald-500/30 bg-emerald-950/30 text-emerald-200",
                        testResults[pool.id].status === "failed" && "border-rose-500/30 bg-rose-950/30 text-rose-200",
                      )}
                    >
                      {testResults[pool.id].status === "running" ? (
                        <LoaderCircle className="mt-0.5 size-3.5 shrink-0 animate-spin" />
                      ) : testResults[pool.id].status === "succeeded" ? (
                        <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
                      ) : (
                        <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                      )}
                      <div className="min-w-0">
                        <div className="break-words font-medium">{providerTestResultText(testResults[pool.id])}</div>
                        <div className="mt-0.5 text-[11px] opacity-70">{formatProviderTime(testResults[pool.id].testedAt)}</div>
                      </div>
                    </div>
                  </div>
                ) : null}

                <div className={settingsTableWrapClass} style={{ border: 0, borderRadius: 0 }}>
                  <div className="grid min-w-[1240px] grid-cols-[minmax(120px,0.9fr)_130px_minmax(120px,1.05fr)_minmax(130px,1.05fr)_150px_170px_280px] border-b border-[var(--app-border)] px-4 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-[var(--app-text-muted)]">
                    <div>成员</div>
                    <div>状态</div>
                    <div>Base URL</div>
                    <div>模型</div>
                    <div>调度</div>
                    <div>健康</div>
                    <div className="text-right">操作</div>
                  </div>
                  {pool.members.length === 0 ? (
                    <div className="min-w-[1240px] px-4 py-8 text-center text-sm text-[var(--app-text-muted)]">
                      当前池还没有成员
                    </div>
                  ) : (
                    pool.members.map((member) => {
                      const testResult = testResults[member.id];
                      return (
                        <div key={member.id} className="border-b border-[var(--app-border)] last:border-b-0 transition-colors hover:bg-[var(--app-bg-surface-hover)]">
                          <div className="grid min-w-[1240px] grid-cols-[minmax(120px,0.9fr)_130px_minmax(120px,1.05fr)_minmax(130px,1.05fr)_150px_170px_280px] items-center px-4 py-3 text-sm">
                            <div className="min-w-0">
                              <div className="truncate font-medium text-[var(--app-text-primary)]">{member.name}</div>
                              <div className="mt-1 text-xs text-[var(--app-text-muted)]">Key {maskSecret(member.apiKey)}</div>
                            </div>
                            <div className="flex flex-wrap items-center gap-1.5">
                              <span className={cn("app-badge", member.enabled ? "ok" : "off")}>
                                {member.enabled ? "启用" : "禁用"}
                              </span>
                              <span className={cn("app-badge", statusBadgeClass(member.status))}>{member.status}</span>
                            </div>
                            <div className="min-w-0 break-all pr-4 text-xs leading-5 text-[var(--app-text-muted)]">
                              {member.baseUrl}
                            </div>
                            <div className="min-w-0 pr-4">
                              <div className="truncate text-xs text-[var(--app-text-secondary)]">{member.defaultModel}</div>
                              <div className="mt-1 text-xs text-[var(--app-text-muted)]">更新 {formatProviderTime(member.updatedAt)}</div>
                            </div>
                            <div className="text-xs leading-5 text-[var(--app-text-secondary)]">
                              <div>优先级 {member.priority}</div>
                              <div>权重 {member.weight || 1}</div>
                              <div>并发 {member.maxConcurrent ? member.maxConcurrent : "不限"}</div>
                            </div>
                            <div className="text-xs leading-5 text-[var(--app-text-muted)]">
                              <div>成功 {member.successCount}</div>
                              <div>失败 {member.failCount}</div>
                              <div>连续 {member.consecutiveFailures || 0}/{member.failureThreshold || 5}</div>
                              {member.cooldownUntil ? <div>冷却至 {formatProviderTime(member.cooldownUntil)}</div> : null}
                            </div>
                            <div className="flex items-center justify-end gap-1">
                              <button
                                type="button"
                                className="app-btn"
                                title={testingId === member.id ? "测试中" : "测试连通性"}
                                onClick={() => void handleTestMember(member)}
                                disabled={busyId === member.id}
                              >
                                {busyId === member.id ? <LoaderCircle className="size-3.5 animate-spin" /> : <Send className="size-3.5" />}
                              </button>
                              <button type="button" className="app-btn" title="编辑" onClick={() => openEditMember(member)} disabled={busyId === member.id}>
                                <Pencil className="size-3.5" />
                              </button>
                              <button type="button" className="app-btn" title={member.enabled ? "禁用" : "启用"} onClick={() => void handleToggleMember(member)} disabled={busyId === member.id}>
                                <Power className="size-3.5" />
                              </button>
                              <button
                                type="button"
                                className="app-btn"
                                title="恢复为 active"
                                onClick={() => void handleRecoverMember(member)}
                                disabled={busyId === member.id || (member.status === "active" && !member.cooldownUntil && (member.consecutiveFailures || 0) === 0)}
                              >
                                <CheckCircle2 className="size-3.5" />
                              </button>
                              <button
                                type="button"
                                className="app-btn"
                                title="删除"
                                style={{ color: "#fca5a5", borderColor: "rgba(252,165,165,0.32)" }}
                                onClick={() => void handleDeleteMember(member)}
                                disabled={busyId === member.id}
                              >
                                <Trash2 className="size-3.5" />
                              </button>
                            </div>
                          </div>
                          {member.lastError || testResult ? (
                            <div className="px-4 pb-3">
                              {testResult ? (
                                <div
                                  className={cn(
                                    "flex items-start gap-2 rounded-[var(--app-radius-md)] border px-3 py-2 text-xs leading-5",
                                    testResult.status === "running" && "border-sky-500/30 bg-sky-950/30 text-sky-200",
                                    testResult.status === "succeeded" && "border-emerald-500/30 bg-emerald-950/30 text-emerald-200",
                                    testResult.status === "failed" && "border-rose-500/30 bg-rose-950/30 text-rose-200",
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
                                    <div className="break-words font-medium">{providerTestResultText(testResult)}</div>
                                    <div className="mt-0.5 text-[11px] opacity-70">{formatProviderTime(testResult.testedAt)}</div>
                                  </div>
                                </div>
                              ) : null}
                              {member.lastError ? (
                                <div className="mt-2 rounded-[var(--app-radius-md)] border border-rose-500/25 bg-rose-950/20 px-3 py-2 text-xs leading-5 text-rose-200">
                                  最近错误：{member.lastError}
                                </div>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      );
                    })
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      <AppModal
        open={dialog.type === "group"}
        onClose={() => { if (!isSaving) closeDialog(); }}
        title={dialog.type === "group" && dialog.id ? "编辑 Provider 池" : "创建 Provider 池"}
        footer={
          <>
            <button type="button" className="app-btn" onClick={closeDialog} disabled={isSaving}>
              取消
            </button>
            <button type="button" className="app-btn-primary" onClick={() => void saveGroup()} disabled={isSaving}>
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              {dialog.type === "group" && dialog.id ? "更新" : "创建"}
            </button>
          </>
        }
      >
        <p style={{ marginTop: -4, marginBottom: 14, fontSize: 12, color: "var(--app-text-muted)" }}>
          Provider 池决定同一平台下优先使用哪组上游成员。
        </p>
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="池名称" hint="用于区分不同上游池。">
            <input
              className="app-input"
              value={groupDraft.name}
              onChange={(event) => updateGroupDraft("name", event.target.value)}
              placeholder="默认 gpt-image 池"
            />
          </Field>
          <Field
            label="平台"
            hint="池内成员必须和池平台一致。已有成员后不建议修改。"
            tooltip={
              <TooltipDetails
                items={[
                  { title: "OpenAI", body: <>OpenAI 兼容图片接口。</> },
                  { title: "Google", body: <>Gemini 图片接口。</> },
                  { title: "其他平台", body: <>OpenAI 兼容图片接口，按模型目录路由。</> },
                ]}
              />
            }
          >
            <AppSelect
              value={groupDraft.platform}
              onChange={(value) => updateGroupDraft("platform", value as APIAccessPlatform)}
              options={providerPlatformOptions.map((item) => ({ value: item.value, label: item.label }))}
            />
          </Field>
          <Field label="优先级" hint="数字越小越优先。默认池会先于非默认池。">
            <input
              className="app-input"
              type="number"
              min={0}
              value={groupDraft.priority}
              onChange={(event) => updateGroupDraft("priority", Number(event.target.value) || 0)}
            />
          </Field>
          <Field label="匹配模式" hint="兜底池在没有标签池命中时参与调度。">
            <AppSelect
              value={groupDraft.matchMode}
              onChange={(value) => updateGroupDraft("matchMode", value as BusinessProviderGroupInput["matchMode"])}
              options={matchModeOptions.map((item) => ({ value: item.value, label: item.label }))}
            />
          </Field>
          <Field label="描述" hint="内部备注，用于说明这个池的用途。">
            <input
              className="app-input"
              value={groupDraft.description}
              onChange={(event) => updateGroupDraft("description", event.target.value)}
              placeholder="低价池、备用池、稳定池..."
            />
          </Field>
          <Field
            label="调度标签"
            hint="用英文逗号或空格分隔，例如 quality:high mode:edit。"
            fullWidth
          >
            <input
              className="app-input"
              value={groupDraft.tags}
              onChange={(event) => updateGroupDraft("tags", event.target.value)}
              placeholder="quality:high, mode:edit, model:gpt-image-2"
            />
          </Field>
          <div className="flex flex-wrap items-center gap-2 md:col-span-2">
            <button
              type="button"
              className={groupDraft.enabled ? "app-btn-primary" : "app-btn"}
              onClick={() => updateGroupDraft("enabled", !groupDraft.enabled)}
            >
              {groupDraft.enabled ? "已启用" : "已禁用"}
            </button>
            <button
              type="button"
              className={groupDraft.isDefault ? "app-btn-primary" : "app-btn"}
              onClick={() => updateGroupDraft("isDefault", !groupDraft.isDefault)}
            >
              <Star className="size-3.5" />
              {groupDraft.isDefault ? "默认池" : "设为默认池"}
            </button>
          </div>
        </div>
      </AppModal>

      <AppModal
        open={dialog.type === "member"}
        onClose={() => { if (!isSaving) closeDialog(); }}
        title={dialog.type === "member" && dialog.id ? "编辑 Provider 成员" : "添加 Provider 成员"}
        footer={
          <>
            <button type="button" className="app-btn" onClick={closeDialog} disabled={isSaving}>
              取消
            </button>
            <button type="button" className="app-btn-primary" onClick={() => void saveMember()} disabled={isSaving || !memberDraft.groupId}>
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : null}
              {dialog.type === "member" && dialog.id ? "更新" : "添加"}
            </button>
          </>
        }
      >
        <p style={{ marginTop: -4, marginBottom: 14, fontSize: 12, color: "var(--app-text-muted)" }}>
          Provider 成员是实际请求上游图片接口的地址和密钥。
        </p>
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="所属池" hint="成员会继承所属池的平台。">
            <AppSelect
              value={memberDraft.groupId}
              onChange={(value) => updateMemberDraft("groupId", value)}
              options={groupOptions(pools)}
            />
          </Field>
          <Field label="成员名称" hint="用于区分不同上游账号或服务。">
            <input
              className="app-input"
              value={memberDraft.name}
              onChange={(event) => updateMemberDraft("name", event.target.value)}
              placeholder="主账号 01"
            />
          </Field>
          <Field label="平台" hint="由所属池决定，成员平台必须一致。">
            <input className="app-input" value={platformLabel(memberPlatform)} disabled readOnly />
          </Field>
          <Field label="优先级" hint="同一池内数字越小越优先。">
            <input
              className="app-input"
              type="number"
              min={0}
              value={memberDraft.priority}
              onChange={(event) => updateMemberDraft("priority", Number(event.target.value) || 0)}
            />
          </Field>
          <Field label="权重" hint="同优先级内的调度权重。数字越大，复用间隔越短。">
            <input
              className="app-input"
              type="number"
              min={1}
              value={memberDraft.weight}
              onChange={(event) => updateMemberDraft("weight", Math.max(1, Number(event.target.value) || 1))}
            />
          </Field>
          <Field label="单成员并发" hint="该 member 同时 running 任务上限。填 0 表示不限制。">
            <input
              className="app-input"
              type="number"
              min={0}
              value={memberDraft.maxConcurrent}
              onChange={(event) => updateMemberDraft("maxConcurrent", Math.max(0, Number(event.target.value) || 0))}
            />
          </Field>
          <Field label="Base URL 地址" hint="填写上游服务地址。" fullWidth>
            <input
              className="app-input"
              value={memberDraft.baseUrl}
              onChange={(event) => updateMemberDraft("baseUrl", event.target.value)}
              placeholder={
                isGeminiPlatform(memberPlatform)
                  ? "https://generativelanguage.googleapis.com"
                  : "http://127.0.0.1:8080"
              }
            />
          </Field>
          <Field
            label={isGeminiPlatform(memberPlatform) ? "请求模型" : "默认模型"}
            hint="读取模型目录中同平台启用模型的上游模型。"
          >
            <AppSelect
              value={memberDraft.defaultModel || configuredDefaultModel(memberPlatform)}
              onChange={(value) => updateMemberDraft("defaultModel", value)}
              options={memberModelOptions.map((model) => ({ value: model, label: model }))}
            />
          </Field>
          <Field label="API Key" hint="后端请求上游时使用。">
            <input
              className="app-input"
              type="password"
              value={memberDraft.apiKey}
              onChange={(event) => updateMemberDraft("apiKey", event.target.value)}
              placeholder={isGeminiPlatform(memberPlatform) ? "AIza..." : "sk-..."}
            />
          </Field>
          <Field label="状态" hint="active 会参与调度；limited 会在冷却结束后参与；unavailable 不参与。">
            <AppSelect
              value={memberDraft.status}
              onChange={(value) => updateMemberDraft("status", value as BusinessProviderMemberStatus)}
              options={statusOptions.map((item) => ({ value: item.value, label: item.label }))}
            />
          </Field>
          <Field label="冷却秒数" hint="普通失败后该成员暂停参与调度的秒数。">
            <input
              className="app-input"
              type="number"
              min={1}
              value={memberDraft.cooldownSeconds}
              onChange={(event) => updateMemberDraft("cooldownSeconds", Math.max(1, Number(event.target.value) || 120))}
            />
          </Field>
          <Field label="失败阈值" hint="连续失败达到阈值后自动标记为 unavailable。">
            <input
              className="app-input"
              type="number"
              min={1}
              value={memberDraft.failureThreshold}
              onChange={(event) => updateMemberDraft("failureThreshold", Math.max(1, Number(event.target.value) || 5))}
            />
          </Field>
          <div className="flex flex-wrap items-center gap-2 md:col-span-2">
            <button
              type="button"
              className={memberDraft.enabled ? "app-btn-primary" : "app-btn"}
              onClick={() => updateMemberDraft("enabled", !memberDraft.enabled)}
            >
              {memberDraft.enabled ? "已启用" : "已禁用"}
            </button>
          </div>
        </div>
      </AppModal>
    </ConfigSection>
  );
}
