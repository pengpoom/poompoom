"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Calendar, ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { type TimeRangePreset, type TimeRangeValue } from "@/components/time-range-utils";

/* ── AnnounceModal: 紧凑公告弹窗(大 emoji + 居中标题 + 短描述 + 单按钮) ── */

export function AnnounceModal({
  open,
  onClose,
  icon = "🔔",
  title,
  body,
  confirmText = "我知道了",
  onConfirm,
}: {
  open: boolean;
  onClose: () => void;
  icon?: ReactNode;
  title: string;
  body: ReactNode;
  confirmText?: string;
  onConfirm?: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  if (!open) return null;

  const handleConfirm = () => {
    onConfirm?.();
    onClose();
  };

  return createPortal(
    <div
      className="app-modal-overlay open"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="app-modal announce-modal">
        <div className="app-modal-body">
          <div className="ann-icon">{icon}</div>
          <div className="ann-title">{title}</div>
          <div className="ann-body">{body}</div>
        </div>
        <div className="app-modal-foot">
          <button className="app-btn-primary" type="button" style={{ minWidth: 120 }} onClick={handleConfirm}>
            {confirmText}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/* ── AppToast: 右下角浮动通知 ── */

export type AppToastLevel = "success" | "warning" | "fail" | "info";

export type AppToastItem = {
  id: string;
  level: AppToastLevel;
  title: string;
  summary?: ReactNode;
  durationMs?: number;
};

export function AppToastStack({
  items,
  onDismiss,
}: {
  items: AppToastItem[];
  onDismiss: (id: string) => void;
}) {
  if (typeof document === "undefined") return null;
  return createPortal(
    <div className="app-toast-wrap">
      {items.map((item) => (
        <AppToast key={item.id} item={item} onDismiss={onDismiss} />
      ))}
    </div>,
    document.body,
  );
}

function AppToast({ item, onDismiss }: { item: AppToastItem; onDismiss: (id: string) => void }) {
  const [show, setShow] = useState(false);
  const [out, setOut] = useState(false);

  const dismiss = useCallback(() => {
    setOut(true);
    window.setTimeout(() => onDismiss(item.id), 260);
  }, [item.id, onDismiss]);

  useEffect(() => {
    const enter = window.setTimeout(() => setShow(true), 16);
    const duration = item.durationMs ?? 4000;
    const auto = duration > 0 ? window.setTimeout(dismiss, duration) : 0;
    return () => {
      window.clearTimeout(enter);
      if (auto) window.clearTimeout(auto);
    };
  }, [dismiss, item.durationMs]);

  return (
    <div className={cn("app-toast", show && !out && "show", out && "out")}>
      <div className={cn("toast-bar", item.level)} />
      <div className="toast-body">
        <div className="toast-title">{item.title}</div>
        {item.summary ? <div className="toast-summary">{item.summary}</div> : null}
      </div>
      <button className="toast-close" type="button" onClick={dismiss} aria-label="关闭">×</button>
    </div>
  );
}

/* ── AppModal ── */

export function AppModal({
  open,
  onClose,
  title,
  children,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = prev; };
  }, [open]);

  const downOnOverlayRef = useRef(false);

  if (!open) return null;

  const handleMouseDown = (e: ReactMouseEvent<HTMLDivElement>) => {
    downOnOverlayRef.current = e.target === e.currentTarget;
  };
  const handleClick = (e: ReactMouseEvent<HTMLDivElement>) => {
    if (downOnOverlayRef.current && e.target === e.currentTarget) {
      onClose();
    }
    downOnOverlayRef.current = false;
  };

  return createPortal(
    <div className="app-modal-overlay open" onMouseDown={handleMouseDown} onClick={handleClick}>
      <div className="app-modal">
        <div className="app-modal-head">
          <h3>{title}</h3>
          <button className="app-modal-close" type="button" onClick={onClose} aria-label="关闭">×</button>
        </div>
        <div className="app-modal-body">
          {children}
        </div>
        {footer ? (
          <div className="app-modal-foot">
            {footer}
          </div>
        ) : null}
      </div>
    </div>,
    document.body,
  );
}

/* ── AppSelect ── */

export type AppSelectOption = { value: string; label: string };

export function AppSelect({
  value,
  onChange,
  options,
  placeholder = "请选择",
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  options: AppSelectOption[];
  placeholder?: string;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", handler);
    return () => document.removeEventListener("pointerdown", handler);
  }, [open]);

  const selected = options.find((o) => o.value === value);

  return (
    <div className={cn("app-cs", open && "open", className)} ref={ref}>
      <button className="app-cs-trigger" type="button" onClick={() => setOpen(!open)}>
        <span>{selected?.label || placeholder}</span>
        <ChevronDown className="app-cs-arrow" />
      </button>
      {open ? (
        <div className="app-cs-panel">
          {options.map((opt) => (
            <button
              key={opt.value}
              className={cn("app-cs-opt", value === opt.value && "is-on")}
              type="button"
              onClick={() => { onChange(opt.value); setOpen(false); }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

/* ── Calendar helpers ── */

const WEEK_LABELS = ["日", "一", "二", "三", "四", "五", "六"];

function pad2(n: number) { return n < 10 ? `0${n}` : `${n}`; }

function toDateStr(y: number, m: number, d: number) {
  return `${y}-${pad2(m + 1)}-${pad2(d)}`;
}

function buildCalendarDays(year: number, month: number) {
  const firstDay = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const daysInPrev = new Date(year, month, 0).getDate();
  const days: Array<{ day: number; month: number; year: number; other: boolean }> = [];

  for (let i = firstDay - 1; i >= 0; i--) {
    const pm = month === 0 ? 11 : month - 1;
    const py = month === 0 ? year - 1 : year;
    days.push({ day: daysInPrev - i, month: pm, year: py, other: true });
  }
  for (let d = 1; d <= daysInMonth; d++) {
    days.push({ day: d, month, year, other: false });
  }
  const remaining = 42 - days.length;
  for (let d = 1; d <= remaining; d++) {
    const nm = month === 11 ? 0 : month + 1;
    const ny = month === 11 ? year + 1 : year;
    days.push({ day: d, month: nm, year: ny, other: true });
  }
  return days;
}

/* ── AppDatePicker ── */

function splitValue(value: string): { date: string; time: string } {
  if (!value) return { date: "", time: "" };
  const [datePart, timePart] = value.split("T");
  return { date: datePart || "", time: (timePart || "").slice(0, 5) };
}

function formatDateDisplay(value: string, withTime: boolean) {
  if (!value) return "";
  const { date, time } = splitValue(value);
  if (!date) return "";
  const [y, m, d] = date.split("-");
  const datePart = `${y} / ${m} / ${d}`;
  if (!withTime) return datePart;
  return `${datePart} ${time || "00:00"}`;
}

export function AppDatePicker({
  value,
  onChange,
  placeholder = "选择日期",
  withTime = false,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  withTime?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popRef = useRef<HTMLDivElement>(null);
  const [popStyle, setPopStyle] = useState<React.CSSProperties>({});

  const { date: dateValue, time: timeValue } = useMemo(() => splitValue(value), [value]);

  const parsed = useMemo(() => {
    if (!dateValue) return null;
    const d = new Date(dateValue);
    return Number.isNaN(d.getTime()) ? null : d;
  }, [dateValue]);

  const [viewYear, setViewYear] = useState(() => parsed?.getFullYear() ?? new Date().getFullYear());
  const [viewMonth, setViewMonth] = useState(() => parsed?.getMonth() ?? new Date().getMonth());

  const days = useMemo(() => buildCalendarDays(viewYear, viewMonth), [viewYear, viewMonth]);

  const today = useMemo(() => {
    const n = new Date();
    return toDateStr(n.getFullYear(), n.getMonth(), n.getDate());
  }, []);

  const positionPop = useCallback(() => {
    const btn = triggerRef.current;
    if (!btn) return;
    const rect = btn.getBoundingClientRect();
    setPopStyle({
      position: "fixed",
      zIndex: 90,
      top: rect.bottom + 6,
      left: rect.left,
    });
  }, []);

  useEffect(() => {
    if (!open) return;
    positionPop();
    const handler = (e: PointerEvent) => {
      if (ref.current?.contains(e.target as Node) || popRef.current?.contains(e.target as Node)) return;
      setOpen(false);
    };
    document.addEventListener("pointerdown", handler);
    window.addEventListener("scroll", positionPop, true);
    window.addEventListener("resize", positionPop);
    return () => {
      document.removeEventListener("pointerdown", handler);
      window.removeEventListener("scroll", positionPop, true);
      window.removeEventListener("resize", positionPop);
    };
  }, [open, positionPop]);

  const handleOpen = () => {
    if (!open && parsed) {
      setViewYear(parsed.getFullYear());
      setViewMonth(parsed.getMonth());
    }
    setOpen(!open);
  };

  const prevMonth = () => {
    if (viewMonth === 0) { setViewYear(viewYear - 1); setViewMonth(11); }
    else setViewMonth(viewMonth - 1);
  };
  const nextMonth = () => {
    if (viewMonth === 11) { setViewYear(viewYear + 1); setViewMonth(0); }
    else setViewMonth(viewMonth + 1);
  };

  const handleSelect = (d: typeof days[0]) => {
    const dateStr = toDateStr(d.year, d.month, d.day);
    if (withTime) {
      onChange(`${dateStr}T${timeValue || "00:00"}`);
    } else {
      onChange(dateStr);
      setOpen(false);
    }
  };

  const handleTimeChange = (next: string) => {
    if (!dateValue) {
      const n = new Date();
      const today = toDateStr(n.getFullYear(), n.getMonth(), n.getDate());
      onChange(`${today}T${next}`);
    } else {
      onChange(`${dateValue}T${next}`);
    }
  };

  return (
    <div className={cn("app-dp", open && "open")} ref={ref}>
      <button className="app-dp-trigger" type="button" ref={triggerRef} onClick={handleOpen}>
        <span>{value ? formatDateDisplay(value, withTime) : placeholder}</span>
        <Calendar className="app-dp-icon" />
      </button>
      {open ? createPortal(
        <div className="app-dp-pop" ref={popRef} style={popStyle}>
          <div className="app-dp-head">
            <button type="button" onClick={prevMonth}><ChevronLeft /></button>
            <span>{viewYear} 年 {viewMonth + 1} 月</span>
            <button type="button" onClick={nextMonth}><ChevronRight /></button>
          </div>
          <div className="app-dp-week">
            {WEEK_LABELS.map((w) => <span key={w}>{w}</span>)}
          </div>
          <div className="app-dp-grid">
            {days.map((d, i) => {
              const dateStr = toDateStr(d.year, d.month, d.day);
              return (
                <button
                  key={i}
                  type="button"
                  className={cn(
                    "app-dp-day",
                    d.other && "other",
                    dateStr === today && "today",
                    dateStr === dateValue && "sel",
                  )}
                  onClick={() => handleSelect(d)}
                >
                  {d.day}
                </button>
              );
            })}
          </div>
          {withTime ? (
            <div className="app-dp-time">
              <span>时间</span>
              <input
                type="time"
                className="app-input"
                value={timeValue || "00:00"}
                onChange={(e) => handleTimeChange(e.target.value)}
              />
              <button className="app-btn-primary" type="button" onClick={() => setOpen(false)}>确定</button>
            </div>
          ) : null}
        </div>,
        document.body,
      ) : null}
    </div>
  );
}

/* ── AppTimeSeg ── */

const segPresets: Array<{ key: string; label: string; preset?: TimeRangePreset }> = [
  { key: "today", label: "今天", preset: "today" },
  { key: "last7", label: "近 7 天", preset: "last7" },
  { key: "last30", label: "近 30 天", preset: "last30" },
  { key: "custom", label: "自定义" },
  { key: "all", label: "全部" },
];

export function AppTimeSeg({
  value,
  onChange,
  defaultSeg = "last7",
}: {
  value: TimeRangeValue;
  onChange: (value: TimeRangeValue) => void;
  defaultSeg?: string;
}) {
  const [activeSeg, setActiveSeg] = useState(defaultSeg);
  const [showRange, setShowRange] = useState(false);
  const [fromDate, setFromDate] = useState(value.from || "");
  const [toDate, setToDate] = useState(value.to || "");
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showRange) return;
    const handler = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setShowRange(false);
    };
    document.addEventListener("pointerdown", handler);
    return () => document.removeEventListener("pointerdown", handler);
  }, [showRange]);

  const handleSeg = (key: string, preset?: TimeRangePreset) => {
    setActiveSeg(key);
    if (key === "custom") {
      setShowRange(true);
      return;
    }
    setShowRange(false);
    if (preset) {
      onChange({ preset, from: "", to: "" });
    } else {
      onChange({ preset: undefined as unknown as TimeRangePreset, from: "", to: "" });
    }
  };

  const handleApply = () => {
    onChange({ preset: undefined as unknown as TimeRangePreset, from: fromDate, to: toDate });
    setShowRange(false);
  };

  return (
    <div ref={wrapRef} style={{ position: "relative" }}>
      <div className="app-seg">
        {segPresets.map((s) => (
          <button
            key={s.key}
            type="button"
            className={activeSeg === s.key ? "on" : ""}
            onClick={() => handleSeg(s.key, s.preset)}
          >
            {s.label}
          </button>
        ))}
      </div>
      {showRange ? (
        <div className="app-daterange" style={{ position: "absolute", top: "calc(100% + 8px)", left: 0, padding: "10px 14px" }}>
          <span className="dr-label">开始</span>
          <AppDatePicker value={fromDate} onChange={setFromDate} placeholder="起始日期" />
          <span className="sep">~</span>
          <span className="dr-label">结束</span>
          <AppDatePicker value={toDate} onChange={setToDate} placeholder="结束日期" />
          <button className="app-btn-primary" type="button" onClick={handleApply}>应用</button>
        </div>
      ) : null}
    </div>
  );
}
