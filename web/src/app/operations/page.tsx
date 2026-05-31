"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Cpu,
  Database,
  Gauge,
  HardDrive,
  Link2,
  LoaderCircle,
  MemoryStick,
  RefreshCw,
  Server,
  Star,
  X,
  type LucideIcon,
} from "lucide-react";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminSectionTitle,
  AdminStatCard,
} from "@/components/admin-layout";
import {
  adminSubPanelClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { AppSelect, AppTimeSeg } from "@/components/app-controls";
import { timeRangeQuery, type TimeRangeValue } from "@/components/time-range-utils";
import {
  fetchAdminBusinessImageJobs,
  fetchBusinessAPIProviders,
  fetchBusinessProviderPools,
  fetchBusinessTrackerSummary,
  fetchBusinessUsers,
  fetchMaintenanceStatus,
  fetchRuntimeStatus,
  updateMaintenanceStatus,
  type BusinessAPIProvider,
  type BusinessProviderPool,
  type BusinessImageJob,
  type BusinessTrackerSummary,
  type BusinessUser,
  type MaintenanceStatus,
  type RuntimeStatusResponse,
} from "@/lib/api";
import { cn } from "@/lib/utils";

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function limitText(value: number | undefined) {
  const normalized = Number(value || 0);
  return normalized > 0 ? normalized.toLocaleString() : "不限";
}

function runtimeCapacity(runtime: RuntimeStatusResponse | null) {
  return runtime?.capacity ?? {
    maxUserActiveJobs: 0,
    maxProviderRunningJobs: 0,
    maxQueuedJobs: 0,
    queuedJobs: 0,
  };
}

function databaseName(database: NonNullable<RuntimeStatusResponse["system"]>["database"] | undefined) {
  const driver = (database?.driver || "").toLowerCase();
  if (driver === "postgres" || driver === "postgresql") {
    return "PostgreSQL";
  }
  return database?.name || driver || "数据库";
}

function databaseSummary(database: NonNullable<RuntimeStatusResponse["system"]>["database"] | undefined) {
  const driver = (database?.driver || "").toLowerCase();
  if (driver === "postgres" || driver === "postgresql") {
    return `连接 ${numberText(database?.openConns)} / 使用中 ${numberText(database?.inUseConns)}`;
  }
  return database?.status || "-";
}

function databaseDetail(database: NonNullable<RuntimeStatusResponse["system"]>["database"] | undefined) {
  if (!database) {
    return undefined;
  }
  return database.error || database.path || database.dsn || undefined;
}

function formatDateTime(value: string | undefined) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatDurationMs(value: number | undefined) {
  const ms = Math.max(0, Number(value || 0));
  if (ms <= 0) {
    return "-";
  }
  if (ms < 1000) {
    return `${ms} ms`;
  }
  return `${(ms / 1000).toFixed(ms >= 10000 ? 1 : 2)} 秒`;
}

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes <= 0) {
    return "-";
  }
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  }
  if (bytes < 1024 * 1024 * 1024 * 1024) {
    return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
  }
  return `${(bytes / 1024 / 1024 / 1024 / 1024).toFixed(1)} TB`;
}

function admissionPressure(runtime: RuntimeStatusResponse | null) {
  const maxRunning = Number(runtime?.admission.maxConcurrency || 0);
  const running = Number(runtime?.admission.inflight || 0);
  if (maxRunning <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((running / maxRunning) * 100));
}

function percentText(value: number | undefined) {
  if (!Number.isFinite(Number(value))) {
    return "-";
  }
  return `${Number(value || 0).toFixed(1)}%`;
}

function enabledProviderCount(items: BusinessAPIProvider[]) {
  return items.filter((item) => item.enabled).length;
}

function defaultProviderNames(items: BusinessAPIProvider[]) {
  const names = items
    .filter((item) => item.enabled && item.isDefault)
    .map((item) => item.name || item.platform);
  return names.length > 0 ? names.join(" / ") : "-";
}

function poolDefaultNames(items: BusinessProviderPool[]) {
  const names = items
    .filter((item) => item.enabled && item.isDefault)
    .map((item) => item.name || item.platform);
  return names.length > 0 ? names.join(" / ") : "-";
}

function platformLabel(value: string) {
  switch (value) {
    case "gpt-image":
      return "gpt-image";
    case "gemini-banana":
      return "gemini-banana";
    default:
      return value || "-";
  }
}

function jobStatusLabel(status: string) {
  switch (status) {
    case "queued":
      return "排队中";
    case "running":
      return "运行中";
    case "cancel_request":
    case "cancel_requested":
      return "取消中";
    case "cancelled":
      return "已取消";
    case "succeeded":
      return "成功";
    case "failed":
      return "失败";
    default:
      return status || "-";
  }
}

function jobStatusVariant(status: string): "success" | "warning" | "danger" | "info" {
  switch (status) {
    case "succeeded":
      return "success";
    case "failed":
      return "danger";
    case "queued":
    case "running":
    case "cancel_requested":
      return "warning";
    default:
      return "info";
  }
}

function userErrorTypeLabel(type?: string) {
  switch (type) {
    case "credit":
      return "点数不足";
    case "input":
      return "输入不完整";
    case "queue":
      return "队列/并发";
    case "busy":
      return "服务繁忙";
    case "refused":
      return "内容拒绝";
    case "overload":
      return "规格过高";
    case "timeout":
      return "生成超时";
    case "cancelled":
      return "用户取消";
    case "unknown":
      return "未知错误";
    default:
      return "";
  }
}

function badgeClass(variant: "success" | "warning" | "danger" | "info") {
  if (variant === "success") return "ok";
  if (variant === "danger") return "fail";
  if (variant === "info") return "off";
  return "warn";
}

function upstreamJobLabel(job: BusinessImageJob) {
  return job.upstreamSent || job.upstreamStatus === "sent" ? "已发上游" : "未发上游";
}

function upstreamJobVariant(job: BusinessImageJob): "warning" | "info" {
  return job.upstreamSent || job.upstreamStatus === "sent" ? "warning" : "info";
}

function providerSourceLabel(job: BusinessImageJob) {
  switch (job.providerSource) {
    case "provider_pool":
      return "号池";
    case "legacy_provider":
      return "API 接入";
    case "api_access":
      return "配置接入";
    case "environment":
      return "环境变量";
    default:
      return job.providerId ? "API 接入" : "-";
  }
}

function providerSourceVariant(job: BusinessImageJob): "success" | "warning" | "danger" | "info" {
  if (job.providerSource === "provider_pool") {
    return "success";
  }
  if (job.providerSource === "legacy_provider" || job.providerSource === "api_access") {
    return "info";
  }
  if (job.providerSource === "environment") {
    return "warning";
  }
  return "info";
}

function dispatchStrategyLabel(value?: string) {
  switch (value) {
    case "tagged_pool":
      return "标签池";
    case "tagged_pool_fallback":
      return "标签池回退";
    case "fallback_pool":
      return "兜底池";
    case "api_fallback":
      return "API 兜底";
    case "api_access":
      return "API 策略";
    default:
      return "";
  }
}

function dispatchStrategyVariant(value?: string): "success" | "warning" | "danger" | "info" {
  switch (value) {
    case "tagged_pool":
      return "success";
    case "tagged_pool_fallback":
    case "fallback_pool":
    case "api_fallback":
      return "warning";
    default:
      return "info";
  }
}

function providerMatchModeLabel(value?: string) {
  switch (value) {
    case "any":
      return "任一标签";
    case "all":
      return "全部标签";
    case "fallback":
      return "fallback";
    default:
      return "";
  }
}

function fullTagList(tags?: string[]) {
  const items = (tags || []).map((tag) => tag.trim()).filter(Boolean);
  return items.length > 0 ? items.join(" · ") : "-";
}

function prettyPayload(payload?: Record<string, unknown>) {
  if (!payload || Object.keys(payload).length === 0) {
    return "{}";
  }
  const { sourceImages: _sourceImages, sourceReference: _sourceReference, ...safePayload } = payload;
  if (Object.keys(safePayload).length === 0) {
    return "{}";
  }
  return JSON.stringify(safePayload, null, 2);
}

function jobDispatchTrace(job: BusinessImageJob) {
  if ((job.dispatchTrace || []).length > 0) {
    return job.dispatchTrace || [];
  }
  switch (job.dispatchStrategy) {
    case "tagged_pool":
      return [
        "请求标签命中标签池",
        `调度到池 ${job.providerGroupName || job.providerGroupId || "-"}`,
        `调度到成员 ${job.providerMemberName || job.providerName || job.providerMemberId || "-"}`,
      ];
    case "tagged_pool_fallback":
      return [
        "请求标签命中标签池",
        "标签池无可用成员",
        `回退 fallback 池 ${job.providerGroupName || job.providerGroupId || "-"}`,
        `调度到成员 ${job.providerMemberName || job.providerName || job.providerMemberId || "-"}`,
      ];
    case "fallback_pool":
      return [
        "没有标签池命中",
        `调度到 fallback 池 ${job.providerGroupName || job.providerGroupId || "-"}`,
        `调度到成员 ${job.providerMemberName || job.providerName || job.providerMemberId || "-"}`,
      ];
    case "api_fallback":
      return [
        "号池无可用成员",
        `回退 API 接入 ${job.providerName || job.providerId || providerSourceLabel(job)}`,
      ];
    case "api_access":
      return [`使用 API 接入 ${job.providerName || job.providerId || providerSourceLabel(job)}`];
    default:
      if (job.providerSource === "provider_pool") {
        return [
          `调度到池 ${job.providerGroupName || job.providerGroupId || "-"}`,
          `调度到成员 ${job.providerMemberName || job.providerName || job.providerMemberId || "-"}`,
        ];
      }
      if (job.providerSource) {
        return [`使用 ${providerSourceLabel(job)} ${job.providerName || job.providerId || ""}`.trim()];
      }
      return ["未记录调度策略"];
  }
}

function statusVariant(runtime: RuntimeStatusResponse | null, providers: BusinessAPIProvider[], tracker: BusinessTrackerSummary | null): "success" | "warning" | "danger" {
  if (!runtime) {
    return "warning";
  }
  if (providers.length === 0 || enabledProviderCount(providers) === 0) {
    return "danger";
  }
  if ((tracker?.failed || 0) > 0 || runtime.admission.queued > 0) {
    return "warning";
  }
  return "success";
}

function statusText(runtime: RuntimeStatusResponse | null, providers: BusinessAPIProvider[], tracker: BusinessTrackerSummary | null) {
  if (!runtime) {
    return "未连接";
  }
  if (providers.length === 0) {
    return "API 未配置";
  }
  if (enabledProviderCount(providers) === 0) {
    return "API 未启用";
  }
  if ((tracker?.failed || 0) > 0) {
    return "有错误";
  }
  if (runtime.admission.queued > 0) {
    return "有排队";
  }
  return "正常";
}

function MetricCard({
  label,
  value,
  sub,
  icon: Icon,
  color,
}: {
  label: string;
  value: ReactNode;
  sub: ReactNode;
  icon: LucideIcon;
  color: string;
}) {
  return (
    <AdminStatCard label={label} value={value} sub={sub} icon={Icon} color={color} />
  );
}

function SystemStatusCard({
  label,
  value,
  sub,
  detail,
  badge,
  badgeVariant,
  icon: Icon,
  color,
}: {
  label: string;
  value: ReactNode;
  sub: ReactNode;
  detail?: ReactNode;
  badge: ReactNode;
  badgeVariant: "success" | "warning" | "danger" | "info";
  icon: LucideIcon;
  color: string;
}) {
  return (
    <div className={cn(adminSubPanelClass, "min-w-0 p-4")}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs text-[var(--app-text-muted)]">{label}</div>
          <div className={cn("mt-2 truncate text-xl font-semibold", color)}>{value}</div>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-2">
          <Icon className={cn("size-5", color)} />
          <span className={cn("app-badge", badgeClass(badgeVariant))}>{badge}</span>
        </div>
      </div>
      <div className="mt-3 truncate text-xs text-[var(--app-text-muted)]">{sub}</div>
      {detail ? (
        <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]">{detail}</div>
      ) : null}
    </div>
  );
}

function SectionTitle({ title, action }: { title: string; action?: ReactNode }) {
  return <AdminSectionTitle title={title} action={action} />;
}

function DetailRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-[var(--app-radius-md)] bg-[var(--app-bg-surface)] px-3 py-2 text-sm">
      <span className="text-[var(--app-text-muted)]">{label}</span>
      <span className="text-right font-medium text-[var(--app-text-primary)]">{value}</span>
    </div>
  );
}

function DetailBlock({
  title,
  children,
  tone = "default",
}: {
  title: string;
  children: ReactNode;
  tone?: "default" | "danger";
}) {
  return (
    <section className={cn(
      "rounded-[var(--app-radius-md)] border p-4",
      tone === "danger"
        ? "border-rose-400/20 bg-rose-500/10"
        : "border-[var(--app-border)] bg-[var(--app-bg-surface)]",
    )}>
      <h4 className="mb-3 text-sm font-semibold text-[var(--app-text-primary)]">{title}</h4>
      {children}
    </section>
  );
}

function JobDetailDrawer({
  job,
  username,
  onClose,
}: {
  job: BusinessImageJob | null;
  username: string;
  onClose: () => void;
}) {
  if (!job) {
    return null;
  }
  return (
    <div className="fixed inset-0 z-50">
      <button
        type="button"
        className="absolute inset-0 bg-black/55"
        aria-label="关闭 Job 详情"
        onClick={onClose}
      />
      <aside className="absolute right-0 top-0 h-full w-full max-w-[760px] overflow-y-auto border-l border-[var(--app-border)] bg-[var(--app-bg-popover-solid)] shadow-2xl">
        <div className="sticky top-0 z-10 flex items-start justify-between gap-4 border-b border-[var(--app-border)] bg-[var(--app-bg-popover-solid)] px-5 py-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className={cn("app-badge", badgeClass(jobStatusVariant(job.status)))}>{jobStatusLabel(job.status)}</span>
              <span className={cn("app-badge", badgeClass(providerSourceVariant(job)))}>{providerSourceLabel(job)}</span>
              {dispatchStrategyLabel(job.dispatchStrategy) ? (
                <span className={cn("app-badge", badgeClass(dispatchStrategyVariant(job.dispatchStrategy)))}>
                  {dispatchStrategyLabel(job.dispatchStrategy)}
                </span>
              ) : null}
            </div>
            <div className="mt-3 flex min-w-0 items-center gap-2">
              <h3 className="truncate text-base font-semibold text-[var(--app-text-primary)]">{job.prompt || "无提示词"}</h3>
              {job.hasAttachment ? <span className="app-badge off shrink-0">含附件</span> : null}
            </div>
            <div className="mt-1 truncate text-xs text-[var(--app-text-muted)]">{job.id}</div>
          </div>
          <button type="button" className="app-btn" onClick={onClose} aria-label="关闭">
            <X className="size-4" />
          </button>
        </div>
        <div className="grid gap-4 bg-[var(--app-bg-root)]/35 p-5 text-sm text-[var(--app-text-secondary)]">
          <section className="grid gap-2 sm:grid-cols-2">
            <DetailRow label="用户" value={username || job.userId} />
            <DetailRow label="阶段" value={job.stage || "-"} />
            <DetailRow label="平台" value={`${platformLabel(String(job.platform || ""))} / ${job.model || "-"}`} />
            <DetailRow label="规格" value={`${job.size || "-"} / ${job.quality || "-"}`} />
            <DetailRow label="图片" value={`${numberText(job.actualCount)} / ${numberText(job.requestedCount)} 张`} />
            <DetailRow label="扣点" value={`${numberText(job.creditReserved)} / 退 ${numberText(job.creditRefunded)}`} />
          </section>

          <div className="grid gap-4 lg:grid-cols-2">
            <DetailBlock title="用户看到的错误" tone={job.userErrorType || job.userErrorMessage ? "danger" : "default"}>
              <div className="grid gap-2 text-xs leading-5">
                <div className="flex flex-wrap items-center gap-2">
                  {job.userErrorType ? <span className="app-badge fail">{userErrorTypeLabel(job.userErrorType) || job.userErrorType}</span> : null}
                  <span className="break-words text-[var(--app-text-secondary)]">{job.userErrorMessage || "-"}</span>
                </div>
              </div>
            </DetailBlock>

            <DetailBlock title="真实上游错误" tone={job.errorCode || job.errorMessage ? "danger" : "default"}>
              <div className="grid gap-2 text-xs leading-5">
                <div><span className="text-[var(--app-text-muted)]">错误码：</span>{job.errorCode || "-"}</div>
                <div className="break-words text-rose-200">{job.errorMessage || "-"}</div>
              </div>
            </DetailBlock>
          </div>

          <DetailBlock title="调度链路">
            <div className="grid gap-3 text-xs leading-5">
              {jobDispatchTrace(job).map((item, index) => (
                <div key={`${item}-${index}`} className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-3">
                  <span className="app-badge off">{index + 1}</span>
                  <span className="min-w-0 break-words">{item}</span>
                </div>
              ))}
            </div>
          </DetailBlock>

          <DetailBlock title="Provider">
            <div className="grid gap-2 text-xs leading-5 sm:grid-cols-2">
              <div>来源：{providerSourceLabel(job)}</div>
              <div>策略：{dispatchStrategyLabel(job.dispatchStrategy) || "-"}</div>
              <div>池：{job.providerGroupName || job.providerGroupId || "-"}</div>
              <div>成员：{job.providerMemberName || job.providerName || job.providerMemberId || job.providerId || "-"}</div>
              <div>池匹配：{providerMatchModeLabel(job.providerGroupMatchMode) || "-"}</div>
              <div>池标签：{fullTagList(job.providerGroupTags)}</div>
            </div>
          </DetailBlock>

          <DetailBlock title="标签">
            <div className="grid gap-2 text-xs leading-5">
              <div>请求：<span className="font-mono">{fullTagList(job.requestDispatchTags)}</span></div>
              <div>用户：<span className="font-mono">{fullTagList(job.userDispatchTags)}</span></div>
              <div>最终：<span className="font-mono">{fullTagList(job.dispatchTags)}</span></div>
            </div>
          </DetailBlock>

          <DetailBlock title="耗时链路">
            <div className="grid gap-2 text-xs leading-5 sm:grid-cols-2">
              <div>排队：{formatDurationMs(job.queueWaitMs)}</div>
              <div>上游：{formatDurationMs(job.upstreamDurationMs)}</div>
              <div>落盘：{formatDurationMs(job.persistDurationMs)}</div>
              <div>总计：{formatDurationMs(job.totalDurationMs)}</div>
              <div>创建：{formatDateTime(job.createdAt)}</div>
              <div>开始：{formatDateTime(job.startedAt)}</div>
              <div>结束：{formatDateTime(job.finishedAt)}</div>
              <div>更新：{formatDateTime(job.updatedAt)}</div>
            </div>
          </DetailBlock>

          <DetailBlock title="Payload">
            <pre className="max-h-[360px] overflow-auto rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-black/20 p-3 text-xs leading-5 text-[var(--app-text-secondary)]">
              {prettyPayload(job.payload)}
            </pre>
          </DetailBlock>
        </div>
      </aside>
    </div>
  );
}

const jobTableColumnCount = 6;


export default function OperationsPage() {
  const [runtime, setRuntime] = useState<RuntimeStatusResponse | null>(null);
  const [tracker, setTracker] = useState<BusinessTrackerSummary | null>(null);
  const [providers, setProviders] = useState<BusinessAPIProvider[]>([]);
  const [providerPools, setProviderPools] = useState<BusinessProviderPool[]>([]);
  const [users, setUsers] = useState<BusinessUser[]>([]);
  const [jobs, setJobs] = useState<BusinessImageJob[]>([]);
  const [jobsPage, setJobsPage] = useState({ page: 1, pageSize: 20, total: 0 });
  const [jobStatus, setJobStatus] = useState("");
  const [jobErrorType, setJobErrorType] = useState("");
  const [jobUserId, setJobUserId] = useState("");
  const [jobPlatform, setJobPlatform] = useState("");
  const [jobTimeRange, setJobTimeRange] = useState<TimeRangeValue>({ preset: "last7", from: "", to: "" });
  const [loading, setLoading] = useState(true);
  const [jobsLoading, setJobsLoading] = useState(true);
  const [maintenance, setMaintenance] = useState<MaintenanceStatus | null>(null);
  const [maintenanceSaving, setMaintenanceSaving] = useState(false);
  const [selectedJob, setSelectedJob] = useState<BusinessImageJob | null>(null);

  const jobQuery = useMemo(
    () => ({
      status: jobStatus || undefined,
      errorType: jobErrorType || undefined,
      userId: jobUserId || undefined,
      platform: jobPlatform || undefined,
      ...timeRangeQuery(jobTimeRange),
    }),
    [jobErrorType, jobPlatform, jobStatus, jobTimeRange, jobUserId],
  );

  const loadStatus = useCallback(async () => {
    setLoading(true);
    try {
      const [runtimePayload, trackerPayload, providersPayload, poolsPayload, maintenancePayload] = await Promise.all([
        fetchRuntimeStatus(),
        fetchBusinessTrackerSummary(600),
        fetchBusinessAPIProviders(),
        fetchBusinessProviderPools(),
        fetchMaintenanceStatus(),
      ]);
      setRuntime(runtimePayload);
      setTracker(trackerPayload);
      setProviders(providersPayload.items);
      setProviderPools(poolsPayload.items);
      setMaintenance(maintenancePayload);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取运维监控失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const handleToggleMaintenance = useCallback(async () => {
    const nextEnabled = !maintenance?.enabled;
    setMaintenanceSaving(true);
    try {
      const payload = await updateMaintenanceStatus(nextEnabled);
      setMaintenance(payload);
      toast.success(payload.enabled ? "维护模式已开启" : "维护模式已关闭");
      void loadStatus();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新维护模式失败");
    } finally {
      setMaintenanceSaving(false);
    }
  }, [loadStatus, maintenance?.enabled]);

  const loadJobs = useCallback(async (page = 1) => {
    setJobsLoading(true);
    try {
      const payload = await fetchAdminBusinessImageJobs({
        page,
        pageSize: jobsPage.pageSize,
        ...jobQuery,
      });
      setJobs(payload.items || []);
      setJobsPage(payload.page);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取 Job 列表失败");
    } finally {
      setJobsLoading(false);
    }
  }, [jobQuery, jobsPage.pageSize]);

  const loadUsers = useCallback(async () => {
    try {
      const payload = await fetchBusinessUsers();
      setUsers(payload.items || []);
    } catch {
      setUsers([]);
    }
  }, []);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  useEffect(() => {
    void loadJobs(1);
  }, [loadJobs]);

  const pressure = useMemo(() => admissionPressure(runtime), [runtime]);
  const enabledProviders = useMemo(() => enabledProviderCount(providers), [providers]);
  const defaultProviders = useMemo(() => defaultProviderNames(providers), [providers]);
  const defaultPools = useMemo(() => poolDefaultNames(providerPools), [providerPools]);
  const providerPool = runtime?.providerPool;
  const providerPoolIssues = providerPool?.dispatchIssues || [];
  const recentFailure = tracker?.recentFailures[0];
  const system = runtime?.system;
  const capacity = runtimeCapacity(runtime);
  const usernamesByID = useMemo(() => {
    const next = new Map<string, string>();
    users.forEach((user) => next.set(user.id, user.username));
    return next;
  }, [users]);
  const selectedJobUsername = selectedJob ? usernamesByID.get(selectedJob.userId) || selectedJob.userId : "";

  return (
    <AdminPage>
        <AdminHeader
          title="运维监控"
          description="查看准入并发、上游配置健康摘要、tracker 成功率和最近错误。"
          actions={
            <>
              <button
                className={maintenance?.enabled ? "app-btn-primary" : "app-btn"}
                type="button"
                onClick={() => void handleToggleMaintenance()}
                disabled={loading || maintenanceSaving}
                style={maintenance?.enabled ? { background: "linear-gradient(135deg, #f59e0b, #d97706)" } : undefined}
              >
                {maintenanceSaving ? <LoaderCircle className="size-4 animate-spin" /> : <AlertTriangle className="size-4" />}
                {maintenance?.enabled ? "关闭维护模式" : "开启维护模式"}
              </button>
              <button className="app-btn" type="button" onClick={() => void loadStatus()} disabled={loading}>
                {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                刷新
              </button>
            </>
          }
        >
          {(() => {
            const variant = statusVariant(runtime, providers, tracker);
            const cls = variant === "success" ? "ok" : variant === "danger" ? "fail" : "warn";
            return <span className={`app-badge ${cls}`}>{statusText(runtime, providers, tracker)}</span>;
          })()}
          {maintenance?.enabled ? <span className="app-badge warn">维护模式</span> : null}
        </AdminHeader>

        {maintenance?.enabled ? (
          <section className="rounded-[var(--app-radius-lg)] border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm text-amber-100 shadow-[0_14px_36px_rgba(0,0,0,0.16)]">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2 font-medium">
                <AlertTriangle className="size-4" />
                {maintenance.message || "系统正在升级维护，请稍等几分钟后重新提交"}
              </div>
              <div className="text-xs text-amber-100/70">
                {maintenance.updatedAt ? `开启时间 ${formatDateTime(maintenance.updatedAt)}` : "重启后会自动关闭"}
              </div>
            </div>
          </section>
        ) : null}

        <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <MetricCard
            label="并发"
            value={loading ? "-" : `${numberText(runtime?.admission.inflight)}/${numberText(runtime?.admission.maxConcurrency)}`}
            sub={`使用率 ${pressure}%`}
            icon={Gauge}
            color="text-sky-300"
          />
          <MetricCard
            label="排队"
            value={loading ? "-" : numberText(runtime?.admission.queued)}
            sub={`上限 ${numberText(runtime?.admission.queueLimit)} / 超时 ${formatDurationMs(runtime?.admission.queueTimeoutMs)}`}
            icon={Clock3}
            color="text-amber-300"
          />
          <MetricCard
            label="数据库队列"
            value={loading ? "-" : `${numberText(capacity.queuedJobs)}/${limitText(capacity.maxQueuedJobs)}`}
            sub={`单用户 ${limitText(capacity.maxUserActiveJobs)} / 接入 ${limitText(capacity.maxProviderRunningJobs)}`}
            icon={Database}
            color="text-cyan-300"
          />
          <MetricCard
            label="成功率"
            value={loading ? "-" : percentText(tracker?.successRate)}
            sub={`近 ${numberText(tracker?.windowSeconds)} 秒 ${numberText(tracker?.succeeded)}/${numberText(tracker?.total)}`}
            icon={CheckCircle2}
            color="text-emerald-300"
          />
          <MetricCard
            label="上游耗时"
            value={loading ? "-" : formatDurationMs(tracker?.avgUpstreamMs)}
            sub={`P95 ${formatDurationMs(tracker?.p95UpstreamMs)}`}
            icon={Link2}
            color="text-indigo-300"
          />
        </section>

        <AdminPanel>
          <SectionTitle
            title="系统状态"
            action={<span className="text-xs text-[var(--app-text-muted)]">快照 {formatDateTime(runtime?.timestamp)}</span>}
          />
          <div className="grid gap-3 p-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
            <SystemStatusCard
              label="CPU"
              value={loading ? "-" : system?.cpu.available ? percentText(system.cpu.usagePercent) : "不可用"}
              sub={loading ? "读取中" : system?.cpu.error || "当前采样"}
              badge={loading ? "读取中" : system?.cpu.available ? "正常" : "不可用"}
              badgeVariant={loading ? "warning" : system?.cpu.available ? "success" : "warning"}
              icon={Cpu}
              color="text-sky-300"
            />
            <SystemStatusCard
              label="内存"
              value={loading ? "-" : system?.memory.available ? percentText(system.memory.usagePercent) : "不可用"}
              sub={
                loading
                  ? "读取中"
                  : system?.memory.available
                    ? `${formatBytes(system.memory.usedBytes)} / ${formatBytes(system.memory.totalBytes)}`
                    : system?.memory.error || "无采样"
              }
              detail={
                system?.memory.source === "cgroup"
                  ? "容器内存"
                  : system?.memory.source === "system"
                    ? "系统内存"
                    : system?.memory.source === "go"
                      ? "Go runtime"
                      : undefined
              }
              badge={loading ? "读取中" : system?.memory.available ? "正常" : "不可用"}
              badgeVariant={loading ? "warning" : system?.memory.available ? "success" : "warning"}
              icon={MemoryStick}
              color="text-emerald-300"
            />
            <SystemStatusCard
              label="数据库"
              value={loading ? "-" : system?.database.ok ? "正常" : "异常"}
              sub={loading ? "读取中" : `${databaseName(system?.database)} ${databaseSummary(system?.database)}`}
              detail={databaseDetail(system?.database)}
              badge={loading ? "读取中" : system?.database.ok ? "正常" : "异常"}
              badgeVariant={loading ? "warning" : system?.database.ok ? "success" : "danger"}
              icon={Database}
              color="text-violet-300"
            />
            <SystemStatusCard
              label="Redis"
              value={loading ? "-" : !system?.redis.enabled ? "未启用" : system.redis.ok ? "正常" : "异常"}
              sub={loading ? "读取中" : system?.redis.enabled ? system.redis.addr || "-" : "当前未使用 Redis"}
              detail={system?.redis.error || undefined}
              badge={loading ? "读取中" : !system?.redis.enabled ? "未启用" : system.redis.ok ? "正常" : "异常"}
              badgeVariant={loading ? "warning" : !system?.redis.enabled ? "info" : system.redis.ok ? "success" : "danger"}
              icon={Server}
              color="text-indigo-300"
            />
            <SystemStatusCard
              label="磁盘"
              value={loading ? "-" : system?.disk?.available ? formatBytes(system.disk.freeBytes) : "不可用"}
              sub={
                loading
                  ? "读取中"
                  : system?.disk?.available
                    ? `剩余 / 总计 ${formatBytes(system.disk.totalBytes)}`
                    : system?.disk?.error || "无采样"
              }
              detail={system?.disk?.available ? `已用 ${percentText(system.disk.usedPercent)}` : system?.disk?.path || undefined}
              badge={loading ? "读取中" : system?.disk?.available ? "正常" : "不可用"}
              badgeVariant={loading ? "warning" : system?.disk?.available ? "success" : "warning"}
              icon={HardDrive}
              color="text-teal-300"
            />
            <SystemStatusCard
              label="协程 / 队列"
              value={loading ? "-" : numberText(system?.runtime.goroutines)}
              sub={`运行 ${loading ? "-" : numberText(system?.runtime.inflight)} / 排队 ${loading ? "-" : numberText(system?.runtime.queued)}`}
              detail="Go 协程 / Job 队列"
              badge={loading ? "读取中" : (system?.runtime.queued || 0) > 0 ? "排队中" : "正常"}
              badgeVariant={loading ? "warning" : (system?.runtime.queued || 0) > 0 ? "warning" : "success"}
              icon={Activity}
              color="text-amber-300"
            />
          </div>
        </AdminPanel>

        <section className="order-4 grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.75fr)]">
          <AdminPanel>
            <SectionTitle
              title="上游健康摘要"
              action={
                <span className={cn("app-badge", enabledProviders > 0 ? "ok" : "fail")}>
                  启用 {numberText(enabledProviders)}/{numberText(providers.length)}
                </span>
              }
            />
            <div className="space-y-4 p-4">
              <div className="grid gap-3 sm:grid-cols-3">
                <DetailRow label="接入总数" value={loading ? "-" : numberText(providers.length)} />
                <DetailRow label="已启用" value={loading ? "-" : numberText(enabledProviders)} />
                <DetailRow label="默认接入" value={loading ? "-" : defaultProviders} />
              </div>

              {loading ? (
                <div className="py-8 text-center text-[var(--app-text-muted)]">
                  <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                  读取中
                </div>
              ) : providerPool && (providerPool.groups > 0 || providerPool.members > 0) ? (
                <div className="space-y-4">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <DetailRow label="默认池" value={defaultPools} />
                    <DetailRow label="池 / 成员" value={`${numberText(providerPool.groups)} / ${numberText(providerPool.members)}`} />
                    <DetailRow label="可用成员" value={numberText(providerPool.availableMembers)} />
                    <DetailRow label="冷却成员" value={numberText(providerPool.coolingMembers)} />
                    <DetailRow label="限流成员" value={numberText(providerPool.limitedMembers)} />
                    <DetailRow label="并发已满" value={numberText(providerPool.concurrencyLimitedMembers)} />
                    <DetailRow label="不可用成员" value={numberText(providerPool.unavailableMembers)} />
                  </div>
                  <div className="rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-4">
                    <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--app-text-primary)]">
                      <Gauge className="size-4 text-sky-300" />
                      调度诊断
                    </div>
                    {providerPoolIssues.length > 0 ? (
                      <div className="grid gap-2 sm:grid-cols-2">
                        {providerPoolIssues.map((issue) => (
                          <div
                            key={issue.code}
                            className={cn(
                              "rounded-[var(--app-radius-md)] border px-3 py-2 text-sm",
                              issue.code === "no_api_fallback" || issue.code === "no_available_member"
                                ? "border-rose-400/20 bg-rose-400/10 text-rose-100"
                                : "border-amber-400/20 bg-amber-400/10 text-amber-100",
                            )}
                          >
                            <div className="flex items-center justify-between gap-3">
                              <span className="font-medium">{issue.label}</span>
                              <span className="app-badge off">{numberText(issue.count)}</span>
                            </div>
                            {issue.detail ? (
                              <div className="mt-1 text-xs leading-5 opacity-80">{issue.detail}</div>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="rounded-[var(--app-radius-md)] border border-emerald-400/20 bg-emerald-400/10 px-3 py-2 text-sm text-emerald-100">
                        当前调度无明显阻塞。
                      </div>
                    )}
                  </div>
                  {providerPool.lastError ? (
                    <div className="rounded-[var(--app-radius-md)] border border-amber-400/20 bg-amber-400/10 p-4 text-sm text-amber-100">
                      <div className="flex items-center gap-2 font-semibold">
                        <AlertTriangle className="size-4" />
                        号池最近错误
                      </div>
                      <div className="mt-2 text-xs leading-5 text-amber-100/80">
                        <div>成员：{providerPool.lastErrorMember || "-"}</div>
                        <div>时间：{formatDateTime(providerPool.lastErrorAt)}</div>
                        <div className="mt-1 break-words">{providerPool.lastError}</div>
                      </div>
                    </div>
                  ) : (
                    <div className="rounded-[var(--app-radius-md)] border border-emerald-400/20 bg-emerald-400/10 p-4 text-sm text-emerald-100">
                      Provider 号池暂无成员错误。
                    </div>
                  )}
                  {providerPool.availableMembers === 0 ? (
                    <div className="rounded-[var(--app-radius-md)] border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
                      当前没有可调度号池成员。若未配置 API 接入兜底，生图会直接失败。
                    </div>
                  ) : null}
                </div>
              ) : providers.length === 0 ? (
                <div className="rounded-[var(--app-radius-md)] border border-amber-400/20 bg-amber-400/10 p-4 text-sm text-amber-100">
                  当前没有上游接入。请先到上游管理添加 gpt-image 或 gemini-banana 接入。
                </div>
              ) : (
                <div className="space-y-3">
                  {providers.map((provider) => (
                    <div
                      key={provider.id}
                      className={cn(adminSubPanelClass, "p-4")}
                    >
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-medium text-[var(--app-text-primary)]">{provider.name || provider.platform}</span>
                          <span className={cn("app-badge", provider.enabled ? "ok" : "warn")}>{provider.enabled ? "已启用" : "已禁用"}</span>
                          {provider.isDefault ? (
                            <span className="app-badge off">
                              <Star className="size-3" />
                              默认
                            </span>
                          ) : null}
                        </div>
                        <div className="grid gap-2 text-xs text-[var(--app-text-muted)] sm:grid-cols-2">
                          <span>平台：{platformLabel(provider.platform)}</span>
                          <span>模型：{provider.defaultModel || "-"}</span>
                          <span className="truncate sm:col-span-2">Base URL：{provider.baseUrl || "-"}</span>
                          <span>状态更新时间：{formatDateTime(provider.updatedAt)}</span>
                          <span>配置创建时间：{formatDateTime(provider.createdAt)}</span>
                        </div>
                        <div className="text-xs text-[var(--app-text-muted)]">
                          这里只展示配置健康摘要；新增、编辑、删除和上游连通性测试请到上游管理处理。
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </AdminPanel>

          <AdminPanel>
            <SectionTitle
              title="最近错误"
              action={
                tracker?.failed ? (
                  <span className="app-badge warn">近 {numberText(tracker.windowSeconds)} 秒 {numberText(tracker.failed)} 次</span>
                ) : (
                  <span className="app-badge ok">暂无错误</span>
                )
              }
            />
            <div className="p-4">
              {loading ? (
                <div className="py-8 text-center text-[var(--app-text-muted)]">
                  <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                  读取中
                </div>
              ) : recentFailure ? (
                <div className="rounded-[var(--app-radius-md)] border border-amber-400/20 bg-amber-400/10 p-4 text-sm text-amber-100">
                  <div className="flex items-center gap-2 font-semibold">
                    <AlertTriangle className="size-4" />
                    {recentFailure.errorCode || recentFailure.stage || "最近错误"}
                  </div>
                  <div className="mt-2 leading-6">{recentFailure.errorMessage || "-"}</div>
                  <div className="mt-3 grid gap-2 text-xs text-amber-100/70 sm:grid-cols-2">
                    <span>时间：{formatDateTime(recentFailure.createdAt)}</span>
                    <span>平台：{platformLabel(recentFailure.platform || "")}</span>
                    <span>模型：{recentFailure.model || "-"}</span>
                    <span>上游：{formatDurationMs(recentFailure.upstreamDurationMs)}</span>
                  </div>
                </div>
              ) : (
                <div className="rounded-[var(--app-radius-md)] border border-emerald-400/20 bg-emerald-400/10 p-4 text-sm text-emerald-100">
                  <div className="flex items-center gap-2 font-semibold">
                    <CheckCircle2 className="size-4" />
                    最近没有记录到失败请求
                  </div>
                </div>
              )}
            </div>
          </AdminPanel>
        </section>

        <AdminPanel className="order-3 overflow-visible">
          <SectionTitle title="Job 明细" />
          <div className="space-y-4 p-4">
            <div className="app-toolbar">
              <AppTimeSeg value={jobTimeRange} onChange={setJobTimeRange} />
              <AppSelect
                value={jobStatus}
                onChange={setJobStatus}
                options={[
                  { value: "", label: "全部状态" },
                  { value: "queued", label: "排队中" },
                  { value: "running", label: "运行中" },
                  { value: "cancel_requested", label: "取消中" },
                  { value: "cancelled", label: "已取消" },
                  { value: "succeeded", label: "成功" },
                  { value: "failed", label: "失败" },
                ]}
                placeholder="全部状态"
              />
              <AppSelect
                value={jobErrorType}
                onChange={setJobErrorType}
                options={[
                  { value: "", label: "全部错误" },
                  { value: "credit", label: "点数不足" },
                  { value: "input", label: "输入不完整" },
                  { value: "queue", label: "队列/并发" },
                  { value: "busy", label: "服务繁忙" },
                  { value: "refused", label: "内容拒绝" },
                  { value: "overload", label: "规格过高" },
                  { value: "timeout", label: "生成超时" },
                  { value: "cancelled", label: "用户取消" },
                  { value: "unknown", label: "未知错误" },
                ]}
                placeholder="全部错误"
              />
              <AppSelect
                value={jobUserId}
                onChange={setJobUserId}
                options={[
                  { value: "", label: "全部用户" },
                  ...users.map((user) => ({ value: user.id, label: user.username })),
                ]}
                placeholder="全部用户"
              />
              <AppSelect
                value={jobPlatform}
                onChange={setJobPlatform}
                options={[
                  { value: "", label: "全部平台" },
                  { value: "gpt-image", label: "gpt-image" },
                  { value: "gemini-banana", label: "gemini-banana" },
                ]}
                placeholder="全部平台"
              />
              <span className="spacer" />
              <button className="app-btn" type="button" onClick={() => void loadJobs(jobsPage.page)} disabled={jobsLoading}>
                {jobsLoading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                刷新
              </button>
            </div>

            <div className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)]">
              <table className={cn(adminTableClass, "table-fixed text-left")}>
                <colgroup>
                  <col className="w-[14%]" />
                  <col className="w-[18%]" />
                  <col className="w-[22%]" />
                  <col className="w-[22%]" />
                  <col className="w-[12%]" />
                  <col className="w-[12%]" />
                </colgroup>
                <thead className={adminTableHeadClass}>
                  <tr>
                    <th className="px-3 py-3 font-medium">状态 / 用户</th>
                    <th className="px-3 py-3 font-medium">平台 / 来源</th>
                    <th className="px-3 py-3 font-medium">来源详情</th>
                    <th className="px-3 py-3 font-medium">提示词</th>
                    <th className="px-3 py-3 font-medium">结果</th>
                    <th className="px-3 py-3 font-medium">错误</th>
                  </tr>
                </thead>
                <tbody className={adminTableBodyClass}>
                  {jobsLoading ? (
                    <tr key="jobs-loading">
                      <td colSpan={jobTableColumnCount} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
                        <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
                        读取 Job 明细
                      </td>
                    </tr>
                  ) : jobs.length === 0 ? (
                    <tr key="jobs-empty">
                      <td colSpan={jobTableColumnCount} className="px-4 py-10 text-center text-[var(--app-text-muted)]">
                        当前筛选条件下没有 Job
                      </td>
                    </tr>
                  ) : (
                    jobs.map((job) => (
                      <tr
                        key={job.id}
                        className={cn(adminTableRowClass, "cursor-pointer align-top text-[var(--app-text-secondary)]")}
                        onClick={() => setSelectedJob(job)}
                      >
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className={cn("app-badge w-fit", badgeClass(jobStatusVariant(job.status)))}>{jobStatusLabel(job.status)}</span>
                              <span className="truncate font-medium text-[var(--app-text-primary)]">
                                {usernamesByID.get(job.userId) || job.userId}
                              </span>
                            </div>
                            <div className="mt-0.5 truncate text-[11px] text-[var(--app-text-muted)]" title={`${job.userId} · ${job.stage || ""}`}>
                              {job.userId} · {job.stage || "-"}
                            </div>
                          </div>
                        </td>
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            <div className="truncate font-medium text-[var(--app-text-primary)]">
                              {platformLabel(String(job.platform || ""))} · {job.model || "-"}
                            </div>
                            <div className="mt-1 flex items-center gap-1 overflow-hidden">
                              <span className={cn("app-badge shrink-0", badgeClass(providerSourceVariant(job)))}>{providerSourceLabel(job)}</span>
                            </div>
                          </div>
                        </td>
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            <div className="truncate font-medium text-[var(--app-text-primary)]" title={job.providerGroupName || job.providerGroupId || job.providerName || job.providerId || ""}>
                              {job.providerSource === "provider_pool"
                                ? job.providerGroupName || job.providerGroupId || "-"
                                : job.providerName || job.providerId || "-"}
                            </div>
                            <div className="truncate text-[11px] text-[var(--app-text-muted)]" title={job.providerMemberName || job.providerName || job.providerMemberId || job.providerId || ""}>
                              {job.providerSource === "provider_pool"
                                ? `成员 ${job.providerMemberName || job.providerName || job.providerMemberId || job.providerId || "-"} · ${providerMatchModeLabel(job.providerGroupMatchMode) || "-"}`
                                : "API 接入"}
                            </div>
                            {dispatchStrategyLabel(job.dispatchStrategy) ? (
                              <div className="mt-1">
                                <span className={cn("app-badge shrink-0", badgeClass(dispatchStrategyVariant(job.dispatchStrategy)))}>
                                  {dispatchStrategyLabel(job.dispatchStrategy)}
                                </span>
                              </div>
                            ) : null}
                          </div>
                        </td>
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            <div className="flex min-w-0 items-center gap-2">
                              <div className="truncate text-[var(--app-text-primary)]" title={job.prompt || ""}>{job.prompt || "-"}</div>
                              {job.hasAttachment ? <span className="app-badge off shrink-0">含附件</span> : null}
                            </div>
                          </div>
                        </td>
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            <div className="truncate text-[var(--app-text-primary)]">
                              {numberText(job.actualCount)}/{numberText(job.requestedCount)} 张
                            </div>
                            <div className="mt-0.5 truncate text-xs text-[var(--app-text-muted)]">
                              {formatBytes(job.storageBytes)} · {formatDurationMs(job.totalDurationMs)}
                            </div>
                            <div className="mt-0.5 truncate text-xs text-[var(--app-text-muted)]">
                              点 {numberText(job.creditReserved)} / 退 {numberText(job.creditRefunded)}
                            </div>
                          </div>
                        </td>
                        <td className="whitespace-normal px-3 py-2">
                          <div className="min-w-0">
                            {job.userErrorType || job.errorCode || job.errorMessage ? (
                              <div className="flex min-w-0 items-center gap-1">
                                {job.userErrorType ? (
                                  <span className="app-badge fail shrink-0">{userErrorTypeLabel(job.userErrorType) || job.userErrorType}</span>
                                ) : null}
                                <span className="truncate text-xs text-rose-300" title={job.errorMessage || job.errorCode || ""}>
                                  {job.userErrorMessage || job.errorCode || job.errorMessage || "-"}
                                </span>
                              </div>
                            ) : (
                              <div className="mt-0.5 text-xs text-[var(--app-text-muted)]">-</div>
                            )}
                            <div className="mt-0.5 truncate text-xs text-[var(--app-text-muted)]">
                              {formatDateTime(job.finishedAt || job.updatedAt)}
                            </div>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            <div className="app-pager">
              <span className="info">共 {numberText(jobsPage.total)} 条 · 每页 {jobsPage.pageSize} 条</span>
              <div className="pages">
                <button type="button" disabled={jobsLoading || jobsPage.page <= 1} onClick={() => void loadJobs(Math.max(1, jobsPage.page - 1))}>上一页</button>
                <button type="button" disabled={jobsLoading || jobsPage.page * jobsPage.pageSize >= jobsPage.total} onClick={() => void loadJobs(jobsPage.page + 1)}>下一页</button>
              </div>
            </div>
          </div>
        </AdminPanel>

        <AdminPanel className="order-5">
          <SectionTitle title="说明" />
          <div className="grid gap-3 p-4 text-sm text-[var(--app-text-secondary)] sm:grid-cols-2 xl:grid-cols-4">
            <div className={cn(adminSubPanelClass, "p-4")}>
              <Server className="mb-2 size-4 text-sky-300" />
              准入队列控制新业务生图请求进入上游 API 的并发和排队。
            </div>
            <div className={cn(adminSubPanelClass, "p-4")}>
              <Clock3 className="mb-2 size-4 text-amber-300" />
              排队满或超时会提示用户稍后使用，不会进入扣点生成流程。
            </div>
            <div className={cn(adminSubPanelClass, "p-4")}>
              <Link2 className="mb-2 size-4 text-emerald-300" />
              上游健康摘要只读展示配置状态，不在运维页发起增删改或测试请求。
            </div>
            <div className={cn(adminSubPanelClass, "p-4")}>
              <AlertTriangle className="mb-2 size-4 text-indigo-300" />
              最近错误来自 tracker 记录，用于定位准入、扣点、上游和持久化阶段的问题。
            </div>
          </div>
        </AdminPanel>
        <JobDetailDrawer job={selectedJob} username={selectedJobUsername} onClose={() => setSelectedJob(null)} />
    </AdminPage>
  );
}
