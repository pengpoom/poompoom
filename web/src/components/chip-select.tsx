"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";

export type ChipSelectOption<T extends string> = {
  value: T;
  label: ReactNode;
  description?: string;
  disabled?: boolean;
};

export function ChipSelect<T extends string>({
  value,
  options,
  onChange,
  triggerClassName,
  triggerIcon,
  triggerLabel,
  disabled,
  title,
}: {
  value: T;
  options: Array<ChipSelectOption<T>>;
  onChange: (value: T) => void;
  triggerClassName?: string;
  triggerIcon?: ReactNode;
  triggerLabel: ReactNode;
  disabled?: boolean;
  title?: string;
}) {
  const [open, setOpen] = useState(false);
  const [panelStyle, setPanelStyle] = useState<CSSProperties | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const closePanel = useCallback(() => {
    setOpen(false);
    setPanelStyle(null);
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    const update = () => {
      const button = triggerRef.current;
      if (!button) return;
      const rect = button.getBoundingClientRect();
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;
      const margin = 12;
      const gap = 6;
      const panelWidth = rect.width;
      const panelHeight = panelRef.current?.offsetHeight || 0;
      const left = Math.min(Math.max(rect.left, margin), Math.max(margin, viewportWidth - panelWidth - margin));
      const hasRoomAbove = rect.top - gap - panelHeight > margin;
      const top = hasRoomAbove ? rect.top - gap - panelHeight : Math.min(rect.bottom + gap, viewportHeight - panelHeight - margin);
      setPanelStyle({ position: "fixed", left, top: Math.max(margin, top), minWidth: panelWidth, zIndex: 80 });
    };
    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: PointerEvent) => {
      const target = e.target as Node;
      if (panelRef.current?.contains(target) || triggerRef.current?.contains(target)) return;
      closePanel();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closePanel();
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [closePanel, open]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={cn(triggerClassName, "inline-flex items-center")}
        onClick={() => {
          if (disabled) return;
          setOpen((current) => {
            if (current) setPanelStyle(null);
            return !current;
          });
        }}
        aria-expanded={open}
        disabled={disabled}
        title={title}
      >
        {triggerIcon}
        <span className="min-w-0 flex-1 truncate text-center">{triggerLabel}</span>
        <ChevronDown className={cn("size-4 opacity-65 transition", open && "rotate-180")} />
      </button>
      {open && typeof document !== "undefined"
        ? createPortal(
            <div
              ref={panelRef}
              className="app-cs-panel"
              style={panelStyle ?? { position: "fixed", top: 0, left: 0, visibility: "hidden", zIndex: 80, minWidth: 0 }}
            >
              {options.map((opt) => (
                <button
                  key={opt.value}
                  className={cn("app-cs-opt", value === opt.value && "is-on")}
                  type="button"
                  disabled={opt.disabled}
                  title={opt.description}
                  onClick={() => {
                    onChange(opt.value);
                    closePanel();
                  }}
                >
                  {opt.label}
                </button>
              ))}
            </div>,
            document.body,
          )
        : null}
    </>
  );
}
