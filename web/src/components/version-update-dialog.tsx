"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ExternalLink, Loader2, RefreshCw, RotateCw, ServerCrash } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  fetchSystemUpdateStatus,
  fetchVersionInfo,
  startSystemUpdate,
  type SystemUpdateJob,
  type SystemUpdateStatus,
} from "@/lib/api";
import { cn } from "@/lib/utils";

const updateStateKey = "image-studio:update:pending";
const repositoryUrl = "https://github.com/your-org/image-studio";

type UpdateStage = "idle" | "checking" | "updating" | "restarting" | "complete" | "failed";

type VersionUpdateDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  versionLabel: string;
  onVersionChange?: (version: string) => void;
};

type PendingUpdateState = {
  startedAt: number;
  jobId?: string;
};

function readPendingUpdate(): PendingUpdateState | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(updateStateKey);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as PendingUpdateState;
    if (!parsed.startedAt || Date.now() - parsed.startedAt > 15 * 60 * 1000) {
      window.localStorage.removeItem(updateStateKey);
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function writePendingUpdate(job?: SystemUpdateJob | null) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    updateStateKey,
    JSON.stringify({
      startedAt: Date.now(),
      jobId: job?.id,
    } satisfies PendingUpdateState),
  );
}

function clearPendingUpdate() {
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(updateStateKey);
  }
}

async function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function formatJobStatus(job?: SystemUpdateJob | null) {
  if (!job) {
    return "等待更新任务";
  }
  if (job.status === "running") {
    if (job.phase === "pull") return "正在拉取最新镜像";
    if (job.phase === "recreate") return "正在重建服务";
    if (job.phase === "inspect") return "正在确认容器状态";
    return "更新任务运行中";
  }
  if (job.status === "succeeded") return "更新完成";
  if (job.status === "failed") return job.error || "更新失败";
  return job.status || "等待更新任务";
}

function formatErrorDetail(unavailableReason: string, error: string, job?: SystemUpdateJob | null) {
  return unavailableReason || error || job?.error || "请查看 updater 容器日志";
}

function formatVersionDisplay(value: string) {
  const normalized = String(value || "").trim();
  if (!normalized) return "";
  return /^v/i.test(normalized) ? normalized : `v${normalized}`;
}

export function VersionUpdateDialog({
  open,
  onOpenChange,
  versionLabel,
  onVersionChange,
}: VersionUpdateDialogProps) {
  const [stage, setStage] = useState<UpdateStage>("idle");
  const [status, setStatus] = useState<SystemUpdateStatus | null>(null);
  const [error, setError] = useState("");
  const [polling, setPolling] = useState(false);
  const [checking, setChecking] = useState(false);
  const [latestVersion, setLatestVersion] = useState(versionLabel);
  const [hasUpdate, setHasUpdate] = useState<boolean | null>(null);
  const [versionWarning, setVersionWarning] = useState("");
  const pollStartedRef = useRef(false);

  const job = status?.job || null;
  const unavailableReason = status && !status.enabled ? status.reason || "当前部署未启用 Web 一键更新" : "";
  const canStart = hasUpdate === true && !unavailableReason && stage !== "updating" && stage !== "restarting" && stage !== "checking";
  const subtitle = useMemo(() => {
    if (stage === "restarting") return "服务正在重启，页面会在恢复后自动刷新";
    if (stage === "complete") return "服务已恢复，可以刷新页面使用新版本";
    if (stage === "failed") return "更新失败，请查看下方错误详情";
    if (versionWarning) return "版本检测失败，请稍后重试";
    if (hasUpdate === true) return latestVersion ? `最新版本：${latestVersion}` : "发现新版本";
    if (hasUpdate === false) return "已是最新版本";
    return unavailableReason || formatJobStatus(job);
  }, [hasUpdate, job, latestVersion, stage, unavailableReason, versionWarning]);

  useEffect(() => {
    if (!open) return;
    void refreshStatus();
    void refreshVersion(false);
  }, [open]);

  useEffect(() => {
    const pending = readPendingUpdate();
    if (!pending || pollStartedRef.current) {
      return;
    }
    pollStartedRef.current = true;
    setStage("restarting");
    onOpenChange(true);
    void waitForRestartThenRecovery();
  }, [onOpenChange]);

  async function refreshStatus() {
    setChecking(true);
    setError("");
    try {
      const payload = await fetchSystemUpdateStatus();
      setStatus(payload);
      if (payload.job?.status === "running") {
        setStage("updating");
      } else if (payload.job?.status === "failed") {
        setError(payload.job.error || "更新失败");
        setStage("failed");
      } else if (stage === "checking") {
        setStage("idle");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "读取更新状态失败";
      setError(message);
      setStage("failed");
    } finally {
      setChecking(false);
    }
  }

  async function refreshVersion(force = true) {
    setChecking(true);
    setError("");
    setVersionWarning("");
    try {
      const payload = await fetchVersionInfo(force);
      const nextVersion = payload.latest_version || payload.version || versionLabel;
      setLatestVersion(formatVersionDisplay(nextVersion));
      setHasUpdate(Boolean(payload.has_update));
      setVersionWarning(payload.warning || "");
      onVersionChange?.(payload.version || payload.current_version || versionLabel);
      if (force) {
        toast.success("版本信息已刷新");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "版本信息刷新失败";
      setError(message);
      toast.error(message);
    } finally {
      setChecking(false);
    }
  }

  async function handleUpdate() {
    if (!canStart) return;
    setStage("updating");
    setError("");
    try {
      const payload = await startSystemUpdate();
      const nextStatus = payload.status || {
        enabled: true,
        job: payload.job || null,
      };
      setStatus(nextStatus);
      writePendingUpdate(payload.job);
      void waitForRestartThenRecovery();
    } catch (err) {
      const message = err instanceof Error ? err.message : "启动更新失败";
      setError(message);
      setStage("failed");
      toast.error(message);
    }
  }

  async function waitForRestartThenRecovery() {
    if (polling) return;
    setPolling(true);
    setStage("updating");

    let sawRestartWindow = false;
    for (let attempt = 0; attempt < 180; attempt += 1) {
      try {
        const payload = await fetchSystemUpdateStatus();
        setStatus(payload);
        const currentJob = payload.job;
        if (currentJob?.status === "failed") {
          clearPendingUpdate();
          setStage("failed");
          setError(currentJob.error || "更新失败");
          setPolling(false);
          return;
        }
        if (currentJob?.phase === "recreate" || currentJob?.status === "succeeded") {
          sawRestartWindow = true;
          setStage("restarting");
        }
      } catch {
        sawRestartWindow = true;
        setStage("restarting");
      }

      if (sawRestartWindow && (await probeServiceRecovered())) {
        setPolling(false);
        return;
      }
      await wait(1000);
    }

    setPolling(false);
    setStage("failed");
    setError("服务恢复超时，请稍后手动刷新页面或查看服务器日志");
  }

  async function probeServiceRecovered() {
    try {
      const response = await fetch("/health", { cache: "no-store" });
      if (!response.ok) {
        return false;
      }
      const version = await fetchVersionInfo(true);
      clearPendingUpdate();
      setLatestVersion(version.version || versionLabel);
      onVersionChange?.(version.version || versionLabel);
      setStage("complete");
      window.setTimeout(() => window.location.reload(), 1200);
      return true;
    } catch {
      return false;
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,420px)] rounded-[20px] p-0" showCloseButton={stage !== "restarting"}>
        <DialogHeader className="border-b border-stone-200 px-5 py-4 dark:border-[var(--studio-border)]">
          <DialogTitle className="text-base">当前版本</DialogTitle>
          <DialogDescription>查看并管理 Docker Compose 更新</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 px-5 pb-5 pt-3">
          <div className="text-center">
            <div className="inline-flex items-center gap-2">
              <span className="text-3xl font-semibold tracking-normal text-stone-950 dark:text-[var(--studio-text-strong)]">
                {versionLabel}
              </span>
              {(!versionWarning && hasUpdate === false) || stage === "complete" ? (
                <span className="inline-flex size-6 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">
                  <Check className="size-3.5" />
                </span>
              ) : null}
            </div>
            <p className="mt-1 text-sm text-stone-500 dark:text-[var(--studio-text-muted)]">
              {subtitle}
            </p>
          </div>

          {stage === "failed" || unavailableReason ? (
            <div className="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-3 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
              <span className="mt-0.5 inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-red-100 dark:bg-red-950">
                <ServerCrash className="size-4" />
              </span>
              <div className="min-w-0 overflow-hidden">
                <p className="text-sm font-medium">{unavailableReason ? "一键更新不可用" : "更新失败"}</p>
                <p className="mt-0.5 max-h-40 overflow-y-auto whitespace-pre-wrap break-all text-left text-xs opacity-80">
                  {formatErrorDetail(unavailableReason, error, job)}
                </p>
              </div>
            </div>
          ) : (
            <div
              className={cn(
                "flex items-center gap-3 rounded-lg border p-3",
                stage === "complete"
                  ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300"
                  : versionWarning
                    ? "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300"
                    : hasUpdate
                    ? "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300"
                    : "border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-300",
              )}
            >
              <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-white/70 dark:bg-black/20">
                {stage === "updating" || stage === "restarting" ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Check className="size-4" />
                )}
              </span>
              <div className="min-w-0">
                <p className="text-sm font-medium">
                  {stage === "complete"
                    ? "更新完成"
                    : versionWarning
                      ? "版本检测失败"
                      : hasUpdate
                        ? "发现新版本"
                        : hasUpdate === false
                          ? "已是最新版本"
                          : formatJobStatus(job)}
                </p>
                <p className="mt-0.5 text-xs opacity-80">
                  {stage === "restarting"
                    ? "正在等待新容器通过健康检查"
                    : hasUpdate
                      ? `可更新到 ${latestVersion || "最新版本"}`
                      : versionWarning || "没有检测到可用更新"}
                </p>
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-2">
            <Button variant="outline" type="button" onClick={refreshStatus} disabled={checking || stage === "restarting"}>
              {checking ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              刷新状态
            </Button>
            <Button variant="outline" type="button" onClick={() => refreshVersion(true)} disabled={checking || stage === "restarting"}>
              <RefreshCw className="size-4" />
              刷新版本
            </Button>
          </div>

          <Button className="w-full" type="button" onClick={handleUpdate} disabled={!canStart}>
            {stage === "updating" || stage === "restarting" ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <RotateCw className="size-4" />
            )}
            {stage === "restarting"
              ? "正在重启..."
              : stage === "updating"
                ? "正在更新..."
                : versionWarning
                  ? "版本检测失败"
                  : hasUpdate === false
                    ? "已是最新版本"
                    : "立即更新"}
          </Button>

          <a
            href={repositoryUrl}
            target="_blank"
            rel="noreferrer"
            className="flex items-center justify-center gap-1 text-xs text-stone-500 transition hover:text-stone-800 dark:text-[var(--studio-text-muted)] dark:hover:text-[var(--studio-text)]"
          >
            查看 GitHub 仓库
            <ExternalLink className="size-3" />
          </a>
        </div>
      </DialogContent>
    </Dialog>
  );
}
