"use client";

import { useCallback, useRef } from "react";
import { toast } from "sonner";

import {
  generateImageWithOptions,
  type APIAccessPlatform,
  type ImageSourcePayload,
  type ImageModel,
  type ImageQuality,
  type ImageResolutionAccess,
} from "@/lib/api";
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
  mergeResultImages,
} from "../submit-utils";
import { buildSourceRequestImageUrl } from "../view-utils";

type UseImageSubmitOptions = {
  mode: ImageMode;
  imagePrompt: string;
  imageModel: ImageModel;
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
    scale: draftTurn.scale,
    sourceImages: draftTurn.sourceImages,
    images: draftTurn.images,
    createdAt: draftTurn.createdAt,
    status: draftTurn.status,
    error: draftTurn.error,
    turns: [draftTurn],
  };
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
  return platform === "gemini-banana" ? "gemini-2.5-flash-image" : "gpt-image-2";
}

function imageModelForPlatform(
  platform: APIAccessPlatform,
  model: ImageModel | undefined,
): ImageModel {
  const trimmed = String(model || "").trim();
  if (platform === "gemini-banana" && (!trimmed || trimmed.startsWith("gpt-image-"))) {
    return defaultImageModelForPlatform(platform);
  }
  if (platform === "gpt-image" && (!trimmed || trimmed.startsWith("gemini-"))) {
    return defaultImageModelForPlatform(platform);
  }
  return trimmed || defaultImageModelForPlatform(platform);
}

export function useImageSubmit({
  mode,
  imagePrompt,
  imageModel,
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
    }: {
      prompt: string;
      mask: {
        dataUrl: string;
        previewDataUrl: string;
      };
      aspectRatio?: string;
      resolutionTier?: string;
      quality?: string;
    }) => {
      if (isSelectionEditDispatchingRef.current || !editorTarget) {
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
      const turnId = makeId();
      const jobId = makeId();
      const now = new Date().toISOString();
      const requestModel = imageModelForPlatform(providerPlatform, imageModel);
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
        providerPlatform,
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
          model: requestModel,
          count: 1,
          size: supportsEditableOutputOptions ? imageSize : undefined,
          quality: nextQuality,
          platform: providerPlatform,
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
        const message =
          error instanceof Error ? error.message : "提交编辑失败";
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
      focusConversation,
      imageModel,
      imageQuality,
      imageResolutionAccess,
      imageSize,
      providerPlatform,
      makeId,
      persistConversation,
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
      const requestModel = imageModelForPlatform(turnProviderPlatform, turn.model);
      const isSingleImageRetry =
        turnMode === "generate" &&
        typeof imageIndex === "number" &&
        imageIndex >= 0 &&
        (turn.count || 1) > 1;
      const displayCount = Math.max(1, turn.count || 1);
      const requestCount = isSingleImageRetry ? 1 : displayCount;

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
        sourceImages: turnSourceImages,
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
          model: requestModel,
          count: requestCount,
          size: turn.size,
          quality: turnQuality,
          platform: turnProviderPlatform,
          jobId: retryJobId,
          conversationId,
          turnId: turn.id,
          title: buildConversationTitle(turnMode, prompt),
          sourceImages: turnMode === "edit" ? sourceImagePayloads(turnSourceImages) : undefined,
          sourceReference: turn.sourceReference,
        });
        const resultImages = mergeResultImages(
          turn.id,
          response.data || [],
          requestCount,
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
        const message =
          error instanceof Error ? error.message : "提交任务失败";
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
    const turnId = makeId();
    const jobId = makeId();
    const expectedCount = mode === "generate" ? parsedCount : 1;
    const requestModel = imageModelForPlatform(providerPlatform, imageModel);
    const draftTurn = createConversationTurn({
      turnId,
      title: buildConversationTitle(mode, prompt),
      mode,
      prompt,
      model: requestModel,
      count: expectedCount,
      size: imageSize,
      resolutionAccess: imageResolutionAccess,
      quality: imageQuality,
      providerPlatform,
      sourceImages,
      images: createLoadingImages(expectedCount, turnId),
      createdAt: new Date().toISOString(),
      status: "queued",
      jobId,
    });

    setSubmitElapsedSeconds(0);
    focusConversation(conversationId);
    setImagePrompt("");
    setSourceImages([]);

    try {
      if (selectedConversationId) {
        await updateConversation(conversationId, (current) => ({
          ...(current ?? buildConversationBase(conversationId, draftTurn)),
          turns: [...(current?.turns ?? []), draftTurn],
        }));
      } else {
        await persistConversation(
          buildConversationBase(conversationId, draftTurn),
        );
      }

      const response = await generateImageWithOptions(prompt, {
        mode,
        model: requestModel,
        count: expectedCount,
        size: imageSize,
        quality: imageQuality,
        platform: providerPlatform,
        jobId,
        conversationId,
        turnId,
        title: draftTurn.title,
        sourceImages: mode === "edit" ? sourceImagePayloads(sourceImages) : undefined,
      });
      const resultImages = mergeResultImages(
        turnId,
        response.data || [],
        expectedCount,
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
                    ? "部分图片生成失败"
                    : undefined,
                  images: resultImages,
                }
              : turn,
          ),
        }));
        toast.success("图片生成完成");
      }
      resetComposer(mode === "generate" ? "generate" : "edit");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "提交任务失败";
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
      isSubmitDispatchingRef.current = false;
      onSubmitSettled?.();
    }
  }, [
    focusConversation,
    imageModel,
    imagePrompt,
    providerPlatform,
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
