"use client";

import { useCallback, useState, type ClipboardEvent as ReactClipboardEvent } from "react";
import { toast } from "sonner";

import type { APIAccessPlatform, ImageModel } from "@/lib/api";
import type { ImageMode } from "@/store/image-conversations";
import type { StoredImage, StoredSourceImage } from "@/store/image-conversations";

import { buildImageDataUrl, buildSourceImageUrl } from "../view-utils";

export type EditorTarget = {
  conversationId: string | null;
  image: StoredImage | null;
  imageName: string;
  sourceDataUrl: string;
  model?: ImageModel;
  providerPlatform?: APIAccessPlatform;
  modelId?: string;
  modelLabel?: string;
  vendor?: string;
  vendorLabel?: string;
  adapter?: string;
};

type UseImageSourceInputsOptions = {
  mode: ImageMode;
  selectedConversationId: string | null;
  makeId: () => string;
};

async function fileToDataUrl(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(new Error(`读取 ${file.name} 失败`));
    reader.readAsDataURL(file);
  });
}

export function useImageSourceInputs({
  mode,
  selectedConversationId,
  makeId,
}: UseImageSourceInputsOptions) {
  const [sourceImages, setSourceImages] = useState<StoredSourceImage[]>([]);
  const [editorTarget, setEditorTarget] = useState<EditorTarget | null>(null);

  const appendFiles = useCallback(async (files: File[] | FileList | null, role: "image" | "mask") => {
    const normalizedFiles = files ? Array.from(files) : [];
    if (normalizedFiles.length === 0) {
      return;
    }
    const nextItems = await Promise.all(
      normalizedFiles.map(async (file) => ({
        id: makeId(),
        role,
        name: file.name,
        dataUrl: await fileToDataUrl(file),
      })),
    );
    setSourceImages((prev) => {
      if (role === "mask") {
        return [...prev.filter((item) => item.role !== "mask"), nextItems[0]];
      }
      return [...prev.filter((item) => item.role !== "mask"), ...prev.filter((item) => item.role === "mask"), ...nextItems];
    });
  }, [makeId]);

  const handlePromptPaste = useCallback((event: ReactClipboardEvent<HTMLTextAreaElement>) => {
    const clipboardImages = Array.from(event.clipboardData.items)
      .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
      .map((item) => item.getAsFile())
      .filter((file): file is File => Boolean(file));

    if (clipboardImages.length === 0) {
      return;
    }

    event.preventDefault();
    void appendFiles(clipboardImages, "image");
    toast.success(
      mode === "generate"
        ? "已从剪贴板添加参考图"
        : "已从剪贴板添加源图",
    );
  }, [appendFiles, mode]);

  const removeSourceImage = useCallback((id: string) => {
    setSourceImages((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const openSelectionEditor = useCallback((conversationId: string, turn: {
    model?: ImageModel;
    providerPlatform?: APIAccessPlatform;
    modelId?: string;
    modelLabel?: string;
    vendor?: string;
    vendorLabel?: string;
    adapter?: string;
  }, image: StoredImage, imageName: string) => {
    const dataUrl = buildImageDataUrl(image);
    if (!dataUrl) {
      toast.error("当前图片没有可复用的数据");
      return;
    }
    setEditorTarget({
      conversationId,
      image,
      imageName,
      sourceDataUrl: dataUrl,
      model: turn.model,
      providerPlatform: turn.providerPlatform,
      modelId: turn.modelId,
      modelLabel: turn.modelLabel,
      vendor: turn.vendor,
      vendorLabel: turn.vendorLabel,
      adapter: turn.adapter,
    });
  }, []);

  const openSourceSelectionEditor = useCallback((sourceImageId: string) => {
    const sourceImage = sourceImages.find((item) => item.id === sourceImageId && item.role === "image");
    if (!sourceImage) {
      toast.error("当前源图不可用于选区编辑");
      return;
    }
    const sourceURL = buildSourceImageUrl(sourceImage);
    if (!sourceURL) {
      toast.error("当前源图不可用于选区编辑");
      return;
    }

    setEditorTarget({
      conversationId: selectedConversationId,
      image: null,
      imageName: sourceImage.name || "source.png",
      sourceDataUrl: sourceURL,
    });
  }, [selectedConversationId, sourceImages]);

  const closeSelectionEditor = useCallback(() => {
    setEditorTarget(null);
  }, []);

  return {
    sourceImages,
    setSourceImages,
    editorTarget,
    setEditorTarget,
    appendFiles,
    handlePromptPaste,
    removeSourceImage,
    openSelectionEditor,
    openSourceSelectionEditor,
    closeSelectionEditor,
  };
}
