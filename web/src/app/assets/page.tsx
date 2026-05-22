"use client";

import { useEffect, useMemo, useState } from "react";
import { ImageIcon, LoaderCircle } from "lucide-react";
import { Link } from "react-router-dom";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import { buildImageDataUrl } from "@/app/image/view-utils";
import { listImageConversations, type ImageConversation, type StoredImage } from "@/store/image-conversations";

type AssetItem = {
  id: string;
  conversationId: string;
  title: string;
  prompt: string;
  createdAt: string;
  image: StoredImage;
};

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

function collectAssets(conversations: ImageConversation[]): AssetItem[] {
  return conversations
    .flatMap((conversation) => {
      const turns = conversation.turns?.length ? conversation.turns : [];
      const source = turns.length
        ? turns.flatMap((turn) =>
            turn.images.map((image, index) => ({
              id: `${turn.id}-${image.id || index}`,
              conversationId: conversation.id,
              title: turn.title || conversation.title,
              prompt: turn.prompt || conversation.prompt,
              createdAt: turn.createdAt || conversation.createdAt,
              image,
            })),
          )
        : conversation.images.map((image, index) => ({
            id: `${conversation.id}-${image.id || index}`,
            conversationId: conversation.id,
            title: conversation.title,
            prompt: conversation.prompt,
            createdAt: conversation.createdAt,
            image,
          }));
      return source;
    })
    .filter((item) => item.image.status === "success" && buildImageDataUrl(item.image))
    .sort((left, right) => right.createdAt.localeCompare(left.createdAt));
}

export default function AssetsPage() {
  const [conversations, setConversations] = useState<ImageConversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void listImageConversations()
      .then((items) => {
        if (!cancelled) {
          setConversations(items);
          setError("");
        }
      })
      .catch((nextError) => {
        if (!cancelled) {
          setError(nextError instanceof Error ? nextError.message : "读取资产失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const assets = useMemo(() => collectAssets(conversations), [conversations]);

  return (
    <AdminPage>
        <AdminHeader
          title="资产"
          description="展示当前用户生成过的所有成功图片，按生成时间倒序排列，后续可以继续加相册、搜索和批量管理。"
        >
          <div className="mb-3 inline-flex items-center gap-2 rounded-full bg-[var(--app-bg-surface)] px-3 py-1.5 text-xs font-semibold text-[var(--app-text-secondary)]">
            <ImageIcon className="size-4 text-[var(--app-accent-cyan)]" />
            Personal gallery
          </div>
        </AdminHeader>

        {loading ? (
          <div className="grid min-h-[360px] place-items-center rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-[var(--app-text-secondary)]">
            <div className="inline-flex items-center gap-2">
              <LoaderCircle className="size-5 animate-spin" />
              正在读取资产
            </div>
          </div>
        ) : error ? (
          <div className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-6 text-sm text-rose-300">{error}</div>
        ) : assets.length === 0 ? (
          <div className="grid min-h-[360px] place-items-center rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-6 text-center">
            <div>
              <ImageIcon className="mx-auto size-10 text-[var(--app-text-muted)]" />
              <h2 className="mt-4 text-lg font-semibold text-[var(--app-text-primary)]">还没有图片资产</h2>
              <p className="mt-2 text-sm text-[var(--app-text-secondary)]">去生成一张图片后，它会自动出现在这里。</p>
              <Link to="/image/history" className="mt-5 inline-flex h-10 items-center rounded-full bg-[var(--app-text-primary)] px-5 text-sm font-semibold text-[var(--app-bg-root)]">
                开始生成
              </Link>
            </div>
          </div>
        ) : (
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
            {assets.map((asset) => (
              <article key={asset.id} className="overflow-hidden rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)]">
                <div className="aspect-square bg-[var(--app-bg-sidebar)]">
                  <img src={buildImageDataUrl(asset.image)} alt={asset.title} className="size-full object-cover" loading="lazy" />
                </div>
                <div className="p-4">
                  <h2 className="truncate text-sm font-semibold text-[var(--app-text-primary)]">{asset.title}</h2>
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-[var(--app-text-muted)]">{asset.prompt}</p>
                  <time className="mt-3 block text-xs text-[var(--app-text-secondary)]">{formatAssetTime(asset.createdAt)}</time>
                </div>
              </article>
            ))}
          </section>
        )}
    </AdminPage>
  );
}
