"use client";

import { useEffect, useMemo, useState } from "react";
import { Download, RefreshCw, ShieldCheck, TriangleAlert, XCircle } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel } from "@/components/admin-layout";
import { adminSubPanelClass } from "@/components/admin-styles";
import { downloadDiagnosticsExport, fetchStartupCheck, type StartupCheckResponse } from "@/lib/api";
import { cn } from "@/lib/utils";

function statusBadgeClass(status: string) {
  if (status === "pass") return "ok";
  if (status === "warn") return "warn";
  if (status === "fail") return "fail";
  return "off";
}

function statusLabel(status: string) {
  if (status === "pass") {
    return "通过";
  }
  if (status === "warn") {
    return "警告";
  }
  if (status === "fail") {
    return "失败";
  }
  return status || "未知";
}

export default function StartupCheckPage() {
  const [isLoading, setIsLoading] = useState(true);
  const [isDownloading, setIsDownloading] = useState(false);
  const [result, setResult] = useState<StartupCheckResponse | null>(null);

  const runCheck = async () => {
    setIsLoading(true);
    try {
      const data = await fetchStartupCheck();
      setResult(data);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "启动体检失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void runCheck();
  }, []);

  const headerIcon = useMemo(() => {
    if (!result || result.overall === "pass") {
      return <ShieldCheck className="size-5" />;
    }
    if (result.overall === "warn") {
      return <TriangleAlert className="size-5" />;
    }
    return <XCircle className="size-5" />;
  }, [result]);

  const handleDownloadDiagnostics = async () => {
    setIsDownloading(true);
    try {
      const { blob, fileName } = await downloadDiagnosticsExport();
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = fileName;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
      toast.success("诊断包已下载");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "诊断包下载失败");
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <AdminPage>
        <AdminHeader
          title="启动体检"
          description={
            <>
              检查服务和 API 接入可用性，方便在正式使用前快速排障。
              {result ? <span className="mt-2 block text-xs text-[var(--app-text-muted)]">{result.summaryText}</span> : null}
            </>
          }
          actions={
            <>
              <button
                type="button"
                className="app-btn"
                onClick={() => void runCheck()}
                disabled={isLoading}
              >
                <RefreshCw className={isLoading ? "size-4 animate-spin" : "size-4"} />
                重新检测
              </button>
              <button
                type="button"
                className="app-btn"
                onClick={() => void handleDownloadDiagnostics()}
                disabled={isDownloading}
              >
                <Download className={isDownloading ? "size-4 animate-pulse" : "size-4"} />
                导出诊断包
              </button>
            </>
          }
        >
          <div className="mb-3 inline-flex size-12 shrink-0 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            {headerIcon}
          </div>
        </AdminHeader>

        <AdminPanel className="space-y-4 p-6">
            {result?.checks?.map((item) => (
              <div key={item.key} className={cn(adminSubPanelClass, "p-4")}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-sm font-semibold text-[var(--app-text-primary)]">{item.label}</div>
                  <div className="flex items-center gap-2">
                    <span className={`app-badge ${statusBadgeClass(item.status)}`}>{statusLabel(item.status)}</span>
                    <span className="text-xs text-[var(--app-text-muted)]">{item.durationMs} ms</span>
                  </div>
                </div>
                <div className="mt-2 text-sm text-[var(--app-text-secondary)]">{item.detail}</div>
                {item.hint ? <div className="mt-1 text-xs text-[var(--app-text-muted)]">建议：{item.hint}</div> : null}
              </div>
            ))}
            {!isLoading && (!result || result.checks.length === 0) ? (
              <div className={cn(adminSubPanelClass, "p-4 text-sm text-[var(--app-text-muted)]")}>
                暂无检测结果，请点击“重新检测”。
              </div>
            ) : null}
        </AdminPanel>
    </AdminPage>
  );
}
