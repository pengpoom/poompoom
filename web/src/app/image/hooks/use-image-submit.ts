"use client";

import { useCallback, useRef } from "react";
import { toast } from "sonner";

import {
  generateImageWithOptions,
  type APIAccessPlatform,
  type ImageModelId,
  type ImageSourcePayload,
  type ImageModel,
  type ImageQuality,
  type ImageResolutionAccess,
} from "@/lib/api";
import {
  defaultModelForPlatform as defaultProviderModelForPlatform,
  isGeminiPlatform,
} from "@/lib/provider-platforms";
import type {
  ImageConversation,
  ImageConversationTurn,
  ImageMode,
  StoredSourceImage,
} from "@/store/image-conversations";

import type { EditorTarget } from "./use-image-source-inputs";
import {
  buildConversationTitle,
  buildInpaintSourceReference,
  createConversationTurn,
  createLoadingImages,
  formatImageError,
  mergeResultImages,
} from "../submit-utils";
import { buildSourceRequestImageUrl } from "../view-utils";

export type CompareModelSelection = {
  id: ImageModelId;
  label: string;
  vendor: string;
  vendorLabel: string;
  adapter: string;
  platform: APIAccessPlatform;
  model: ImageModel;
};

export type ImageModelSelection = CompareModelSelection;

type UseImageSubmitOptions = {
  mode: ImageMode;
  imagePrompt: string;
  selectedModel: ImageModelSelection;
  editModels: ImageModelSelection[];
  compareEnabled: boolean;
  compareModels: ImageModelSelection[];
  imageSources: StoredSourceImage[];
  sourceImages: StoredSourceImage[];
  parsedCount: number;
  imageSize: string;
  imageResolutionAccess: ImageResolutionAccess;
  imageQuality: ImageQuality;
  providerPlatform: APIAccessPlatform;
  selectedConversationId: string | null;
  editorTarget: EditorTarget | null;
  makeId: () => string;
  focusConversation: (conversationId: string) => void;
  closeSelectionEditor: () => void;
  setImagePrompt: (value: string) => void;
  setSourceImages: (value: StoredSourceImage[]) => void;
  setSubmitElapsedSeconds: (value: number) => void;
  persistConversation: (conversation: ImageConversation) => Promise<void>;
  updateConversation: (
    conversationId: string,
    updater: (current: ImageConversation | null) => ImageConversation,
  ) => Promise<void>;
  resetComposer: (nextMode?: ImageMode) => void;
  onSubmitSettled?: () => void;
};

function buildConversationBase(
  conversationId: string,
  draftTurn: ImageConversationTurn,
): ImageConversation {
  return {
    id: conversationId,
    title: draftTurn.title,
    mode: draftTurn.mode,
    prompt: draftTurn.prompt,
    model: draftTurn.model,
    count: draftTurn.count,
    size: draftTurn.size,
    resolutionAccess: draftTurn.resolutionAccess,
    quality: draftTurn.quality,
    providerPlatform: draftTurn.providerPlatform,
    modelId: draftTurn.modelId,
    modelLabel: draftTurn.modelLabel,
    vendor: draftTurn.vendor,
    vendorLabel: draftTurn.vendorLabel,
    adapter: draftTurn.adapter,
    scale: draftTurn.scale,
    compareBatchId: draftTurn.compareBatchId,
    compareGroupId: draftTurn.compareGroupId,
    compareModelLabel: draftTurn.compareModelLabel,
    compareModelIndex: draftTurn.compareModelIndex,
    compareModelCount: draftTurn.compareModelCount,
    sourceImages: draftTurn.sourceImages,
    images: draftTurn.images,
    createdAt: draftTurn.createdAt,
    status: draftTurn.status,
    error: draftTurn.error,
    turns: [draftTurn],
  };
}

function resultImageCount(items: Array<{ url?: string; b64_json?: string }>) {
  return items.filter((item) => item.url || item.b64_json).length;
}

function shouldRunModelCompare(
  mode: ImageMode,
  enabled: boolean,
  models: ImageModelSelection[],
) {
  return mode === "generate" && enabled && models.length > 1;
}

function buildSourceReference(payload: {
  id: string;
  role: "image" | "mask";
  name: string;
  url: string;
}): StoredSourceImage {
  if (payload.url.startsWith("data:")) {
    return {
      id: payload.id,
      role: payload.role,
      name: payload.name,
      dataUrl: payload.url,
    };
  }
  return {
    id: payload.id,
    role: payload.role,
    name: payload.name,
    url: payload.url,
  };
}

function sourceImagePayloads(items: StoredSourceImage[]): ImageSourcePayload[] {
  return items
    .map((item) => ({
      id: item.id,
      role: item.role,
      name: item.name,
      dataUrl: item.dataUrl,
      url: item.url,
    }))
    .filter((item) => Boolean(item.dataUrl || item.url));
}

function normalizeImageQuality(value: string | undefined, fallback: ImageQuality) {
  const trimmed = String(value || "").trim();
  if (trimmed === "low" || trimmed === "medium" || trimmed === "high") {
    return trimmed;
  }
  return fallback;
}

function defaultImageModelForPlatform(platform: APIAccessPlatform): ImageModel {
  return defaultProviderModelForPlatform(platform);
}

function imageModelForPlatform(
  platform: APIAccessPlatform,
  model: ImageModel | undefined,
): ImageModel {
  const trimmed = String(model || "").trim();
  if (isGeminiPlatform(platform) && (!trimmed || trimmed.startsWith("gpt-image-"))) {
    return defaultImageModelForPlatform(platform);
  }
  if (!isGeminiPlatform(platform) && (!trimmed || trimmed.startsWith("gemini-"))) {
    return defaultImageModelForPlatform(platform);
  }
  return trimmed || defaultImageModelForPlatform(platform);
}

function modelSelectionFromTurn(
  turn: ImageConversationTurn,
  fallback: ImageModelSelection,
): ImageModelSelection {
  const platform = turn.providerPlatform ?? fallback.platform;
  const model = imageModelForPlatform(platform, turn.model || fallback.model);
  return {
    id: String(turn.modelId || fallback.id || model).trim(),
    label: String(turn.modelLabel || turn.compareModelLabel || fallback.label || model).trim(),
    vendor: String(turn.vendor || fallback.vendor || "").trim(),
    vendorLabel: String(turn.vendorLabel || fallback.vendorLabel || "").trim(),
    adapter: String(turn.adapter || fallback.adapter || "").trim(),
    platform,
    model,
  };
}

export function useImageSubmit({
  mode,
  imagePrompt,
  selectedModel,
  editModels,
  compareEnabled,
  compareModels,
  imageSources,
  sourceImages,
  parsedCount,
  imageSize,
  imageResolutionAccess,
  imageQuality,
  providerPlatform,
  selectedConversationId,
  editorTarget,
  makeId,
  focusConversation,
  closeSelectionEditor,
  setImagePrompt,
  setSourceImages,
  setSubmitElapsedSeconds,
  persistConversation,
  updateConversation,
  resetComposer,
  onSubmitSettled,
}: UseImageSubmitOptions) {
  const isSelectionEditDispatchingRef = useRef(false);
  const isSubmitDispatchingRef = useRef(false);
  const retryingTurnIdsRef = useRef<Set<string>>(new Set());

  const handleSelectionEditSubmit = useCallback(
    async ({
      prompt,
      mask,
      aspectRatio: _aspectRatio,
      resolutionTier: _resolutionTier,
      quality: overrideQuality,
      modelId: overrideModelId,
    }: {
      prompt: string;
      mask: {
        dataUrl: string;
        previewDataUrl: string;
      };
      aspectRatio?: string;
      resolutionTier?: string;
      quality?: string;
      modelId?: ImageModelId;
    }) => {
      if (isSelectionEditDispatchingRef.current || !editorTarget) {
        return;
      }
      const baseSelection =
        (overrideModelId
          ? editModels.find((item) => item.id === overrideModelId)
          : undefined) ??
        (editorTarget.modelId
          ? editModels.find((item) => item.id === editorTarget.modelId)
          : undefined) ??
        (editorTarget.providerPlatform && editorTarget.model
          ? editModels.find(
              (item) =>
                item.platform === editorTarget.providerPlatform &&
                item.model === editorTarget.model,
            )
          : undefined) ??
        editModels.find((item) => item.id === selectedModel.id) ??
        editModels[0];
      if (!baseSelection) {
        toast.error("当前没有可用于编辑的模型");
        return;
      }
      isSelectionEditDispatchingRef.current = true;

      const sourceReference = editorTarget.image
        ? buildInpaintSourceReference(editorTarget.image)
        : undefined;
      const targetConversationId =
        editorTarget.conversationId ?? selectedConversationId;
      const conversationId = targetConversationId ?? makeId();
      const supportsEditableOutputOptions = true;
      const nextQuality = supportsEditableOutputOptions
        ? normalizeImageQuality(overrideQuality, imageQuality)
        : imageQuality;
      const nextProviderPlatform = baseSelection.platform;
      const turnId = makeId();
      const jobId = makeId();
      const now = new Date().toISOString();
      const requestModel = imageModelForPlatform(nextProviderPlatform, baseSelection.model);
      const draftTurn = createConversationTurn({
        turnId,
        title: buildConversationTitle("edit", prompt),
        mode: "edit",
        prompt,
        model: requestModel,
        count: 1,
        size: supportsEditableOutputOptions ? imageSize : undefined,
        resolutionAccess: supportsEditableOutputOptions
          ? imageResolutionAccess
          : undefined,
        quality: supportsEditableOutputOptions ? nextQuality : undefined,
        providerPlatform: nextProviderPlatform,
        modelId: baseSelection.id,
        modelLabel: baseSelection.label,
        vendor: baseSelection.vendor,
        vendorLabel: baseSelection.vendorLabel,
        adapter: baseSelection.adapter,
        sourceImages: [
          buildSourceReference({
            id: makeId(),
            role: "image",
            name: editorTarget.imageName,
            url: editorTarget.sourceDataUrl,
          }),
          {
            id: makeId(),
            role: "mask",
            name: "mask.png",
            dataUrl: mask.dataUrl,
            previewDataUrl: mask.previewDataUrl,
          },
        ],
        sourceReference,
        images: createLoadingImages(1, turnId),
        createdAt: now,
        status: "queued",
        jobId,
      });

      setSubmitElapsedSeconds(0);
      focusConversation(conversationId);
      setImagePrompt("");
      setSourceImages([]);
      closeSelectionEditor();

      try {
        if (targetConversationId) {
          await updateConversation(conversationId, (current) => {
            if (!current) {
              return buildConversationBase(conversationId, draftTurn);
            }
            return {
              ...current,
              turns: [...(current.turns ?? []), draftTurn],
            };
          });
        } else {
          await persistConversation(
            buildConversationBase(conversationId, draftTurn),
          );
        }

        const response = await generateImageWithOptions(prompt, {
          mode: "edit",
          modelId: baseSelection.id,
          model: requestModel,
          modelLabel: baseSelection.label,
          vendor: baseSelection.vendor,
          vendorLabel: baseSelection.vendorLabel,
          adapter: baseSelection.adapter,
          count: 1,
          size: supportsEditableOutputOptions ? imageSize : undefined,
          quality: nextQuality,
          platform: nextProviderPlatform,
          jobId,
          conversationId,
          turnId,
          title: draftTurn.title,
          sourceImages: sourceImagePayloads(draftTurn.sourceImages || []),
          sourceReference,
        });
        const resultImages = mergeResultImages(
          turnId,
          response.data || [],
          1,
          jobId,
        );
        const jobOnlyResponse =
          Boolean(response.jobId || response.job) &&
          (response.data || []).length === 0;

        if (!jobOnlyResponse) {
          await updateConversation(conversationId, (current) => ({
            ...(current ?? buildConversationBase(conversationId, draftTurn)),
            turns: (current?.turns ?? [draftTurn]).map((turn) =>
              turn.id === turnId
                ? {
                    ...turn,
                    status: resultImages.some((image) => image.status === "error")
                      ? "error"
                      : "success",
                    error: resultImages.some((image) => image.status === "error")
                      ? "图片编辑失败"
                      : undefined,
                    images: resultImages,
                  }
                : turn,
            ),
          }));
          toast.success("图片编辑完成");
        }
      } catch (error) {
        const message = formatImageError(error || "提交编辑失败");
        await updateConversation(conversationId, (current) => ({
          ...(current ?? buildConversationBase(conversationId, draftTurn)),
          turns: (current?.turns ?? [draftTurn]).map((turn) =>
            turn.id === turnId
              ? {
                  ...turn,
                  status: "error",
                  error: message,
                  images: turn.images.map((image) => ({
                    ...image,
                    status: "error" as const,
                    error: message,
                  })),
                }
              : turn,
          ),
        }));
        toast.error(message);
      } finally {
        isSelectionEditDispatchingRef.current = false;
        onSubmitSettled?.();
      }
    },
    [
      closeSelectionEditor,
      editorTarget,
      editModels,
      focusConversation,
      imageQuality,
      imageResolutionAccess,
      imageSize,
      makeId,
      persistConversation,
      selectedModel.id,
      selectedConversationId,
      setImagePrompt,
      setSourceImages,
      setSubmitElapsedSeconds,
      updateConversation,
      onSubmitSettled,
    ],
  );

  const handleRetryTurn = useCallback(
    async (
      conversationId: string,
      turn: ImageConversationTurn,
      imageIndex?: number,
    ) => {
      if (retryingTurnIdsRef.current.has(turn.id)) {
        return;
      }

      const prompt = turn.prompt?.trim() ?? "";
      const turnMode = turn.mode || "generate";
      const turnSourceImages = Array.isArray(turn.sourceImages)
        ? turn.sourceImages
        : [];
      const turnImageSources = turnSourceImages.filter(
        (item) => item.role === "image" && buildSourceRequestImageUrl(item),
      );
      const turnQuality = turn.quality || "high";
      const turnProviderPlatform = turn.providerPlatform ?? providerPlatform;
      const turnSelection = modelSelectionFromTurn(turn, selectedModel);
      const requestModel = imageModelForPlatform(turnProviderPlatform, turnSelection.model);
      const previousCount = Math.max(1, turn.count || 1);
      const isSingleImageRetry =
        turnMode === "generate" &&
        typeof imageIndex === "number" &&
        imageIndex >= 0 &&
        previousCount > 1;
      const displayCount = isSingleImageRetry ? previousCount : 1;
      const requestCount = 1;

      if (turnMode === "generate" && !prompt) {
        toast.error("该记录缺少提示词，无法重试");
        return;
      }
      if (turnMode === "edit" && turnImageSources.length === 0) {
        toast.error("该记录缺少源图，无法重试");
        return;
      }

      retryingTurnIdsRef.current.add(turn.id);
      const nextImages = isSingleImageRetry
        ? turn.images.map((image, index) =>
            index === imageIndex
              ? {
                  ...image,
                  status: "loading" as const,
                  error: undefined,
                }
              : image,
          )
        : createLoadingImages(requestCount, turn.id);
      const retryJobId = makeId();
      const draftTurn = createConversationTurn({
        turnId: turn.id,
        title: buildConversationTitle(turnMode, prompt),
        mode: turnMode,
        prompt,
        model: requestModel,
        count: displayCount,
        size: turn.size,
        resolutionAccess: turn.resolutionAccess,
        quality: turnQuality,
        providerPlatform: turnProviderPlatform,
        modelId: turnSelection.id,
        modelLabel: turnSelection.label,
        vendor: turnSelection.vendor,
        vendorLabel: turnSelection.vendorLabel,
        adapter: turnSelection.adapter,
        compareBatchId: turn.compareBatchId || turn.compareGroupId,
        compareGroupId: turn.compareGroupId || turn.compareBatchId,
        compareModelLabel: turn.compareModelLabel,
        compareModelIndex: turn.compareModelIndex,
        compareModelCount: turn.compareModelCount,
        sourceImages: turnSourceImages,
        hasAttachment: turnSourceImages.length > 0 || turn.hasAttachment,
        sourceReference: turn.sourceReference,
        images: nextImages,
        createdAt: new Date().toISOString(),
        status: isSingleImageRetry ? "running" : "queued",
        jobId: retryJobId,
      });

      setSubmitElapsedSeconds(0);
      focusConversation(conversationId);

      try {
        await updateConversation(conversationId, (current) => ({
          ...(current ?? buildConversationBase(conversationId, draftTurn)),
          turns:
            current?.turns?.map((item) =>
              item.id === turn.id ? draftTurn : item,
            ) ?? [draftTurn],
        }));

        const response = await generateImageWithOptions(prompt, {
          mode: turnMode,
          modelId: turnSelection.id,
          model: requestModel,
          modelLabel: turnSelection.label,
          vendor: turnSelection.vendor,
          vendorLabel: turnSelection.vendorLabel,
          adapter: turnSelection.adapter,
          count: requestCount,
          size: turn.size,
          quality: turnQuality,
          platform: turnProviderPlatform,
          jobId: retryJobId,
          conversationId,
          turnId: turn.id,
          title: buildConversationTitle(turnMode, prompt),
          compareBatchId: turn.compareBatchId || turn.compareGroupId,
          compareGroupId: turn.compareGroupId || turn.compareBatchId,
          compareModelLabel: turn.compareModelLabel,
          compareModelIndex: turn.compareModelIndex,
          compareModelCount: turn.compareModelCount,
          sourceImages: sourceImagePayloads(turnSourceImages),
          hasAttachment: turnSourceImages.length > 0 || turn.hasAttachment,
          sourceReference: turn.sourceReference,
        });
        const responseItems = response.data || [];
        const resultImages = mergeResultImages(
          turn.id,
          responseItems,
          Math.max(requestCount, resultImageCount(responseItems)),
          retryJobId,
        );
        const jobOnlyResponse =
          Boolean(response.jobId || response.job) &&
          (response.data || []).length === 0;

        if (!jobOnlyResponse) {
          await updateConversation(conversationId, (current) => ({
            ...(current ?? buildConversationBase(conversationId, draftTurn)),
            turns: (current?.turns ?? [draftTurn]).map((item) =>
              item.id === turn.id
                ? {
                    ...item,
                    status: resultImages.some((image) => image.status === "error")
                      ? "error"
                      : "success",
                    error: resultImages.some((image) => image.status === "error")
                      ? "部分图片生成失败"
                      : undefined,
                    images: isSingleImageRetry && typeof imageIndex === "number"
                      ? item.images.map((image, index) =>
                          index === imageIndex
                            ? (resultImages[0] ?? image)
                            : image,
                        )
                      : resultImages,
                  }
                : item,
            ),
          }));
          toast.success(isSingleImageRetry ? "失败图片已重新生成" : "已重新生成");
        }
      } catch (error) {
        const message = formatImageError(error || "提交任务失败");
        await updateConversation(conversationId, (current) => ({
          ...(current ?? buildConversationBase(conversationId, draftTurn)),
          turns: (current?.turns ?? [draftTurn]).map((item) =>
            item.id === turn.id
              ? {
                  ...item,
                  status:
                    isSingleImageRetry
                      ? item.images.some(
                          (image, index) =>
                            index !== imageIndex && image.status === "loading",
                        )
                        ? "running"
                        : "error"
                      : "error",
                  error: message,
                  images: isSingleImageRetry
                    ? item.images.map((image, index) =>
                        index === imageIndex
                          ? {
                              ...image,
                              status: "error" as const,
                              error: message,
                            }
                          : image,
                      )
                    : item.images.map((image) => ({
                        ...image,
                        status: "error" as const,
                        error: message,
                      })),
                }
              : item,
          ),
        }));
        toast.error(message);
      } finally {
        retryingTurnIdsRef.current.delete(turn.id);
        onSubmitSettled?.();
      }
    },
    [
      focusConversation,
      makeId,
      onSubmitSettled,
      providerPlatform,
      selectedModel,
      setSubmitElapsedSeconds,
      updateConversation,
    ],
  );

  const handleSubmit = useCallback(async () => {
    if (isSubmitDispatchingRef.current) {
      return;
    }
    const prompt = imagePrompt.trim();
    if (mode === "generate" && !prompt) {
      toast.error("请输入提示词");
      return;
    }
    if (mode === "edit" && imageSources.length === 0) {
      toast.error("编辑模式至少需要一张源图");
      return;
    }
    if (mode === "edit" && !prompt) {
      toast.error("编辑模式需要提示词");
      return;
    }
    isSubmitDispatchingRef.current = true;

    const conversationId = selectedConversationId ?? makeId();
    const expectedCount = mode === "generate" ? parsedCount : 1;
    const runCompare = shouldRunModelCompare(mode, compareEnabled, compareModels);
    const compareBatchId = runCompare ? makeId() : undefined;
    const selectedModels = runCompare ? compareModels : [selectedModel];
    const now = new Date().toISOString();
    const draftTurns = selectedModels.map((selection, index) => {
      const turnId = makeId();
      const requestModel = imageModelForPlatform(selection.platform, selection.model);
      return createConversationTurn({
        turnId,
        title: buildConversationTitle(mode, prompt),
        mode,
        prompt,
        model: requestModel,
        count: expectedCount,
        size: imageSize,
        resolutionAccess: imageResolutionAccess,
        quality: imageQuality,
        providerPlatform: selection.platform,
        modelId: selection.id,
        modelLabel: selection.label,
        vendor: selection.vendor,
        vendorLabel: selection.vendorLabel,
        adapter: selection.adapter,
        compareBatchId,
        compareGroupId: compareBatchId,
        compareModelLabel: runCompare ? selection.label : undefined,
        compareModelIndex: runCompare ? index : undefined,
        compareModelCount: runCompare ? selectedModels.length : undefined,
        sourceImages,
        hasAttachment: sourceImages.length > 0,
        images: createLoadingImages(expectedCount, turnId),
        createdAt: now,
        status: "queued",
        jobId: makeId(),
      });
    });
    const firstDraftTurn = draftTurns[0];

    setSubmitElapsedSeconds(0);
    focusConversation(conversationId);
    setImagePrompt("");
    setSourceImages([]);

    try {
      if (selectedConversationId) {
        await updateConversation(conversationId, (current) => ({
          ...(current ?? buildConversationBase(conversationId, firstDraftTurn)),
          turns: [...(current?.turns ?? []), ...draftTurns],
        }));
      } else {
        await persistConversation(
          {
            ...buildConversationBase(conversationId, firstDraftTurn),
            turns: draftTurns,
          },
        );
      }

      const submitResults = await Promise.allSettled(
        draftTurns.map(async (draftTurn) => {
          try {
            const response = await generateImageWithOptions(prompt, {
              mode,
              modelId: draftTurn.modelId,
              model: draftTurn.model,
              modelLabel: draftTurn.modelLabel,
              vendor: draftTurn.vendor,
              vendorLabel: draftTurn.vendorLabel,
              adapter: draftTurn.adapter,
              count: expectedCount,
              size: imageSize,
              quality: imageQuality,
              platform: draftTurn.providerPlatform,
              jobId: draftTurn.jobId,
              conversationId,
              turnId: draftTurn.id,
              title: draftTurn.title,
              compareBatchId: draftTurn.compareBatchId || draftTurn.compareGroupId,
              compareGroupId: draftTurn.compareGroupId || draftTurn.compareBatchId,
              compareModelLabel: draftTurn.compareModelLabel,
              compareModelIndex: draftTurn.compareModelIndex,
              compareModelCount: draftTurn.compareModelCount,
              sourceImages: sourceImagePayloads(sourceImages),
              hasAttachment: sourceImages.length > 0,
            });
            const responseItems = response.data || [];
            const resultImages = mergeResultImages(
              draftTurn.id,
              responseItems,
              Math.max(expectedCount, resultImageCount(responseItems)),
              draftTurn.jobId,
            );
            const jobOnlyResponse =
              Boolean(response.jobId || response.job) &&
              (response.data || []).length === 0;

            if (!jobOnlyResponse) {
              await updateConversation(conversationId, (current) => ({
                ...(current ?? buildConversationBase(conversationId, firstDraftTurn)),
                turns: (current?.turns ?? draftTurns).map((turn) =>
                  turn.id === draftTurn.id
                    ? {
                        ...turn,
                        status: resultImages.some((image) => image.status === "error")
                          ? "error"
                          : "success",
                        error: resultImages.some((image) => image.status === "error")
                          ? "部分图片生成失败"
                          : undefined,
                        images: resultImages,
                      }
                    : turn,
                ),
              }));
            }
          } catch (error) {
            const message = formatImageError(error || "提交任务失败");
            await updateConversation(conversationId, (current) => ({
              ...(current ?? buildConversationBase(conversationId, firstDraftTurn)),
              turns: (current?.turns ?? draftTurns).map((turn) =>
                turn.id === draftTurn.id
                  ? {
                      ...turn,
                      status: "error",
                      error: message,
                      images: turn.images.map((image) => ({
                        ...image,
                        status: "error" as const,
                        error: message,
                      })),
                    }
                  : turn,
              ),
            }));
            throw error;
          }
        }),
      );
      const failedSubmitCount = submitResults.filter(
        (result) => result.status === "rejected",
      ).length;
      if (failedSubmitCount > 0) {
        if (!runCompare || failedSubmitCount === draftTurns.length) {
          throw new Error("提交任务失败");
        }
        toast.warning(`部分模型提交失败，已提交 ${draftTurns.length - failedSubmitCount} 个对比任务`);
      } else {
        toast.success(runCompare ? "模型对比任务已提交" : "图片任务已提交");
      }
      resetComposer(mode === "generate" ? "generate" : "edit");
    } catch (error) {
      const message = formatImageError(error || "提交任务失败");
      toast.error(message);
    } finally {
      isSubmitDispatchingRef.current = false;
      onSubmitSettled?.();
    }
  }, [
    focusConversation,
    compareEnabled,
    compareModels,
    imagePrompt,
    imageSources,
    makeId,
    mode,
    imageSize,
    imageResolutionAccess,
    imageQuality,
    parsedCount,
    persistConversation,
    resetComposer,
    selectedConversationId,
    selectedModel,
    setImagePrompt,
    setSourceImages,
    setSubmitElapsedSeconds,
    sourceImages,
    updateConversation,
    onSubmitSettled,
  ]);

  return {
    handleSelectionEditSubmit,
    handleRetryTurn,
    handleSubmit,
  };
}
