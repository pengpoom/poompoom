"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ClipboardEvent as ReactClipboardEvent,
  type CSSProperties,
  type ReactNode,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import Zoom from "react-medium-image-zoom";
import { ArrowUp, Brain, Brush, Check, ChevronDown, Columns3, Cpu, Mic2, SquarePlus, Trash2 } from "lucide-react";

import { ChipSelect } from "@/components/chip-select";
import { HtmlImage as Image } from "@/components/html-image";
import type { BusinessImageModel, ImageQuality } from "@/lib/api";
import type { ImageModelSelection } from "../hooks/use-image-submit";
import type { ImageMode, StoredSourceImage } from "@/store/image-conversations";
import { cn } from "@/lib/utils";
import { buildSourceImageUrl } from "../view-utils";

type PromptComposerProps = {
  mode: ImageMode;
  modeOptions: Array<{ label: string; value: ImageMode; description: string }>;
  imageAspectRatio: string;
  imageAspectRatioOptions: Array<{ label: string; value: string }>;
  imageResolutionTier: string;
  imageResolutionTierLabel: string;
  imageResolutionTierOptions: Array<{ label: string; value: string; disabled?: boolean }>;
  imageSizeHint: ReactNode;
  selectedModelId: string;
  modelOptions: BusinessImageModel[];
  imageQuality: ImageQuality;
  imageQualityOptions: Array<{ label: string; value: ImageQuality; description: string }>;
  imageQualityDisabled: boolean;
  imageQualityDisabledReason: string;
  compareEnabled: boolean;
  compareModelOptions: ImageModelSelection[];
  selectedCompareModelIds: string[];
  sourceImages: StoredSourceImage[];
  imagePrompt: string;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  uploadInputRef: RefObject<HTMLInputElement | null>;
  maskInputRef: RefObject<HTMLInputElement | null>;
  onModeChange: (mode: ImageMode) => void;
  onImageAspectRatioChange: (value: string) => void;
  onImageResolutionTierChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onImageQualityChange: (value: string) => void;
  onCompareEnabledChange: (value: boolean) => void;
  onCompareModelToggle: (id: string) => void;
  onPromptChange: (value: string) => void;
  onPromptPaste: (event: ReactClipboardEvent<HTMLTextAreaElement>) => void;
  onRemoveSourceImage: (id: string) => void;
  onOpenSourceSelectionEditor: (sourceImageId: string) => void;
  onAppendFiles: (files: FileList | null, role: "image" | "mask") => Promise<void>;
  onMobileCollapsedChange?: (collapsed: boolean) => void;
  composerResetKey?: number;
  placement?: "bottom" | "inline";
  onSubmit: () => Promise<void>;
};

function SparkModeIcon({ mode }: { mode: ImageMode }) {
  if (mode === "edit") {
    return <Brush className="size-4" />;
  }
  return <Brain className="size-4" />;
}

function AspectPreviewIcon({ ratio }: { ratio: string }) {
  const className =
    ratio === "16:9"
      ? "h-2.5 w-5"
      : ratio === "9:16"
        ? "h-5 w-2.5"
        : ratio === "2:3"
          ? "h-5 w-3.5"
          : ratio === "3:2"
            ? "h-3.5 w-5"
            : ratio === "3:4"
              ? "h-5 w-4"
              : ratio === "4:3"
                ? "h-4 w-5"
                : "size-4";

  return (
    <span
      className={cn(
        "block rounded-[2px] border-2 border-current opacity-85",
        className,
      )}
      aria-hidden="true"
    />
  );
}

function AspectResolutionPicker({
  aspectRatio,
  aspectRatioOptions,
  resolutionTier,
  resolutionTierLabel,
  resolutionTierOptions,
  triggerClassName,
  onAspectRatioChange,
  onResolutionTierChange,
}: {
  aspectRatio: string;
  aspectRatioOptions: Array<{ label: string; value: string }>;
  resolutionTier: string;
  resolutionTierLabel: string;
  resolutionTierOptions: Array<{ label: string; value: string; disabled?: boolean }>;
  triggerClassName?: string;
  onAspectRatioChange: (value: string) => void;
  onResolutionTierChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [panelStyle, setPanelStyle] = useState<CSSProperties | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const currentAspectLabel =
    aspectRatioOptions.find((item) => item.value === aspectRatio)?.label ??
    aspectRatio;
  const currentResolutionLabel =
    resolutionTierLabel ||
    resolutionTierOptions.find((item) => item.value === resolutionTier)?.label ||
    resolutionTier;
  const compactResolutionLabel = currentResolutionLabel.toUpperCase();
  const triggerLabel = `${compactResolutionLabel} · ${currentAspectLabel}`;
  const closePanel = useCallback(() => {
    setOpen(false);
    setPanelStyle(null);
  }, []);

  useLayoutEffect(() => {
    if (!open) {
      return;
    }

    const updatePanelStyle = () => {
      const button = buttonRef.current;
      if (!button) {
        return;
      }

      const rect = button.getBoundingClientRect();
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;
      const margin = 12;
      const gap = 10;
      const panelWidth = Math.min(320, viewportWidth - margin * 2);
      const panelHeight = panelRef.current?.offsetHeight || 0;
      const left = Math.min(
        Math.max(rect.left, margin),
        Math.max(margin, viewportWidth - panelWidth - margin),
      );
      const hasRoomAbove = rect.top - gap - panelHeight > margin;
      const top = hasRoomAbove
        ? rect.top - gap - panelHeight
        : Math.min(rect.bottom + gap, viewportHeight - panelHeight - margin);

      setPanelStyle({
        position: "fixed",
        left,
        top: Math.max(margin, top),
        width: panelWidth,
        zIndex: 80,
      });
    };

    updatePanelStyle();
    window.addEventListener("resize", updatePanelStyle);
    window.addEventListener("scroll", updatePanelStyle, true);
    return () => {
      window.removeEventListener("resize", updatePanelStyle);
      window.removeEventListener("scroll", updatePanelStyle, true);
    };
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (panelRef.current?.contains(target) || buttonRef.current?.contains(target)) {
        return;
      }
      closePanel();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closePanel();
      }
    };
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [closePanel, open]);

  return (
    <div className="relative shrink-0" data-aspect-resolution-picker>
      <button
        ref={buttonRef}
        type="button"
        className={cn(triggerClassName, "inline-flex items-center transition")}
        onClick={() => {
          setOpen((current) => {
            if (current) {
              setPanelStyle(null);
            }
            return !current;
          });
        }}
        aria-expanded={open}
        aria-label="选择比例和分辨率"
      >
        <AspectPreviewIcon ratio={aspectRatio} />
        {triggerLabel}
        <ChevronDown className={cn("size-4 opacity-65 transition", open && "rotate-180")} />
      </button>

      {open && typeof document !== "undefined" ? createPortal(
        <div
          ref={panelRef}
          data-aspect-resolution-panel
          style={panelStyle ?? { position: "fixed", top: 0, left: 0, visibility: "hidden", zIndex: 80, width: 320 }}
          className="rounded-[20px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-4 text-[var(--app-text-primary)] shadow-[var(--app-shadow-floating)] backdrop-blur-2xl"
        >
          <div className="text-[13px] font-bold text-[var(--app-text-muted)]">比例</div>
          <div className="mt-3 grid grid-cols-4 gap-2 rounded-[18px] bg-[var(--app-bg-surface)] p-2">
            {aspectRatioOptions.map((item) => {
              const active = item.value === aspectRatio;
              return (
                <button
                  key={item.value}
                  type="button"
                  className={cn(
                    "grid min-h-[74px] place-items-center gap-1 rounded-[14px] px-2 py-2 text-[13px] font-semibold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                    active && "bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)] shadow-[var(--app-shadow-floating)]",
                  )}
                  onClick={() => onAspectRatioChange(item.value)}
                >
                  <AspectPreviewIcon ratio={item.value} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>

          <div className="mt-5 text-[13px] font-bold text-[var(--app-text-muted)]">分辨率</div>
          <div className="mt-3 grid grid-cols-3 gap-2 rounded-[18px] bg-[var(--app-bg-surface)] p-1.5">
            {resolutionTierOptions.map((item) => {
              const active = item.value === resolutionTier;
              return (
                <button
                  key={item.value}
                  type="button"
                  disabled={item.disabled}
                  className={cn(
                    "inline-flex h-11 items-center justify-center gap-2 rounded-[14px] text-[15px] font-bold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] disabled:cursor-not-allowed disabled:opacity-40",
                    active && "bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)] shadow-[var(--app-shadow-floating)]",
                  )}
                  onClick={() => onResolutionTierChange(item.value)}
                >
                  {item.label}
                  {active ? <Check className="size-4 text-[var(--app-accent-cyan)]" /> : null}
                </button>
              );
            })}
          </div>
        </div>
        , document.body) : null}
    </div>
  );
}

export function PromptComposer({
  mode,
  modeOptions,
  imageAspectRatio,
  imageAspectRatioOptions,
  imageResolutionTier,
  imageResolutionTierLabel,
  imageResolutionTierOptions,
  imageSizeHint,
  selectedModelId,
  modelOptions,
  imageQuality,
  imageQualityOptions,
  imageQualityDisabled,
  imageQualityDisabledReason,
  compareEnabled,
  compareModelOptions,
  selectedCompareModelIds,
  sourceImages,
  imagePrompt,
  textareaRef,
  uploadInputRef,
  maskInputRef,
  onModeChange,
  onImageAspectRatioChange,
  onImageResolutionTierChange,
  onModelChange,
  onImageQualityChange,
  onCompareEnabledChange,
  onCompareModelToggle,
  onPromptChange,
  onPromptPaste,
  onRemoveSourceImage,
  onOpenSourceSelectionEditor,
  onAppendFiles,
  onMobileCollapsedChange,
  composerResetKey,
  placement = "bottom",
  onSubmit,
}: PromptComposerProps) {
  const imageQualityLabel = imageQualityOptions.find((item) => item.value === imageQuality)?.label ?? imageQuality;
  const imageQualityPrefix = mode === "edit" ? "输出质量" : "清晰度";
  const modeLabel = modeOptions.find((item) => item.value === mode)?.label ?? "模式";
  const selectedModel = modelOptions.find((item) => item.id === selectedModelId) ?? modelOptions[0];
  const modelLabel = selectedModel
    ? selectedModel.displayName
    : "选择模型";
  const hasComposerContent = imagePrompt.trim().length > 0 || sourceImages.length > 0;
  const shouldFocusAfterExpandRef = useRef(false);
  const [isDesktopComposer, setIsDesktopComposer] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.matchMedia("(min-width: 768px)").matches;
  });
  const [isMobileComposerExpanded, setIsMobileComposerExpanded] = useState(hasComposerContent);
  const isMobileComposerCollapsed = !isMobileComposerExpanded;
  const isComposerCollapsed = !isDesktopComposer && isMobileComposerCollapsed;
  const showMobileExpandedSections = !isMobileComposerCollapsed;

  const focusTextarea = useCallback(() => {
    textareaRef.current?.focus();
  }, [textareaRef]);

  const expandAndFocusTextarea = useCallback(() => {
    shouldFocusAfterExpandRef.current = true;
    setIsMobileComposerExpanded(true);
    focusTextarea();
  }, [focusTextarea]);

  useEffect(() => {
    if (hasComposerContent) {
      setIsMobileComposerExpanded(true);
    }
  }, [hasComposerContent]);

  useEffect(() => {
    const media = window.matchMedia("(min-width: 768px)");
    const syncDesktopComposer = () => setIsDesktopComposer(media.matches);

    syncDesktopComposer();
    media.addEventListener("change", syncDesktopComposer);
    return () => media.removeEventListener("change", syncDesktopComposer);
  }, []);

  useEffect(() => {
    if (composerResetKey === undefined) {
      return;
    }
    shouldFocusAfterExpandRef.current = false;
    setIsMobileComposerExpanded(false);
  }, [composerResetKey]);

  useEffect(() => {
    if (!isMobileComposerExpanded || !shouldFocusAfterExpandRef.current) {
      return;
    }
    shouldFocusAfterExpandRef.current = false;
    const frame = window.requestAnimationFrame(focusTextarea);
    return () => window.cancelAnimationFrame(frame);
  }, [focusTextarea, isMobileComposerExpanded]);

  useEffect(() => {
    onMobileCollapsedChange?.(isMobileComposerCollapsed);
  }, [isMobileComposerCollapsed, onMobileCollapsedChange]);

  const controlButtonClass = "app-btn";

  const inlinePlacement = placement === "inline";
  const selectedCompareCount = selectedCompareModelIds.length;

  return (
    <div
      className={cn(
        inlinePlacement
          ? "relative z-10 mx-auto w-full max-w-[920px] px-0"
          : "fixed inset-x-0 bottom-0 z-30 px-3 pb-4 sm:px-6 md:absolute md:inset-x-8 md:bottom-8 md:p-0",
      )}
    >
      <div
        data-image-composer="panel"
        className={cn(
          "mx-auto w-full max-w-[920px] rounded-[18px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-3 py-3 shadow-[var(--app-shadow-floating)] backdrop-blur-2xl sm:px-4 sm:py-4",
          inlinePlacement
            ? "min-h-[132px]"
            : isComposerCollapsed
              ? "min-h-[86px] md:min-h-[118px]"
              : "min-h-[164px] md:min-h-[118px]",
        )}
        onPointerDown={(event) => {
          if (inlinePlacement || !isComposerCollapsed) {
            return;
          }
          event.preventDefault();
          expandAndFocusTextarea();
        }}
      >
        {sourceImages.length > 0 ? (
          <div
            className={cn(
              "hide-scrollbar mb-3 gap-3 overflow-x-auto border-b border-[var(--app-border)] pb-3",
              showMobileExpandedSections ? "flex" : "hidden md:flex",
            )}
          >
            {sourceImages.map((item) => (
              <div
                key={item.id}
                className="w-[112px] shrink-0 overflow-hidden rounded-xl border border-[var(--app-border)] bg-[var(--app-bg-surface)]"
              >
                <div className="flex items-center justify-between border-b border-[var(--app-border)] px-2.5 py-2 text-[11px] font-semibold text-[var(--app-text-muted)]">
                  <span>{item.role === "mask" ? "遮罩" : "参考"}</span>
                  <div className="flex items-center gap-1">
                    {mode === "edit" && item.role === "image" ? (
                      <button
                        type="button"
                        onClick={(event) => {
                          event.stopPropagation();
                          onOpenSourceSelectionEditor(item.id);
                        }}
                        className="rounded-md p-1 text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]"
                        title="选区编辑"
                        aria-label="选区编辑"
                      >
                        <Brush className="size-3.5" />
                      </button>
                    ) : null}
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        onRemoveSourceImage(item.id);
                      }}
                      className="rounded-md p-1 text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-rose-400"
                      aria-label="移除参考图"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </div>
                </div>
                <Zoom>
                  <Image
                    src={buildSourceImageUrl(item)}
                    alt={item.name}
                    width={160}
                    height={110}
                    unoptimized
                    className="block h-16 w-full cursor-zoom-in bg-black object-contain sm:h-20"
                  />
                </Zoom>
              </div>
            ))}
          </div>
        ) : null}

        <div className="relative">
          {isComposerCollapsed && !inlinePlacement ? (
            <button
              type="button"
              className="flex min-h-[42px] w-full items-start px-1 text-left text-[14px] font-medium leading-6 text-[var(--app-text-muted)]"
              onPointerDown={(event) => {
                event.preventDefault();
                expandAndFocusTextarea();
              }}
            >
              <span className="block w-full truncate">
                {imagePrompt.trim() ||
                  (mode === "generate"
                    ? "描述你想生成的图片，也可以上传参考"
                    : "描述你想如何修改当前图片")}
              </span>
            </button>
          ) : (
            <textarea
              ref={textareaRef}
              className="pc-textarea"
              value={imagePrompt}
              onChange={(event) => onPromptChange(event.target.value)}
              placeholder={
                mode === "generate"
                  ? "描述你想生成的图片，也可以上传参考"
                  : "描述你想如何修改当前图片"
              }
              onPaste={onPromptPaste}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  void onSubmit();
                }
              }}
              onFocus={() => setIsMobileComposerExpanded(true)}
            />
          )}
        </div>

        <div className={cn("mt-3", inlinePlacement || showMobileExpandedSections ? "block" : "hidden md:block")}>
          <div className="flex items-end justify-between gap-3">
            <div className="flex min-w-0 flex-1 items-center gap-2 overflow-hidden pb-0.5">
              <button
                type="button"
                className="app-btn"
                style={{ width: 36, padding: 0, justifyContent: "center", flexShrink: 0 }}
                onClick={(event) => {
                  event.stopPropagation();
                  uploadInputRef.current?.click();
                }}
                aria-label={mode === "generate" ? "上传参考图" : "上传源图"}
              >
                <SquarePlus />
              </button>

              <ChipSelect<ImageMode>
                value={mode}
                options={modeOptions.map((item) => ({ value: item.value, label: item.label, description: item.description }))}
                onChange={(value) => onModeChange(value)}
                triggerClassName={cn(controlButtonClass, "w-[96px] shrink-0 justify-center px-2")}
                triggerIcon={<SparkModeIcon mode={mode} />}
                triggerLabel={modeLabel}
              />

              <ChipSelect<string>
                value={selectedModelId}
                options={modelOptions.map((item) => ({
                  value: item.id,
                  label: `${item.displayName}${item.preview ? " · Preview" : ""}`,
                  disabled: !item.enabled,
                }))}
                onChange={(value) => onModelChange(value)}
                triggerClassName={cn(
                  controlButtonClass,
                  "w-[clamp(160px,20vw,260px)] shrink",
                  compareEnabled && mode === "generate" && "is-disabled",
                )}
                triggerIcon={<Cpu className="size-4" />}
                triggerLabel={modelLabel}
                disabled={compareEnabled && mode === "generate"}
                title={compareEnabled && mode === "generate" ? "多模型对比将使用下方选中的模型" : undefined}
              />

              {mode === "generate" ? (
                <button
                  type="button"
                  className={cn(
                    controlButtonClass,
                    "w-[132px] shrink-0 justify-center px-2",
                    compareEnabled &&
                      "border-[var(--app-accent-cyan)] bg-[rgba(91,214,255,0.18)] text-[var(--app-text-primary)] shadow-[0_0_0_1px_rgba(91,214,255,0.34),inset_0_1px_0_rgba(255,255,255,0.08)]",
                  )}
                  onClick={() => onCompareEnabledChange(!compareEnabled)}
                  aria-pressed={compareEnabled}
                  title="使用同一提示词同时生成多个模型结果"
                >
                  <Columns3 className="size-4" />
                  <span>模型对比</span>
                  {compareEnabled ? (
                    <span className="rounded-full bg-[var(--app-accent-cyan)] px-1.5 text-[11px] font-bold text-black">
                      {selectedCompareCount}
                    </span>
                  ) : null}
                </button>
              ) : null}

              <AspectResolutionPicker
                aspectRatio={imageAspectRatio}
                aspectRatioOptions={imageAspectRatioOptions}
                resolutionTier={imageResolutionTier}
                resolutionTierLabel={imageResolutionTierLabel}
                resolutionTierOptions={imageResolutionTierOptions}
                triggerClassName={cn(controlButtonClass, "w-[136px] shrink-0 px-2")}
                onAspectRatioChange={onImageAspectRatioChange}
                onResolutionTierChange={onImageResolutionTierChange}
              />

              <ChipSelect<string>
                value={imageQuality}
                options={imageQualityOptions.map((item) => ({
                  value: item.value,
                  label: `${imageQualityPrefix} ${item.label}`,
                  description: item.description,
                }))}
                onChange={onImageQualityChange}
                triggerClassName={cn(controlButtonClass, "w-[152px] shrink-0 px-2", imageQualityDisabled && "is-disabled")}
                triggerIcon={<Mic2 className="size-4" />}
                triggerLabel={`${imageQualityPrefix} ${imageQualityLabel}`}
                disabled={imageQualityDisabled}
                title={
                  imageQualityDisabled
                    ? imageQualityDisabledReason
                    : imageQualityOptions.find((item) => item.value === imageQuality)?.description
                }
              />

            </div>

            <button
              type="button"
              onClick={() => void onSubmit()}
              className="pc-submit"
              aria-label="提交图片任务"
            >
              <ArrowUp className="size-6" />
            </button>
          </div>

          {mode === "generate" && compareEnabled ? (
            <div className="mt-3 flex flex-wrap gap-2 border-t border-[var(--app-border)] pt-3">
              {compareModelOptions.map((item) => {
                const active = selectedCompareModelIds.includes(item.id);
                return (
                  <button
                    key={item.id}
                    type="button"
                    className={cn(
                      "inline-flex h-9 items-center gap-2 rounded-lg border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-3 text-[12px] font-bold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                      active &&
                        "border-[var(--app-accent-cyan)] bg-[rgba(91,214,255,0.12)] text-[var(--app-text-primary)] shadow-[0_0_0_1px_rgba(91,214,255,0.22)]",
                    )}
                    onClick={() => onCompareModelToggle(item.id)}
                    aria-pressed={active}
                    title={`${item.vendorLabel} · ${item.model}`}
                  >
                    <span
                      className={cn(
                        "grid size-4 place-items-center rounded border border-[var(--app-border-strong)]",
                        active && "border-[var(--app-accent-cyan)] bg-[var(--app-accent-cyan)] text-black",
                      )}
                      aria-hidden="true"
                    >
                      {active ? <Check className="size-3" /> : null}
                    </span>
                    <span className="max-w-[220px] truncate">{item.vendorLabel} · {item.label}</span>
                  </button>
                );
              })}
            </div>
          ) : null}
        </div>

        {isMobileComposerExpanded && !inlinePlacement ? (
          <button
            type="button"
            className="absolute right-3 top-3 grid size-8 place-items-center rounded-full text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] sm:hidden"
            onClick={(event) => {
              event.stopPropagation();
              setIsMobileComposerExpanded(false);
              textareaRef.current?.blur();
            }}
            aria-label="收起输入框"
            title="收起输入框"
          >
            <ChevronDown className="size-4" />
          </button>
        ) : null}

        <input
          ref={uploadInputRef}
          name="image_upload"
          type="file"
          accept="image/*"
          multiple
          className="hidden"
          onChange={(event) => {
            void onAppendFiles(event.target.files, "image");
            event.currentTarget.value = "";
          }}
        />
        <input
          ref={maskInputRef}
          name="mask_upload"
          type="file"
          accept="image/*"
          className="hidden"
          onChange={(event) => {
            void onAppendFiles(event.target.files, "mask");
            event.currentTarget.value = "";
          }}
        />
      </div>
    </div>
  );
}
