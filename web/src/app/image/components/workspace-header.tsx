"use client";

import { Box } from "lucide-react";

type WorkspaceHeaderProps = {
  selectedConversationTitle?: string | null;
};

export function WorkspaceHeader({
  selectedConversationTitle,
}: WorkspaceHeaderProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "flex-end",
        minHeight: 82,
        padding: "20px 16px 0",
      }}
    >
      <div className="md:hidden" style={{ display: "flex", minWidth: 0, flex: 1, alignItems: "center", gap: 12 }}>
        <h1
          style={{
            margin: 0,
            fontSize: 18,
            fontWeight: 700,
            color: "var(--app-text-primary)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {selectedConversationTitle || "生图"}
        </h1>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 12, flexShrink: 0 }}>
        <button className="app-btn" type="button">
          <Box className="size-4" />
          <span className="hidden sm:inline">收藏库</span>
        </button>
      </div>
    </div>
  );
}
