"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { CalendarClock, ImageIcon, LoaderCircle } from "lucide-react";
import { Link } from "react-router-dom";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import { AppModal } from "@/components/app-controls";
import webConfig from "@/constants/common-env";
import {
  fetchBusinessAssets,
  type BusinessImageAsset,
  type PaginationMeta,
} from "@/lib/api";

const assetPageSize = 8;

function formatAssetTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function buildAssetURL(asset: BusinessImageAsset) {
  const raw = String(asset.url || "").trim();
  if (!raw) {
    return "";
  }
  if (/^(data:|https?:\/\/)/i.test(raw)) {
    return raw;
  }
  const base = webConfig.apiUrl.replace(/\/$/, "");
  return `${base}${raw.startsWith("/") ? raw : `/${raw}`}`;
}

function assetDisplayName(asset: BusinessImageAsset) {
  return asset.conversation_title || "图片资产";
}

function mergeAssets(current: BusinessImageAsset[], incoming: BusinessImageAsset[]) {
  const seen = new Set(current.map((item) => item.id || item.file_name));
  const next = [...current];
  incoming.forEach((item) => {
    const key = item.id || item.file_name;
    if (!key || seen.has(key)) {
      return;
    }
    seen.add(key);
    next.push(item);
  });
  return next;
}

export default function AssetsPage() {
  const [assets, setAssets] = useState<BusinessImageAsset[]>([]);
  const [page, setPage] = useState<PaginationMeta>({
    page: 1,
    pageSize: assetPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [selectedAsset, setSelectedAsset] = useState<BusinessImageAsset | null>(null);

  const hasMore = assets.length < page.total;

  const loadAssets = useCallback(async (targetPage: number) => {
    const isFirstPage = targetPage <= 1;
    if (isFirstPage) {
      setLoading(true);
    } else {
      setLoadingMore(true);
    }
    try {
      const payload = await fetchBusinessAssets({
        page: targetPage,
        pageSize: assetPageSize,
      });
      setPage(payload.page);
      setAssets((current) =>
        isFirstPage ? payload.items : mergeAssets(current, payload.items),
      );
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "读取资产失败");
    } finally {
      if (isFirstPage) {
        setLoading(false);
      } else {
        setLoadingMore(false);
      }
    }
  }, []);

  useEffect(() => {
    void loadAssets(1);
  }, [loadAssets]);

  const selectedAssetURL = useMemo(
    () => (selectedAsset ? buildAssetURL(selectedAsset) : ""),
    [selectedAsset],
  );

  return (
    <AdminPage>
      <AdminHeader
        title="资产"
        description="当前账号生成成功的图片资产。"
        icon={ImageIcon}
      />

      {loading ? (
        <div
          className="app-panel"
          style={{ minHeight: 360, display: "grid", placeItems: "center", color: "var(--app-text-secondary)" }}
        >
          <div style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
            <LoaderCircle className="size-5 animate-spin" />
            正在读取资产
          </div>
        </div>
      ) : error && assets.length === 0 ? (
        <div className="app-panel" style={{ padding: 24, fontSize: 13, color: "#fb7185" }}>{error}</div>
      ) : assets.length === 0 ? (
        <div
          className="app-panel"
          style={{ minHeight: 360, display: "grid", placeItems: "center", padding: 24, textAlign: "center" }}
        >
          <div>
            <ImageIcon className="size-10" style={{ margin: "0 auto", color: "var(--app-text-muted)" }} />
            <h2 style={{ marginTop: 16, fontSize: 16, fontWeight: 600, color: "var(--app-text-primary)" }}>还没有图片资产</h2>
            <p style={{ marginTop: 8, fontSize: 13, color: "var(--app-text-secondary)" }}>生成成功的图片会自动出现在这里。</p>
            <Link
              to="/image/history"
              className="app-btn-primary"
              style={{ marginTop: 20, display: "inline-flex" }}
            >
              开始生成
            </Link>
          </div>
        </div>
      ) : (
        <section style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <div
            style={{
              display: "grid",
              gap: 14,
              gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
            }}
          >
            {assets.map((asset) => {
              const imageURL = buildAssetURL(asset);
              return (
                <button
                  type="button"
                  key={asset.id || asset.file_name}
                  className="app-panel"
                  style={{
                    overflow: "hidden",
                    padding: 0,
                    textAlign: "left",
                    cursor: "pointer",
                    transition: "border-color 0.2s ease, background 0.2s ease",
                  }}
                  onClick={() => setSelectedAsset(asset)}
                >
                  <div style={{ aspectRatio: "1 / 1", background: "var(--app-bg-sidebar)" }}>
                    {imageURL ? (
                      <img
                        src={imageURL}
                        alt={assetDisplayName(asset)}
                        loading="lazy"
                        style={{ width: "100%", height: "100%", objectFit: "cover" }}
                      />
                    ) : (
                      <div style={{ display: "grid", placeItems: "center", width: "100%", height: "100%", color: "var(--app-text-muted)" }}>
                        <ImageIcon className="size-8" />
                      </div>
                    )}
                  </div>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      height: 48,
                      padding: "0 12px",
                      fontSize: 12,
                      color: "var(--app-text-secondary)",
                    }}
                  >
                    <CalendarClock className="size-3" style={{ marginRight: 8, flexShrink: 0, color: "var(--app-text-muted)" }} />
                    <time style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {formatAssetTime(asset.created_at)}
                    </time>
                  </div>
                </button>
              );
            })}
          </div>

          <div style={{ display: "flex", justifyContent: "center" }}>
            {error ? (
              <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 12, textAlign: "center" }}>
                <p style={{ fontSize: 13, color: "#fb7185" }}>{error}</p>
                <button
                  type="button"
                  className="app-btn"
                  onClick={() => void loadAssets(page.page + 1)}
                  disabled={loadingMore}
                >
                  重新加载
                </button>
              </div>
            ) : hasMore ? (
              <button
                type="button"
                className="app-btn"
                onClick={() => void loadAssets(page.page + 1)}
                disabled={loadingMore}
              >
                {loadingMore ? (
                  <>
                    <LoaderCircle className="size-4 animate-spin" />
                    正在加载
                  </>
                ) : (
                  "加载更多"
                )}
              </button>
            ) : (
              <p style={{ fontSize: 12, color: "var(--app-text-muted)" }}>已显示全部 {page.total} 张图片</p>
            )}
          </div>
        </section>
      )}

      <AppModal
        open={selectedAsset !== null}
        onClose={() => setSelectedAsset(null)}
        title="图片详情"
      >
        {selectedAsset ? (
          <div style={{ display: "grid", gap: 16 }}>
            <div
              style={{
                display: "grid",
                placeItems: "center",
                minHeight: 240,
                padding: 12,
                borderRadius: 12,
                background: "rgba(0, 0, 0, 0.35)",
              }}
            >
              {selectedAssetURL ? (
                <img
                  src={selectedAssetURL}
                  alt={assetDisplayName(selectedAsset)}
                  style={{ maxHeight: "50vh", width: "auto", maxWidth: "100%", borderRadius: 10, objectFit: "contain" }}
                />
              ) : (
                <div style={{ display: "grid", placeItems: "center", color: "var(--app-text-muted)" }}>
                  <ImageIcon className="size-10" />
                </div>
              )}
            </div>
            <div style={{ display: "grid", gap: 12, fontSize: 13 }}>
              <DetailRow label="会话" value={selectedAsset.conversation_title || "未命名会话"} />
              <DetailRow label="提示词" value={selectedAsset.prompt || "-"} multiline />
              <DetailRow label="时间" value={formatAssetTime(selectedAsset.created_at)} />
              <DetailRow label="模型" value={selectedAsset.model || "-"} />
              <DetailRow label="尺寸" value={selectedAsset.size || "-"} />
              <DetailRow label="质量" value={selectedAsset.quality || "-"} />
            </div>
          </div>
        ) : null}
      </AppModal>
    </AdminPage>
  );
}

function DetailRow({ label, value, multiline = false }: { label: string; value: string; multiline?: boolean }) {
  return (
    <div>
      <dt style={{ fontSize: 11, fontWeight: 600, color: "var(--app-text-muted)", textTransform: "uppercase", letterSpacing: 0.4 }}>{label}</dt>
      <dd
        style={
          multiline
            ? { marginTop: 4, whiteSpace: "pre-wrap", wordBreak: "break-word", lineHeight: 1.6, color: "var(--app-text-secondary)" }
            : { marginTop: 4, wordBreak: "break-word", color: "var(--app-text-secondary)" }
        }
      >
        {value}
      </dd>
    </div>
  );
}
