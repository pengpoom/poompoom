"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { CalendarDays, ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";
import type { TimeRangePreset, TimeRangeValue } from "@/components/time-range-utils";

const presetLabels: Record<TimeRangePreset, string> = {
  custom: "自定义",
  today: "今天",
  yesterday: "昨天",
  last24h: "近24小时",
  last7: "近 7 天",
  last14: "近 14 天",
  last30: "近 30 天",
  month: "本月",
  last_month: "上月",
};

const presetOptions: Array<{ value: TimeRangePreset; label: string }> = [
  { value: "today", label: presetLabels.today },
  { value: "yesterday", label: presetLabels.yesterday },
  { value: "last24h", label: presetLabels.last24h },
  { value: "last7", label: presetLabels.last7 },
  { value: "last14", label: presetLabels.last14 },
  { value: "last30", label: presetLabels.last30 },
  { value: "month", label: presetLabels.month },
  { value: "last_month", label: presetLabels.last_month },
];

function localDateValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function presetRange(preset: TimeRangePreset) {
  const today = new Date();
  const start = new Date(today);
  const end = new Date(today);
  if (preset === "yesterday") {
    start.setDate(today.getDate() - 1);
    end.setDate(today.getDate() - 1);
  } else if (preset === "last24h") {
    start.setDate(today.getDate() - 1);
  } else if (preset === "last7") {
    start.setDate(today.getDate() - 6);
  } else if (preset === "last14") {
    start.setDate(today.getDate() - 13);
  } else if (preset === "last30") {
    start.setDate(today.getDate() - 29);
  } else if (preset === "month") {
    start.setDate(1);
  } else if (preset === "last_month") {
    start.setDate(1);
    start.setMonth(today.getMonth() - 1);
    end.setDate(0);
  }
  return { from: localDateValue(start), to: localDateValue(end) };
}

export function TimeRangeFilter({
  value,
  onApply,
  className,
  panelAlign = "start",
  panelClassName,
}: {
  value: TimeRangeValue;
  onApply: (value: TimeRangeValue) => void;
  className?: string;
  panelAlign?: "start" | "end";
  panelClassName?: string;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const [panelStyle, setPanelStyle] = useState<CSSProperties | null>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open) {
      setDraft(value);
    }
  }, [open, value]);

  useEffect(() => {
    if (!open) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const root = rootRef.current;
      const panel = panelRef.current;
      if (!root || root.contains(event.target as Node) || panel?.contains(event.target as Node)) {
        return;
      }
      setOpen(false);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  useLayoutEffect(() => {
    if (!open) {
      return;
    }

    const updatePanelStyle = () => {
      const root = rootRef.current;
      if (!root) {
        return;
      }
      const rect = root.getBoundingClientRect();
      const width = Math.min(Math.max(rect.width, 320), Math.min(window.innerWidth - 32, 520));
      const left = panelAlign === "end"
        ? Math.max(16, Math.min(window.innerWidth - width - 16, rect.right - width))
        : Math.max(16, Math.min(window.innerWidth - width - 16, rect.left));
      const panelHeight = panelRef.current?.offsetHeight || 320;
      const belowTop = rect.bottom + 10;
      const top =
        belowTop + panelHeight + 16 > window.innerHeight && rect.top > panelHeight + 26
          ? rect.top - panelHeight - 10
          : Math.min(belowTop, Math.max(16, window.innerHeight - panelHeight - 16));
      setPanelStyle({
        position: "fixed",
        top,
        left,
        width,
        zIndex: 80,
      });
    };

    updatePanelStyle();
    window.addEventListener("resize", updatePanelStyle);
    window.addEventListener("scroll", updatePanelStyle, true);
    return () => {
      window.removeEventListener("resize", updatePanelStyle);
      window.removeEventListener("scroll", updatePanelStyle, true);
    };
  }, [open, panelAlign]);

  const buttonLabel = useMemo(() => {
    if (value.preset !== "custom") {
      return presetLabels[value.preset];
    }
    if (value.from || value.to) {
      return `${value.from || "开始"} - ${value.to || "结束"}`;
    }
    return "自定义";
  }, [value]);

  const choosePreset = (preset: TimeRangePreset) => {
    const range = presetRange(preset);
    setDraft({ preset, from: range.from, to: range.to });
  };

  const apply = () => {
    onApply(draft);
    setOpen(false);
  };

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        type="button"
        className="app-cs-trigger"
        style={{ width: "100%", justifyContent: "space-between" }}
        onClick={() => setOpen((current) => !current)}
      >
        <span className="inline-flex min-w-0 items-center gap-2">
          <CalendarDays className="size-4 shrink-0 text-[var(--app-text-muted)]" />
          <span className="truncate">{buttonLabel}</span>
        </span>
        <ChevronDown className={cn("size-4 shrink-0 text-[var(--app-text-muted)] transition", open ? "rotate-180" : "")} />
      </button>

      {mounted && open && panelStyle ? createPortal(
        <div
          ref={panelRef}
          style={panelStyle}
          className={cn(
            "overflow-hidden rounded-[18px] border border-[var(--app-border)] bg-[rgba(7,10,18,0.96)] shadow-[0_24px_70px_rgba(0,0,0,0.38),inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl",
            panelClassName,
          )}
        >
          <div className="grid grid-cols-2 gap-2 p-3">
            {presetOptions.map((option) => (
              <button
                key={option.value}
                type="button"
                className={cn(
                  "h-10 rounded-xl text-sm text-[var(--app-text-secondary)] transition hover:bg-white/[0.075] hover:text-white",
                  draft.preset === option.value ? "bg-cyan-400/16 text-cyan-100 ring-1 ring-cyan-300/25" : "",
                )}
                onClick={() => choosePreset(option.value)}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="grid gap-3 border-t border-[var(--app-border)] p-4 sm:grid-cols-[1fr_auto_1fr] sm:items-end">
            <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
              开始日期
              <input
                type="date"
                className="app-input"
                value={draft.from}
                onChange={(event) => setDraft((current) => ({ ...current, preset: "custom", from: event.target.value }))}
              />
            </label>
            <span className="hidden pb-2 text-[var(--app-text-muted)] sm:block">{"->"}</span>
            <label className="grid gap-1.5 text-xs text-[var(--app-text-muted)]">
              结束日期
              <input
                type="date"
                className="app-input"
                value={draft.to}
                onChange={(event) => setDraft((current) => ({ ...current, preset: "custom", to: event.target.value }))}
              />
            </label>
          </div>

          <div className="flex justify-end gap-2 px-4 pb-4">
            <button type="button" className="app-btn" onClick={() => setOpen(false)}>
              取消
            </button>
            <button type="button" className="app-btn-primary" onClick={apply}>
              应用
            </button>
          </div>
        </div>
      , document.body) : null}
    </div>
  );
}
