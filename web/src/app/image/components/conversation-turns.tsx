"use client";

import { memo } from "react";
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

type ProcessingStatus = {
  title: string;
  detail: string;
};

function formatTurnSizeLabel(size?: string) {
  return String(size || "")
    .trim()
    .replace("x", "X");
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
    turnId: string,
    image: StoredImage,
    imageName: string,
  ) => void;
  onSeedFromResult: (
    conversationId: string,
    image: StoredImage,
    nextMode: ImageMode,
  ) => void;
  onRetryTurn: (
    conversationId: string,
    turn: ImageConversationTurn,
    imageIndex?: number,
  ) => Promise<void>;
  onCancelTurn: (conversationId: string, turn: ImageConversationTurn) => Promise<void>;
};

const metaPillClass =
  "inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-3 text-[12px] font-semibold text-[var(--app-text-secondary)] shadow-none";

const iconButtonClass =
  "inline-flex size-8 items-center justify-center rounded-lg border border-[var(--app-border)] bg-[var(--app-bg-surface)] text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]";

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
  onSeedFromResult,
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
        const cancelRequested = Boolean(turn.cancelRequested);
        const showQueuedState = turn.status === "queued";
        const showRunningState = turn.status === "running" || turn.status === "generating";

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

            <div className="w-full max-w-[720px] space-y-5" data-image-result-group>
              {turn.images.length > 0 ? (
                <div
                  className={cn(
                    "grid justify-items-start gap-5",
                    turn.images.length === 1 ? "grid-cols-1" : "grid-cols-1 xl:grid-cols-2",
                  )}
                >
                  {turn.images.map((image, index) => {
                    const imageDataUrl = buildImageDataUrl(image);
                    const downloadName = buildDownloadName(turn.createdAt, turn.id, index);

                    return (
                      <div
                        key={image.id}
                        data-generated-image-card
                        className={cn(
                          "w-full max-w-[350px]",
                        )}
                      >
                        <div className="overflow-hidden rounded-xl bg-[var(--app-bg-surface)] shadow-[inset_0_0_0_1px_var(--app-border)]">
                          {image.status === "success" && imageDataUrl ? (
                            <Zoom>
                              <Image
                                src={imageDataUrl}
                                alt={`Generated result ${index + 1}`}
                                width={1024}
                                height={1024}
                                unoptimized
                                className="block h-auto max-h-[350px] w-auto max-w-full cursor-zoom-in object-contain"
                              />
                            </Zoom>
                          ) : turn.status === "cancelled" ? (
                            <div className="flex min-h-[260px] items-center justify-center px-6 py-8 text-center text-sm font-semibold leading-7 text-[var(--app-text-muted)]">
                              本次生成已取消
                            </div>
                          ) : image.status === "error" ? (
                            <div className="flex min-h-[320px] items-center justify-center whitespace-pre-line bg-rose-950/20 px-6 py-8 text-center text-sm font-semibold leading-7 text-rose-300">
                              {formatImageErrorMessage(image.error || "处理失败")}
                            </div>
                          ) : (
                            <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 px-6 py-8 text-center text-[var(--app-text-muted)]">
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

                        <div className="mt-4 flex flex-wrap items-center gap-2 text-[12px]">
                          <span className={metaPillClass}>
                            <span className="text-[var(--app-text-muted)]">模型</span>
                            {turn.providerPlatform || turn.model}
                          </span>
                          <span className={metaPillClass}>
                            <span className="text-[var(--app-text-muted)]">模式</span>
                            {modeLabelMap[turn.mode]}
                          </span>
                          {turn.size ? (
                            <span className={metaPillClass}>
                              <span className="text-[var(--app-text-muted)]">尺寸</span>
                              {formatTurnSizeLabel(turn.size)}
                            </span>
                          ) : null}
                          {turn.quality ? (
                            <span className={metaPillClass}>
                              <span className="text-[var(--app-text-muted)]">清晰度</span>
                              {turn.quality}
                            </span>
                          ) : null}
                          <span className={metaPillClass}>
                            <Clock3 className="size-3.5 text-[var(--app-text-muted)]" />
                            {formatConversationTime(turn.createdAt)}
                          </span>

                          {image.status === "success" && imageDataUrl ? (
                            <>
                              <button
                                type="button"
                                className={iconButtonClass}
                                onClick={() =>
                                  onOpenSelectionEditor(conversationId, turn.id, image, downloadName)
                                }
                                title="选区"
                                aria-label="选区"
                              >
                                <Brush className="size-4" />
                              </button>
                              <button
                                type="button"
                                className={iconButtonClass}
                                onClick={() => onSeedFromResult(conversationId, image, "edit")}
                                title="引用"
                                aria-label="引用"
                              >
                                <Copy className="size-4" />
                              </button>
                              <a
                                href={imageDataUrl}
                                download={downloadName}
                                className={iconButtonClass}
                                title="下载"
                                aria-label="下载"
                              >
                                <Download className="size-4" />
                              </a>
                            </>
                          ) : null}

                          {image.status === "error" ? (
                            <button
                              type="button"
                              className={cn(iconButtonClass, "text-rose-300 disabled:cursor-not-allowed disabled:opacity-60")}
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
                              className={cn(iconButtonClass, "text-rose-300 disabled:cursor-not-allowed disabled:opacity-60")}
                              title={cancelRequested ? "取消中" : "取消任务"}
                              aria-label={cancelRequested ? "取消中" : "取消任务"}
                            >
                              <X className="size-4" />
                            </button>
                          ) : null}
                        </div>
                      </div>
                    );
                  })}
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
