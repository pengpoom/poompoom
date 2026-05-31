"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import {
  clearImageConversations,
  deleteImageConversation,
  getCachedImageConversationsSnapshot,
  getImageConversation,
  listImageConversations,
  normalizeConversation,
  type ImageConversation,
} from "@/store/image-conversations";

const SELECTED_IMAGE_CONVERSATION_KEY = "image-studio.selected-conversation-id.v1";

type UseImageHistoryOptions = {
  normalizeHistory: (
    items: ImageConversation[],
  ) => Promise<ImageConversation[]>;
  mountedRef: React.RefObject<boolean>;
  draftSelectionRef: React.RefObject<boolean>;
  processingConversationIds: Set<string>;
  preferredProcessingConversationId: string | null;
};

function mergeProcessingConversations(
  currentItems: ImageConversation[],
  refreshedItems: ImageConversation[],
  processingConversationIds: Set<string>,
) {
  const nextById = new Map(refreshedItems.map((item) => [item.id, item]));

  if (processingConversationIds.size === 0) {
    return refreshedItems;
  }

  const missingActiveItems = currentItems.filter(
    (item) => processingConversationIds.has(item.id) && !nextById.has(item.id),
  );
  if (missingActiveItems.length === 0) {
    return refreshedItems;
  }

  return [...missingActiveItems, ...refreshedItems].sort((a, b) =>
    b.createdAt.localeCompare(a.createdAt),
  );
}

function isConversationProcessing(conversation: ImageConversation) {
  return (conversation.turns || []).some(
    (turn) =>
      turn.status === "queued" ||
      turn.status === "running" ||
      turn.status === "generating",
  );
}

function collectProcessingConversationIds(items: ImageConversation[]) {
  return new Set(
    items
      .filter((conversation) => isConversationProcessing(conversation))
      .map((conversation) => conversation.id),
  );
}

function readStoredSelectedConversationId() {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    return window.localStorage.getItem(SELECTED_IMAGE_CONVERSATION_KEY);
  } catch {
    return null;
  }
}

function writeStoredSelectedConversationId(conversationId: string | null) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (conversationId) {
      window.localStorage.setItem(SELECTED_IMAGE_CONVERSATION_KEY, conversationId);
      return;
    }
    window.localStorage.removeItem(SELECTED_IMAGE_CONVERSATION_KEY);
  } catch {
    // Ignore localStorage failures; selection can still live in React state.
  }
}

function mergeConversationIdSets(
  left: Set<string>,
  right: Set<string>,
) {
  if (right.size === 0) {
    return left;
  }
  const next = new Set(left);
  for (const value of right) {
    next.add(value);
  }
  return next;
}

export function useImageHistory({
  normalizeHistory,
  mountedRef,
  draftSelectionRef,
  processingConversationIds,
  preferredProcessingConversationId,
}: UseImageHistoryOptions) {
  const cachedConversations = getCachedImageConversationsSnapshot();
  const conversationsRef = useRef<ImageConversation[]>(
    cachedConversations ?? [],
  );
  const [conversations, setConversations] = useState<ImageConversation[]>(
    cachedConversations ?? [],
  );
  const [selectedConversationId, setSelectedConversationId] = useState<
    string | null
  >(() => {
    const storedSelectedId = readStoredSelectedConversationId();
    if (
      storedSelectedId &&
      cachedConversations?.some((item) => item.id === storedSelectedId)
    ) {
      return storedSelectedId;
    }
    return cachedConversations?.[0]?.id ?? null;
  });
  const [isLoadingHistory, setIsLoadingHistory] =
    useState(!cachedConversations);

  useEffect(() => {
    conversationsRef.current = conversations;
  }, [conversations]);

  const focusConversation = useCallback(
    (conversationId: string) => {
      draftSelectionRef.current = false;
      writeStoredSelectedConversationId(conversationId);
      setSelectedConversationId(conversationId);
    },
    [draftSelectionRef],
  );

  const openDraftConversation = useCallback(() => {
    draftSelectionRef.current = true;
    writeStoredSelectedConversationId(null);
    setSelectedConversationId(null);
  }, [draftSelectionRef]);

  const hasProcessingConversation = useCallback((conversationId?: string) => {
    const processingIds = collectProcessingConversationIds(
      conversationsRef.current,
    );
    if (!conversationId) {
      return processingConversationIds.size > 0 || processingIds.size > 0;
    }
    return (
      processingConversationIds.has(conversationId) ||
      processingIds.has(conversationId)
    );
  }, [processingConversationIds]);

  const refreshHistory = useCallback(
    async (
      options: {
        normalize?: boolean;
        silent?: boolean;
        withLoading?: boolean;
      } = {},
    ) => {
      const {
        normalize = false,
        silent = false,
        withLoading = false,
      } = options;

      try {
        if (
          withLoading &&
          mountedRef.current &&
          !getCachedImageConversationsSnapshot()
        ) {
          setIsLoadingHistory(true);
        }
        const items = await listImageConversations();
        const nextItems = normalize ? await normalizeHistory(items) : items;
        if (!mountedRef.current) {
          return;
        }
        const currentItems =
          getCachedImageConversationsSnapshot() ?? conversationsRef.current;
        const mergedItems = mergeProcessingConversations(
          currentItems,
          nextItems,
          mergeConversationIdSets(
            processingConversationIds,
            collectProcessingConversationIds(currentItems),
          ),
        );
        conversationsRef.current = mergedItems;
        setConversations(mergedItems);
        setSelectedConversationId((current) => {
          if (current && mergedItems.some((item) => item.id === current)) {
            writeStoredSelectedConversationId(current);
            return current;
          }
          if (draftSelectionRef.current) {
            writeStoredSelectedConversationId(null);
            return null;
          }
          const storedSelectedId = readStoredSelectedConversationId();
          if (
            storedSelectedId &&
            mergedItems.some((item) => item.id === storedSelectedId)
          ) {
            writeStoredSelectedConversationId(storedSelectedId);
            return storedSelectedId;
          }
          if (
            preferredProcessingConversationId &&
            mergedItems.some((item) => item.id === preferredProcessingConversationId)
          ) {
            writeStoredSelectedConversationId(preferredProcessingConversationId);
            return preferredProcessingConversationId;
          }
          const nextSelectedId = mergedItems[0]?.id ?? null;
          writeStoredSelectedConversationId(nextSelectedId);
          return nextSelectedId;
        });
      } catch (error) {
        if (!silent && mountedRef.current) {
          const message =
            error instanceof Error ? error.message : "加载会话失败";
          toast.error(message);
        }
      } finally {
        if (withLoading && mountedRef.current) {
          setIsLoadingHistory(false);
        }
      }
    },
    [
      draftSelectionRef,
      mountedRef,
      normalizeHistory,
      preferredProcessingConversationId,
      processingConversationIds,
    ],
  );

  const refreshConversation = useCallback(
    async (
      conversationId: string,
      options: {
        silent?: boolean;
      } = {},
    ) => {
      const { silent = true } = options;
      try {
        const item = await getImageConversation(conversationId);
        if (!mountedRef.current || !item) {
          return null;
        }
        const nextItem = normalizeConversation(item);
        setConversations((current) => {
          const next = [
            nextItem,
            ...current.filter((conversation) => conversation.id !== nextItem.id),
          ].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
          conversationsRef.current = next;
          return next;
        });
        return nextItem;
      } catch (error) {
        if (!silent && mountedRef.current) {
          const message =
            error instanceof Error ? error.message : "加载会话失败";
          toast.error(message);
        }
        return null;
      }
    },
    [mountedRef],
  );

  const handleCreateDraft = useCallback(
    (
      resetComposer: (nextMode?: "generate" | "edit") => void,
      textareaRef: React.RefObject<HTMLTextAreaElement | null>,
    ) => {
      openDraftConversation();
      resetComposer("generate");
      textareaRef.current?.focus();
    },
    [openDraftConversation],
  );

  const handleDeleteConversation = useCallback(
    async (id: string) => {
      if (hasProcessingConversation(id)) {
        toast.error("当前会话仍在处理中，请等待任务完成后再删除");
        return;
      }

      const previousSelectedId = selectedConversationId;
      const previousDraftSelection = draftSelectionRef.current;

      setConversations((current) => {
        const nextConversations = current.filter((item) => item.id !== id);
        conversationsRef.current = nextConversations;
        setSelectedConversationId((prev) => {
          if (prev !== id) {
            return prev;
          }
          draftSelectionRef.current = false;
          const nextSelectedId = nextConversations[0]?.id ?? null;
          writeStoredSelectedConversationId(nextSelectedId);
          return nextSelectedId;
        });
        return nextConversations;
      });

      try {
        await deleteImageConversation(id);
      } catch (error) {
        const message = error instanceof Error ? error.message : "删除会话失败";
        toast.error(message);
        const items = await listImageConversations();
        if (!mountedRef.current) {
          return;
        }
        draftSelectionRef.current = previousDraftSelection;
        conversationsRef.current = items;
        setConversations(items);
        setSelectedConversationId(() => {
          if (
            previousSelectedId &&
            items.some((item) => item.id === previousSelectedId)
          ) {
            writeStoredSelectedConversationId(previousSelectedId);
            return previousSelectedId;
          }
          if (previousDraftSelection) {
            writeStoredSelectedConversationId(null);
            return null;
          }
          const nextSelectedId = items[0]?.id ?? null;
          writeStoredSelectedConversationId(nextSelectedId);
          return nextSelectedId;
        });
      }
    },
    [draftSelectionRef, hasProcessingConversation, mountedRef, selectedConversationId],
  );

  const handleClearHistory = useCallback(async () => {
    if (hasProcessingConversation()) {
      toast.error("仍有图片任务在处理中，暂时不能清空历史记录");
      return;
    }

    try {
      await clearImageConversations();
      draftSelectionRef.current = true;
      conversationsRef.current = [];
      setConversations([]);
      writeStoredSelectedConversationId(null);
      setSelectedConversationId(null);
      toast.success("已清空历史记录");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "清空历史记录失败";
      toast.error(message);
    }
  }, [draftSelectionRef, hasProcessingConversation]);

  return {
    conversations,
    selectedConversationId,
    isLoadingHistory,
    setConversations,
    setSelectedConversationId,
    setIsLoadingHistory,
    focusConversation,
    openDraftConversation,
    refreshHistory,
    refreshConversation,
    handleCreateDraft,
    handleDeleteConversation,
    handleClearHistory,
  };
}
