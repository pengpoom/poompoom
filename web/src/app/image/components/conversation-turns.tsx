"use client";

import { Fragment, type CSSProperties, memo, useEffect, useMemo, useState } from "react";
import Zoom from "react-medium-image-zoom";
import {
  Brush,
  Clock3,
  Copy,
  Download,
  LoaderCircle,
  RotateCcw,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { HtmlImage as Image } from "@/components/html-image";
import { cn } from "@/lib/utils";
import type {
  ImageConversationTurn,
  ImageMode,
  StoredImage,
} from "@/store/image-conversations";

import { formatImageErrorMessage } from "../submit-utils";
import {
  buildConversationSourceLabel,
  buildImageDataUrl,
  buildSourceImageUrl,
} from "../view-utils";

type ActiveRequestState = {
  conversationId: string;
  turnId: string;
  mode: ImageMode;
  count: number;
  variant: "standard" | "selection-edit";
};

type TurnRenderGroup = {
  id: string;
  turns: ImageConversationTurn[];
  compare: boolean;
};

type ProcessingStatus = {
  title: string;
  detail: string;
};

type ResultFrameMetrics = {
  width: number;
  height: number;
  style: CSSProperties;
};

type ImageDimensions = {
  width: number;
  height: number;
};

const resultFrameMaxSide = 300;
const resultFrameMinWidth = 168;
const commonAspectRatios = [
  { label: "1:1", ratio: 1 },
  { label: "2:3", ratio: 2 / 3 },
  { label: "3:2", ratio: 3 / 2 },
  { label: "3:4", ratio: 3 / 4 },
  { label: "4:3", ratio: 4 / 3 },
  { label: "5:4", ratio: 5 / 4 },
  { label: "4:5", ratio: 4 / 5 },
  { label: "9:16", ratio: 9 / 16 },
  { label: "16:9", ratio: 16 / 9 },
  { label: "21:9", ratio: 21 / 9 },
];

function fallbackResultFrameMetrics(): ResultFrameMetrics {
  return {
    width: 1024,
    height: 1024,
    style: {
      aspectRatio: "1 / 1",
      maxWidth: `${resultFrameMaxSide}px`,
    },
  };
}

function parseSizeDimensions(size?: string): ImageDimensions | null {
  const match = String(size || "")
    .trim()
    .match(/^(\d+)\s*x\s*(\d+)$/i);
  if (!match) {
    return null;
  }

  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return null;
  }
  return { width, height };
}

function resultFrameMetrics(dimensions: ImageDimensions | null): ResultFrameMetrics {
  if (!dimensions) {
    return fallbackResultFrameMetrics();
  }

  const { width, height } = dimensions;
  const ratio = width / height;
  const maxWidth =
    ratio < 1 ? Math.max(resultFrameMinWidth, Math.round(resultFrameMaxSide * ratio)) : resultFrameMaxSide;
  return {
    width,
    height,
    style: {
      aspectRatio: `${width} / ${height}`,
      maxWidth: `${maxWidth}px`,
    },
  };
}

function greatestCommonDivisor(left: number, right: number): number {
  let a = Math.abs(Math.round(left));
  let b = Math.abs(Math.round(right));
  while (b > 0) {
    const next = a % b;
    a = b;
    b = next;
  }
  return a || 1;
}

function approximateAspectRatio(width: number, height: number) {
  const ratio = width / height;
  let best = { width: 1, height: 1, diff: Number.POSITIVE_INFINITY };
  for (let denominator = 1; denominator <= 24; denominator += 1) {
    const numerator = Math.max(1, Math.round(ratio * denominator));
    const diff = Math.abs(numerator / denominator - ratio);
    if (diff < best.diff) {
      best = { width: numerator, height: denominator, diff };
    }
  }
  const divisor = greatestCommonDivisor(best.width, best.height);
  return `${Math.round(best.width / divisor)}:${Math.round(best.height / divisor)}`;
}

function formatAspectRatioLabel(dimensions: ImageDimensions | null, fallbackSize?: string) {
  const current = dimensions ?? parseSizeDimensions(fallbackSize);
  if (!current) {
    return "";
  }
  const ratio = current.width / current.height;
  const matched = commonAspectRatios
    .map((item) => ({
      ...item,
      diff: Math.abs(item.ratio - ratio) / item.ratio,
    }))
    .sort((left, right) => left.diff - right.diff)[0];
  if (matched && matched.diff <= 0.025) {
    return matched.label;
  }
  return approximateAspectRatio(current.width, current.height);
}

function buildDownloadName(createdAt: string, turnId: string, index: number) {
  const date = new Date(createdAt);
  const safeIndex = String(index + 1).padStart(2, "0");
  if (Number.isNaN(date.getTime())) {
    return `generated-image-${turnId.slice(0, 8)}-${safeIndex}.png`;
  }

  const yyyy = String(date.getFullYear());
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  const hh = String(date.getHours()).padStart(2, "0");
  const min = String(date.getMinutes()).padStart(2, "0");
  const sec = String(date.getSeconds()).padStart(2, "0");
  return `generated-image-${yyyy}${mm}${dd}-${hh}${min}${sec}-${safeIndex}.png`;
}

async function copyPromptToClipboard(prompt: string) {
  const text = prompt.trim();
  if (!text) {
    toast.warning("没有可复制的提示词");
    return;
  }

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const input = document.createElement("textarea");
      input.value = text;
      input.setAttribute("readonly", "");
      input.style.position = "fixed";
      input.style.left = "-9999px";
      document.body.appendChild(input);
      input.select();
      document.execCommand("copy");
      document.body.removeChild(input);
    }
    toast.success("提示词已复制");
  } catch {
    toast.error("复制失败");
  }
}

async function copyImageToClipboard(imageDataUrl: string) {
  if (!imageDataUrl) {
    toast.warning("没有可复制的图片");
    return;
  }

  try {
    if (!navigator.clipboard?.write || typeof ClipboardItem === "undefined") {
      throw new Error("clipboard image unsupported");
    }
    const response = await fetch(imageDataUrl);
    const blob = await response.blob();
    const type = blob.type || "image/png";
    await navigator.clipboard.write([
      new ClipboardItem({
        [type]: blob,
      }),
    ]);
    toast.success("图片已复制");
  } catch {
    toast.error("当前浏览器不支持直接复制图片，请使用下载");
  }
}

type ConversationTurnsProps = {
  conversationId: string;
  turns: ImageConversationTurn[];
  modeLabelMap: Record<ImageMode, string>;
  activeRequest: ActiveRequestState | null;
  processingStatus: ProcessingStatus | null;
  waitingDots: string;
  submitElapsedSeconds: number;
  formatConversationTime: (value: string) => string;
  formatProcessingDuration: (seconds: number) => string;
  onOpenSelectionEditor: (
    conversationId: string,
    turn: ImageConversationTurn,
    image: StoredImage,
    imageName: string,
  ) => void;
  onRetryTurn: (
    conversationId: string,
    turn: ImageConversationTurn,
    imageIndex?: number,
  ) => Promise<void>;
  onCancelTurn: (conversationId: string, turn: ImageConversationTurn) => Promise<void>;
};

const metaTextClass =
  "min-w-0 text-[12px] font-semibold leading-5 text-[var(--app-text-secondary)]";

const metaMutedClass = "text-[var(--app-text-muted)]";

const iconButtonClass =
  "inline-flex size-9 items-center justify-center rounded-lg border border-white/15 bg-black/55 text-white shadow-[0_8px_24px_rgba(0,0,0,0.24)] backdrop-blur transition hover:bg-black/70 disabled:cursor-not-allowed disabled:opacity-55";

function buildTurnRenderGroups(turns: ImageConversationTurn[]): TurnRenderGroup[] {
  const groups: TurnRenderGroup[] = [];
  const compareGroupIndexes = new Map<string, number>();

  for (const turn of turns) {
    const compareGroupId = String(turn.compareBatchId || turn.compareGroupId || "").trim();
    if (!compareGroupId) {
      groups.push({
        id: turn.id,
        turns: [turn],
        compare: false,
      });
      continue;
    }

    const existingIndex = compareGroupIndexes.get(compareGroupId);
    if (existingIndex === undefined) {
      compareGroupIndexes.set(compareGroupId, groups.length);
      groups.push({
        id: compareGroupId,
        turns: [turn],
        compare: true,
      });
      continue;
    }

    groups[existingIndex] = {
      ...groups[existingIndex],
      turns: [...groups[existingIndex].turns, turn],
    };
  }

  return groups.map((group) =>
    group.compare
      ? {
          ...group,
          turns: [...group.turns].sort(
            (left, right) =>
              (left.compareModelIndex ?? 0) - (right.compareModelIndex ?? 0),
          ),
        }
      : group,
  );
}

function isTurnProcessing(
  activeRequest: ActiveRequestState | null,
  conversationId: string,
  turnId: string,
) {
  return Boolean(
    activeRequest &&
      activeRequest.conversationId === conversationId &&
      activeRequest.turnId === turnId,
  );
}

function compareGridClass(count: number) {
  if (count === 3) {
    return "grid-cols-[repeat(3,300px)]";
  }
  if (count === 2) {
    return "grid-cols-[repeat(2,300px)]";
  }
  if (count === 4) {
    return "grid-cols-1 sm:grid-cols-[repeat(2,300px)]";
  }
  if (count >= 5) {
    return "grid-cols-1 sm:grid-cols-[repeat(2,300px)] xl:grid-cols-[repeat(3,300px)]";
  }
  return "grid-cols-[300px]";
}

function compareGroupMetaItems(
  turn: ImageConversationTurn,
  modeLabelMap: Record<ImageMode, string>,
  formatConversationTime: (value: string) => string,
) {
  const items = [modeLabelMap[turn.mode]];
  const dimensions = parseSizeDimensions(turn.size);
  const aspectRatioLabel = formatAspectRatioLabel(dimensions, turn.size);
  if (aspectRatioLabel) {
    items.push(aspectRatioLabel);
  }
  if (turn.quality) {
    items.push(turn.quality);
  }
  const time = formatConversationTime(turn.createdAt);
  if (time) {
    items.push(time);
  }
  return items.filter(Boolean);
}

function compareBatchStatusText(turns: ImageConversationTurn[]) {
  const total = turns.length;
  const succeeded = turns.filter((turn) => turn.status === "success").length;
  const failed = turns.filter((turn) => turn.status === "error").length;
  const running = turns.filter((turn) => turn.status === "running" || turn.status === "generating").length;
  const queued = turns.filter((turn) => turn.status === "queued").length;
  const cancelled = turns.filter((turn) => turn.status === "cancelled").length;
  const parts = [`${succeeded}/${total} 完成`];
  if (failed > 0) {
    parts.push(`${failed} 失败`);
  }
  if (running > 0) {
    parts.push(`${running} 运行中`);
  }
  if (queued > 0) {
    parts.push(`${queued} 排队中`);
  }
  if (cancelled > 0) {
    parts.push(`${cancelled} 已取消`);
  }
  return parts.join(" · ");
}

type GeneratedImageCardProps = {
  conversationId: string;
  turn: ImageConversationTurn;
  image: StoredImage;
  index: number;
  modeLabelMap: Record<ImageMode, string>;
  turnProcessing: boolean;
  compactMeta?: boolean;
  formatConversationTime: (value: string) => string;
  onOpenSelectionEditor: (
    conversationId: string,
    turn: ImageConversationTurn,
    image: StoredImage,
    imageName: string,
  ) => void;
  onRetryTurn: (
    conversationId: string,
    turn: ImageConversationTurn,
    imageIndex?: number,
  ) => Promise<void>;
  onCancelTurn: (conversationId: string, turn: ImageConversationTurn) => Promise<void>;
};

function GeneratedImageCard({
  conversationId,
  turn,
  image,
  index,
  modeLabelMap,
  turnProcessing,
  compactMeta = false,
  formatConversationTime,
  onOpenSelectionEditor,
  onRetryTurn,
  onCancelTurn,
}: GeneratedImageCardProps) {
  const [actualDimensions, setActualDimensions] = useState<ImageDimensions | null>(null);
  const [imageLoadFailed, setImageLoadFailed] = useState(false);
  const imageDataUrl = buildImageDataUrl(image);
  const downloadName = buildDownloadName(turn.createdAt, turn.id, index);
  const cancelRequested = Boolean(turn.cancelRequested);
  const showQueuedState = turn.status === "queued";
  const showRunningState = turn.status === "running" || turn.status === "generating";
  const showSuccessActions = image.status === "success" && imageDataUrl && !imageLoadFailed;
  const showRetryAction = image.status === "error" || (image.status === "success" && imageLoadFailed);
  const showCancelAction = (showQueuedState || showRunningState) && Boolean(turn.jobId);
  const modelLabel = turn.compareModelLabel || turn.modelLabel || turn.model || turn.providerPlatform;
  const statusLabel = image.status === "success"
    ? imageLoadFailed
      ? "图片不可用"
      : "完成"
    : image.status === "error"
      ? "失败"
      : turn.status === "cancelled"
        ? "已取消"
        : cancelRequested
          ? "取消中"
          : showQueuedState
            ? "排队中"
            : "生成中";
  const statusClass = image.status === "success" && !imageLoadFailed
    ? "text-emerald-300"
    : image.status === "error" || imageLoadFailed
      ? "text-rose-300"
      : cancelRequested || turn.status === "cancelled"
        ? "text-amber-300"
        : "text-[var(--app-text-muted)]";
  const frameMetrics = resultFrameMetrics(actualDimensions ?? parseSizeDimensions(turn.size));
  const aspectRatioLabel = formatAspectRatioLabel(actualDimensions, turn.size);
  const frameStyle = {
    aspectRatio: frameMetrics.style.aspectRatio,
    maxWidth: frameMetrics.style.maxWidth,
  } satisfies CSSProperties;

  useEffect(() => {
    setImageLoadFailed(false);
    setActualDimensions(null);
  }, [imageDataUrl]);

  return (
    <div
      data-generated-image-card
      data-job-id={image.jobId || turn.jobId || undefined}
      className="w-full min-w-0 max-w-full"
      style={{ maxWidth: frameMetrics.style.maxWidth }}
    >
      <div
        className="relative w-full overflow-hidden rounded-xl bg-[var(--app-bg-surface)] shadow-[inset_0_0_0_1px_var(--app-border)] [&_[data-rmiz-content]]:h-full [&_[data-rmiz-content]]:w-full [&_[data-rmiz]]:h-full [&_[data-rmiz]]:w-full"
        style={frameStyle}
      >
        {image.status === "success" && imageDataUrl && !imageLoadFailed ? (
          <>
            <Zoom>
              <Image
                src={imageDataUrl}
                alt={`Generated result ${index + 1}`}
                width={frameMetrics.width}
                height={frameMetrics.height}
                unoptimized
                onLoad={(event) => {
                  const { naturalWidth, naturalHeight } = event.currentTarget;
                  if (naturalWidth <= 0 || naturalHeight <= 0) {
                    return;
                  }
                  setImageLoadFailed(false);
                  setActualDimensions((current) =>
                    current?.width === naturalWidth && current.height === naturalHeight
                      ? current
                      : { width: naturalWidth, height: naturalHeight },
                  );
                }}
                onError={() => setImageLoadFailed(true)}
                className="block h-full w-full cursor-zoom-in object-contain"
              />
            </Zoom>
          </>
        ) : image.status === "success" && imageLoadFailed ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 bg-[var(--app-img-fail-bg)] px-6 pb-16 pt-8 text-center text-[var(--app-img-fail-fg)]">
            <div className="rounded-full bg-[var(--app-img-fail-ic-bg)] p-3">
              <X className="size-5" />
            </div>
            <p className="text-sm font-semibold leading-7">图片地址暂时不可用，请稍后刷新或重试。</p>
          </div>
        ) : turn.status === "cancelled" ? (
          <div className="flex h-full items-center justify-center px-6 py-8 text-center text-sm font-semibold leading-7 text-[var(--app-text-muted)]">
            本次生成已取消
          </div>
        ) : image.status === "error" ? (
          <div className="flex h-full items-center justify-center whitespace-pre-line bg-[var(--app-img-fail-bg)] px-6 pb-16 pt-8 text-center text-sm font-semibold leading-7 text-[var(--app-img-fail-fg)]">
            {formatImageErrorMessage(image.error || "处理失败")}
          </div>
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 px-6 pb-16 pt-8 text-center text-[var(--app-text-muted)]">
            {cancelRequested ? (
              <div className="rounded-full bg-rose-500/10 p-3 text-rose-300">
                <X className="size-5" />
              </div>
            ) : (
              <div className="rounded-full bg-[var(--app-bg-surface-hover)] p-3 text-[var(--app-text-primary)]">
                <LoaderCircle className="size-5 animate-spin" />
              </div>
            )}
            <p className="text-sm font-bold text-[var(--app-text-primary)]">
              {cancelRequested
                ? "正在取消任务"
                : "绘制画面中..."}
            </p>
            <p className="text-xs leading-6">
              {cancelRequested
                ? "正在等待当前请求结束，取消后将丢弃本次结果"
                : "请保持页面开启，结果完成后会自动显示"}
            </p>
          </div>
        )}
        {showSuccessActions || showRetryAction || showCancelAction ? (
          <div className="absolute bottom-3 right-3 z-10 flex items-center gap-2">
            {showSuccessActions ? (
              <>
                <button
                  type="button"
                  className={iconButtonClass}
                  onClick={(event) => {
                    event.stopPropagation();
                    void copyImageToClipboard(imageDataUrl);
                  }}
                  title="复制"
                  aria-label="复制"
                >
                  <Copy className="size-4" />
                </button>
                <a
                  href={imageDataUrl}
                  download={downloadName}
                  className={iconButtonClass}
                  onClick={(event) => event.stopPropagation()}
                  title="下载"
                  aria-label="下载"
                >
                  <Download className="size-4" />
                </a>
                <button
                  type="button"
                  className={iconButtonClass}
                  onClick={(event) => {
                    event.stopPropagation();
                    onOpenSelectionEditor(conversationId, turn, image, downloadName);
                  }}
                  title="编辑"
                  aria-label="编辑"
                >
                  <Brush className="size-4" />
                </button>
              </>
            ) : null}
            {showRetryAction ? (
              <button
                type="button"
                className={iconButtonClass}
                onClick={(event) => {
                  event.stopPropagation();
                  void onRetryTurn(conversationId, turn, index);
                }}
                disabled={turnProcessing}
                title={turnProcessing ? "处理中" : "重试"}
                aria-label="重试"
              >
                <RotateCcw className="size-4" />
              </button>
            ) : null}
            {showCancelAction ? (
              <button
                type="button"
                className={iconButtonClass}
                onClick={(event) => {
                  event.stopPropagation();
                  void onCancelTurn(conversationId, turn);
                }}
                disabled={cancelRequested}
                title={cancelRequested ? "取消中" : "取消任务"}
                aria-label={cancelRequested ? "取消中" : "取消任务"}
              >
                <X className="size-4" />
              </button>
            ) : null}
          </div>
        ) : null}
      </div>

      <div
        className={cn(
          "mt-2 flex min-h-7 items-center gap-x-1.5 text-[12px]",
          compactMeta
            ? "w-full min-w-0 max-w-full overflow-hidden"
            : "w-max max-w-[min(760px,calc(100vw-2rem))] flex-nowrap overflow-visible",
        )}
      >
        <span
          className={cn(
            metaTextClass,
            compactMeta
              ? "min-w-0 flex-1 truncate"
              : "shrink-0 whitespace-nowrap",
          )}
        >
          <span className={metaMutedClass}>模型</span>{" "}
          {modelLabel}
        </span>
        <span className={cn(metaTextClass, "shrink-0 whitespace-nowrap", statusClass)}>
          {statusLabel}
        </span>
        {!compactMeta ? (
          <>
            <span className="shrink-0 text-[var(--app-text-muted)]">·</span>
            <span className={cn(metaTextClass, "shrink-0 whitespace-nowrap")}>
              <span className={metaMutedClass}>模式</span> {modeLabelMap[turn.mode]}
            </span>
          </>
        ) : null}
        {!compactMeta && aspectRatioLabel ? (
          <>
            <span className="shrink-0 text-[var(--app-text-muted)]">·</span>
            <span className={cn(metaTextClass, "shrink-0 whitespace-nowrap")}>
              <span className={metaMutedClass}>尺寸</span> {aspectRatioLabel}
            </span>
          </>
        ) : null}
        {!compactMeta && turn.quality ? (
          <>
            <span className="shrink-0 text-[var(--app-text-muted)]">·</span>
            <span className={cn(metaTextClass, "shrink-0 whitespace-nowrap")}>
              <span className={metaMutedClass}>清晰度</span> {turn.quality}
            </span>
          </>
        ) : null}
        {!compactMeta ? (
          <>
            <span className="shrink-0 text-[var(--app-text-muted)]">·</span>
            <span className={cn(metaTextClass, "inline-flex shrink-0 items-center gap-1 whitespace-nowrap")}>
              <Clock3 className="size-3.5 shrink-0 text-[var(--app-text-muted)]" />
              {formatConversationTime(turn.createdAt)}
            </span>
          </>
        ) : null}
      </div>
    </div>
  );
}

export const ConversationTurns = memo(function ConversationTurns({
  conversationId,
  turns,
  modeLabelMap,
  activeRequest,
  processingStatus,
  waitingDots,
  submitElapsedSeconds,
  formatConversationTime,
  formatProcessingDuration,
  onOpenSelectionEditor,
  onRetryTurn,
  onCancelTurn,
}: ConversationTurnsProps) {
  const renderGroups = useMemo(() => buildTurnRenderGroups(turns), [turns]);

  return (
    <div className="mx-auto flex w-full max-w-[920px] flex-col gap-10 px-4 pb-10 pt-0 sm:px-6 lg:px-0">
      {renderGroups.map((group) => {
        const primaryTurn = group.turns[0];
        const groupMetaItems = group.compare
          ? compareGroupMetaItems(
              primaryTurn,
              modeLabelMap,
              formatConversationTime,
            )
          : [];
        const compareStatusText = group.compare ? compareBatchStatusText(group.turns) : "";
        return (
          <div key={group.id} data-image-turn={primaryTurn.id} className="space-y-10">
            <div className="flex justify-end">
              <div className="group flex w-full max-w-[560px] flex-col items-end gap-2.5">
                {primaryTurn.prompt ? (
                  <div className="flex max-w-full flex-row-reverse items-start justify-end gap-2">
                    <div className="max-w-full whitespace-pre-wrap break-words rounded-[28px] border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-5 py-3.5 text-[14px] font-semibold leading-6 text-[var(--app-text-primary)] sm:px-9">
                      {primaryTurn.prompt}
                    </div>
                    <button
                      type="button"
                      onClick={() => void copyPromptToClipboard(primaryTurn.prompt || "")}
                      className="inline-flex h-7 shrink-0 items-center gap-1 rounded-full bg-[var(--app-bg-surface)] px-2.5 text-xs font-semibold text-[var(--app-text-muted)] opacity-0 transition hover:text-[var(--app-text-primary)] focus-visible:opacity-100 focus-visible:outline-none group-hover:opacity-100"
                      title="复制提示词"
                      aria-label="复制提示词"
                    >
                      <Copy className="size-3.5" />
                      复制
                    </button>
                  </div>
                ) : null}
                {primaryTurn.sourceImages && primaryTurn.sourceImages.length > 0 ? (
                  <div className="flex flex-wrap justify-end gap-2.5">
                    {primaryTurn.sourceImages.map((source) => (
                      <div
                        key={source.id}
                        className="w-[136px] overflow-hidden rounded-xl border border-[var(--app-border)] bg-[var(--app-bg-surface)]"
                      >
                        <div className="border-b border-[var(--app-border)] px-3 py-2 text-left text-[11px] font-semibold text-[var(--app-text-muted)]">
                          {buildConversationSourceLabel(source)}
                        </div>
                        <Zoom>
                          <Image
                            src={buildSourceImageUrl(source)}
                            alt={source.name}
                            width={220}
                            height={160}
                            unoptimized
                            className="block h-24 w-full cursor-zoom-in bg-black object-contain"
                          />
                        </Zoom>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>

            <div className="w-full space-y-5" data-image-result-group>
              {group.compare ? (
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] font-bold text-[var(--app-text-muted)]">
                  <span className="rounded-full bg-[var(--app-bg-surface)] px-2.5 py-1">
                    模型对比
                  </span>
                  <span>{group.turns.length} 个结果</span>
                  {compareStatusText ? (
                    <>
                      <span className="text-[var(--app-text-muted)]">·</span>
                      <span>{compareStatusText}</span>
                    </>
                  ) : null}
                  {groupMetaItems.map((item) => (
                    <Fragment key={item}>
                      <span className="text-[var(--app-text-muted)]">·</span>
                      <span>{item}</span>
                    </Fragment>
                  ))}
                </div>
              ) : null}
              <div
                className={cn(
                  "grid w-full justify-items-start gap-5",
                  group.compare
                    ? compareGridClass(group.turns.length)
                    : primaryTurn.images.length === 1
                      ? "grid-cols-[minmax(180px,350px)]"
                      : "grid-cols-[minmax(180px,350px)] xl:grid-cols-[minmax(180px,350px)_minmax(180px,350px)]",
                )}
              >
                {group.turns.map((turn) => (
                  <Fragment key={turn.id}>
                    {turn.images.map((image, index) => (
                      <GeneratedImageCard
                        key={image.id}
                        conversationId={conversationId}
                        turn={turn}
                        image={image}
                        index={index}
                        modeLabelMap={modeLabelMap}
                        turnProcessing={isTurnProcessing(activeRequest, conversationId, turn.id)}
                        compactMeta={group.compare}
                        formatConversationTime={formatConversationTime}
                        onOpenSelectionEditor={onOpenSelectionEditor}
                        onRetryTurn={onRetryTurn}
                        onCancelTurn={onCancelTurn}
                      />
                    ))}
                  </Fragment>
                ))}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
});

ConversationTurns.displayName = "ConversationTurns";
