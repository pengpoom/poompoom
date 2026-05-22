"use client";

import { Box } from "lucide-react";

import { Button } from "@/components/ui/button";

type WorkspaceHeaderProps = {
  selectedConversationTitle?: string | null;
};

export function WorkspaceHeader({
  selectedConversationTitle,
}: WorkspaceHeaderProps) {
  return (
    <div className="flex min-h-[82px] items-start justify-end bg-transparent px-4 pt-5 sm:min-h-[104px] sm:px-[50px] sm:pt-[39px]">
      <div className="flex min-w-0 flex-1 items-center gap-3 lg:hidden">
        <h1 className="truncate text-lg font-bold text-[var(--app-text-primary)]">
          {selectedConversationTitle || "生图"}
        </h1>
      </div>
      <div className="flex shrink-0 items-center gap-3 sm:gap-6">
        <Button
          type="button"
          variant="ghost"
          className="h-10 gap-2 rounded-lg border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-3 text-[14px] font-semibold text-[var(--app-text-primary)] shadow-none hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)] sm:px-4"
        >
          <Box className="size-[18px]" />
          <span className="hidden sm:inline">收藏库</span>
        </Button>
      </div>
    </div>
  );
}
