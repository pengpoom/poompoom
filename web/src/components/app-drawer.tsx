"use client";

import { useEffect, useRef, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";

export function AppDrawer({
  open,
  onClose,
  width = "min(900px, 55vw)",
  title,
  actions,
  children,
}: {
  open: boolean;
  onClose: () => void;
  width?: string | number;
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
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
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  const downOnOverlayRef = useRef(false);
  if (typeof document === "undefined") return null;

  return createPortal(
    <div
      className={`app-drawer-overlay${open ? " open" : ""}`}
      aria-hidden={!open}
      onMouseDown={(e: ReactMouseEvent<HTMLDivElement>) => {
        downOnOverlayRef.current = e.target === e.currentTarget;
      }}
      onClick={(e: ReactMouseEvent<HTMLDivElement>) => {
        if (downOnOverlayRef.current && e.target === e.currentTarget) {
          onClose();
        }
        downOnOverlayRef.current = false;
      }}
    >
      <div className="app-drawer" style={{ width }}>
        {title || actions ? (
          <div className="app-drawer-head">
            <div style={{ minWidth: 0, flex: 1 }}>{title}</div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              {actions}
              <button className="app-drawer-close" type="button" onClick={onClose} aria-label="关闭">×</button>
            </div>
          </div>
        ) : null}
        <div className="app-drawer-body">{children}</div>
      </div>
    </div>,
    document.body,
  );
}
