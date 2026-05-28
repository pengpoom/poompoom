"use client";

import { Sparkles } from "lucide-react";
import type { ReactNode } from "react";

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
    <div
      style={{
        marginInline: "auto",
        maxWidth: 1120,
        minHeight: "min(720px, calc(100vh - 170px))",
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        gap: 28,
        padding: "32px 16px",
      }}
    >
      <div style={{ marginInline: "auto", maxWidth: 760, textAlign: "center" }}>
        <div
          style={{
            marginInline: "auto",
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            width: 56,
            height: 56,
            borderRadius: 20,
            border: "1px solid var(--app-border)",
            background: "var(--app-bg-surface)",
            color: "var(--app-text-primary)",
          }}
        >
          <Sparkles className="size-5" />
        </div>
        <h1
          style={{
            marginTop: 24,
            fontSize: 36,
            fontWeight: 600,
            letterSpacing: -0.5,
            color: "var(--app-text-primary)",
          }}
        >
          把想法变成画面
        </h1>
      </div>

      {composer}

      <div
        style={{
          display: "grid",
          gap: 12,
          gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
        }}
      >
        {inspirationExamples.map((example) => (
          <button
            key={example.id}
            type="button"
            onClick={() => onApplyPromptExample(example)}
            className="app-panel"
            style={{
              padding: 0,
              overflow: "hidden",
              textAlign: "left",
              cursor: "pointer",
              transition: "transform 0.2s ease, border-color 0.2s ease",
            }}
          >
            <div style={{ height: 80, background: example.tone }} />
            <div style={{ padding: "14px 16px", display: "grid", gap: 8 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 11, color: "var(--app-text-muted)" }}>
                <span
                  style={{
                    padding: "2px 10px",
                    borderRadius: 999,
                    background: "var(--app-bg-surface)",
                    fontWeight: 500,
                  }}
                >
                  Prompt
                </span>
              </div>
              <div style={{ fontSize: 14, fontWeight: 600, color: "var(--app-text-primary)" }}>{example.title}</div>
              <div
                style={{
                  fontSize: 13,
                  lineHeight: 1.6,
                  color: "var(--app-text-secondary)",
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {example.prompt}
              </div>
              <div
                style={{
                  borderTop: "1px solid var(--app-border)",
                  paddingTop: 8,
                  fontSize: 11.5,
                  lineHeight: 1.6,
                  color: "var(--app-text-muted)",
                }}
              >
                {example.hint}
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
