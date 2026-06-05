"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  Ban,
  CircleAlert,
  LoaderCircle,
  PlayCircle,
  RefreshCw,
  Save,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminSectionTitle,
  AdminStatCard,
  AdminToolbar,
} from "@/components/admin-layout";
import {
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { AppModal, AppSelect } from "@/components/app-controls";
import {
  deleteBusinessRiskControlPolicy,
  fetchBusinessRiskControlConfig,
  fetchBusinessRiskControlLogs,
  fetchBusinessRiskControlPolicies,
  fetchBusinessRiskControlStatus,
  testBusinessRiskControl,
  updateBusinessRiskControlConfig,
  upsertBusinessRiskControlPolicy,
  type BusinessRiskControlConfig,
  type BusinessRiskControlDecision,
  type BusinessRiskControlFailMode,
  type BusinessRiskControlLog,
  type BusinessRiskControlMode,
  type BusinessRiskControlPolicy,
  type BusinessRiskControlPolicyScope,
  type BusinessRiskControlRiskLevel,
  type BusinessRiskControlStatus,
} from "@/lib/api";
import { providerPlatformLabel, providerPlatformOptions } from "@/lib/provider-platforms";
import { cn } from "@/lib/utils";

const pageSize = 50;

const defaultConfig: BusinessRiskControlConfig = {
  enabled: false,
  mode: "observe",
  provider: "openai",
  providerChain: ["openai"],
  failMode: "fail_closed",
  baseUrl: "https://api.openai.com",
  model: "omni-moderation-latest",
  apiKeyConfigured: false,
  apiKeyMasked: "",
  openaiBaseUrl: "https://api.openai.com",
  openaiModel: "omni-moderation-latest",
  openaiApiKeyConfigured: false,
  openaiApiKeyMasked: "",
  aliyunAccessKeyId: "",
  aliyunAccessKeySecretConfigured: false,
  aliyunAccessKeySecretMasked: "",
  aliyunRegionId: "ap-southeast-1",
  aliyunEndpoint: "green-cip.ap-southeast-1.aliyuncs.com",
  aliyunTextService: "ugc_moderation_byllm_cb",
  aliyunBlockRiskLevel: "low",
  timeoutMs: 3000,
  recordNonHits: false,
  blockMessage: "内容审计命中风险规则，请调整输入后重试",
  thresholds: {},
};

const resultOptions = [
  { value: "", label: "全部结果" },
  { value: "flagged", label: "命中" },
  { value: "blocked", label: "已拦截" },
  { value: "allowed", label: "放行" },
  { value: "error", label: "审核错误" },
];

const modeOptions = [
  { value: "observe", label: "观察" },
  { value: "pre_block", label: "请求前拦截" },
];

const providerChainOptions = [
  { value: "openai", label: "仅 OpenAI" },
  { value: "aliyun", label: "仅阿里云" },
  { value: "aliyun_openai", label: "阿里云优先，OpenAI fallback" },
  { value: "openai_aliyun", label: "OpenAI 优先，阿里云 fallback" },
];

const failModeOptions = [
  { value: "fail_closed", label: "全部失败时报错拦截" },
  { value: "fail_open", label: "全部失败时放行" },
];

const policyScopeOptions = [
  { value: "global", label: "全局" },
  { value: "plan", label: "套餐" },
  { value: "user", label: "用户" },
  { value: "api_key", label: "API Key" },
];

const policyModeOptions = [
  { value: "", label: "继承" },
  ...modeOptions,
];

const riskLevelOptions = [
  { value: "", label: "继承" },
  { value: "low", label: "低" },
  { value: "medium", label: "中" },
  { value: "high", label: "高" },
];

type PolicyForm = {
  id: string;
  scope: BusinessRiskControlPolicyScope;
  targetId: string;
  enabled: boolean;
  mode: "" | BusinessRiskControlMode;
  riskLevel: BusinessRiskControlRiskLevel;
  blockMessage: string;
  thresholdsText: string;
};

const emptyPolicyForm: PolicyForm = {
  id: "",
  scope: "plan",
  targetId: "",
  enabled: true,
  mode: "",
  riskLevel: "",
  blockMessage: "",
  thresholdsText: "{}",
};

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function percentScore(value: number | undefined) {
  if (!Number.isFinite(Number(value))) return "-";
  return `${(Number(value || 0) * 100).toFixed(1)}%`;
}

function formatDateTime(value: string | undefined) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) return value || "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatLatency(value: number | undefined) {
  const ms = Math.max(0, Number(value || 0));
  if (ms <= 0) return "-";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(2)} 秒`;
}

function modeLabel(value: string) {
  if (!value) return "继承";
  return value === "pre_block" ? "请求前拦截" : "观察";
}

function providerLabel(value: string) {
  if (value === "aliyun") return "阿里云";
  if (value === "openai") return "OpenAI";
  return value || "-";
}

function providerChainValue(chain: string[] | undefined) {
  const normalized = Array.from(new Set((chain || []).filter((item) => item === "openai" || item === "aliyun")));
  if (normalized.length === 1) return normalized[0];
  if (normalized[0] === "aliyun") return "aliyun_openai";
  if (normalized[0] === "openai") return "openai_aliyun";
  return "openai";
}

function providerChainFromValue(value: string): Array<"openai" | "aliyun"> {
  if (value === "aliyun_openai") return ["aliyun", "openai"];
  if (value === "openai_aliyun") return ["openai", "aliyun"];
  if (value === "aliyun") return ["aliyun"];
  return ["openai"];
}

function providerChainLabel(chain: string[] | undefined) {
  const value = providerChainValue(chain);
  return providerChainOptions.find((item) => item.value === value)?.label || "-";
}

function failModeLabel(value: string | undefined) {
  return failModeOptions.find((item) => item.value === value)?.label || "全部失败时报错拦截";
}

function scopeLabel(value: string) {
  return policyScopeOptions.find((item) => item.value === value)?.label || value || "-";
}

function riskLevelLabel(value: string | undefined) {
  return riskLevelOptions.find((item) => item.value === (value || ""))?.label || value || "继承";
}

function resultLabel(item: BusinessRiskControlLog) {
  if (item.action === "block") return "拦截";
  if (item.action === "error") return "错误";
  if (item.flagged) return "命中";
  if (item.action === "allow") return "放行";
  return item.action || "-";
}

function resultBadgeClass(item: Pick<BusinessRiskControlLog, "action" | "flagged">) {
  if (item.action === "block") return "fail";
  if (item.action === "error") return "warn";
  if (item.flagged) return "warn";
  return "ok";
}

function sortedScores(scores: Record<string, number>) {
  return Object.entries(scores || {})
    .sort((a, b) => Number(b[1] || 0) - Number(a[1] || 0))
    .slice(0, 6);
}

function normalizeThresholdText(value: Record<string, number>) {
  return JSON.stringify(value || {}, null, 2);
}

function parseThresholds(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return {};
  const parsed = JSON.parse(trimmed) as Record<string, unknown>;
  const next: Record<string, number> = {};
  Object.entries(parsed).forEach(([key, raw]) => {
    const score = Number(raw);
    if (key.trim() && Number.isFinite(score)) {
      next[key.trim()] = Math.min(1, Math.max(0, score));
    }
  });
  return next;
}

function policyThresholdCount(item: BusinessRiskControlPolicy) {
  return Object.keys(item.thresholds || {}).length;
}

function policyToForm(item: BusinessRiskControlPolicy): PolicyForm {
  return {
    id: item.id,
    scope: item.scope,
    targetId: item.targetId || "",
    enabled: item.enabled,
    mode: (item.mode || "") as "" | BusinessRiskControlMode,
    riskLevel: (item.riskLevel || "") as BusinessRiskControlRiskLevel,
    blockMessage: item.blockMessage || "",
    thresholdsText: normalizeThresholdText(item.thresholds || {}),
  };
}

function DecisionPreview({ decision }: { decision: BusinessRiskControlDecision | null }) {
  if (!decision) return null;
  return (
    <div className="app-subpanel p-4">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className={cn("app-badge", decision.flagged ? "warn" : "ok")}>
          {decision.flagged ? "命中风险" : "未命中"}
        </span>
        <span className="text-[var(--app-text-muted)]">
          {decision.highestCategory || "-"} · {percentScore(decision.highestScore)}
        </span>
      </div>
      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        {sortedScores(decision.categoryScores).map(([category, score]) => (
          <div key={category} className="rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-3 py-2 text-xs">
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-[var(--app-text-muted)]">{category}</span>
              <span className="font-semibold text-[var(--app-text-primary)]">{percentScore(score)}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function RiskControlPage() {
  const [config, setConfig] = useState<BusinessRiskControlConfig>(defaultConfig);
  const [savedConfig, setSavedConfig] = useState<BusinessRiskControlConfig>(defaultConfig);
  const [status, setStatus] = useState<BusinessRiskControlStatus | null>(null);
  const [logs, setLogs] = useState<BusinessRiskControlLog[]>([]);
  const [policies, setPolicies] = useState<BusinessRiskControlPolicy[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [result, setResult] = useState("");
  const [platform, setPlatform] = useState("");
  const [search, setSearch] = useState("");
  const [newOpenAIAPIKey, setNewOpenAIAPIKey] = useState("");
  const [clearOpenAIAPIKey, setClearOpenAIAPIKey] = useState(false);
  const [newAliyunAccessKeySecret, setNewAliyunAccessKeySecret] = useState("");
  const [clearAliyunAccessKeySecret, setClearAliyunAccessKeySecret] = useState(false);
  const [thresholdText, setThresholdText] = useState("{}");
  const [configOpen, setConfigOpen] = useState(false);
  const [policyOpen, setPolicyOpen] = useState(false);
  const [testOpen, setTestOpen] = useState(false);
  const [policyForm, setPolicyForm] = useState<PolicyForm>(emptyPolicyForm);
  const [deletePolicyTarget, setDeletePolicyTarget] = useState<BusinessRiskControlPolicy | null>(null);
  const [testPrompt, setTestPrompt] = useState("");
  const [testDecision, setTestDecision] = useState<BusinessRiskControlDecision | null>(null);
  const [testLatency, setTestLatency] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [logLoading, setLogLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [policySaving, setPolicySaving] = useState(false);

  const savedThresholdText = useMemo(() => normalizeThresholdText(savedConfig.thresholds), [savedConfig.thresholds]);
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const openAIKeyReady = clearOpenAIAPIKey ? false : Boolean(newOpenAIAPIKey.trim() || config.openaiApiKeyConfigured || config.apiKeyConfigured);
  const aliyunKeyReady = clearAliyunAccessKeySecret ? false : Boolean(config.aliyunAccessKeyId.trim() && (newAliyunAccessKeySecret.trim() || config.aliyunAccessKeySecretConfigured));
  const testProviderLabel = `${providerChainLabel(config.providerChain)} · ${failModeLabel(config.failMode)}`;
  const dirty = useMemo(
    () =>
      JSON.stringify(config) !== JSON.stringify(savedConfig) ||
      thresholdText !== savedThresholdText ||
      newOpenAIAPIKey.trim() !== "" ||
      clearOpenAIAPIKey ||
      newAliyunAccessKeySecret.trim() !== "" ||
      clearAliyunAccessKeySecret,
    [clearAliyunAccessKeySecret, clearOpenAIAPIKey, config, newAliyunAccessKeySecret, newOpenAIAPIKey, savedConfig, savedThresholdText, thresholdText],
  );

  const loadConfigAndStatus = useCallback(async () => {
    const [configPayload, statusPayload, policyPayload] = await Promise.all([
      fetchBusinessRiskControlConfig(),
      fetchBusinessRiskControlStatus(),
      fetchBusinessRiskControlPolicies(),
    ]);
    setConfig(configPayload.config);
    setSavedConfig(configPayload.config);
    setThresholdText(normalizeThresholdText(configPayload.config.thresholds));
    setStatus(statusPayload.status);
    setPolicies(policyPayload.items || []);
    setNewOpenAIAPIKey("");
    setClearOpenAIAPIKey(false);
    setNewAliyunAccessKeySecret("");
    setClearAliyunAccessKeySecret(false);
  }, []);

  const loadLogs = useCallback(async () => {
    setLogLoading(true);
    try {
      const payload = await fetchBusinessRiskControlLogs({
        page,
        pageSize,
        result,
        platform,
        search: search.trim() || undefined,
      });
      setLogs(payload.items);
      setTotal(payload.total);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载风控日志失败");
    } finally {
      setLogLoading(false);
    }
  }, [page, platform, result, search]);

  const refreshAll = useCallback(async () => {
    setRefreshing(true);
    try {
      await Promise.all([loadConfigAndStatus(), loadLogs()]);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "刷新风控中心失败");
    } finally {
      setRefreshing(false);
    }
  }, [loadConfigAndStatus, loadLogs]);

  useEffect(() => {
    void loadConfigAndStatus().catch((error) => {
      toast.error(error instanceof Error ? error.message : "加载风控配置失败");
    });
  }, [loadConfigAndStatus]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadLogs();
    }, 180);
    return () => window.clearTimeout(timer);
  }, [loadLogs]);

  useEffect(() => {
    setTestDecision(null);
    setTestLatency(null);
  }, [clearAliyunAccessKeySecret, clearOpenAIAPIKey, config, newAliyunAccessKeySecret, newOpenAIAPIKey, testPrompt, thresholdText]);

  const openConfigDialog = () => {
    setConfig(savedConfig);
    setThresholdText(savedThresholdText);
    setNewOpenAIAPIKey("");
    setClearOpenAIAPIKey(false);
    setNewAliyunAccessKeySecret("");
    setClearAliyunAccessKeySecret(false);
    setTestDecision(null);
    setTestLatency(null);
    setConfigOpen(true);
  };

  const closeConfigDialog = () => {
    setConfigOpen(false);
    setConfig(savedConfig);
    setThresholdText(savedThresholdText);
    setNewOpenAIAPIKey("");
    setClearOpenAIAPIKey(false);
    setNewAliyunAccessKeySecret("");
    setClearAliyunAccessKeySecret(false);
  };

  const openTestDialog = () => {
    setTestOpen(true);
    setTestDecision(null);
    setTestLatency(null);
  };

  const openPolicyDialog = (item?: BusinessRiskControlPolicy) => {
    setPolicyForm(item ? policyToForm(item) : emptyPolicyForm);
    setPolicyOpen(true);
  };

  const saveConfig = async () => {
    let thresholds: Record<string, number>;
    try {
      thresholds = parseThresholds(thresholdText);
    } catch {
      toast.error("阈值 JSON 格式不正确");
      return;
    }
    setSaving(true);
    try {
      const payload = await updateBusinessRiskControlConfig({
        enabled: config.enabled,
        mode: config.mode,
        provider: config.provider,
        providerChain: config.providerChain,
        failMode: config.failMode,
        baseUrl: config.openaiBaseUrl || config.baseUrl,
        apiKey: newOpenAIAPIKey.trim() || undefined,
        clearApiKey: clearOpenAIAPIKey,
        model: config.openaiModel || config.model,
        openaiBaseUrl: config.openaiBaseUrl,
        openaiApiKey: newOpenAIAPIKey.trim() || undefined,
        clearOpenaiApiKey: clearOpenAIAPIKey,
        openaiModel: config.openaiModel,
        aliyunAccessKeyId: config.aliyunAccessKeyId,
        aliyunAccessKeySecret: newAliyunAccessKeySecret.trim() || undefined,
        clearAliyunAccessKeySecret,
        aliyunRegionId: config.aliyunRegionId,
        aliyunEndpoint: config.aliyunEndpoint,
        aliyunTextService: config.aliyunTextService,
        aliyunBlockRiskLevel: config.aliyunBlockRiskLevel,
        timeoutMs: Number(config.timeoutMs || 3000),
        recordNonHits: config.recordNonHits,
        blockMessage: config.blockMessage,
        thresholds,
      });
      setConfig(payload.config);
      setSavedConfig(payload.config);
      setThresholdText(normalizeThresholdText(payload.config.thresholds));
      setNewOpenAIAPIKey("");
      setClearOpenAIAPIKey(false);
      setNewAliyunAccessKeySecret("");
      setClearAliyunAccessKeySecret(false);
      setConfigOpen(false);
      const statusPayload = await fetchBusinessRiskControlStatus();
      setStatus(statusPayload.status);
      toast.success("风控配置已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存风控配置失败");
    } finally {
      setSaving(false);
    }
  };

  const savePolicy = async () => {
    if (policyForm.scope !== "global" && !policyForm.targetId.trim()) {
      toast.error("请输入策略目标");
      return;
    }
    let thresholds: Record<string, number>;
    try {
      thresholds = parseThresholds(policyForm.thresholdsText);
    } catch {
      toast.error("阈值 JSON 格式不正确");
      return;
    }
    setPolicySaving(true);
    try {
      const response = await upsertBusinessRiskControlPolicy({
        scope: policyForm.scope,
        targetId: policyForm.targetId.trim(),
        enabled: policyForm.enabled,
        mode: policyForm.mode,
        riskLevel: policyForm.riskLevel,
        blockMessage: policyForm.blockMessage,
        thresholds,
      });
      setPolicies((current) => {
        const exists = current.some((item) => item.id === response.item.id);
        if (exists) return current.map((item) => (item.id === response.item.id ? response.item : item));
        return [...current, response.item];
      });
      setPolicyOpen(false);
      toast.success("策略已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存策略失败");
    } finally {
      setPolicySaving(false);
    }
  };

  const deletePolicy = async () => {
    if (!deletePolicyTarget) return;
    setPolicySaving(true);
    try {
      await deleteBusinessRiskControlPolicy(deletePolicyTarget.id);
      setPolicies((current) => current.filter((item) => item.id !== deletePolicyTarget.id));
      setDeletePolicyTarget(null);
      toast.success("策略已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除策略失败");
    } finally {
      setPolicySaving(false);
    }
  };

  const runTest = async () => {
    if (!testPrompt.trim()) {
      toast.error("请输入测试内容");
      return;
    }
    setTesting(true);
    setTestDecision(null);
    setTestLatency(null);
    try {
      const thresholds = parseThresholds(thresholdText);
      const payload = await testBusinessRiskControl({
        prompt: testPrompt,
        mode: config.mode,
        providerChain: config.providerChain,
        failMode: config.failMode,
        baseUrl: config.openaiBaseUrl || config.baseUrl,
        apiKey: newOpenAIAPIKey.trim() || undefined,
        clearApiKey: clearOpenAIAPIKey,
        model: config.openaiModel || config.model,
        openaiBaseUrl: config.openaiBaseUrl,
        openaiApiKey: newOpenAIAPIKey.trim() || undefined,
        clearOpenaiApiKey: clearOpenAIAPIKey,
        openaiModel: config.openaiModel,
        aliyunAccessKeyId: config.aliyunAccessKeyId,
        aliyunAccessKeySecret: newAliyunAccessKeySecret.trim() || undefined,
        clearAliyunAccessKeySecret,
        aliyunRegionId: config.aliyunRegionId,
        aliyunEndpoint: config.aliyunEndpoint,
        aliyunTextService: config.aliyunTextService,
        aliyunBlockRiskLevel: config.aliyunBlockRiskLevel,
        timeoutMs: Number(config.timeoutMs || 3000),
        blockMessage: config.blockMessage,
        thresholds,
      });
      setTestDecision(payload.result.decision);
      setTestLatency(payload.result.latencyMs);
    } catch (error) {
      if (error instanceof SyntaxError) {
        toast.error("阈值 JSON 格式不正确");
        return;
      }
      toast.error(error instanceof Error ? error.message : "测试审核失败");
    } finally {
      setTesting(false);
    }
  };

  return (
    <AdminPage>
      <AdminHeader
        title="风控中心"
        description="配置生图请求的内容审核策略，查看命中、拦截和审核错误日志。"
        icon={ShieldCheck}
        iconVariant="sky"
        actions={
          <>
            <button type="button" className="app-btn" onClick={() => void refreshAll()} disabled={refreshing}>
              {refreshing ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新
            </button>
            <button type="button" className="app-btn" onClick={openTestDialog}>
              <PlayCircle className="size-4" />
              测试审核
            </button>
            <button type="button" className="app-btn-primary" onClick={openConfigDialog}>
              <Settings2 className="size-4" />
              审核配置
            </button>
          </>
        }
      />

      <section className="app-stats">
        <AdminStatCard
          label="24h 审核"
          value={numberText(status?.last24hTotal)}
          sub={status?.enabled ? modeLabel(status.mode) : "未启用"}
          icon={ShieldCheck}
          color="text-cyan-300"
        />
        <AdminStatCard
          label="24h 命中"
          value={numberText(status?.last24hFlagged)}
          sub={`${openAIKeyReady ? "OpenAI 已配置" : "OpenAI 未配置"} / ${aliyunKeyReady ? "阿里云已配置" : "阿里云未配置"}`}
          icon={AlertTriangle}
          color="text-amber-300"
        />
        <AdminStatCard
          label="24h 拦截"
          value={numberText(status?.last24hBlocked)}
          sub={modeLabel(status?.mode || savedConfig.mode)}
          icon={Ban}
          color="text-rose-300"
        />
        <AdminStatCard
          label="24h 错误"
          value={numberText(status?.last24hErrors)}
          sub={providerChainLabel(status?.providerChain || savedConfig.providerChain)}
          icon={CircleAlert}
          color="text-violet-300"
        />
      </section>

      <AdminToolbar>
        <AppSelect value={result} onChange={(value) => { setPage(1); setResult(value); }} options={resultOptions} />
        <AppSelect
          value={platform}
          onChange={(value) => { setPage(1); setPlatform(value); }}
          options={[{ value: "", label: "全部平台" }, ...providerPlatformOptions]}
        />
        <input
          className="app-input"
          type="search"
          value={search}
          onChange={(event) => { setPage(1); setSearch(event.target.value); }}
          placeholder="搜索用户、模型或输入摘要..."
        />
        <span className="spacer" />
        <span className={cn("app-badge", status?.enabled ? "ok" : "off")}>{status?.enabled ? "已启用" : "未启用"}</span>
        <span className={cn("app-badge", status?.mode === "pre_block" ? "warn" : "run")}>
          {modeLabel(status?.mode || savedConfig.mode)}
        </span>
        <span className={cn("app-badge", status?.apiKeyConfigured ? "ok" : "off")}>
          {status?.apiKeyConfigured ? "链路密钥已配置" : "链路密钥未配置"}
        </span>
        <span className="app-badge run">{providerChainLabel(status?.providerChain || savedConfig.providerChain)}</span>
      </AdminToolbar>

      <AdminPanel>
        <AdminSectionTitle
          title="策略覆盖"
          action={
            <button className="app-btn" type="button" onClick={() => openPolicyDialog()}>
              <SlidersHorizontal className="size-4" />
              新增策略
            </button>
          }
        />
        <div className="app-table-wrap">
          <table className={adminTableClass}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th>范围</th>
                <th>目标</th>
                <th>状态</th>
                <th>模式</th>
                <th>风险等级</th>
                <th>阈值</th>
                <th>更新时间</th>
                <th style={{ textAlign: "right" }}>操作</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {policies.length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-10 text-center text-sm text-[var(--app-text-muted)]">
                    暂无覆盖策略，当前使用全局审核配置。
                  </td>
                </tr>
              ) : (
                policies.map((item) => (
                  <tr key={item.id} className={adminTableRowClass}>
                    <td>{scopeLabel(item.scope)}</td>
                    <td className="max-w-[220px] truncate">{item.scope === "global" ? "-" : item.targetId || "-"}</td>
                    <td><span className={cn("app-badge", item.enabled ? "ok" : "off")}>{item.enabled ? "启用" : "停用"}</span></td>
                    <td>{modeLabel(item.mode || "")}</td>
                    <td>{riskLevelLabel(item.riskLevel)}</td>
                    <td>{policyThresholdCount(item) ? `${policyThresholdCount(item)} 项` : "继承"}</td>
                    <td>{formatDateTime(item.updatedAt)}</td>
                    <td>
                      <div className="app-act justify-end">
                        <button type="button" onClick={() => openPolicyDialog(item)} aria-label="编辑策略" title="编辑策略">
                          <Settings2 className="size-3.5" />
                        </button>
                        <button type="button" className="danger" onClick={() => setDeletePolicyTarget(item)} aria-label="删除策略" title="删除策略">
                          <Trash2 className="size-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </AdminPanel>

      <AdminPanel>
        <AdminSectionTitle title="风控日志" action={<span className="panel-count">{numberText(total)}</span>} />
        <div className="app-table-wrap">
          <table className={adminTableClass}>
            <thead className={adminTableHeadClass}>
              <tr>
                <th>时间</th>
                <th>结果</th>
                <th>用户</th>
                <th>平台/模型</th>
                <th>审核</th>
                <th>最高风险</th>
                <th>输入摘要</th>
                <th>耗时</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {logLoading ? (
                <tr>
                  <td colSpan={8} className="py-12 text-center text-sm text-[var(--app-text-muted)]">
                    <LoaderCircle className="mb-2 inline-block size-5 animate-spin" />
                    <div>读取中</div>
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-12 text-center text-sm text-[var(--app-text-muted)]">
                    暂无风控日志
                  </td>
                </tr>
              ) : (
                logs.map((item) => (
                  <tr key={item.id} className={adminTableRowClass}>
                    <td>{formatDateTime(item.createdAt)}</td>
                    <td><span className={cn("app-badge", resultBadgeClass(item))}>{resultLabel(item)}</span></td>
                    <td>
                      <div className="max-w-[160px] truncate">{item.userId || "-"}</div>
                      <div className="text-xs text-[var(--app-text-muted)]">{modeLabel(item.mode)}</div>
                    </td>
                    <td>
                      <div>{providerPlatformLabel(item.platform) || item.platform || "-"}</div>
                      <div className="max-w-[220px] truncate text-xs text-[var(--app-text-muted)]">{item.model || "-"}</div>
                    </td>
                    <td title={item.providerReason || ""}>
                      <div>{providerLabel(item.provider)}</div>
                      <div className="text-xs text-[var(--app-text-muted)]">{item.riskLevel || "-"}</div>
                    </td>
                    <td>
                      <div>{item.highestCategory || "-"}</div>
                      <div className="text-xs text-[var(--app-text-muted)]">{percentScore(item.highestScore)}</div>
                    </td>
                    <td className="prompt" title={item.error || item.inputExcerpt}>
                      {item.error || item.inputExcerpt || "-"}
                    </td>
                    <td>
                      <div>{formatLatency(item.providerLatencyMs || item.latencyMs)}</div>
                      {item.providerLatencyMs && item.latencyMs && item.providerLatencyMs !== item.latencyMs ? (
                        <div className="text-xs text-[var(--app-text-muted)]">总 {formatLatency(item.latencyMs)}</div>
                      ) : null}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        <div className="app-pager">
          <span className="info">
            第 {page} 页，共 {totalPages} 页
          </span>
          <div className="pages">
            <button type="button" disabled={page <= 1 || logLoading} onClick={() => setPage((current) => Math.max(1, current - 1))}>
              上一页
            </button>
            <button type="button" className="on" disabled>
              {page}
            </button>
            <button type="button" disabled={page >= totalPages || logLoading} onClick={() => setPage((current) => Math.min(totalPages, current + 1))}>
              下一页
            </button>
          </div>
        </div>
      </AdminPanel>

      <AppModal
        open={configOpen}
        onClose={closeConfigDialog}
        title="审核配置"
        footer={
          <>
            <button className="app-btn" type="button" onClick={closeConfigDialog} disabled={saving}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void saveConfig()} disabled={saving || !dirty}>
              {saving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存配置
            </button>
          </>
        }
      >
        <div className="app-form-grid">
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">启用风控</span>
              <span className="fd">关闭后不会审核生图请求。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", config.enabled && "on")}
              aria-pressed={config.enabled}
              aria-label="启用风控"
              onClick={() => setConfig((current) => ({ ...current, enabled: !current.enabled }))}
            />
          </div>
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">记录未命中</span>
              <span className="fd">开启后所有审核请求都会进入日志。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", config.recordNonHits && "on")}
              aria-pressed={config.recordNonHits}
              aria-label="记录未命中"
              onClick={() => setConfig((current) => ({ ...current, recordNonHits: !current.recordNonHits }))}
            />
          </div>
          <div className="app-fld">
            <span className="fl">模式</span>
            <AppSelect
              value={config.mode}
              onChange={(value) => setConfig((current) => ({ ...current, mode: value as BusinessRiskControlMode }))}
              options={modeOptions}
            />
          </div>
          <div className="app-fld">
            <span className="fl">优先顺序</span>
            <AppSelect
              value={providerChainValue(config.providerChain)}
              onChange={(value) => {
                const providerChain = providerChainFromValue(value);
                setConfig((current) => ({
                  ...current,
                  providerChain,
                  provider: providerChain[0],
                }));
              }}
              options={providerChainOptions}
            />
          </div>
          <div className="app-fld">
            <span className="fl">失败策略</span>
            <AppSelect
              value={config.failMode}
              onChange={(value) => setConfig((current) => ({ ...current, failMode: value as BusinessRiskControlFailMode }))}
              options={failModeOptions}
            />
          </div>
          <div className="app-fld">
            <span className="fl">超时毫秒</span>
            <input
              className="app-input"
              type="number"
              min={500}
              max={30000}
              value={config.timeoutMs}
              onChange={(event) => setConfig((current) => ({ ...current, timeoutMs: Number(event.target.value) }))}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">OpenAI Base URL</span>
            <input
              className="app-input"
              value={config.openaiBaseUrl || config.baseUrl}
              onChange={(event) => setConfig((current) => ({ ...current, openaiBaseUrl: event.target.value, baseUrl: event.target.value }))}
              placeholder="https://api.openai.com"
            />
          </div>
          <div className="app-fld">
            <span className="fl">OpenAI 模型</span>
            <input
              className="app-input"
              value={config.openaiModel || config.model}
              onChange={(event) => setConfig((current) => ({ ...current, openaiModel: event.target.value, model: event.target.value }))}
              placeholder="omni-moderation-latest"
            />
          </div>
          <div className="app-fld">
            <span className="fl">OpenAI API Key</span>
            <input
              className="app-input"
              type="password"
              value={newOpenAIAPIKey}
              onChange={(event) => setNewOpenAIAPIKey(event.target.value)}
              placeholder="留空则保留当前密钥"
              disabled={clearOpenAIAPIKey}
            />
            <span className="fd">
              {clearOpenAIAPIKey ? "保存后清空密钥" : config.openaiApiKeyConfigured || config.apiKeyConfigured ? `当前：${config.openaiApiKeyMasked || config.apiKeyMasked}` : "未配置"}
            </span>
          </div>
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">清空 OpenAI API Key</span>
              <span className="fd">保存后移除已存 OpenAI 密钥。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", clearOpenAIAPIKey && "on")}
              aria-pressed={clearOpenAIAPIKey}
              aria-label="清空 OpenAI API Key"
              onClick={() => {
                setClearOpenAIAPIKey((current) => !current);
                setNewOpenAIAPIKey("");
              }}
            />
          </div>
          <div className="app-fld">
            <span className="fl">阿里云 AccessKey ID</span>
            <input
              className="app-input"
              value={config.aliyunAccessKeyId}
              onChange={(event) => setConfig((current) => ({ ...current, aliyunAccessKeyId: event.target.value }))}
              placeholder="LTAI..."
            />
          </div>
          <div className="app-fld">
            <span className="fl">阿里云 AccessKey Secret</span>
            <input
              className="app-input"
              type="password"
              value={newAliyunAccessKeySecret}
              onChange={(event) => setNewAliyunAccessKeySecret(event.target.value)}
              placeholder="留空则保留当前密钥"
              disabled={clearAliyunAccessKeySecret}
            />
            <span className="fd">
              {clearAliyunAccessKeySecret ? "保存后清空密钥" : config.aliyunAccessKeySecretConfigured ? `当前：${config.aliyunAccessKeySecretMasked}` : "未配置"}
            </span>
          </div>
          <div className="app-fld">
            <span className="fl">阿里云 Region</span>
            <input
              className="app-input"
              value={config.aliyunRegionId}
              onChange={(event) => setConfig((current) => ({ ...current, aliyunRegionId: event.target.value }))}
              placeholder="ap-southeast-1"
            />
          </div>
          <div className="app-fld">
            <span className="fl">阿里云 Endpoint</span>
            <input
              className="app-input"
              value={config.aliyunEndpoint}
              onChange={(event) => setConfig((current) => ({ ...current, aliyunEndpoint: event.target.value }))}
              placeholder="green-cip.ap-southeast-1.aliyuncs.com"
            />
          </div>
          <div className="app-fld">
            <span className="fl">阿里云文本服务</span>
            <input
              className="app-input"
              value={config.aliyunTextService}
              onChange={(event) => setConfig((current) => ({ ...current, aliyunTextService: event.target.value }))}
              placeholder="ugc_moderation_byllm_cb"
            />
          </div>
          <div className="app-fld">
            <span className="fl">阿里云拦截等级</span>
            <AppSelect
              value={config.aliyunBlockRiskLevel}
              onChange={(value) => setConfig((current) => ({ ...current, aliyunBlockRiskLevel: value as "low" | "medium" | "high" }))}
              options={riskLevelOptions.filter((item) => item.value)}
            />
          </div>
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">清空阿里云 Secret</span>
              <span className="fd">保存后移除已存阿里云 AccessKey Secret。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", clearAliyunAccessKeySecret && "on")}
              aria-pressed={clearAliyunAccessKeySecret}
              aria-label="清空阿里云 Secret"
              onClick={() => {
                setClearAliyunAccessKeySecret((current) => !current);
                setNewAliyunAccessKeySecret("");
              }}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">拦截提示</span>
            <input
              className="app-input"
              value={config.blockMessage}
              onChange={(event) => setConfig((current) => ({ ...current, blockMessage: event.target.value }))}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">阈值 JSON</span>
            <textarea
              className="app-textarea min-h-[190px] font-mono text-xs"
              value={thresholdText}
              onChange={(event) => setThresholdText(event.target.value)}
              spellCheck={false}
            />
            <span className="fd">分数范围 0-1；保存时会自动裁剪非法分数。</span>
          </div>
          <div className="app-fld full">
            <span className="fl">测试当前配置</span>
            <textarea
              className="app-textarea min-h-[120px]"
              value={testPrompt}
              onChange={(event) => setTestPrompt(event.target.value)}
              placeholder="输入一段 prompt 测试当前审核配置"
            />
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="fd">
                {modeLabel(config.mode)} · {testProviderLabel}
              </span>
              <button className="app-btn" type="button" onClick={() => void runTest()} disabled={testing || !testPrompt.trim()}>
                {testing ? <LoaderCircle className="size-4 animate-spin" /> : <PlayCircle className="size-4" />}
                测试审核
              </button>
            </div>
            {testLatency !== null ? (
              <div className="mt-3">
                <span className="fl">审核结果 · {formatLatency(testLatency)}</span>
                <DecisionPreview decision={testDecision} />
              </div>
            ) : null}
          </div>
        </div>
      </AppModal>

      <AppModal
        open={testOpen}
        onClose={() => setTestOpen(false)}
        title="审核测试"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setTestOpen(false)} disabled={testing}>关闭</button>
            <button className="app-btn-primary" type="button" onClick={() => void runTest()} disabled={testing || !testPrompt.trim()}>
              {testing ? <LoaderCircle className="size-4 animate-spin" /> : <PlayCircle className="size-4" />}
              测试审核
            </button>
          </>
        }
      >
        <div className="app-form-grid">
          <div className="app-fld full">
            <span className="fl">测试内容</span>
            <textarea
              className="app-textarea min-h-[180px]"
              value={testPrompt}
              onChange={(event) => setTestPrompt(event.target.value)}
              placeholder="输入一段 prompt 测试当前审核配置"
            />
            <span className="fd">
              使用当前配置 · {modeLabel(config.mode)} · {testProviderLabel}
            </span>
          </div>
          {testLatency !== null ? (
            <div className="app-fld full">
              <span className="fl">审核结果 · {formatLatency(testLatency)}</span>
              <DecisionPreview decision={testDecision} />
            </div>
          ) : null}
        </div>
      </AppModal>

      <AppModal
        open={policyOpen}
        onClose={() => setPolicyOpen(false)}
        title={policyForm.id ? "编辑策略" : "新增策略"}
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setPolicyOpen(false)} disabled={policySaving}>取消</button>
            <button className="app-btn-primary" type="button" onClick={() => void savePolicy()} disabled={policySaving}>
              {policySaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存策略
            </button>
          </>
        }
      >
        <div className="app-form-grid">
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">启用策略</span>
              <span className="fd">停用后保留配置但不参与覆盖。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", policyForm.enabled && "on")}
              aria-pressed={policyForm.enabled}
              aria-label="启用策略"
              onClick={() => setPolicyForm((current) => ({ ...current, enabled: !current.enabled }))}
            />
          </div>
          <div className="app-fld">
            <span className="fl">范围</span>
            <AppSelect
              value={policyForm.scope}
              onChange={(value) => setPolicyForm((current) => ({
                ...current,
                scope: value as BusinessRiskControlPolicyScope,
                targetId: value === "global" ? "" : current.targetId,
              }))}
              options={policyScopeOptions}
            />
          </div>
          <div className="app-fld">
            <span className="fl">目标</span>
            <input
              className="app-input"
              value={policyForm.targetId}
              onChange={(event) => setPolicyForm((current) => ({ ...current, targetId: event.target.value }))}
              disabled={policyForm.scope === "global"}
              placeholder={policyForm.scope === "plan" ? "pro / ultra / enterprise" : policyForm.scope === "api_key" ? "API Key ID" : "用户 ID"}
            />
          </div>
          <div className="app-fld">
            <span className="fl">模式</span>
            <AppSelect
              value={policyForm.mode}
              onChange={(value) => setPolicyForm((current) => ({ ...current, mode: value as "" | BusinessRiskControlMode }))}
              options={policyModeOptions}
            />
          </div>
          <div className="app-fld">
            <span className="fl">风险等级</span>
            <AppSelect
              value={policyForm.riskLevel}
              onChange={(value) => setPolicyForm((current) => ({ ...current, riskLevel: value as BusinessRiskControlRiskLevel }))}
              options={riskLevelOptions}
            />
          </div>
          <div className="app-fld full">
            <span className="fl">拦截提示</span>
            <input
              className="app-input"
              value={policyForm.blockMessage}
              onChange={(event) => setPolicyForm((current) => ({ ...current, blockMessage: event.target.value }))}
              placeholder="留空继承全局提示"
            />
          </div>
          <div className="app-fld full">
            <span className="fl">阈值 JSON</span>
            <textarea
              className="app-textarea min-h-[180px] font-mono text-xs"
              value={policyForm.thresholdsText}
              onChange={(event) => setPolicyForm((current) => ({ ...current, thresholdsText: event.target.value }))}
              spellCheck={false}
            />
          </div>
        </div>
      </AppModal>

      <AppModal
        open={!!deletePolicyTarget}
        onClose={() => setDeletePolicyTarget(null)}
        title="删除策略"
        footer={
          <>
            <button className="app-btn" type="button" onClick={() => setDeletePolicyTarget(null)} disabled={policySaving}>取消</button>
            <button className="app-btn-danger" type="button" onClick={() => void deletePolicy()} disabled={policySaving}>
              {policySaving ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              删除
            </button>
          </>
        }
      >
        <p className="text-sm text-[var(--app-text-secondary)]">
          确认删除 {scopeLabel(deletePolicyTarget?.scope || "")} 策略 {deletePolicyTarget?.targetId || "global"} 吗？
        </p>
      </AppModal>
    </AdminPage>
  );
}
