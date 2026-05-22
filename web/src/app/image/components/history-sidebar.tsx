"use client";

import { memo } from "react";
import { LoaderCircle, MessageSquarePlus, PanelLeftClose, Search, Trash2 } from "lucide-react";

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

function confirmDeleteConversation(title: string) {
  return window.confirm(
    `确认删除会话「${title || "未命名会话"}」？\n\n删除后会从数据库移除这条会话和生成记录，并清理不再被引用的本地图片文件。`,
  );
}

function confirmClearHistory() {
  return window.confirm(
    "确认清空全部历史记录？\n\n这会删除当前账号下所有会话、生成记录和不再被引用的本地图片文件。",
  );
}

export const HistorySidebar = memo(
  function HistorySidebar({
    conversations,
    selectedConversationId,
    isLoadingHistory,
    hasProcessingConversations,
    processingConversationIds,
    onCreateDraft,
    onClearHistory,
    onFocusConversation,
    onDeleteConversation,
    onCollapse,
    standalone = false,
  }: HistorySidebarProps) {
    const draftActive = selectedConversationId === null;

    return (
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
            <button
              type="button"
              className="flex min-h-[38px] items-center gap-[18px] rounded-full px-[15px] text-[14px] font-semibold text-[var(--app-text-secondary)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]"
            >
              <Search className="size-5" />
              搜索
            </button>
          </div>

          <section className="mt-[86px] min-h-0 flex-1">
            <div className="mb-3 flex items-center justify-between gap-3 pl-2">
              <h2 className="m-0 text-[13px] font-semibold text-[var(--app-text-muted)]">
                最近对话
              </h2>
              <button
                type="button"
                onClick={() => {
                  if (confirmClearHistory()) {
                    void onClearHistory();
                  }
                }}
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
              ) : (
                <div className="grid gap-1.5">
                  {conversations.map((conversation) => {
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
                          onClick={() => {
                            if (confirmDeleteConversation(title)) {
                              void onDeleteConversation(conversation.id);
                            }
                          }}
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
