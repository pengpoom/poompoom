"use client";

import { type CSSProperties, memo, useEffect, useState } from "react";
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
import type { APIAccessPlatform, ImageModel } from "@/lib/api";
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

const resultFrameMaxSide = 350;
const resultFrameMinWidth = 180;
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
    turn: { model?: ImageModel; providerPlatform?: APIAccessPlatform },
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

const metaPillClass =
  "inline-flex shrink-0 items-center gap-1 text-[12px] font-semibold leading-5 text-[var(--app-text-secondary)]";

const metaLabelClass = "shrink-0 text-[var(--app-text-muted)]";

const iconButtonClass =
  "inline-flex size-9 items-center justify-center rounded-lg border border-white/15 bg-black/55 text-white shadow-[0_8px_24px_rgba(0,0,0,0.24)] backdrop-blur transition hover:bg-black/70";

type GeneratedImageCardProps = {
  conversationId: string;
  turn: ImageConversationTurn;
  image: StoredImage;
  index: number;
  modeLabelMap: Record<ImageMode, string>;
  turnProcessing: boolean;
  processingStatus: ProcessingStatus | null;
  waitingDots: string;
  submitElapsedSeconds: number;
  formatConversationTime: (value: string) => string;
  formatProcessingDuration: (seconds: number) => string;
  onOpenSelectionEditor: (
    conversationId: string,
    turn: { model?: ImageModel; providerPlatform?: APIAccessPlatform },
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
  processingStatus,
  waitingDots,
  submitElapsedSeconds,
  formatConversationTime,
  formatProcessingDuration,
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
      className="w-full max-w-full"
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
            <div className="absolute bottom-3 right-3 z-10 flex items-center gap-2">
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
            </div>
          </>
        ) : image.status === "success" && imageLoadFailed ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 bg-rose-950/20 px-6 py-8 text-center text-rose-300">
            <div className="rounded-full bg-rose-500/10 p-3">
              <X className="size-5" />
            </div>
            <p className="text-sm font-bold">图片加载失败</p>
            <p className="text-xs leading-6 text-rose-200/80">图片地址不可用或文件暂时无法访问。</p>
            <button
              type="button"
              className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-rose-300/25 px-3 text-xs font-semibold text-rose-100 transition hover:bg-rose-300/10 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => void onRetryTurn(conversationId, turn, index)}
              disabled={turnProcessing}
            >
              <RotateCcw className="size-3.5" />
              重试
            </button>
          </div>
        ) : turn.status === "cancelled" ? (
          <div className="flex h-full items-center justify-center px-6 py-8 text-center text-sm font-semibold leading-7 text-[var(--app-text-muted)]">
            本次生成已取消
          </div>
        ) : image.status === "error" ? (
          <div className="flex h-full items-center justify-center whitespace-pre-line bg-rose-950/20 px-6 py-8 text-center text-sm font-semibold leading-7 text-rose-300">
            {formatImageErrorMessage(image.error || "处理失败")}
          </div>
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 px-6 py-8 text-center text-[var(--app-text-muted)]">
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
                : showQueuedState
                  ? "已加入等候队列"
                  : turnProcessing && processingStatus
                    ? `${processingStatus.title}${waitingDots}`
                    : "正在处理图片..."}
            </p>
            <p className="text-xs leading-6">
              {cancelRequested
                ? "正在等待当前请求结束，取消后将丢弃本次结果"
                : showQueuedState
                  ? `${turn.waitingDetail || "等待后端准入和上游处理"}${(turn.queuePosition ?? 0) > 1 ? ` · 前面还有 ${turn.queuePosition! - 1} 个` : ""}`
                  : turnProcessing && processingStatus
                    ? `${processingStatus.detail} · 已等待 ${formatProcessingDuration(submitElapsedSeconds)}`
                    : "图片处理通常需要几分钟，请稍候"}
            </p>
          </div>
        )}
      </div>

      <div className="mt-4 flex w-full flex-wrap items-center gap-2 text-[12px]">
        <span className={metaPillClass}>
          <span className={metaLabelClass}>模型</span>
          <span>{turn.providerPlatform || turn.model}</span>
        </span>
        <span className={metaPillClass}>
          <span className={metaLabelClass}>模式</span>
          {modeLabelMap[turn.mode]}
        </span>
        {aspectRatioLabel ? (
          <span className={metaPillClass}>
            <span className={metaLabelClass}>尺寸</span>
            {aspectRatioLabel}
          </span>
        ) : null}
        {turn.quality ? (
          <span className={metaPillClass}>
            <span className={metaLabelClass}>清晰度</span>
            {turn.quality}
          </span>
        ) : null}
        <span className={metaPillClass}>
          <Clock3 className="size-3.5 shrink-0 text-[var(--app-text-muted)]" />
          {formatConversationTime(turn.createdAt)}
        </span>

        {image.status === "error" ? (
          <button
            type="button"
            className={cn(metaPillClass, "text-rose-300 disabled:cursor-not-allowed disabled:opacity-60")}
            onClick={() => void onRetryTurn(conversationId, turn, index)}
            disabled={turnProcessing}
            title={turnProcessing ? "处理中" : "重试"}
            aria-label="重试"
          >
            <RotateCcw className="size-4" />
          </button>
        ) : null}

        {(showQueuedState || showRunningState) && turn.jobId ? (
          <button
            type="button"
            onClick={() => void onCancelTurn(conversationId, turn)}
            disabled={cancelRequested}
            className={cn(metaPillClass, "text-rose-300 disabled:cursor-not-allowed disabled:opacity-60")}
            title={cancelRequested ? "取消中" : "取消任务"}
            aria-label={cancelRequested ? "取消中" : "取消任务"}
          >
            <X className="size-4" />
          </button>
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
  return (
    <div className="mx-auto flex w-full max-w-[920px] flex-col gap-10 px-4 pb-10 pt-0 sm:px-6 lg:px-0">
      {turns.map((turn) => {
        const turnProcessing = Boolean(
          activeRequest &&
            activeRequest.conversationId === conversationId &&
            activeRequest.turnId === turn.id,
        );
        return (
          <div key={turn.id} data-image-turn={turn.id} className="space-y-10">
            <div className="flex justify-end">
              <div className="group flex w-full max-w-[560px] flex-col items-end gap-3">
                {turn.sourceImages && turn.sourceImages.length > 0 ? (
                  <div className="flex flex-wrap justify-end gap-2.5">
                    {turn.sourceImages.map((source) => (
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

                <div className="max-w-full whitespace-pre-wrap break-words rounded-[28px] border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-5 py-3.5 text-[14px] font-semibold leading-6 text-[var(--app-text-primary)] sm:px-9">
                  {turn.prompt || "无额外提示词"}
                </div>
                <button
                  type="button"
                  onClick={() => void copyPromptToClipboard(turn.prompt || "")}
                  className="inline-flex h-7 shrink-0 items-center gap-1 rounded-full bg-[var(--app-bg-surface)] px-2.5 text-xs font-semibold text-[var(--app-text-muted)] opacity-0 transition hover:text-[var(--app-text-primary)] focus-visible:opacity-100 focus-visible:outline-none group-hover:opacity-100"
                  title="复制提示词"
                  aria-label="复制提示词"
                >
                  <Copy className="size-3.5" />
                  复制
                </button>
              </div>
            </div>

            <div className="w-full max-w-[760px] space-y-5" data-image-result-group>
              {turn.images.length > 0 ? (
                <div
                  className={cn(
                    "grid w-full justify-items-start gap-5",
                    turn.images.length === 1
                      ? "grid-cols-[minmax(180px,350px)]"
                      : "grid-cols-[minmax(180px,350px)] xl:grid-cols-[minmax(180px,350px)_minmax(180px,350px)]",
                  )}
                >
                  {turn.images.map((image, index) => (
                    <GeneratedImageCard
                      key={image.id}
                      conversationId={conversationId}
                      turn={turn}
                      image={image}
                      index={index}
                      modeLabelMap={modeLabelMap}
                      turnProcessing={turnProcessing}
                      processingStatus={processingStatus}
                      waitingDots={waitingDots}
                      submitElapsedSeconds={submitElapsedSeconds}
                      formatConversationTime={formatConversationTime}
                      formatProcessingDuration={formatProcessingDuration}
                      onOpenSelectionEditor={onOpenSelectionEditor}
                      onRetryTurn={onRetryTurn}
                      onCancelTurn={onCancelTurn}
                    />
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );
});

ConversationTurns.displayName = "ConversationTurns";
