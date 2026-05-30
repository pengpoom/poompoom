"use client";

import localforage from "localforage";

import {
  fetchConfig,
  fetchBusinessImageJobs,
  type APIAccessPlatform,
  type BusinessImageJob,
  type ImageResponseItem,
  type ImageModel,
  type ImageQuality,
  type ImageResolutionAccess,
  type InpaintSourceReference,
} from "@/lib/api";
import webConfig from "@/constants/common-env";
import { httpRequest } from "@/lib/request";

export type ImageMode = "generate" | "edit";

export type StoredSourceImage = {
  id: string;
  role: "image" | "mask";
  name: string;
  dataUrl?: string;
  url?: string;
  previewDataUrl?: string;
};

export type StoredImage = {
  id: string;
  status?: "loading" | "success" | "error";
  b64_json?: string;
  url?: string;
  revised_prompt?: string;
  file_id?: string;
  gen_id?: string;
  conversation_id?: string;
  parent_message_id?: string;
  source_account_id?: string;
  error?: string;
};

export type ImageConversationStatus =
  | "queued"
  | "running"
  | "generating"
  | "success"
  | "error"
  | "cancelled";

export type ImageConversationTurn = {
  id: string;
  title: string;
  mode: ImageMode;
  prompt: string;
  model: ImageModel;
  count: number;
  size?: string;
  resolutionAccess?: ImageResolutionAccess;
  quality?: ImageQuality;
  providerPlatform?: APIAccessPlatform;
  scale?: string;
  sourceImages?: StoredSourceImage[];
  sourceReference?: InpaintSourceReference;
  images: StoredImage[];
  createdAt: string;
  status: ImageConversationStatus;
  error?: string;
  jobId?: string;
  queuePosition?: number;
  waitingReason?: string;
  waitingDetail?: string;
  waitingSince?: string;
  startedAt?: string;
  finishedAt?: string;
  cancelRequested?: boolean;
};

export type ImageConversation = {
  id: string;
  title: string;
  mode: ImageMode;
  prompt: string;
  model: ImageModel;
  count: number;
  size?: string;
  resolutionAccess?: ImageResolutionAccess;
  quality?: ImageQuality;
  providerPlatform?: APIAccessPlatform;
  scale?: string;
  sourceImages?: StoredSourceImage[];
  images: StoredImage[];
  createdAt: string;
  status: ImageConversationStatus;
  error?: string;
  turns?: ImageConversationTurn[];
};

export type ImageConversationStorageMode = "browser" | "server";

type BusinessImageConversation = {
  id: string;
  user_id: string;
  title: string;
  created_at: string;
  updated_at: string;
};

type BusinessImageGeneration = {
  id: string;
  user_id: string;
  conversation_id: string;
  turn_id: string;
  prompt: string;
  model: string;
  size?: string;
  quality?: string;
  count: number;
  status: string;
  response?: unknown;
  error?: string;
  created_at: string;
  finished_at?: string;
};

type BusinessImageConversationDetail = {
  conversation: BusinessImageConversation;
  generations: BusinessImageGeneration[];
  jobs?: BusinessImageJob[];
};

type BusinessJobPayloadSourceImage = {
  id?: unknown;
  role?: unknown;
  name?: unknown;
  dataUrl?: unknown;
  url?: unknown;
};

const imageConversationStorage = localforage.createInstance({
  name: "image-studio",
  storeName: "image_conversations",
});

const IMAGE_CONVERSATIONS_KEY = "items";
const IMAGE_CONVERSATION_STORAGE_MODE_KEY =
  "imagestudio:image-conversation-storage-mode";
let cachedConversations: ImageConversation[] | null = null;
let cachedConversationsStorageMode: ImageConversationStorageMode | null = null;
let loadPromise: Promise<ImageConversation[]> | null = null;
let writeQueue: Promise<void> = Promise.resolve();
let cachedImageConversationStorageMode: "browser" | "server" | null =
  readPersistedImageConversationStorageMode();

function sortConversations(items: ImageConversation[]) {
  return [...items].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}

function isBusinessProxyMode() {
  return webConfig.backendMode === "business_proxy";
}

function readPersistedImageConversationStorageMode(): ImageConversationStorageMode | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(
      IMAGE_CONVERSATION_STORAGE_MODE_KEY,
    );
    return raw === "server" ? "server" : raw === "browser" ? "browser" : null;
  } catch {
    return null;
  }
}

function persistImageConversationStorageMode(
  mode: ImageConversationStorageMode | null,
) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (mode) {
      window.localStorage.setItem(IMAGE_CONVERSATION_STORAGE_MODE_KEY, mode);
      return;
    }
    window.localStorage.removeItem(IMAGE_CONVERSATION_STORAGE_MODE_KEY);
  } catch {
    // Ignore localStorage write failures and keep using in-memory state.
  }
}

async function loadConversationCache(): Promise<ImageConversation[]> {
  if (cachedConversations && cachedConversationsStorageMode === "browser") {
    return cachedConversations;
  }

  if (!loadPromise) {
    loadPromise = imageConversationStorage
      .getItem<ImageConversation[]>(IMAGE_CONVERSATIONS_KEY)
      .then((items) => {
        cachedConversations = sortConversations(
          (items || []).map(normalizeConversation),
        );
        cachedConversationsStorageMode = "browser";
        return cachedConversations;
      })
      .finally(() => {
        loadPromise = null;
      });
  }

  return loadPromise;
}

export function getCachedImageConversationsSnapshot():
  | ImageConversation[]
  | null {
  if (!cachedConversations) {
    return null;
  }
  if (!cachedImageConversationStorageMode) {
    return null;
  }
  if (
    cachedConversationsStorageMode &&
    cachedImageConversationStorageMode &&
    cachedConversationsStorageMode !== cachedImageConversationStorageMode
  ) {
    return null;
  }
  return sortConversations(cachedConversations.map(normalizeConversation));
}

function setCachedConversationsSnapshot(
  items: ImageConversation[],
  storageMode: ImageConversationStorageMode,
) {
  cachedConversations = sortConversations(items.map(normalizeConversation));
  cachedConversationsStorageMode = storageMode;
  cachedImageConversationStorageMode = storageMode;
  return cachedConversations;
}

export function setCachedImageConversationStorageMode(
  mode: ImageConversationStorageMode | null,
) {
  cachedImageConversationStorageMode = mode;
  persistImageConversationStorageMode(mode);
}

async function persistConversationCache() {
  const snapshot = sortConversations(
    (cachedConversations || []).map(normalizeConversation),
  );
  cachedConversations = snapshot;
  cachedConversationsStorageMode = "browser";
  writeQueue = writeQueue.then(async () => {
    await imageConversationStorage.setItem(IMAGE_CONVERSATIONS_KEY, snapshot);
  });
  await writeQueue;
}

function normalizeStorageMode(
  value: string | null | undefined,
): ImageConversationStorageMode {
  return value === "server" ? "server" : "browser";
}

function toAbsoluteImageURL(raw: string | undefined) {
  const trimmed = String(raw || "").trim();
  if (!trimmed) {
    return "";
  }
  if (/^(data:|https?:\/\/)/i.test(trimmed)) {
    return trimmed;
  }
  const base = webConfig.apiUrl.replace(/\/$/, "");
  return `${base}${trimmed.startsWith("/") ? trimmed : `/${trimmed}`}`;
}

async function blobToDataURL(blob: Blob) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(new Error("读取图片失败"));
    reader.readAsDataURL(blob);
  });
}

async function imageURLToDataURL(raw: string) {
  const response = await fetch(toAbsoluteImageURL(raw), {
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(`读取图片失败 (${response.status})`);
  }
  const blob = await response.blob();
  return blobToDataURL(blob);
}

async function materializeConversationImagesForBrowser(
  conversation: ImageConversation,
): Promise<ImageConversation> {
  const materializedTurns = await Promise.all(
    (conversation.turns || []).map(async (turn) => {
      const sourceImages = await Promise.all(
        (turn.sourceImages || []).map(async (source) => {
          if (source.dataUrl || !source.url) {
            return source;
          }
          return {
            ...source,
            dataUrl: await imageURLToDataURL(source.url),
          };
        }),
      );
      const images = await Promise.all(
        (turn.images || []).map(async (image) => {
          if (image.b64_json || !image.url) {
            return image;
          }
          const dataUrl = await imageURLToDataURL(image.url);
          const commaIndex = dataUrl.indexOf(",");
          return {
            ...image,
            b64_json: commaIndex >= 0 ? dataUrl.slice(commaIndex + 1) : "",
          };
        }),
      );
      return normalizeTurn({
        ...turn,
        sourceImages,
        images,
      });
    }),
  );
  return normalizeConversation({
    ...conversation,
    turns: materializedTurns,
  });
}

function normalizeStoredImage(image: StoredImage): StoredImage {
  if (
    image.status === "loading" ||
    image.status === "error" ||
    image.status === "success"
  ) {
    return image;
  }
  return {
    ...image,
    status: image.b64_json || image.url ? "success" : "loading",
  };
}

function normalizeImageQuality(
  value: string | undefined,
): ImageQuality | undefined {
  return value === "low" || value === "medium" || value === "high"
    ? value
    : undefined;
}

function normalizeResolutionAccess(
  value: ImageConversationTurn["resolutionAccess"],
): ImageResolutionAccess | undefined {
  return value === "free" || value === "paid" ? value : undefined;
}

function normalizeImageMode(value: unknown): ImageMode {
  // Keep old local history readable after the deprecated upscale mode was removed.
  return value === "edit" || value === "upscale" ? "edit" : "generate";
}

function normalizeImageModel(value: unknown): ImageModel {
  const trimmed = String(value || "").trim();
  return trimmed || "gpt-image-2";
}

function normalizeAPIAccessPlatform(value: unknown): APIAccessPlatform | undefined {
  return value === "gpt-image" || value === "gemini-banana"
    ? value
    : undefined;
}

function buildBusinessImageTitle(prompt: string) {
  const trimmed = String(prompt || "").trim();
  if (!trimmed) {
    return "新建生图会话";
  }
  return trimmed.length <= 8
    ? trimmed
    : `${trimmed.slice(0, 8)}...`;
}

function businessGenerationTurnID(generation: BusinessImageGeneration) {
  const turnID = String(generation.turn_id || "").trim();
  const generationID = String(generation.id || "").trim();
  if (!turnID) {
    return generationID;
  }
  if (!generationID || generationID === turnID) {
    return turnID;
  }
  return `${turnID}-${generationID}`;
}

function businessGenerationStatus(
  generation: BusinessImageGeneration,
): ImageConversationStatus {
  const status = String(generation.status || "").trim().toLowerCase();
  if (status === "queued") {
    return "queued";
  }
  if (status === "running") {
    return "running";
  }
  if (status === "cancelled" || status === "canceled") {
    return "cancelled";
  }
  if (status === "failed" || status === "error" || generation.error) {
    return "error";
  }
  return "success";
}

function businessJobStatus(job?: BusinessImageJob): ImageConversationStatus | undefined {
  const status = String(job?.status || "").trim().toLowerCase();
  if (status === "queued") {
    return "queued";
  }
  if (status === "running") {
    return "running";
  }
  if (status === "failed" || status === "error") {
    return "error";
  }
  if (status === "cancel_requested") {
    return "cancelled";
  }
  if (status === "cancelled" || status === "canceled") {
    return "cancelled";
  }
  if (status === "succeeded" || status === "success") {
    return "success";
  }
  return undefined;
}

function businessJobByGenerationID(jobs?: BusinessImageJob[]) {
  const byGenerationID = new Map<string, BusinessImageJob>();
  for (const job of jobs || []) {
    const generationID = String(job.generationId || "").trim();
    if (!generationID) {
      continue;
    }
    const previous = byGenerationID.get(generationID);
    if (
      !previous ||
      String(job.updatedAt || "").localeCompare(String(previous.updatedAt || "")) >= 0
    ) {
      byGenerationID.set(generationID, job);
    }
  }
  return byGenerationID;
}

function mergeBusinessGenerationStatus(
  generation: BusinessImageGeneration,
  job?: BusinessImageJob,
) {
  const generationStatus = businessGenerationStatus(generation);
  const jobStatus = businessJobStatus(job);
  if (!jobStatus) {
    return generationStatus;
  }
  if (
    jobStatus === "queued" ||
    jobStatus === "running" ||
    jobStatus === "cancelled" ||
    jobStatus === "error"
  ) {
    return jobStatus;
  }
  return generationStatus;
}

function businessGenerationResponseItems(
  generation: BusinessImageGeneration,
): ImageResponseItem[] {
  if (!generation.response || typeof generation.response !== "object") {
    return [];
  }
  const data = (generation.response as { data?: unknown }).data;
  if (!Array.isArray(data)) {
    return [];
  }
  return data
    .filter((item): item is Record<string, unknown> =>
      Boolean(item && typeof item === "object"),
    )
    .map((item) => ({
      url: typeof item.url === "string" ? item.url : undefined,
      b64_json: typeof item.b64_json === "string" ? item.b64_json : undefined,
      revised_prompt:
        typeof item.revised_prompt === "string" ? item.revised_prompt : undefined,
      file_id: typeof item.file_id === "string" ? item.file_id : undefined,
      gen_id: typeof item.gen_id === "string" ? item.gen_id : undefined,
      conversation_id:
        typeof item.conversation_id === "string" ? item.conversation_id : undefined,
      parent_message_id:
        typeof item.parent_message_id === "string"
          ? item.parent_message_id
          : undefined,
      source_account_id:
        typeof item.source_account_id === "string"
          ? item.source_account_id
          : undefined,
      error: typeof item.error === "string" ? item.error : undefined,
    }));
}

function businessJobPayload(job?: BusinessImageJob): Record<string, unknown> {
  return job?.payload && typeof job.payload === "object" ? job.payload : {};
}

function businessTurnModeFromJob(job?: BusinessImageJob): ImageMode {
  const payload = businessJobPayload(job);
  const mode = String(payload.mode || "").trim();
  return mode === "edit" || Array.isArray(payload.sourceImages) ? "edit" : "generate";
}

function businessSourceImagesFromJob(job?: BusinessImageJob): StoredSourceImage[] {
  const sourceImages = businessJobPayload(job).sourceImages;
  if (!Array.isArray(sourceImages)) {
    return [];
  }
  return sourceImages
    .filter((item): item is BusinessJobPayloadSourceImage =>
      Boolean(item && typeof item === "object"),
    )
    .map((item, index): StoredSourceImage | null => {
      const dataUrl = String(item.dataUrl || "").trim();
      const url = String(item.url || "").trim();
      if (!dataUrl && !url) {
        return null;
      }
      const role = item.role === "mask" ? "mask" : "image";
      return {
        id: String(item.id || `${job?.id || "source"}-${index}`),
        role,
        name: String(item.name || (role === "mask" ? "mask.png" : "source.png")),
        dataUrl: dataUrl || undefined,
        url: url || undefined,
      };
    })
    .filter((item): item is StoredSourceImage => Boolean(item));
}

function businessSourceReferenceFromJob(
  job?: BusinessImageJob,
): InpaintSourceReference | undefined {
  const sourceReference = businessJobPayload(job).sourceReference;
  if (!sourceReference || typeof sourceReference !== "object") {
    return undefined;
  }
  return normalizeSourceReference(sourceReference as ImageConversationTurn["sourceReference"]);
}

function businessGenerationImages(
  generation: BusinessImageGeneration,
  turnID: string,
  statusOverride?: ImageConversationStatus,
  errorOverride?: string,
): StoredImage[] {
  const items = businessGenerationResponseItems(generation);
  const images = items.map((item, index): StoredImage => {
    if (item.b64_json || item.url) {
      return {
        id: `${turnID}-${index}`,
        status: "success",
        b64_json: item.b64_json,
        url: item.url,
        revised_prompt: item.revised_prompt,
        file_id: item.file_id,
        gen_id: item.gen_id,
        conversation_id: item.conversation_id,
        parent_message_id: item.parent_message_id,
        source_account_id: item.source_account_id,
      };
    }
    return {
      id: `${turnID}-${index}`,
      status: "error",
      error: item.error || errorOverride || generation.error || "接口没有返回图片数据",
    };
  });

  const expected = Math.max(1, generation.count || images.length || 1);
  while (images.length < expected) {
    images.push({
      id: `${turnID}-${images.length}`,
      status:
        statusOverride === "queued" ||
        statusOverride === "running" ||
        statusOverride === "generating"
          ? "loading"
          : "error",
      error: errorOverride || generation.error || "接口返回的图片数量不足",
    });
  }
  return images;
}

export function businessImageConversationDetailToConversation(
  detail: BusinessImageConversationDetail,
): ImageConversation {
  const conversation = detail.conversation;
  const jobsByGenerationID = businessJobByGenerationID(detail.jobs);
  const turns = [...(detail.generations || [])]
    .sort((a, b) => String(a.created_at || "").localeCompare(String(b.created_at || "")))
    .map((generation): ImageConversationTurn => {
      const turnID = businessGenerationTurnID(generation);
      const job = jobsByGenerationID.get(String(generation.id || "").trim());
      const status = mergeBusinessGenerationStatus(generation, job);
      const error = job?.userErrorMessage || job?.errorMessage || generation.error || undefined;
      const mode = businessTurnModeFromJob(job);
      const sourceImages = businessSourceImagesFromJob(job);
      const prompt = generation.prompt || job?.prompt || "";
      return {
        id: turnID,
        title: buildBusinessImageTitle(prompt),
        mode,
        prompt,
        model: normalizeImageModel(generation.model),
        count: Math.max(1, generation.count || 1),
        size: generation.size?.trim() || undefined,
        quality: normalizeImageQuality(generation.quality),
        providerPlatform: normalizeAPIAccessPlatform(
          (generation.response as { platform?: unknown } | undefined)?.platform,
        ),
        sourceImages,
        sourceReference: businessSourceReferenceFromJob(job),
        images: businessGenerationImages(generation, turnID, status, error),
        createdAt: generation.created_at || conversation.updated_at || conversation.created_at,
        status,
        error,
        jobId: generation.id || undefined,
        waitingDetail: job?.stage,
        waitingSince: job?.queuedAt,
        startedAt: job?.startedAt,
        finishedAt: job?.finishedAt || generation.finished_at || undefined,
        cancelRequested: job?.status === "cancel_requested",
      };
    });

  return normalizeConversation({
    id: conversation.id,
    title: conversation.title || turns[turns.length - 1]?.title || "新建生图会话",
    mode: "generate",
    prompt: turns[turns.length - 1]?.prompt || "",
    model: turns[turns.length - 1]?.model || "gpt-image-2",
    count: turns[turns.length - 1]?.count || 1,
    images: turns[turns.length - 1]?.images || [],
    createdAt: conversation.updated_at || conversation.created_at,
    status: turns[turns.length - 1]?.status || "success",
    turns,
  });
}

function normalizeTurn(turn: ImageConversationTurn): ImageConversationTurn {
  return {
    ...turn,
    mode: normalizeImageMode(turn.mode),
    resolutionAccess: normalizeResolutionAccess(turn.resolutionAccess),
    quality: normalizeImageQuality(turn.quality),
    providerPlatform: normalizeAPIAccessPlatform(turn.providerPlatform),
    sourceImages: Array.isArray(turn.sourceImages) ? turn.sourceImages : [],
    sourceReference: normalizeSourceReference(turn.sourceReference),
    images: (turn.images || []).map(normalizeStoredImage),
    status:
      turn.status === "queued" ||
      turn.status === "running" ||
      turn.status === "generating" ||
      turn.status === "success" ||
      turn.status === "error" ||
      turn.status === "cancelled"
        ? turn.status
        : "success",
  };
}

function normalizeSourceReference(
  value: ImageConversationTurn["sourceReference"],
): InpaintSourceReference | undefined {
  if (!value) {
    return undefined;
  }
  const originalFileID = String(value.original_file_id || "").trim();
  const originalGenID = String(value.original_gen_id || "").trim();
  const sourceAccountID = String(value.source_account_id || "").trim();
  if (!originalFileID || !originalGenID || !sourceAccountID) {
    return undefined;
  }
  const conversationID = String(value.conversation_id || "").trim();
  const parentMessageID = String(value.parent_message_id || "").trim();
  return {
    original_file_id: originalFileID,
    original_gen_id: originalGenID,
    conversation_id: conversationID || undefined,
    parent_message_id: parentMessageID || undefined,
    source_account_id: sourceAccountID,
  };
}

export function normalizeConversation(
  conversation: ImageConversation,
): ImageConversation {
  const turns =
    Array.isArray(conversation.turns) && conversation.turns.length > 0
      ? conversation.turns.map(normalizeTurn)
      : [
          normalizeTurn({
            id: `${conversation.id}-legacy`,
            title: conversation.title,
            mode: normalizeImageMode(conversation.mode),
            prompt: conversation.prompt,
            model: conversation.model,
            count: conversation.count,
            size: conversation.size,
            resolutionAccess: conversation.resolutionAccess,
            quality: conversation.quality,
            providerPlatform: conversation.providerPlatform,
            scale: conversation.scale,
            sourceImages: conversation.sourceImages,
            images: conversation.images || [],
            createdAt: conversation.createdAt,
            status: conversation.status,
            error: conversation.error,
          }),
        ];

  const latestTurn = turns[turns.length - 1];
  const title = String(conversation.title || "").trim() || latestTurn.title;
  return {
    ...conversation,
    title,
    mode: latestTurn.mode,
    prompt: latestTurn.prompt,
    model: latestTurn.model,
    count: latestTurn.count,
    size: latestTurn.size,
    resolutionAccess: latestTurn.resolutionAccess,
    quality: latestTurn.quality,
    providerPlatform: latestTurn.providerPlatform,
    scale: latestTurn.scale,
    sourceImages: latestTurn.sourceImages,
    images: latestTurn.images,
    createdAt: latestTurn.createdAt,
    status: latestTurn.status,
    error: latestTurn.error,
    turns,
  };
}

async function getImageConversationStorageMode() {
  if (cachedImageConversationStorageMode) {
    return cachedImageConversationStorageMode;
  }
  try {
    const config = await fetchConfig();
    setCachedImageConversationStorageMode(
      config.storage.imageConversationStorage === "server"
        ? "server"
        : "browser",
    );
    return cachedImageConversationStorageMode;
  } catch (error) {
    if (cachedImageConversationStorageMode) {
      return cachedImageConversationStorageMode;
    }
    throw error instanceof Error
      ? error
      : new Error("无法确定会话记录存储模式");
  }
}

async function listServerImageConversations(): Promise<ImageConversation[]> {
  const data = await httpRequest<{ items: ImageConversation[] }>(
    "/api/image/conversations",
  );
  return setCachedConversationsSnapshot(data.items || [], "server");
}

async function listBusinessImageConversations(): Promise<ImageConversation[]> {
  const data = await httpRequest<{ items: BusinessImageConversation[] }>(
    "/api/business/image/conversations",
  );
  const details = await Promise.all(
    (data.items || []).map(async (item) => {
      const detail = await getBusinessImageConversationDetail(item.id);
      return detail ?? { conversation: item, generations: [] };
    }),
  );
  return setCachedConversationsSnapshot(
    details.map(businessImageConversationDetailToConversation),
    "server",
  );
}

async function getBusinessImageConversationDetail(
  id: string,
): Promise<BusinessImageConversationDetail | null> {
  try {
    const [conversationData, jobsData] = await Promise.all([
      httpRequest<{ item: BusinessImageConversationDetail }>(
        `/api/business/image/conversations/${encodeURIComponent(id)}`,
      ),
      fetchBusinessImageJobs({ conversationId: id, limit: 100 }).catch(() => ({
        items: [],
      })),
    ]);
    if (!conversationData.item) {
      return null;
    }
    return {
      ...conversationData.item,
      jobs: jobsData.items || [],
    };
  } catch (error) {
    if (error instanceof Error && /not found/i.test(error.message)) {
      return null;
    }
    throw error;
  }
}

async function getBusinessImageConversation(
  id: string,
): Promise<ImageConversation | null> {
  const detail = await getBusinessImageConversationDetail(id);
  if (!detail) {
    return null;
  }
  const conversation = businessImageConversationDetailToConversation(detail);
  const currentItems =
    cachedConversationsStorageMode === "server" && cachedConversations
      ? cachedConversations
      : [];
  setCachedConversationsSnapshot(
    [
      conversation,
      ...currentItems.filter((item) => item.id !== conversation.id),
    ],
    "server",
  );
  return conversation;
}

export async function listServerImageConversationsSnapshot(): Promise<
  ImageConversation[]
> {
  return listServerImageConversations();
}

async function getServerImageConversation(
  id: string,
): Promise<ImageConversation | null> {
  try {
    const data = await httpRequest<{ item: ImageConversation }>(
      `/api/image/conversations/${encodeURIComponent(id)}`,
    );
    return data.item ? normalizeConversation(data.item) : null;
  } catch (error) {
    if (error instanceof Error && /not found/i.test(error.message)) {
      return null;
    }
    throw error;
  }
}

async function saveServerImageConversation(
  conversation: ImageConversation,
): Promise<ImageConversation> {
  const normalized = normalizeConversation(conversation);
  const data = await httpRequest<{ item: ImageConversation }>(
    `/api/image/conversations/${encodeURIComponent(normalized.id)}`,
    {
      method: "PUT",
      body: normalized,
    },
  );
  const savedConversation = normalizeConversation(data.item);
  const currentItems =
    cachedConversationsStorageMode === "server" && cachedConversations
      ? cachedConversations
      : [];
  setCachedConversationsSnapshot(
    [
      savedConversation,
      ...currentItems.filter((item) => item.id !== savedConversation.id),
    ],
    "server",
  );
  return savedConversation;
}

export async function exportLocalImageConversationsSnapshot(): Promise<
  ImageConversation[]
> {
  const items = await loadConversationCache();
  return sortConversations(items.map(normalizeConversation));
}

export async function replaceLocalImageConversations(
  items: ImageConversation[],
): Promise<void> {
  setCachedConversationsSnapshot(items, "browser");
  await persistConversationCache();
}

export async function exportServerImageConversationsSnapshot(): Promise<
  ImageConversation[]
> {
  return listServerImageConversations();
}

export async function importServerImageConversations(
  items: ImageConversation[],
): Promise<void> {
  for (const item of items) {
    await saveServerImageConversation(item);
  }
}

export async function importImageConversationsToServerTarget(
  items: ImageConversation[],
  storage: {
    backend: string;
    imageDir: string;
    redisAddr: string;
    redisPassword: string;
    redisDb: number;
    redisPrefix: string;
    imageConversationStorage: "browser" | "server" | string;
    imageDataStorage: "browser" | "server" | string;
  },
): Promise<void> {
  await httpRequest("/api/image/conversations/import", {
    method: "POST",
    body: {
      items,
      storage,
    },
  });
}

export async function migrateImageConversationStorage(options: {
  from: ImageConversationStorageMode;
  to: ImageConversationStorageMode;
  targetImageDataStorage: "browser" | "server";
  sourceItems?: ImageConversation[];
}): Promise<{ migrated: number }> {
  const from = normalizeStorageMode(options.from);
  const to = normalizeStorageMode(options.to);
  if (from === to) {
    return { migrated: 0 };
  }

  const sourceItems =
    options.sourceItems ??
    (from === "server"
      ? await exportServerImageConversationsSnapshot()
      : await exportLocalImageConversationsSnapshot());

  if (sourceItems.length === 0) {
    if (to === "browser") {
      await replaceLocalImageConversations([]);
    }
    return { migrated: 0 };
  }

  if (to === "server") {
    await importServerImageConversations(sourceItems);
    return { migrated: sourceItems.length };
  }

  const nextItems =
    options.targetImageDataStorage === "browser"
      ? await Promise.all(
          sourceItems.map((item) =>
            materializeConversationImagesForBrowser(item),
          ),
        )
      : sourceItems.map(normalizeConversation);
  await replaceLocalImageConversations(nextItems);
  return { migrated: nextItems.length };
}

export async function listImageConversations(): Promise<ImageConversation[]> {
  if (isBusinessProxyMode()) {
    return listBusinessImageConversations();
  }
  if ((await getImageConversationStorageMode()) === "server") {
    return listServerImageConversations();
  }
  const items = await loadConversationCache();
  return sortConversations(items.map(normalizeConversation));
}

export async function getImageConversation(
  id: string,
): Promise<ImageConversation | null> {
  if (isBusinessProxyMode()) {
    return getBusinessImageConversation(id);
  }
  if ((await getImageConversationStorageMode()) === "server") {
    return getServerImageConversation(id);
  }
  const items = await loadConversationCache();
  return items.find((item) => item.id === id) ?? null;
}

export async function saveImageConversation(
  conversation: ImageConversation,
): Promise<void> {
  if (isBusinessProxyMode()) {
    setCachedConversationsSnapshot(
      [
        normalizeConversation(conversation),
        ...(cachedConversations || []).filter(
          (item) => item.id !== conversation.id,
        ),
      ],
      "server",
    );
    return;
  }
  if ((await getImageConversationStorageMode()) === "server") {
    await saveServerImageConversation(conversation);
    return;
  }
  const items = await loadConversationCache();
  cachedConversations = sortConversations([
    normalizeConversation(conversation),
    ...items.filter((item) => item.id !== conversation.id),
  ]);
  await persistConversationCache();
}

export async function updateImageConversation(
  id: string,
  updater: (current: ImageConversation | null) => ImageConversation,
): Promise<ImageConversation> {
  if (isBusinessProxyMode()) {
    const current =
      cachedConversations?.find((item) => item.id === id) ??
      (await getBusinessImageConversation(id));
    const nextConversation = normalizeConversation(updater(current));
    setCachedConversationsSnapshot(
      [
        nextConversation,
        ...(cachedConversations || []).filter((item) => item.id !== id),
      ],
      "server",
    );
    return nextConversation;
  }
  if ((await getImageConversationStorageMode()) === "server") {
    const current = await getServerImageConversation(id);
    return saveServerImageConversation(updater(current));
  }
  const items = await loadConversationCache();
  const current = items.find((item) => item.id === id) ?? null;
  const nextConversation = normalizeConversation(updater(current));
  cachedConversations = sortConversations([
    nextConversation,
    ...items.filter((item) => item.id !== id),
  ]);
  await persistConversationCache();
  return nextConversation;
}

export async function renameImageConversation(
  id: string,
  title: string,
): Promise<ImageConversation> {
  const trimmedTitle = title.trim();
  if (!trimmedTitle) {
    throw new Error("请输入对话名");
  }

  if (isBusinessProxyMode()) {
    const data = await httpRequest<{ item: BusinessImageConversation }>(
      `/api/business/image/conversations/${encodeURIComponent(id)}`,
      {
        method: "PATCH",
        body: { title: trimmedTitle },
      },
    );
    const current =
      cachedConversations?.find((item) => item.id === id) ??
      (await getBusinessImageConversation(id));
    if (!current) {
      throw new Error("会话不存在");
    }
    const renamedConversation = normalizeConversation({
      ...current,
      title: data.item?.title || trimmedTitle,
      createdAt: data.item?.updated_at || current.createdAt,
    });
    setCachedConversationsSnapshot(
      [
        renamedConversation,
        ...(cachedConversations || []).filter((item) => item.id !== id),
      ],
      "server",
    );
    return renamedConversation;
  }

  return updateImageConversation(id, (current) => {
    if (!current) {
      throw new Error("会话不存在");
    }
    return {
      ...current,
      title: trimmedTitle,
    };
  });
}

export async function deleteImageConversation(id: string): Promise<void> {
  if (isBusinessProxyMode()) {
    await httpRequest(
      `/api/business/image/conversations/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
    if (cachedConversationsStorageMode === "server" && cachedConversations) {
      setCachedConversationsSnapshot(
        cachedConversations.filter((item) => item.id !== id),
        "server",
      );
    }
    return;
  }
  if ((await getImageConversationStorageMode()) === "server") {
    await httpRequest(`/api/image/conversations/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
    if (cachedConversationsStorageMode === "server" && cachedConversations) {
      setCachedConversationsSnapshot(
        cachedConversations.filter((item) => item.id !== id),
        "server",
      );
    }
    return;
  }
  const items = await loadConversationCache();
  cachedConversations = items.filter((item) => item.id !== id);
  await persistConversationCache();
}

export async function clearImageConversations(): Promise<void> {
  if (isBusinessProxyMode()) {
    await httpRequest("/api/business/image/conversations", {
      method: "DELETE",
    });
    setCachedConversationsSnapshot([], "server");
    return;
  }
  if ((await getImageConversationStorageMode()) === "server") {
    await httpRequest("/api/image/conversations", { method: "DELETE" });
    setCachedConversationsSnapshot([], "server");
    return;
  }
  cachedConversations = [];
  cachedConversationsStorageMode = "browser";
  loadPromise = null;
  writeQueue = writeQueue.then(async () => {
    await imageConversationStorage.removeItem(IMAGE_CONVERSATIONS_KEY);
  });
  await writeQueue;
}
