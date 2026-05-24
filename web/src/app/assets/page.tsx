"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { CalendarClock, ImageIcon, LoaderCircle } from "lucide-react";
import { Link } from "react-router-dom";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
      >
        <div className="mb-3 inline-flex items-center gap-2 rounded-full bg-[var(--app-bg-surface)] px-3 py-1.5 text-xs font-semibold text-[var(--app-text-secondary)]">
          <ImageIcon className="size-4 text-[var(--app-accent-cyan)]" />
          图片库
        </div>
      </AdminHeader>

      {loading ? (
        <div className="grid min-h-[360px] place-items-center rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-[var(--app-text-secondary)]">
          <div className="inline-flex items-center gap-2">
            <LoaderCircle className="size-5 animate-spin" />
            正在读取资产
          </div>
        </div>
      ) : error && assets.length === 0 ? (
        <div className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-6 text-sm text-rose-300">{error}</div>
      ) : assets.length === 0 ? (
        <div className="grid min-h-[360px] place-items-center rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-6 text-center">
          <div>
            <ImageIcon className="mx-auto size-10 text-[var(--app-text-muted)]" />
            <h2 className="mt-4 text-lg font-semibold text-[var(--app-text-primary)]">还没有图片资产</h2>
            <p className="mt-2 text-sm text-[var(--app-text-secondary)]">生成成功的图片会自动出现在这里。</p>
            <Link to="/image/history" className="mt-5 inline-flex h-10 items-center rounded-full bg-[var(--app-text-primary)] px-5 text-sm font-semibold text-[var(--app-bg-root)]">
              开始生成
            </Link>
          </div>
        </div>
      ) : (
        <section className="space-y-5">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-6">
            {assets.map((asset) => {
              const imageURL = buildAssetURL(asset);
              return (
                <button
                  type="button"
                  key={asset.id || asset.file_name}
                  className="group overflow-hidden rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-left shadow-[var(--app-shadow-soft)] transition hover:border-[rgba(43,223,222,0.45)] hover:bg-[var(--app-bg-surface)] focus:outline-none focus:ring-2 focus:ring-[rgba(43,223,222,0.25)]"
                  onClick={() => setSelectedAsset(asset)}
                >
                  <div className="aspect-square bg-[var(--app-bg-sidebar)]">
                    {imageURL ? (
                      <img
                        src={imageURL}
                        alt={assetDisplayName(asset)}
                        className="size-full object-cover transition duration-200 group-hover:scale-[1.02]"
                        loading="lazy"
                      />
                    ) : (
                      <div className="grid size-full place-items-center text-[var(--app-text-muted)]">
                        <ImageIcon className="size-8" />
                      </div>
                    )}
                  </div>
                  <div className="flex h-12 items-center px-3 text-xs text-[var(--app-text-secondary)]">
                    <CalendarClock className="mr-2 size-3.5 shrink-0 text-[var(--app-text-muted)]" />
                    <time className="truncate">{formatAssetTime(asset.created_at)}</time>
                  </div>
                </button>
              );
            })}
          </div>

          <div className="flex justify-center">
            {error ? (
              <div className="flex flex-col items-center gap-3 text-center">
                <p className="text-sm text-rose-300">{error}</p>
                <button
                  type="button"
                  className="inline-flex h-10 items-center justify-center rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-5 text-sm font-semibold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={() => void loadAssets(page.page + 1)}
                  disabled={loadingMore}
                >
                  重新加载
                </button>
              </div>
            ) : hasMore ? (
              <button
                type="button"
                className="inline-flex h-10 items-center justify-center rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-5 text-sm font-semibold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => void loadAssets(page.page + 1)}
                disabled={loadingMore}
              >
                {loadingMore ? (
                  <>
                    <LoaderCircle className="mr-2 size-4 animate-spin" />
                    正在加载
                  </>
                ) : (
                  "加载更多"
                )}
              </button>
            ) : (
              <p className="text-xs text-[var(--app-text-muted)]">已显示全部 {page.total} 张图片</p>
            )}
          </div>
        </section>
      )}

      <Dialog open={selectedAsset !== null} onOpenChange={(open) => !open && setSelectedAsset(null)}>
        <DialogContent className="w-[min(94vw,1100px)] gap-0 overflow-hidden p-0">
          {selectedAsset ? (
            <div className="grid max-h-[90vh] grid-rows-[minmax(0,1fr)_auto] lg:grid-cols-[minmax(0,1fr)_320px] lg:grid-rows-1">
              <div className="grid min-h-[320px] place-items-center bg-black/40 p-3 sm:p-5">
                {selectedAssetURL ? (
                  <img
                    src={selectedAssetURL}
                    alt={assetDisplayName(selectedAsset)}
                    className="max-h-[72vh] w-auto max-w-full rounded-[var(--app-radius-md)] object-contain"
                  />
                ) : (
                  <div className="grid min-h-[320px] place-items-center text-[var(--app-text-muted)]">
                    <ImageIcon className="size-10" />
                  </div>
                )}
              </div>
              <aside className="border-t border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-5 lg:border-t-0 lg:border-l">
                <DialogHeader>
                  <DialogTitle className="pr-8 text-base">图片详情</DialogTitle>
                  <DialogDescription>生成信息</DialogDescription>
                </DialogHeader>
                <dl className="mt-5 space-y-4 text-sm">
                  <DetailRow label="会话" value={selectedAsset.conversation_title || "未命名会话"} />
                  <DetailRow label="提示词" value={selectedAsset.prompt || "-"} multiline />
                  <DetailRow label="时间" value={formatAssetTime(selectedAsset.created_at)} />
                  <DetailRow label="模型" value={selectedAsset.model || "-"} />
                  <DetailRow label="尺寸" value={selectedAsset.size || "-"} />
                  <DetailRow label="质量" value={selectedAsset.quality || "-"} />
                </dl>
              </aside>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>
    </AdminPage>
  );
}

function DetailRow({ label, value, multiline = false }: { label: string; value: string; multiline?: boolean }) {
  return (
    <div>
      <dt className="text-xs font-semibold text-[var(--app-text-muted)]">{label}</dt>
      <dd className={multiline ? "mt-1 whitespace-pre-wrap break-words leading-6 text-[var(--app-text-secondary)]" : "mt-1 break-words text-[var(--app-text-secondary)]"}>{value}</dd>
    </div>
  );
}
