"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, Archive, Database, FileQuestion, FolderSearch, LoaderCircle, RefreshCw, Wrench } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage, AdminPanel, AdminSectionTitle, AdminStatCard } from "@/components/admin-layout";
import { backfillBusinessStorageAssets, fetchBusinessStorageReport, type BusinessStorageReport } from "@/lib/api";

function formatBytes(value: number | undefined) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) return `${bytes} B`;
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
  if (!raw) return "-";
  if (raw.length <= 72) return raw;
  return `...${raw.slice(-69)}`;
}

function reportIssueCount(report: BusinessStorageReport | null) {
  if (!report) return 0;
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
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            {action}
            <span className={`app-badge ${count > 0 ? "warn" : "ok"}`}>{count}</span>
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
        icon={Database}
        actions={
          <button className="app-btn" type="button" onClick={() => void loadReport()} disabled={loading || backfilling}>
            {loading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            刷新
          </button>
        }
      />

      <section className="app-stats">
        <AdminStatCard label="问题总数" value={numberText(issueCount)} icon={AlertTriangle} color={issueCount > 0 ? "text-amber-300" : "text-emerald-300"} />
        <AdminStatCard label="资产记录" value={numberText(summary?.assetFiles)} icon={Database} color="text-cyan-300" />
        <AdminStatCard label="磁盘文件" value={numberText(summary?.diskFiles)} icon={Archive} color="text-violet-300" />
        <AdminStatCard label="孤儿占用" value={formatBytes(summary?.orphanBytes)} icon={FileQuestion} color="text-amber-300" />
      </section>

      <AdminPanel>
        <div style={{ padding: 16 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)", marginBottom: 8 }}>
            <FolderSearch className="size-4" />
            扫描目录
          </div>
          <div style={{ display: "grid", gap: 6 }}>
            {(report?.directories || []).map((dir) => (
              <div
                key={dir}
                style={{
                  padding: "8px 12px",
                  borderRadius: 8,
                  border: "1px solid var(--app-border)",
                  background: "var(--app-bg-surface)",
                  fontFamily: "ui-monospace, monospace",
                  fontSize: 11.5,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={dir}
              >
                {dir}
              </div>
            ))}
            {!loading && (report?.directories || []).length === 0 ? (
              <div style={{ fontSize: 12, color: "var(--app-text-muted)" }}>暂无目录</div>
            ) : null}
          </div>
        </div>
      </AdminPanel>

      {loading ? (
        <AdminPanel>
          <div style={{ padding: "48px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
            <LoaderCircle className="size-5 animate-spin" style={{ display: "inline-block", marginBottom: 8 }} />
            <div>读取中</div>
          </div>
        </AdminPanel>
      ) : (
        <>
          <IssueSection title="缺失文件" count={report?.summary.missingFiles || 0}>
            <StorageTable emptyText="没有资产表存在但磁盘缺失的文件">
              {(report?.missingFiles || []).map((item) => (
                <tr key={`${item.fileName}:${item.generationId}`}>
                  <td style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.fileName}</td>
                  <td>{item.userId || "-"}</td>
                  <td>{item.generationId || "-"}</td>
                  <td>{formatBytes(item.sizeBytes)}</td>
                  <td style={{ fontFamily: "ui-monospace, monospace", fontSize: 11, color: "var(--app-text-muted)" }} title={item.expectedPath}>{shortPath(item.expectedPath)}</td>
                </tr>
              ))}
            </StorageTable>
          </IssueSection>

          <IssueSection title="孤儿文件" count={report?.summary.orphanFiles || 0}>
            <StorageTable emptyText="没有未被数据库引用的业务图片">
              {(report?.orphanFiles || []).map((item) => (
                <tr key={item.path}>
                  <td style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.fileName}</td>
                  <td>-</td>
                  <td>-</td>
                  <td>{formatBytes(item.sizeBytes)}</td>
                  <td style={{ fontFamily: "ui-monospace, monospace", fontSize: 11, color: "var(--app-text-muted)" }} title={item.path}>{shortPath(item.path)}</td>
                </tr>
              ))}
            </StorageTable>
          </IssueSection>

          <IssueSection
            title="旧引用待补录"
            count={report?.summary.legacyReferencedFiles || 0}
            action={(report?.summary.legacyReferencedFiles || 0) > 0 ? (
              <button className="app-btn" type="button" onClick={() => void handleBackfill()} disabled={backfilling} style={{ height: 30, padding: "0 12px", fontSize: 12 }}>
                {backfilling ? <LoaderCircle className="size-3.5 animate-spin" /> : <Wrench className="size-3.5" />}
                补录资产记录
              </button>
            ) : null}
          >
            <StorageTable emptyText="没有缺少资产表记录的旧图片引用">
              {(report?.legacyReferencedFiles || []).map((item) => (
                <tr key={`${item.fileName}:${item.generationId}`}>
                  <td style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.fileName}</td>
                  <td>{item.userId}</td>
                  <td>{item.generationId}</td>
                  <td>
                    <span className={`app-badge ${item.onDisk ? "ok" : "warn"}`}>{item.onDisk ? "文件存在" : "文件缺失"}</span>
                  </td>
                  <td style={{ color: "var(--app-text-muted)" }}>-</td>
                </tr>
              ))}
            </StorageTable>
          </IssueSection>

          <IssueSection title="坏资产记录" count={report?.summary.brokenAssets || 0}>
            <StorageTable emptyText="没有指向缺失生成记录的资产">
              {(report?.brokenAssets || []).map((item) => (
                <tr key={`${item.fileName}:${item.generationId}`}>
                  <td style={{ fontWeight: 500, color: "var(--app-text-primary)" }}>{item.fileName}</td>
                  <td>{item.userId}</td>
                  <td>{item.generationId}</td>
                  <td>{item.reason}</td>
                  <td style={{ color: "var(--app-text-muted)" }}>-</td>
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
    <div className="app-table-wrap">
      <table className="app-table">
        <thead>
          <tr>
            <th>文件</th>
            <th>用户</th>
            <th>生成记录</th>
            <th>状态 / 大小</th>
            <th>路径</th>
          </tr>
        </thead>
        <tbody>
          {hasRows ? rows : (
            <tr>
              <td colSpan={5} style={{ padding: "32px 16px", textAlign: "center", color: "var(--app-text-muted)" }}>
                {emptyText}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
