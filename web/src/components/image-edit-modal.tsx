"use client";

import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { ArrowUp, Brush, ChevronDown, Cpu, LoaderCircle, Redo2, Trash2, Undo2, X } from "lucide-react";
import { toast } from "sonner";

import type { APIAccessPlatform } from "@/lib/api";
import { ChipSelect } from "@/components/chip-select";
import { cn } from "@/lib/utils";

type StrokePoint = {
  x: number;
  y: number;
};

type Stroke = {
  points: StrokePoint[];
  sizeRatio: number;
};

type MaskPayload = {
  dataUrl: string;
  previewDataUrl: string;
};

type BrushCursor = {
  x: number;
  y: number;
};

type ImageEditModalProps = {
  open: boolean;
  imageName: string;
  imageSrc: string;
  isSubmitting?: boolean;
  allowOutputOptions?: boolean;
  imageAspectRatio?: string;
  imageAspectRatioOptions?: Array<{ label: string; value: string }>;
  imageResolutionTier?: string;
  imageResolutionTierOptions?: Array<{ label: string; value: string; disabled?: boolean }>;
  imageQuality?: string;
  imageQualityOptions?: Array<{ label: string; value: string; description: string }>;
  imageQualityDisabled?: boolean;
  imageQualityDisabledReason?: string;
  providerPlatform?: APIAccessPlatform;
  providerPlatformOptions?: Array<{ label: string; value: APIAccessPlatform; disabled?: boolean }>;
  onImageAspectRatioChange?: (value: string) => void;
  onImageResolutionTierChange?: (value: string) => void;
  onImageQualityChange?: (value: string) => void;
  onProviderPlatformChange?: (value: APIAccessPlatform) => void;
  onClose: () => void;
  onSubmit: (payload: {
    prompt: string;
    mask: MaskPayload;
    aspectRatio?: string;
    resolutionTier?: string;
    quality?: string;
    providerPlatform?: APIAccessPlatform;
  }) => Promise<void>;
};

function clampPoint(value: number) {
  return Math.max(0, Math.min(1, value));
}

function renderStroke(
  ctx: CanvasRenderingContext2D,
  stroke: Stroke,
  width: number,
  height: number,
  color: string,
) {
  if (stroke.points.length === 0) {
    return;
  }

  const lineWidth = stroke.sizeRatio * Math.min(width, height);
  ctx.save();
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.strokeStyle = color;
  ctx.fillStyle = color;
  ctx.lineWidth = Math.max(4, lineWidth);

  if (stroke.points.length === 1) {
    const point = stroke.points[0];
    ctx.beginPath();
    ctx.arc(point.x * width, point.y * height, Math.max(2, lineWidth / 2), 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    return;
  }

  ctx.beginPath();
  ctx.moveTo(stroke.points[0].x * width, stroke.points[0].y * height);
  stroke.points.slice(1).forEach((point) => {
    ctx.lineTo(point.x * width, point.y * height);
  });
  ctx.stroke();
  ctx.restore();
}

export function ImageEditModal({
  open,
  imageName,
  imageSrc,
  isSubmitting = false,
  allowOutputOptions = false,
  imageAspectRatio = "1:1",
  imageAspectRatioOptions = [],
  imageResolutionTier = "sd",
  imageResolutionTierOptions = [],
  imageQuality = "high",
  imageQualityOptions = [],
  imageQualityDisabled = false,
  imageQualityDisabledReason = "",
  providerPlatform = "gpt-image",
  providerPlatformOptions = [],
  onImageAspectRatioChange,
  onImageResolutionTierChange,
  onImageQualityChange,
  onProviderPlatformChange,
  onClose,
  onSubmit,
}: ImageEditModalProps) {
  const imageRef = useRef<HTMLImageElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const previewFrameRef = useRef<HTMLDivElement>(null);
  const pointerActiveRef = useRef(false);

  const [prompt, setPrompt] = useState("");
  const [selectionMode, setSelectionMode] = useState(false);
  const [brushSize, setBrushSize] = useState(42);
  const [displaySize, setDisplaySize] = useState({ width: 0, height: 0 });
  const [naturalSize, setNaturalSize] = useState({ width: 0, height: 0 });
  const [previewFrameSize, setPreviewFrameSize] = useState({ width: 0, height: 0 });
  const [strokes, setStrokes] = useState<Stroke[]>([]);
  const [redoStrokes, setRedoStrokes] = useState<Stroke[]>([]);
  const [currentStroke, setCurrentStroke] = useState<Stroke | null>(null);
  const [brushCursor, setBrushCursor] = useState<BrushCursor | null>(null);
  const [isPromptExpanded, setIsPromptExpanded] = useState(false);

  const hasSelection = strokes.length > 0;
  const requestClose = useCallback(() => {
    if (isSubmitting) {
      return;
    }
    onClose();
  }, [isSubmitting, onClose]);

  useEffect(() => {
    if (!open) {
      return;
    }

    const scrollY = window.scrollY;
    const previousHtmlOverflow = document.documentElement.style.overflow;
    const previousBodyOverflow = document.body.style.overflow;
    const previousBodyPosition = document.body.style.position;
    const previousBodyTop = document.body.style.top;
    const previousBodyWidth = document.body.style.width;
    const previousBodyLeft = document.body.style.left;
    const previousBodyRight = document.body.style.right;

    document.documentElement.style.overflow = "hidden";
    document.body.style.overflow = "hidden";
    document.body.style.position = "fixed";
    document.body.style.top = `-${scrollY}px`;
    document.body.style.width = "100%";
    document.body.style.left = "0";
    document.body.style.right = "0";

    return () => {
      document.documentElement.style.overflow = previousHtmlOverflow;
      document.body.style.overflow = previousBodyOverflow;
      document.body.style.position = previousBodyPosition;
      document.body.style.top = previousBodyTop;
      document.body.style.width = previousBodyWidth;
      document.body.style.left = previousBodyLeft;
      document.body.style.right = previousBodyRight;
      window.scrollTo(0, scrollY);
    };
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }

    pointerActiveRef.current = false;
    setPrompt("");
    setSelectionMode(false);
    setBrushSize(42);
    setStrokes([]);
    setRedoStrokes([]);
    setCurrentStroke(null);
    setBrushCursor(null);
    setIsPromptExpanded(false);
  }, [open, imageSrc]);

  useEffect(() => {
    if (!open || !imageRef.current) {
      return;
    }

    const updateSize = () => {
      const element = imageRef.current;
      if (!element) {
        return;
      }
      const rect = element.getBoundingClientRect();
      setDisplaySize({
        width: Math.max(0, Math.round(rect.width)),
        height: Math.max(0, Math.round(rect.height)),
      });
      if (element.naturalWidth > 0 && element.naturalHeight > 0) {
        setNaturalSize({
          width: element.naturalWidth,
          height: element.naturalHeight,
        });
      }
    };

    updateSize();
    const observer = new ResizeObserver(() => updateSize());
    observer.observe(imageRef.current);
    window.addEventListener("resize", updateSize);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateSize);
    };
  }, [open, imageSrc]);

  useEffect(() => {
    if (!open || !previewFrameRef.current) {
      return;
    }

    const updatePreviewFrameSize = () => {
      const element = previewFrameRef.current;
      if (!element) {
        return;
      }
      setPreviewFrameSize({
        width: Math.max(0, Math.round(element.clientWidth)),
        height: Math.max(0, Math.round(element.clientHeight)),
      });
    };

    updatePreviewFrameSize();
    const observer = new ResizeObserver(() => updatePreviewFrameSize());
    observer.observe(previewFrameRef.current);
    window.addEventListener("resize", updatePreviewFrameSize);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updatePreviewFrameSize);
    };
  }, [open, imageSrc]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || displaySize.width <= 0 || displaySize.height <= 0) {
      return;
    }

    canvas.width = displaySize.width;
    canvas.height = displaySize.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      return;
    }

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    strokes.forEach((stroke) => {
      renderStroke(ctx, stroke, canvas.width, canvas.height, "rgba(80, 120, 255, 0.36)");
    });
    if (currentStroke) {
      renderStroke(ctx, currentStroke, canvas.width, canvas.height, "rgba(80, 120, 255, 0.36)");
    }
  }, [displaySize, strokes, currentStroke]);

  useEffect(() => {
    if (!selectionMode) {
      setBrushCursor(null);
    }
  }, [selectionMode]);

  useEffect(() => {
    if (!open) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }
      event.preventDefault();
      requestClose();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [open, requestClose]);

  const mapClientPoint = (clientX: number, clientY: number) => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return null;
    }
    const rect = canvas.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) {
      return null;
    }
    return {
      x: clampPoint((clientX - rect.left) / rect.width),
      y: clampPoint((clientY - rect.top) / rect.height),
      sizeRatio: brushSize / Math.min(rect.width, rect.height),
      offsetX: clientX - rect.left,
      offsetY: clientY - rect.top,
    };
  };

  const updateBrushCursor = (clientX: number, clientY: number) => {
    const point = mapClientPoint(clientX, clientY);
    if (!point) {
      setBrushCursor(null);
      return null;
    }
    setBrushCursor({
      x: point.offsetX,
      y: point.offsetY,
    });
    return point;
  };

  const handlePointerDown = (event: ReactPointerEvent<HTMLCanvasElement>) => {
    if (!selectionMode || isSubmitting) {
      return;
    }
    event.preventDefault();
    const point = updateBrushCursor(event.clientX, event.clientY);
    if (!point) {
      return;
    }

    pointerActiveRef.current = true;
    setRedoStrokes([]);
    setCurrentStroke({
      points: [{ x: point.x, y: point.y }],
      sizeRatio: point.sizeRatio,
    });
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLCanvasElement>) => {
    if (!selectionMode || isSubmitting) {
      return;
    }
    event.preventDefault();
    const point = updateBrushCursor(event.clientX, event.clientY);
    if (!pointerActiveRef.current) {
      return;
    }
    if (!point) {
      return;
    }
    setCurrentStroke((prev) =>
      prev
        ? {
            ...prev,
            points: [...prev.points, { x: point.x, y: point.y }],
          }
        : prev,
    );
  };

  const finishStroke = (options?: { hideCursor?: boolean }) => {
    pointerActiveRef.current = false;
    if (options?.hideCursor) {
      setBrushCursor(null);
    }
    setCurrentStroke((prev) => {
      if (!prev || prev.points.length === 0) {
        return null;
      }
      setStrokes((current) => [...current, prev]);
      return null;
    });
  };

  const handleUndo = () => {
    if (!hasSelection || isSubmitting) {
      return;
    }
    setStrokes((current) => {
      const next = [...current];
      const removed = next.pop();
      if (removed) {
        setRedoStrokes((redo) => [...redo, removed]);
      }
      return next;
    });
  };

  const handleRedo = () => {
    if (redoStrokes.length === 0 || isSubmitting) {
      return;
    }
    setRedoStrokes((current) => {
      const next = [...current];
      const restored = next.pop();
      if (restored) {
        setStrokes((strokesValue) => [...strokesValue, restored]);
      }
      return next;
    });
  };

  const handleClear = () => {
    if ((!hasSelection && !currentStroke) || isSubmitting) {
      return;
    }
    setStrokes([]);
    setRedoStrokes([]);
    setCurrentStroke(null);
    pointerActiveRef.current = false;
  };

  const buildMaskPayload = async (): Promise<MaskPayload> => {
    if (naturalSize.width <= 0 || naturalSize.height <= 0) {
      throw new Error("图片尺寸读取失败");
    }

    const exportCanvas = document.createElement("canvas");
    exportCanvas.width = naturalSize.width;
    exportCanvas.height = naturalSize.height;
    const ctx = exportCanvas.getContext("2d");
    if (!ctx) {
      throw new Error("无法创建遮罩");
    }

    ctx.fillStyle = "rgba(255,255,255,1)";
    ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
    ctx.globalCompositeOperation = "destination-out";
    strokes.forEach((stroke) => {
      renderStroke(ctx, stroke, exportCanvas.width, exportCanvas.height, "#000000");
    });

    const previewCanvas = document.createElement("canvas");
    previewCanvas.width = naturalSize.width;
    previewCanvas.height = naturalSize.height;
    const previewCtx = previewCanvas.getContext("2d");
    if (!previewCtx) {
      throw new Error("无法创建选区预览");
    }

    const sourceImage = imageRef.current;
    if (sourceImage) {
      previewCtx.drawImage(sourceImage, 0, 0, previewCanvas.width, previewCanvas.height);
    } else {
      previewCtx.fillStyle = "rgba(245,245,244,1)";
      previewCtx.fillRect(0, 0, previewCanvas.width, previewCanvas.height);
    }
    previewCtx.fillStyle = "rgba(15, 23, 42, 0.08)";
    previewCtx.fillRect(0, 0, previewCanvas.width, previewCanvas.height);
    strokes.forEach((stroke) => {
      renderStroke(previewCtx, stroke, previewCanvas.width, previewCanvas.height, "rgba(80, 120, 255, 0.42)");
    });

    return {
      dataUrl: exportCanvas.toDataURL("image/png"),
      previewDataUrl: previewCanvas.toDataURL("image/png"),
    };
  };

  const handleSubmit = async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) {
      toast.error("请输入编辑说明");
      return;
    }
    if (!hasSelection) {
      toast.error("请先点击“选择”并涂抹需要修改的区域");
      return;
    }

    try {
      const mask = await buildMaskPayload();
      await onSubmit({
        prompt: trimmedPrompt,
      mask,
      aspectRatio: allowOutputOptions ? imageAspectRatio : undefined,
      resolutionTier: allowOutputOptions ? imageResolutionTier : undefined,
      quality: allowOutputOptions ? imageQuality : undefined,
      providerPlatform,
    });
    } catch (error) {
      const message = error instanceof Error ? error.message : "提交编辑失败";
      toast.error(message);
    }
  };

  const currentResolutionTierLabel =
    imageResolutionTierOptions.find((item) => item.value === imageResolutionTier)
      ?.label ?? imageResolutionTier;

  if (!open) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-50 overflow-hidden overscroll-none bg-[var(--app-bg-root)] text-[var(--app-text-primary)] backdrop-blur-sm"
      onClick={requestClose}
      role="dialog"
      aria-modal="true"
      aria-label="选区编辑"
    >
      <div className="flex h-full flex-col touch-auto" onClick={(event) => event.stopPropagation()}>
        <header className="border-b border-[var(--app-border)] px-4 py-2.5 sm:px-5 sm:py-3">
          <div className="hide-scrollbar flex items-center gap-2 overflow-x-auto">
            <button
              type="button"
              onClick={requestClose}
              className="inline-flex size-9 shrink-0 items-center justify-center rounded-full text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]"
            >
              <X className="size-5" />
            </button>

            <div className="flex min-w-0 flex-1 items-center justify-end gap-3">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="hidden text-[11px] font-medium uppercase tracking-[0.18em] text-[var(--app-text-muted)] sm:inline sm:text-xs">
                  笔刷
                </span>
                <input
                  type="range"
                  min="16"
                  max="96"
                  step="2"
                  value={brushSize}
                  onChange={(event) => setBrushSize(Number(event.target.value))}
                  className="h-8 w-[112px] sm:w-[148px]"
                  style={{ accentColor: "var(--app-text-primary)" }}
                />
                <span className="min-w-9 text-right text-sm font-medium text-[var(--app-text-secondary)]">
                  {brushSize}px
                </span>
              </div>
              <button
                type="button"
                className="app-btn"
                onClick={handleUndo}
                disabled={!hasSelection || isSubmitting}
                aria-label="撤销"
                title="撤销"
              >
                <Undo2 className="size-4" />
                <span className="hidden sm:inline">撤销</span>
              </button>
              <button
                type="button"
                className="app-btn"
                onClick={handleRedo}
                disabled={redoStrokes.length === 0 || isSubmitting}
                aria-label="重做"
                title="重做"
              >
                <Redo2 className="size-4" />
                <span className="hidden sm:inline">重做</span>
              </button>
              <button
                type="button"
                className="app-btn"
                onClick={handleClear}
                disabled={(!hasSelection && !currentStroke) || isSubmitting}
                aria-label="清空"
                title="清空"
              >
                <Trash2 className="size-4" />
                <span className="hidden sm:inline">清空</span>
              </button>
              <button
                type="button"
                className={selectionMode ? "app-btn-primary" : "app-btn"}
                onClick={() => setSelectionMode((value) => !value)}
                disabled={isSubmitting}
              >
                <Brush className="size-4" />
                <span>{selectionMode ? "选择中" : "选择"}</span>
              </button>
            </div>
          </div>
        </header>

        <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 overflow-hidden px-4 py-2 sm:px-6 sm:py-3">
          <div
            ref={previewFrameRef}
            className="flex min-h-0 w-full flex-1 items-center justify-center overflow-auto rounded-[28px] border border-[var(--app-border)] bg-[var(--app-bg-sidebar)] p-3 touch-auto sm:p-6"
          >
            <div
              className="relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-2xl"
              style={{
                maxWidth: previewFrameSize.width > 0 ? previewFrameSize.width : undefined,
                maxHeight: previewFrameSize.height > 0 ? previewFrameSize.height : undefined,
              }}
            >
              {/* 原生 img 便于直接读取天然尺寸并与遮罩 canvas 做 1:1 叠加。 */}
              <img
                ref={imageRef}
                src={imageSrc}
                alt={imageName}
                className="block h-auto max-h-full w-auto max-w-full rounded-2xl border border-[var(--app-border)] bg-[var(--app-bg-elevated)] object-contain shadow-[0_24px_70px_rgba(0,0,0,0.35)]"
                style={{
                  maxWidth: previewFrameSize.width > 0 ? previewFrameSize.width : undefined,
                  maxHeight: previewFrameSize.height > 0 ? previewFrameSize.height : undefined,
                }}
                onLoad={(event) => {
                  const target = event.currentTarget;
                  setNaturalSize({
                    width: target.naturalWidth,
                    height: target.naturalHeight,
                  });
                  const rect = target.getBoundingClientRect();
                  setDisplaySize({
                    width: Math.round(rect.width),
                    height: Math.round(rect.height),
                  });
                }}
              />
              {displaySize.width > 0 && displaySize.height > 0 ? (
                <canvas
                  ref={canvasRef}
                  className={cn(
                    "absolute rounded-2xl touch-none",
                    selectionMode ? "pointer-events-auto cursor-none" : "pointer-events-none",
                  )}
                  style={{
                    width: displaySize.width,
                    height: displaySize.height,
                    touchAction: selectionMode ? "none" : "auto",
                  }}
                  onPointerDown={handlePointerDown}
                  onPointerMove={handlePointerMove}
                  onPointerEnter={(event) => {
                    if (!selectionMode || isSubmitting) {
                      return;
                    }
                    void updateBrushCursor(event.clientX, event.clientY);
                  }}
                  onPointerUp={() => finishStroke()}
                  onPointerLeave={() => finishStroke({ hideCursor: true })}
                  onPointerCancel={() => finishStroke({ hideCursor: true })}
                />
              ) : null}
              {selectionMode && brushCursor && displaySize.width > 0 && displaySize.height > 0 ? (
                <div
                  className="pointer-events-none absolute rounded-full border border-white/50 bg-white/10 shadow-[0_0_0_1px_rgba(0,0,0,0.45)]"
                  style={{
                    width: brushSize,
                    height: brushSize,
                    left: brushCursor.x - brushSize / 2,
                    top: brushCursor.y - brushSize / 2,
                  }}
                />
              ) : null}
            </div>
          </div>
        </div>

        <footer className="border-t border-[var(--app-border)] px-4 py-2.5 sm:px-6 sm:py-4">
          <div className="relative mx-auto flex w-full max-w-[920px] items-end gap-3 rounded-[28px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-3 py-2.5 shadow-[var(--app-shadow-floating)] sm:gap-4 sm:rounded-[32px] sm:px-5 sm:py-4">
            <div className="min-w-0 flex-1">
              {allowOutputOptions ? (
                <div className="hide-scrollbar -mx-1 mb-2 flex items-center gap-2 overflow-x-auto px-1 pb-1">
                  <ChipSelect<APIAccessPlatform>
                    value={providerPlatform}
                    options={providerPlatformOptions.map((item) => ({ value: item.value as APIAccessPlatform, label: item.label, disabled: item.disabled }))}
                    onChange={(value) => onProviderPlatformChange?.(value)}
                    triggerClassName="app-cs-trigger"
                    triggerIcon={<Cpu className="size-4 shrink-0" />}
                    triggerLabel={providerPlatformOptions.find((item) => item.value === providerPlatform)?.label ?? providerPlatform}
                  />

                  <ChipSelect<string>
                    value={imageAspectRatio}
                    options={imageAspectRatioOptions}
                    onChange={(value) => onImageAspectRatioChange?.(value)}
                    triggerClassName="app-cs-trigger"
                    triggerLabel={imageAspectRatioOptions.find((item) => item.value === imageAspectRatio)?.label ?? imageAspectRatio}
                  />

                  <ChipSelect<string>
                    value={imageResolutionTier}
                    options={imageResolutionTierOptions}
                    onChange={(value) => onImageResolutionTierChange?.(value)}
                    triggerClassName="app-cs-trigger"
                    triggerLabel={currentResolutionTierLabel}
                    title={currentResolutionTierLabel}
                  />

                  <ChipSelect<string>
                    value={imageQuality}
                    options={imageQualityOptions.map((item) => ({ value: item.value, label: `质量 ${item.label}`, description: item.description }))}
                    onChange={(value) => onImageQualityChange?.(value)}
                    triggerClassName="app-cs-trigger"
                    triggerLabel={`质量 ${imageQualityOptions.find((item) => item.value === imageQuality)?.label ?? imageQuality}`}
                    disabled={imageQualityDisabled}
                    title={imageQualityDisabled ? imageQualityDisabledReason : imageQualityOptions.find((item) => item.value === imageQuality)?.description}
                  />
                </div>
              ) : null}
            <div className="relative">
              {isPromptExpanded ? (
                <>
                <button
                  type="button"
                  className="absolute right-0 top-0 inline-flex size-7 items-center justify-center rounded-full text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] sm:hidden"
                  onClick={() => {
                    setIsPromptExpanded(false);
                  }}
                  aria-label="收起输入框"
                  title="收起输入框"
                >
                  <ChevronDown className="size-4" />
                </button>
                <textarea
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  onFocus={() => setIsPromptExpanded(true)}
                  placeholder="描述要怎么改"
                  className="pc-textarea min-h-[72px] max-h-[180px] pr-12 text-[14px] leading-6 sm:min-h-[88px] sm:max-h-none sm:pr-14 sm:text-[15px] sm:leading-7"
                />
                </>
              ) : (
                <>
                  <button
                    type="button"
                    className="flex h-10 w-full items-center px-1 pr-12 text-left text-[14px] leading-5 text-[var(--app-text-muted)] sm:hidden"
                    onClick={() => setIsPromptExpanded(true)}
                  >
                    <span className="block w-full truncate">{prompt.trim() || "描述要怎么改"}</span>
                  </button>
                  <textarea
                    value={prompt}
                    onChange={(event) => setPrompt(event.target.value)}
                    onFocus={() => setIsPromptExpanded(true)}
                    placeholder="描述要怎么改"
                    className="pc-textarea hidden min-h-[88px] text-[15px] leading-7 sm:block"
                  />
                </>
              )}
            </div>
            <div className="absolute right-3 bottom-2.5 flex shrink-0 items-end sm:right-5 sm:bottom-4">
              <button
                type="button"
                className="pc-submit"
                onClick={() => void handleSubmit()}
                disabled={isSubmitting}
                aria-label="提交编辑"
              >
                {isSubmitting ? <LoaderCircle className="size-4 animate-spin sm:size-5" /> : <ArrowUp className="size-4 sm:size-5" />}
              </button>
            </div>
          </div>
          </div>
        </footer>
      </div>
    </div>
  );
}
