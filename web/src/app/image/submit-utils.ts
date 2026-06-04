"use client";

import type {
  InpaintSourceReference,
  APIAccessPlatform,
  ImageModel,
  ImageQuality,
  ImageResolutionAccess,
} from "@/lib/api";
import type {
  ImageConversationTurn,
  ImageMode,
  StoredImage,
  StoredSourceImage,
} from "@/store/image-conversations";

export function buildConversationTitle(mode: ImageMode, prompt: string, scale = "") {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return scale || (mode === "edit" ? "编辑图片" : "新建生图会话");
  }
  if (trimmed.length <= 8) {
    return trimmed;
  }
  return `${trimmed.slice(0, 8)}...`;
}

export function createLoadingImages(count: number, conversationId: string) {
  return Array.from({ length: count }, (_, index) => ({
    id: `${conversationId}-${index}`,
    status: "loading" as const,
  }));
}

export function createConversationTurn(payload: {
  turnId: string;
  title: string;
  mode: ImageMode;
  prompt: string;
  model: ImageModel;
  count: number;
  size?: string;
  resolutionAccess?: ImageResolutionAccess;
  quality?: ImageQuality;
  providerPlatform?: APIAccessPlatform;
  modelId?: string;
  modelLabel?: string;
  vendor?: string;
  vendorLabel?: string;
  adapter?: string;
  scale?: string;
  compareBatchId?: string;
  compareGroupId?: string;
  compareModelLabel?: string;
  compareModelIndex?: number;
  compareModelCount?: number;
  sourceImages?: StoredSourceImage[];
  hasAttachment?: boolean;
  sourceReference?: InpaintSourceReference;
  images: StoredImage[];
  createdAt: string;
  status: "queued" | "running" | "generating" | "success" | "error" | "cancelled";
  error?: string;
  jobId?: string;
}): ImageConversationTurn {
  return {
    id: payload.turnId,
    title: payload.title,
    mode: payload.mode,
    prompt: payload.prompt,
    model: payload.model,
    count: payload.count,
    size: payload.size,
    resolutionAccess: payload.resolutionAccess,
    quality: payload.quality,
    providerPlatform: payload.providerPlatform,
    modelId: payload.modelId,
    modelLabel: payload.modelLabel,
    vendor: payload.vendor,
    vendorLabel: payload.vendorLabel,
    adapter: payload.adapter,
    scale: payload.scale,
    compareBatchId: payload.compareBatchId || payload.compareGroupId,
    compareGroupId: payload.compareGroupId || payload.compareBatchId,
    compareModelLabel: payload.compareModelLabel,
    compareModelIndex: payload.compareModelIndex,
    compareModelCount: payload.compareModelCount,
    sourceImages: payload.sourceImages ?? [],
    hasAttachment: Boolean(payload.hasAttachment || payload.sourceImages?.length),
    sourceReference: payload.sourceReference,
    images: payload.images.map((image) => ({
      ...image,
      jobId: image.jobId || payload.jobId,
    })),
    createdAt: payload.createdAt,
    status: payload.status,
    error: payload.error,
    jobId: payload.jobId,
  };
}

export async function dataUrlToFile(dataUrl: string, fileName: string) {
  const response = await fetch(dataUrl);
  const blob = await response.blob();
  return new File([blob], fileName, { type: blob.type || "image/png" });
}

export function mergeResultImages(
  conversationId: string,
  items: Array<{
    url?: string;
    b64_json?: string;
    revised_prompt?: string;
    file_id?: string;
    gen_id?: string;
    conversation_id?: string;
    parent_message_id?: string;
    source_account_id?: string;
  }>,
  expected: number,
  jobId?: string,
) {
  const results: StoredImage[] = items.map((item, index) =>
    item.b64_json || item.url
      ? {
          id: `${conversationId}-${index}`,
          jobId,
          status: "success",
          b64_json: item.b64_json,
          url: item.url,
          revised_prompt: item.revised_prompt,
          file_id: item.file_id,
          gen_id: item.gen_id,
          conversation_id: item.conversation_id,
          parent_message_id: item.parent_message_id,
          source_account_id: item.source_account_id,
        }
      : {
          id: `${conversationId}-${index}`,
          jobId,
          status: "error",
          error: "接口没有返回图片数据",
        },
  );

  while (results.length < expected) {
    results.push({
      id: `${conversationId}-${results.length}`,
      jobId,
      status: "error",
      error: "接口返回的图片数量不足",
    });
  }
  return results;
}

export function countFailures(images: StoredImage[]) {
  return images.filter((image) => image.status === "error").length;
}

function isCreditFailure(normalized: string) {
  return (
    normalized.includes("点数") ||
    normalized.includes("余额不足") ||
    normalized.includes("insufficient") ||
    normalized.includes("credit")
  );
}

function isInputMissingFailure(normalized: string) {
  return (
    normalized.includes("请输入提示词") ||
    normalized.includes("prompt is required") ||
    normalized.includes("invalid_request") ||
    normalized.includes("编辑模式至少需要一张源图") ||
    normalized.includes("编辑模式需要提示词")
  );
}

function isQueueOrConcurrencyFailure(normalized: string) {
  return (
    normalized.includes("请求较多") ||
    normalized.includes("前方爆满") ||
    normalized.includes("排队") ||
    normalized.includes("队列") ||
    normalized.includes("queue") ||
    normalized.includes("capacity") ||
    normalized.includes("concurrency") ||
    normalized.includes("image_queue_full") ||
    normalized.includes("image_queue_timeout") ||
    normalized.includes("image_user_job_limit") ||
    normalized.includes("image_provider_job_limit")
  );
}

function isUpstreamProviderFailure(normalized: string) {
  return (
    normalized.includes("provider_error") ||
    normalized.includes("provider_request_failed") ||
    normalized.includes("provider_response_failed") ||
    normalized.includes("cloudflare") ||
    normalized.includes("bad gateway") ||
    normalized.includes("502") ||
    normalized.includes("503") ||
    normalized.includes("504") ||
    normalized.includes("too many requests") ||
    normalized.includes("rate limit") ||
    normalized.includes("origin_bad_gateway") ||
    normalized.includes("upstream") ||
    normalized.includes("上游")
  );
}

function isInternalProviderConfigurationFailure(normalized: string) {
  return (
    normalized.includes("号池") ||
    normalized.includes("provider pool") ||
    normalized.includes("provider_pool") ||
    normalized.includes("api 接入") ||
    normalized.includes("api_access") ||
    normalized.includes("image_base_url") ||
    normalized.includes("image_api_key") ||
    normalized.includes("unsupported image_provider") ||
    normalized.includes("unsupported api_access") ||
    normalized.includes("baseurl is required") ||
    normalized.includes("apikey is required") ||
    normalized.includes("base_url is required") ||
    normalized.includes("api_key is required") ||
    normalized.includes("provider_not_configured") ||
    normalized.includes("provider pool member") ||
    normalized.includes("pool member")
  );
}

export function formatImageErrorMessage(message: string) {
  const trimmed = String(message || "").trim();
  if (!trimmed) {
    return "生成失败，请稍后重试。";
  }

  const normalized = trimmed.toLowerCase();
  if (isCreditFailure(normalized)) {
    return "点数余额不足，请充值后再试。";
  }

  if (isInputMissingFailure(normalized)) {
    return "请输入提示词后再生成。";
  }

  if (isQueueOrConcurrencyFailure(normalized)) {
    return "当前生成请求较多，请稍后再试。";
  }

  if (normalized.includes("no images generated") && normalized.includes("model may have refused")) {
    return "内容可能未通过模型安全检查，请调整提示词后重试。";
  }

  if (normalized.includes("an error occurred while processing your request")) {
    return "提示词较长或当前规格较高，请简化描述或降低规格后重试。";
  }

  if (normalized.includes("timed out waiting for async image generation")) {
    return "生成等待超时，请稍后重试或降低规格。";
  }

  if (isInternalProviderConfigurationFailure(normalized)) {
    return "服务繁忙，请稍后重试。";
  }

  if (isUpstreamProviderFailure(normalized)) {
    return "服务繁忙，请稍后重试。";
  }

  return "生成失败，请稍后重试。";
}

export function formatImageError(error: unknown) {
  return formatImageErrorMessage(error instanceof Error ? error.message : String(error || "处理图片失败"));
}

export function buildInpaintSourceReference(image: StoredImage): InpaintSourceReference | undefined {
  if (!image.file_id || !image.gen_id || !image.source_account_id) {
    return undefined;
  }
  return {
    original_file_id: image.file_id,
    original_gen_id: image.gen_id,
    conversation_id: image.conversation_id,
    parent_message_id: image.parent_message_id,
    source_account_id: image.source_account_id,
  };
}

function extractErrorCode(error: unknown) {
  if (!error || typeof error !== "object" || !("code" in error)) {
    return "";
  }
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : "";
}

export function shouldFallbackSelectionEdit(error: unknown) {
  const code = extractErrorCode(error);
  if (["source_account_not_found", "source_account_unavailable", "source_context_missing"].includes(code)) {
    return true;
  }

  const normalized = (error instanceof Error ? error.message : String(error || "")).toLowerCase();
  return (
    normalized.includes("conversation not found") ||
    normalized.includes("source account") ||
    normalized.includes("image account is unavailable") ||
    normalized.includes("原始图片") ||
    normalized.includes("所属账号")
  );
}
