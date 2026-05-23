"use client";

import { memo, useCallback, useMemo, useState } from "react";
import { LoaderCircle, MessageSquarePlus, PanelLeftClose, Search, Trash2 } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ImageConversation, ImageMode } from "@/store/image-conversations";
import { cn } from "@/lib/utils";
import { formatImageConversationTitle } from "../title-utils";

type HistorySidebarProps = {
  conversations: ImageConversation[];
  selectedConversationId: string | null;
  isLoadingHistory: boolean;
  hasProcessingConversations: boolean;
  processingConversationIds: Set<string>;
  modeLabelMap: Record<ImageMode, string>;
  buildConversationPreviewSource: (conversation: ImageConversation) => string;
  formatConversationTime: (value: string) => string;
  onCreateDraft: () => void;
  onClearHistory: () => Promise<void>;
  onFocusConversation: (id: string) => void;
  onDeleteConversation: (id: string) => Promise<void>;
  onCollapse?: () => void;
  standalone?: boolean;
};

type DeleteDialogState =
  | { type: "conversation"; id: string; title: string }
  | { type: "all" };

function hasSameConversationIdSet(left: Set<string>, right: Set<string>) {
  if (left === right) {
    return true;
  }
  if (left.size !== right.size) {
    return false;
  }
  for (const value of left) {
    if (!right.has(value)) {
      return false;
    }
  }
  return true;
}

function buildConversationSearchText(
  conversation: ImageConversation,
  modeLabelMap: Record<ImageMode, string>,
) {
  const turns = conversation.turns || [];
  return [
    conversation.title,
    conversation.prompt,
    conversation.model,
    conversation.providerPlatform,
    conversation.size,
    conversation.quality,
    modeLabelMap[conversation.mode],
    ...turns.flatMap((turn) => [
      turn.title,
      turn.prompt,
      turn.model,
      turn.providerPlatform,
      turn.size,
      turn.quality,
      modeLabelMap[turn.mode],
      turn.error,
      ...turn.images.map((image) => image.revised_prompt || image.error || ""),
    ]),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export const HistorySidebar = memo(
  function HistorySidebar({
    conversations,
    selectedConversationId,
    isLoadingHistory,
    hasProcessingConversations,
    processingConversationIds,
    modeLabelMap,
    onCreateDraft,
    onClearHistory,
    onFocusConversation,
    onDeleteConversation,
    onCollapse,
    standalone = false,
  }: HistorySidebarProps) {
    const draftActive = selectedConversationId === null;
    const [deleteDialog, setDeleteDialog] = useState<DeleteDialogState | null>(null);
    const [searchOpen, setSearchOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");
    const normalizedSearchQuery = searchOpen ? searchQuery.trim().toLowerCase() : "";
    const displayedConversations = useMemo(() => {
      if (!normalizedSearchQuery) {
        return conversations;
      }
      return conversations.filter((conversation) =>
        buildConversationSearchText(conversation, modeLabelMap).includes(normalizedSearchQuery),
      );
    }, [conversations, modeLabelMap, normalizedSearchQuery]);
    const isClearAllDialog = deleteDialog?.type === "all";
    const deleteDialogTitle = isClearAllDialog ? "清空全部历史记录" : "删除历史会话";
    const deleteDialogDescription = isClearAllDialog
      ? "这会删除当前账号下所有会话、生成记录，并清理不再被引用的本地图片文件。"
      : "删除后会从数据库移除这条会话和生成记录，并清理不再被引用的本地图片文件。";
    const deleteDialogTarget =
      deleteDialog?.type === "conversation" ? deleteDialog.title || "未命名会话" : "";

    const handleConfirmDelete = useCallback(() => {
      if (!deleteDialog) {
        return;
      }

      const target = deleteDialog;
      setDeleteDialog(null);
      if (target.type === "all") {
        void onClearHistory();
        return;
      }
      void onDeleteConversation(target.id);
    }, [deleteDialog, onClearHistory, onDeleteConversation]);

    return (
      <>
        <aside
          data-image-history-sidebar
          className={cn(
            "min-h-0 overflow-hidden border-[var(--app-border)] bg-[var(--app-bg-sidebar)] text-[var(--app-text-primary)]",
            standalone
              ? "min-h-[420px] rounded-[18px] border"
              : "hidden border-r lg:block",
          )}
        >
          <div className="flex h-full min-h-0 flex-col px-5 py-8 lg:px-[21px] lg:py-[46px]">
            <header className="flex items-center justify-between gap-4">
              <h1 className="m-0 text-[17px] font-semibold tracking-normal text-[var(--app-text-primary)]">
                开始你的创作
              </h1>
              <button
                type="button"
                data-history-collapse
                onClick={onCollapse}
                disabled={!onCollapse}
                className="grid size-[34px] shrink-0 place-items-center text-[var(--app-text-secondary)] transition hover:text-[var(--app-text-primary)] disabled:cursor-default disabled:opacity-45"
                aria-label="收起侧边栏"
                title={onCollapse ? "收起历史对话" : "当前视图不可收起"}
              >
                <PanelLeftClose className="size-6" />
              </button>
            </header>

            <div className="mt-[38px] grid gap-3">
              <button
                type="button"
                onClick={onCreateDraft}
                className={cn(
                  "flex min-h-[38px] items-center gap-[18px] rounded-full px-[15px] text-[14px] font-semibold transition",
                  draftActive
                    ? "bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)]"
                    : "text-[var(--app-text-secondary)] hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                )}
              >
                <MessageSquarePlus className="size-5" />
                新建对话
              </button>
              {searchOpen ? (
                <label className="relative block">
                  <Search className="pointer-events-none absolute left-[15px] top-1/2 size-5 -translate-y-1/2 text-[var(--app-text-secondary)]" />
                  <input
                    type="search"
                    value={searchQuery}
                    onChange={(event) => setSearchQuery(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") {
                        setSearchQuery("");
                        setSearchOpen(false);
                      }
                    }}
                    onBlur={() => {
                      if (!searchQuery.trim()) {
                        setSearchOpen(false);
                      }
                    }}
                    autoFocus
                    placeholder="搜索"
                    className="h-[38px] w-full rounded-full border-0 bg-[var(--app-bg-surface-hover)] pl-[55px] pr-[15px] text-[14px] font-semibold text-[var(--app-text-primary)] outline-none transition placeholder:text-[var(--app-text-muted)] focus:ring-[3px] focus:ring-[rgba(91,214,255,0.18)]"
                  />
                </label>
              ) : (
                <button
                  type="button"
                  onClick={() => setSearchOpen(true)}
                  className="flex h-[38px] items-center gap-[18px] rounded-full px-[15px] text-[14px] font-semibold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]"
                >
                  <Search className="size-5" />
                  搜索
                </button>
              )}
            </div>

            <section className="mt-8 min-h-0 flex-1">
              <div className="mb-3 flex items-center justify-between gap-3 pl-2">
                <div className="min-w-0">
                  <h2 className="m-0 text-[13px] font-semibold text-[var(--app-text-muted)]">
                    最近对话
                  </h2>
                </div>
                <button
                  type="button"
                  onClick={() => setDeleteDialog({ type: "all" })}
                  disabled={conversations.length === 0 || hasProcessingConversations}
                  className="grid size-8 place-items-center rounded-lg text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
                  title={hasProcessingConversations ? "有生成处理中时不能清空历史" : "清空历史记录"}
                  aria-label="清空历史记录"
                >
                  <Trash2 className="size-4" />
                </button>
              </div>

              <div className="hide-scrollbar min-h-0 overflow-y-auto">
                {isLoadingHistory ? (
                  <div className="flex items-center gap-2 rounded-full px-3 py-3 text-sm text-[var(--app-text-muted)]">
                    <LoaderCircle className="size-4 animate-spin" />
                    正在读取会话记录
                  </div>
                ) : conversations.length === 0 ? (
                  <p className="px-3 py-3 text-sm leading-6 text-[var(--app-text-muted)]">
                    还没有历史记录。
                  </p>
                ) : displayedConversations.length === 0 ? (
                  <p className="px-3 py-3 text-sm leading-6 text-[var(--app-text-muted)]">
                    没有找到匹配的对话。
                  </p>
                ) : (
                  <div className="grid gap-1.5">
                    {displayedConversations.map((conversation) => {
                      const active = conversation.id === selectedConversationId;
                      const deletingDisabled = processingConversationIds.has(conversation.id);
                      const title = formatImageConversationTitle(
                        conversation.title,
                        conversation.prompt,
                      );
                      return (
                        <div key={conversation.id} className="group flex items-center gap-1">
                          <button
                            type="button"
                            onClick={() => onFocusConversation(conversation.id)}
                            className={cn(
                              "min-h-[34px] min-w-0 flex-1 overflow-hidden rounded-full px-[22px] text-left text-[14px] font-semibold transition",
                              active
                                ? "bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)]"
                                : "text-[var(--app-text-secondary)] hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                            )}
                            title={title}
                          >
                            <span className="block min-w-0 truncate" data-history-title>
                              {title}
                            </span>
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteDialog({
                              type: "conversation",
                              id: conversation.id,
                              title,
                            })}
                            disabled={deletingDisabled}
                            className="grid size-8 shrink-0 place-items-center rounded-lg text-[var(--app-text-muted)] opacity-0 transition hover:bg-[var(--app-bg-surface)] hover:text-rose-400 disabled:cursor-not-allowed disabled:opacity-30 group-hover:opacity-100"
                            title={deletingDisabled ? "当前会话仍在处理中，暂时不能删除" : "删除会话"}
                            aria-label="删除会话"
                          >
                            <Trash2 className="size-4" />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </section>
          </div>
        </aside>

        <Dialog open={deleteDialog !== null} onOpenChange={(open) => !open && setDeleteDialog(null)}>
          <DialogContent className="w-[min(92vw,420px)] rounded-[20px] p-0" showCloseButton={false}>
            <DialogHeader className="border-b border-[var(--app-border)] px-5 py-4">
              <div className="flex items-start gap-3">
                <span className="grid size-10 shrink-0 place-items-center rounded-xl border border-rose-400/25 bg-rose-500/10 text-rose-300">
                  <Trash2 className="size-5" />
                </span>
                <div className="min-w-0">
                  <DialogTitle className="text-base">{deleteDialogTitle}</DialogTitle>
                  <DialogDescription className="mt-2 leading-6">
                    {deleteDialogDescription}
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            <div className="px-5 py-4">
              {deleteDialogTarget ? (
                <p className="truncate rounded-xl border border-[var(--app-border)] bg-[#1B1C22] px-3 py-2 text-sm font-semibold text-[var(--app-text-primary)]" title={deleteDialogTarget}>
                  {deleteDialogTarget}
                </p>
              ) : (
                <p className="text-sm leading-6 text-[var(--app-text-secondary)]">
                  该操作会影响当前账号下的全部历史会话。
                </p>
              )}
            </div>

            <DialogFooter className="border-t border-[var(--app-border)] px-5 py-4">
              <button
                type="button"
                onClick={() => setDeleteDialog(null)}
                className="h-10 rounded-full border border-[var(--app-border)] bg-[#1B1C22] px-5 text-sm font-semibold text-[var(--app-text-secondary)] transition hover:bg-[#22242B] hover:text-[var(--app-text-primary)]"
              >
                取消
              </button>
              <button
                type="button"
                onClick={handleConfirmDelete}
                className="h-10 rounded-full border border-rose-300/30 bg-rose-500/90 px-5 text-sm font-semibold text-white shadow-[0_12px_28px_rgba(244,63,94,0.18)] transition hover:bg-rose-400"
              >
                {isClearAllDialog ? "清空历史" : "删除会话"}
              </button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </>
    );
  },
  (prev, next) => {
    return (
      prev.conversations === next.conversations &&
      prev.selectedConversationId === next.selectedConversationId &&
      prev.isLoadingHistory === next.isLoadingHistory &&
      prev.hasProcessingConversations === next.hasProcessingConversations &&
      hasSameConversationIdSet(
        prev.processingConversationIds,
        next.processingConversationIds,
      ) &&
      prev.modeLabelMap === next.modeLabelMap &&
      prev.buildConversationPreviewSource === next.buildConversationPreviewSource &&
      prev.formatConversationTime === next.formatConversationTime &&
      prev.onCreateDraft === next.onCreateDraft &&
      prev.onClearHistory === next.onClearHistory &&
      prev.onFocusConversation === next.onFocusConversation &&
      prev.onDeleteConversation === next.onDeleteConversation &&
      prev.onCollapse === next.onCollapse &&
      prev.standalone === next.standalone
    );
  },
);

HistorySidebar.displayName = "HistorySidebar";
