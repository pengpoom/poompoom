"use client";

import { Sparkles } from "lucide-react";
import type { ReactNode } from "react";

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
  composer?: ReactNode;
  onApplyPromptExample: (example: InspirationExample) => void;
};

export function EmptyState({ inspirationExamples, composer, onApplyPromptExample }: EmptyStateProps) {
  return (
    <div className="mx-auto flex min-h-[min(720px,calc(100vh-170px))] max-w-[1120px] flex-col justify-center gap-7 px-4 py-8 sm:px-6 lg:px-10">
      <div className="mx-auto max-w-[760px] text-center">
        <div className="mx-auto inline-flex size-14 items-center justify-center rounded-[20px] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)] shadow-sm">
          <Sparkles className="size-5" />
        </div>
        <h1 className="mt-6 text-3xl font-semibold tracking-tight text-[var(--app-text-primary)] lg:text-5xl">
          把想法变成画面
        </h1>
      </div>

      {composer}

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
