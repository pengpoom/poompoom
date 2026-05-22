"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, Archive, Database, FileQuestion, FolderSearch, LoaderCircle, RefreshCw, Wrench } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import {
  adminSubPanelClass,
  adminTableBodyClass,
  adminTableClass,
  adminTableHeadClass,
  adminTableRowClass,
} from "@/components/admin-styles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { backfillBusinessStorageAssets, fetchBusinessStorageReport, type BusinessStorageReport } from "@/lib/api";
import { cn } from "@/lib/utils";

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  const units = ["KB", "MB", "GB", "TB"];
  let amount = bytes / 1024;
  let unitIndex = 0;
  while (amount >= 1024 && unitIndex < units.length - 1) {
    amount /= 1024;
    unitIndex += 1;
  }
  return `${amount >= 10 ? amount.toFixed(1) : amount.toFixed(2)} ${units[unitIndex]}`;
}

function numberText(value: number | undefined) {
  return Number(value || 0).toLocaleString();
}

function shortPath(value: string | undefined) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "-";
  }
  if (raw.length <= 72) {
    return raw;
  }
  return `...${raw.slice(-69)}`;
}

function reportIssueCount(report: BusinessStorageReport | null) {
  if (!report) {
    return 0;
  }
  return report.summary.missingFiles + report.summary.orphanFiles + report.summary.legacyReferencedFiles + report.summary.brokenAssets;
}

type SectionProps = {
  title: string;
  count: number;
  children: ReactNode;
  action?: ReactNode;
};

function IssueSection({ title, count, children, action }: SectionProps) {
  return (
    <AdminPanel>
      <AdminSectionTitle
        title={title}
        action={
          <div className="flex items-center gap-2">
            {action}
            <Badge variant={count > 0 ? "warning" : "success"}>{count}</Badge>
          </div>
        }
      />
      {children}
    </AdminPanel>
  );
}

export default function StoragePage() {
  const [report, setReport] = useState<BusinessStorageReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [backfilling, setBackfilling] = useState(false);

  const issueCount = useMemo(() => reportIssueCount(report), [report]);

  const loadReport = async () => {
    setLoading(true);
    try {
      const payload = await fetchBusinessStorageReport();
      setReport(payload);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取存储报告失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadReport();
  }, []);

  const handleBackfill = async () => {
    setBackfilling(true);
    try {
      const payload = await backfillBusinessStorageAssets();
      setReport(payload.report);
      toast.success(`已补录 ${numberText(payload.result.backfilled)} 条资产记录`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "补录资产记录失败");
    } finally {
      setBackfilling(false);
    }
  };

  const summary = report?.summary;

  return (
    <AdminPage>
        <AdminHeader
          title="存储检查"
          description="检查数据库资产记录、磁盘文件和旧图片引用的一致性。"
          actions={
            <Button type="button" variant="outline" onClick={() => void loadReport()} disabled={loading || backfilling}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
            </Button>
          }
        />

        <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { label: "问题总数", value: numberText(issueCount), icon: AlertTriangle, color: issueCount > 0 ? "text-amber-300" : "text-emerald-300" },
            { label: "资产记录", value: numberText(summary?.assetFiles), icon: Database, color: "text-[var(--app-text-primary)]" },
            { label: "磁盘文件", value: numberText(summary?.diskFiles), icon: Archive, color: "text-violet-300" },
            { label: "孤儿占用", value: formatBytes(summary?.orphanBytes), icon: FileQuestion, color: "text-[var(--app-text-muted)]" },
          ].map((item) => {
            return <AdminStatCard key={item.label} {...item} />;
          })}
        </section>

        <AdminPanel className="p-4 text-sm text-[var(--app-text-secondary)]">
          <div className="mb-2 flex items-center gap-2 font-medium text-[var(--app-text-primary)]">
            <FolderSearch className="size-4" />
            扫描目录
          </div>
          <div className="grid gap-1">
            {(report?.directories || []).map((dir) => (
              <div key={dir} className={cn(adminSubPanelClass, "truncate px-3 py-2 font-mono text-xs")} title={dir}>
                {dir}
              </div>
            ))}
            {!loading && (report?.directories || []).length === 0 ? <div className="text-[var(--app-text-muted)]">暂无目录</div> : null}
          </div>
        </AdminPanel>

        {loading ? (
          <AdminPanel className="px-4 py-12 text-center text-[var(--app-text-muted)]">
            <LoaderCircle className="mx-auto mb-2 size-5 animate-spin" />
            读取中
          </AdminPanel>
        ) : (
          <>
            <IssueSection title="缺失文件" count={report?.summary.missingFiles || 0}>
              <StorageTable emptyText="没有资产表存在但磁盘缺失的文件">
                {(report?.missingFiles || []).map((item) => (
                  <tr key={`${item.fileName}:${item.generationId}`} className={adminTableRowClass}>
                    <td className="px-4 py-3 font-medium text-[var(--app-text-primary)]">{item.fileName}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.userId || "-"}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.generationId || "-"}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{formatBytes(item.sizeBytes)}</td>
                    <td className="px-4 py-3 font-mono text-xs text-[var(--app-text-muted)]" title={item.expectedPath}>{shortPath(item.expectedPath)}</td>
                  </tr>
                ))}
              </StorageTable>
            </IssueSection>

            <IssueSection title="孤儿文件" count={report?.summary.orphanFiles || 0}>
              <StorageTable emptyText="没有未被数据库引用的业务图片">
                {(report?.orphanFiles || []).map((item) => (
                  <tr key={item.path} className={adminTableRowClass}>
                    <td className="px-4 py-3 font-medium text-[var(--app-text-primary)]">{item.fileName}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">-</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">-</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{formatBytes(item.sizeBytes)}</td>
                    <td className="px-4 py-3 font-mono text-xs text-[var(--app-text-muted)]" title={item.path}>{shortPath(item.path)}</td>
                  </tr>
                ))}
              </StorageTable>
            </IssueSection>

            <IssueSection
              title="旧引用待补录"
              count={report?.summary.legacyReferencedFiles || 0}
              action={(report?.summary.legacyReferencedFiles || 0) > 0 ? (
                <Button type="button" size="sm" variant="outline" onClick={() => void handleBackfill()} disabled={backfilling}>
                  {backfilling ? <LoaderCircle className="size-4 animate-spin" /> : <Wrench className="size-4" />}
                  补录资产记录
                </Button>
              ) : null}
            >
              <StorageTable emptyText="没有缺少资产表记录的旧图片引用">
                {(report?.legacyReferencedFiles || []).map((item) => (
                  <tr key={`${item.fileName}:${item.generationId}`} className={adminTableRowClass}>
                    <td className="px-4 py-3 font-medium text-[var(--app-text-primary)]">{item.fileName}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.userId}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.generationId}</td>
                    <td className="px-4 py-3">
                      <Badge variant={item.onDisk ? "success" : "warning"}>{item.onDisk ? "文件存在" : "文件缺失"}</Badge>
                    </td>
                    <td className="px-4 py-3 text-[var(--app-text-muted)]">-</td>
                  </tr>
                ))}
              </StorageTable>
            </IssueSection>

            <IssueSection title="坏资产记录" count={report?.summary.brokenAssets || 0}>
              <StorageTable emptyText="没有指向缺失生成记录的资产">
                {(report?.brokenAssets || []).map((item) => (
                  <tr key={`${item.fileName}:${item.generationId}`} className={adminTableRowClass}>
                    <td className="px-4 py-3 font-medium text-[var(--app-text-primary)]">{item.fileName}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.userId}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.generationId}</td>
                    <td className="px-4 py-3 text-[var(--app-text-secondary)]">{item.reason}</td>
                    <td className="px-4 py-3 text-[var(--app-text-muted)]">-</td>
                  </tr>
                ))}
              </StorageTable>
            </IssueSection>
          </>
        )}
    </AdminPage>
  );
}

function StorageTable({ children, emptyText }: { children: ReactNode; emptyText: string }) {
  const rows = Array.isArray(children) ? children.filter(Boolean) : children;
  const hasRows = Array.isArray(rows) ? rows.length > 0 : Boolean(rows);
  return (
    <div className="overflow-x-auto">
      <table className={adminTableClass}>
        <thead className={adminTableHeadClass}>
          <tr>
            <th className="px-4 py-3">文件</th>
            <th className="px-4 py-3">用户</th>
            <th className="px-4 py-3">生成记录</th>
            <th className="px-4 py-3">状态 / 大小</th>
            <th className="px-4 py-3">路径</th>
          </tr>
        </thead>
        <tbody className={adminTableBodyClass}>
          {hasRows ? rows : (
            <tr>
              <td colSpan={5} className="px-4 py-8 text-center text-[var(--app-text-muted)]">
                {emptyText}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
