"use client";

import { Box } from "lucide-react";

export function WorkspaceHeader() {
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
      <div style={{ display: "flex", alignItems: "center", gap: 12, flexShrink: 0 }}>
        <button className="app-btn" type="button">
          <Box className="size-4" />
          <span className="hidden sm:inline">收藏库</span>
        </button>
      </div>
    </div>
  );
}
