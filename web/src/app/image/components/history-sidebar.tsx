"use client";

import { memo, useCallback, useMemo, useState } from "react";
import {
  LoaderCircle,
  MessageSquarePlus,
  PanelLeftClose,
  Pencil,
  Search,
  Trash2,
} from "lucide-react";

import { AppModal } from "@/components/app-controls";
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
  onRenameConversation: (id: string, title: string) => Promise<void>;
  onDeleteConversation: (id: string) => Promise<void>;
  onCollapse?: () => void;
  standalone?: boolean;
};

type DeleteDialogState =
  | { type: "conversation"; id: string; title: string }
  | { type: "all" };

type RenameDialogState = {
  id: string;
  title: string;
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

function buildConversationSearchText(
  conversation: ImageConversation,
  modeLabelMap: Record<ImageMode, string>,
) {
  const turns = conversation.turns || [];
  return [
    conversation.title,
    conversation.prompt,
    conversation.model,
    conversation.modelLabel,
    conversation.vendorLabel,
    conversation.providerPlatform,
    conversation.size,
    conversation.quality,
    modeLabelMap[conversation.mode],
    ...turns.flatMap((turn) => [
      turn.title,
      turn.prompt,
      turn.model,
      turn.modelLabel,
      turn.vendorLabel,
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
    onRenameConversation,
    onDeleteConversation,
    onCollapse,
    standalone = false,
  }: HistorySidebarProps) {
    const draftActive = selectedConversationId === null;
    const [deleteDialog, setDeleteDialog] = useState<DeleteDialogState | null>(null);
    const [renameDialog, setRenameDialog] = useState<RenameDialogState | null>(null);
    const [renameValue, setRenameValue] = useState("");
    const [isRenaming, setIsRenaming] = useState(false);
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
    const normalizedRenameValue = renameValue.trim();

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

    const openRenameDialog = useCallback((id: string, title: string) => {
      setRenameDialog({ id, title });
      setRenameValue(title);
    }, []);

    const handleConfirmRename = useCallback(async () => {
      if (!renameDialog || !normalizedRenameValue || isRenaming) {
        return;
      }
      setIsRenaming(true);
      try {
        await onRenameConversation(renameDialog.id, normalizedRenameValue);
        setRenameDialog(null);
        setRenameValue("");
      } catch {
        return;
      } finally {
        setIsRenaming(false);
      }
    }, [isRenaming, normalizedRenameValue, onRenameConversation, renameDialog]);

    return (
      <>
        <aside
          data-image-history-sidebar
          className={cn("hs-sidebar", standalone ? "hs-sidebar-standalone" : "hs-sidebar-rail")}
          style={{
            minHeight: standalone ? 420 : undefined,
            background: "var(--app-bg-sidebar)",
            color: "var(--app-text-primary)",
            borderRadius: standalone ? 18 : 0,
            border: standalone ? "1px solid var(--app-border)" : undefined,
            borderRight: !standalone ? "1px solid var(--app-border)" : undefined,
            overflow: "hidden",
          }}
        >
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              height: "100%",
              padding: "32px 20px",
            }}
          >
            <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
              <h1 style={{ margin: 0, fontSize: 17, fontWeight: 600, color: "var(--app-text-primary)" }}>
                开始你的创作
              </h1>
              <button
                type="button"
                data-history-collapse
                onClick={onCollapse}
                disabled={!onCollapse}
                aria-label="收起侧边栏"
                title={onCollapse ? "收起历史对话" : "当前视图不可收起"}
                style={{
                  display: "grid",
                  placeItems: "center",
                  width: 34,
                  height: 34,
                  border: 0,
                  background: "transparent",
                  color: "var(--app-text-secondary)",
                  cursor: onCollapse ? "pointer" : "default",
                  opacity: onCollapse ? 1 : 0.45,
                  borderRadius: 8,
                  transition: "color 0.2s ease, background 0.2s ease",
                }}
              >
                <PanelLeftClose className="size-5" />
              </button>
            </header>

            <div style={{ marginTop: 32, display: "grid", gap: 10 }}>
              <button
                type="button"
                onClick={onCreateDraft}
                className={cn("hs-row", draftActive && "active")}
              >
                <MessageSquarePlus className="size-5" />
                新建对话
              </button>
              {searchOpen ? (
                <label className="hs-search">
                  <Search className="hs-search-ic size-5" />
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
                  />
                </label>
              ) : (
                <button type="button" onClick={() => setSearchOpen(true)} className="hs-row">
                  <Search className="size-5" />
                  搜索
                </button>
              )}
            </div>

            <section style={{ marginTop: 28, minHeight: 0, flex: 1, display: "flex", flexDirection: "column" }}>
              <div
                style={{
                  marginBottom: 10,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 12,
                  paddingLeft: 8,
                }}
              >
                <h2 style={{ margin: 0, fontSize: 13, fontWeight: 600, color: "var(--app-text-muted)" }}>
                  最近对话
                </h2>
                <button
                  type="button"
                  onClick={() => setDeleteDialog({ type: "all" })}
                  disabled={conversations.length === 0 || hasProcessingConversations}
                  title={hasProcessingConversations ? "有生成处理中时不能清空历史" : "清空历史记录"}
                  aria-label="清空历史记录"
                  className="hs-icon-btn"
                >
                  <Trash2 className="size-4" />
                </button>
              </div>

              <div className="hide-scrollbar" style={{ minHeight: 0, overflowY: "auto", flex: 1 }}>
                {isLoadingHistory ? (
                  <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 12px", fontSize: 13, color: "var(--app-text-muted)" }}>
                    <LoaderCircle className="size-4 animate-spin" />
                    正在读取会话记录
                  </div>
                ) : conversations.length === 0 ? (
                  <p style={{ padding: "10px 12px", fontSize: 13, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                    还没有历史记录。
                  </p>
                ) : displayedConversations.length === 0 ? (
                  <p style={{ padding: "10px 12px", fontSize: 13, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
                    没有找到匹配的对话。
                  </p>
                ) : (
                  <div style={{ display: "grid", gap: 4 }}>
                    {displayedConversations.map((conversation) => {
                      const active = conversation.id === selectedConversationId;
                      const deletingDisabled = processingConversationIds.has(conversation.id);
                      const title = formatImageConversationTitle(
                        conversation.title,
                        conversation.prompt,
                      );
                      return (
                        <div key={conversation.id} className="hs-conv">
                          <button
                            type="button"
                            onClick={() => onFocusConversation(conversation.id)}
                            className={cn("hs-conv-btn", active && "active")}
                            title={title}
                          >
                            <span data-history-title style={{ display: "block", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                              {title}
                            </span>
                          </button>
                          <button
                            type="button"
                            onClick={() => openRenameDialog(conversation.id, title)}
                            className="hs-conv-act"
                            title="重命名会话"
                            aria-label="重命名会话"
                          >
                            <Pencil className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteDialog({
                              type: "conversation",
                              id: conversation.id,
                              title,
                            })}
                            disabled={deletingDisabled}
                            className="hs-conv-act danger"
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

        <AppModal
          open={deleteDialog !== null}
          onClose={() => setDeleteDialog(null)}
          title={deleteDialogTitle}
          footer={
            <>
              <button className="app-btn" type="button" onClick={() => setDeleteDialog(null)}>
                取消
              </button>
              <button
                className="app-btn-primary"
                type="button"
                onClick={handleConfirmDelete}
                style={{
                  background: "linear-gradient(135deg, rgba(252, 165, 165, 0.95), rgba(244, 63, 94, 0.85))",
                  borderColor: "rgba(252, 165, 165, 0.5)",
                }}
              >
                <Trash2 className="size-4" />
                {isClearAllDialog ? "清空历史" : "删除会话"}
              </button>
            </>
          }
        >
          <p style={{ margin: 0, fontSize: 13, lineHeight: 1.7, color: "var(--app-text-secondary)" }}>
            {deleteDialogDescription}
          </p>
          {deleteDialogTarget ? (
            <p
              style={{
                marginTop: 12,
                padding: "10px 14px",
                borderRadius: 10,
                border: "1px solid var(--app-border)",
                background: "var(--app-bg-surface)",
                fontSize: 13,
                fontWeight: 500,
                color: "var(--app-text-primary)",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
              title={deleteDialogTarget}
            >
              {deleteDialogTarget}
            </p>
          ) : null}
        </AppModal>

        <AppModal
          open={renameDialog !== null}
          onClose={() => {
            if (!isRenaming) {
              setRenameDialog(null);
              setRenameValue("");
            }
          }}
          title="重命名会话"
          footer={
            <>
              <button
                className="app-btn"
                type="button"
                onClick={() => {
                  setRenameDialog(null);
                  setRenameValue("");
                }}
                disabled={isRenaming}
              >
                取消
              </button>
              <button
                className="app-btn-primary"
                type="button"
                onClick={() => void handleConfirmRename()}
                disabled={!normalizedRenameValue || isRenaming}
              >
                {isRenaming ? <LoaderCircle className="size-4 animate-spin" /> : <Pencil className="size-4" />}
                保存
              </button>
            </>
          }
        >
          <p style={{ margin: 0, marginBottom: 12, fontSize: 13, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
            修改后会同步到当前账号的历史记录。
          </p>
          <label className="app-fld">
            <span className="fl">对话名</span>
            <input
              className="app-input"
              type="text"
              value={renameValue}
              onChange={(event) => setRenameValue(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void handleConfirmRename();
                }
                if (event.key === "Escape" && !isRenaming) {
                  setRenameDialog(null);
                  setRenameValue("");
                }
              }}
              maxLength={80}
              autoFocus
              placeholder="输入对话名"
            />
          </label>
        </AppModal>
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
      prev.onRenameConversation === next.onRenameConversation &&
      prev.onDeleteConversation === next.onDeleteConversation &&
      prev.onCollapse === next.onCollapse &&
      prev.standalone === next.standalone
    );
  },
);

HistorySidebar.displayName = "HistorySidebar";
