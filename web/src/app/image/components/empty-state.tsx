"use client";

import { Sparkles } from "lucide-react";

import { cn } from "@/lib/utils";

type InspirationExample = {
  id: string;
  title: string;
  prompt: string;
  hint: string;
  count: number;
  tone: string;
};

type EmptyStateProps = {
  inspirationExamples: InspirationExample[];
  onApplyPromptExample: (example: InspirationExample) => void;
};

export function EmptyState({ inspirationExamples, onApplyPromptExample }: EmptyStateProps) {
  return (
    <div className="mx-auto flex max-w-[1120px] flex-col gap-8 px-4 pb-40 pt-8 sm:px-6 lg:px-10">
      <div className="max-w-[760px]">
        <div className="inline-flex size-14 items-center justify-center rounded-[20px] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)] shadow-sm">
          <Sparkles className="size-5" />
        </div>
        <h1 className="mt-6 text-3xl font-semibold tracking-tight text-[var(--app-text-primary)] lg:text-5xl">
          从一个提示词，开始完整的图像工作流。
        </h1>
      </div>

      <div className="hide-scrollbar flex gap-3 overflow-x-auto pb-1 md:grid md:grid-cols-2 md:overflow-visible xl:grid-cols-4">
        {inspirationExamples.map((example) => (
          <button
            key={example.id}
            type="button"
            onClick={() => onApplyPromptExample(example)}
            className="w-[220px] shrink-0 overflow-hidden rounded-[18px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] text-left transition hover:-translate-y-0.5 hover:border-[var(--app-border-strong)] md:w-auto"
          >
            <div className={cn("h-[4.5rem] bg-gradient-to-br md:h-20", example.tone)} />
            <div className="space-y-2 px-4 py-3.5">
              <div className="flex items-center gap-2 text-[11px] text-[var(--app-text-muted)]">
                <span className="rounded-full bg-[var(--app-bg-surface)] px-2 py-0.5 font-medium">Prompt</span>
              </div>
              <div className="text-sm font-semibold tracking-tight text-[var(--app-text-primary)]">{example.title}</div>
              <div className="line-clamp-2 text-sm leading-6 text-[var(--app-text-secondary)]">{example.prompt}</div>
              <div className="border-t border-[var(--app-border)] pt-2 text-xs leading-5 text-[var(--app-text-muted)]">{example.hint}</div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
