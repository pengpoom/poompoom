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
  fetchBusinessRiskControlConfig,
  fetchBusinessRiskControlLogs,
  fetchBusinessRiskControlStatus,
  testBusinessRiskControl,
  updateBusinessRiskControlConfig,
  type BusinessRiskControlConfig,
  type BusinessRiskControlDecision,
  type BusinessRiskControlLog,
  type BusinessRiskControlMode,
  type BusinessRiskControlStatus,
} from "@/lib/api";
import { providerPlatformLabel, providerPlatformOptions } from "@/lib/provider-platforms";
import { cn } from "@/lib/utils";

const pageSize = 50;

const defaultConfig: BusinessRiskControlConfig = {
  enabled: false,
  mode: "observe",
  provider: "openai",
  baseUrl: "https://api.openai.com",
  model: "omni-moderation-latest",
  apiKeyConfigured: false,
  apiKeyMasked: "",
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
  return value === "pre_block" ? "请求前拦截" : "观察";
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
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [result, setResult] = useState("");
  const [platform, setPlatform] = useState("");
  const [search, setSearch] = useState("");
  const [newApiKey, setNewApiKey] = useState("");
  const [clearApiKey, setClearApiKey] = useState(false);
  const [thresholdText, setThresholdText] = useState("{}");
  const [configOpen, setConfigOpen] = useState(false);
  const [testOpen, setTestOpen] = useState(false);
  const [testPrompt, setTestPrompt] = useState("");
  const [testDecision, setTestDecision] = useState<BusinessRiskControlDecision | null>(null);
  const [testLatency, setTestLatency] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [logLoading, setLogLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  const savedThresholdText = useMemo(() => normalizeThresholdText(savedConfig.thresholds), [savedConfig.thresholds]);
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const testApiKeyLabel = clearApiKey
    ? "API Key 将清空"
    : newApiKey.trim() || config.apiKeyConfigured
      ? "API Key 已配置"
      : "API Key 未配置";
  const dirty = useMemo(
    () =>
      JSON.stringify(config) !== JSON.stringify(savedConfig) ||
      thresholdText !== savedThresholdText ||
      newApiKey.trim() !== "" ||
      clearApiKey,
    [clearApiKey, config, newApiKey, savedConfig, savedThresholdText, thresholdText],
  );

  const loadConfigAndStatus = useCallback(async () => {
    const [configPayload, statusPayload] = await Promise.all([
      fetchBusinessRiskControlConfig(),
      fetchBusinessRiskControlStatus(),
    ]);
    setConfig(configPayload.config);
    setSavedConfig(configPayload.config);
    setThresholdText(normalizeThresholdText(configPayload.config.thresholds));
    setStatus(statusPayload.status);
    setNewApiKey("");
    setClearApiKey(false);
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
  }, [clearApiKey, config, newApiKey, testPrompt, thresholdText]);

  const openConfigDialog = () => {
    setConfig(savedConfig);
    setThresholdText(savedThresholdText);
    setNewApiKey("");
    setClearApiKey(false);
    setTestDecision(null);
    setTestLatency(null);
    setConfigOpen(true);
  };

  const closeConfigDialog = () => {
    setConfigOpen(false);
    setConfig(savedConfig);
    setThresholdText(savedThresholdText);
    setNewApiKey("");
    setClearApiKey(false);
  };

  const openTestDialog = () => {
    setTestOpen(true);
    setTestDecision(null);
    setTestLatency(null);
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
        baseUrl: config.baseUrl,
        apiKey: newApiKey.trim() || undefined,
        clearApiKey,
        model: config.model,
        timeoutMs: Number(config.timeoutMs || 3000),
        recordNonHits: config.recordNonHits,
        blockMessage: config.blockMessage,
        thresholds,
      });
      setConfig(payload.config);
      setSavedConfig(payload.config);
      setThresholdText(normalizeThresholdText(payload.config.thresholds));
      setNewApiKey("");
      setClearApiKey(false);
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
        baseUrl: config.baseUrl,
        apiKey: newApiKey.trim() || undefined,
        clearApiKey,
        model: config.model,
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
          sub={status?.apiKeyConfigured ? "API Key 已配置" : "API Key 未配置"}
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
          sub={savedConfig.provider === "openai" ? "OpenAI moderation" : savedConfig.provider}
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
          {status?.apiKeyConfigured ? "密钥已配置" : "密钥未配置"}
        </span>
      </AdminToolbar>

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
                <th>最高风险</th>
                <th>输入摘要</th>
                <th>耗时</th>
              </tr>
            </thead>
            <tbody className={adminTableBodyClass}>
              {logLoading ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-sm text-[var(--app-text-muted)]">
                    <LoaderCircle className="mb-2 inline-block size-5 animate-spin" />
                    <div>读取中</div>
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-sm text-[var(--app-text-muted)]">
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
                    <td>
                      <div>{item.highestCategory || "-"}</div>
                      <div className="text-xs text-[var(--app-text-muted)]">{percentScore(item.highestScore)}</div>
                    </td>
                    <td className="prompt" title={item.error || item.inputExcerpt}>
                      {item.error || item.inputExcerpt || "-"}
                    </td>
                    <td>{formatLatency(item.latencyMs)}</td>
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
            <span className="fl">Base URL</span>
            <input
              className="app-input"
              value={config.baseUrl}
              onChange={(event) => setConfig((current) => ({ ...current, baseUrl: event.target.value }))}
              placeholder="https://api.openai.com"
            />
          </div>
          <div className="app-fld">
            <span className="fl">模型</span>
            <input
              className="app-input"
              value={config.model}
              onChange={(event) => setConfig((current) => ({ ...current, model: event.target.value }))}
              placeholder="omni-moderation-latest"
            />
          </div>
          <div className="app-fld">
            <span className="fl">API Key</span>
            <input
              className="app-input"
              type="password"
              value={newApiKey}
              onChange={(event) => setNewApiKey(event.target.value)}
              placeholder="留空则保留当前密钥"
              disabled={clearApiKey}
            />
            <span className="fd">
              {clearApiKey ? "保存后清空密钥" : config.apiKeyConfigured ? `当前：${config.apiKeyMasked}` : "未配置"}
            </span>
          </div>
          <div className="app-fld switch-row full">
            <span className="fl-wrap">
              <span className="fl">清空 API Key</span>
              <span className="fd">保存后移除已存密钥。</span>
            </span>
            <button
              type="button"
              className={cn("app-switch", clearApiKey && "on")}
              aria-pressed={clearApiKey}
              aria-label="清空 API Key"
              onClick={() => {
                setClearApiKey((current) => !current);
                setNewApiKey("");
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
                {modeLabel(config.mode)} · {testApiKeyLabel}
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
              使用当前配置 · {modeLabel(config.mode)} · {testApiKeyLabel}
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
    </AdminPage>
  );
}
