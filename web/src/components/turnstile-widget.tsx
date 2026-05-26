"use client";

import { useEffect, useRef, useState } from "react";

type TurnstileWidgetProps = {
  enabled?: boolean;
  siteKey?: string;
  action?: string;
  disabled?: boolean;
  resetKey?: number;
  onTokenChange: (token: string) => void;
};

declare global {
  interface Window {
    turnstile?: {
      render: (
        container: HTMLElement,
        options: {
          sitekey: string;
          action?: string;
          theme?: "auto";
          callback?: (token: string) => void;
          "expired-callback"?: () => void;
          "error-callback"?: () => void;
        },
      ) => string;
      reset: (widgetId?: string) => void;
      remove?: (widgetId?: string) => void;
    };
  }
}

const scriptId = "cloudflare-turnstile-script";
let scriptPromise: Promise<void> | null = null;

function loadTurnstileScript() {
  if (typeof window === "undefined") {
    return Promise.resolve();
  }
  if (window.turnstile) {
    return Promise.resolve();
  }
  if (scriptPromise) {
    return scriptPromise;
  }
  scriptPromise = new Promise((resolve, reject) => {
    const existing = document.getElementById(scriptId) as HTMLScriptElement | null;
    if (existing) {
      existing.addEventListener("load", () => resolve(), { once: true });
      existing.addEventListener("error", () => reject(new Error("turnstile script failed")), { once: true });
      return;
    }
    const script = document.createElement("script");
    script.id = scriptId;
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("turnstile script failed"));
    document.head.appendChild(script);
  });
  return scriptPromise;
}

export function TurnstileWidget({
  enabled = false,
  siteKey = "",
  action,
  disabled = false,
  resetKey = 0,
  onTokenChange,
}: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const widgetIdRef = useRef<string>("");
  const [failed, setFailed] = useState(false);
  const normalizedSiteKey = siteKey.trim();

  useEffect(() => {
    if (!enabled || !normalizedSiteKey || !containerRef.current) {
      onTokenChange("");
      return;
    }
    let cancelled = false;
    setFailed(false);
    onTokenChange("");
    void loadTurnstileScript()
      .then(() => {
        if (cancelled || !containerRef.current || !window.turnstile) {
          return;
        }
        if (widgetIdRef.current) {
          window.turnstile.remove?.(widgetIdRef.current);
          widgetIdRef.current = "";
        }
        widgetIdRef.current = window.turnstile.render(containerRef.current, {
          sitekey: normalizedSiteKey,
          action,
          theme: "auto",
          callback: (token) => onTokenChange(token),
          "expired-callback": () => onTokenChange(""),
          "error-callback": () => {
            onTokenChange("");
            setFailed(true);
          },
        });
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
          onTokenChange("");
        }
      });

    return () => {
      cancelled = true;
      if (widgetIdRef.current && window.turnstile?.remove) {
        window.turnstile.remove(widgetIdRef.current);
        widgetIdRef.current = "";
      }
    };
  }, [action, enabled, normalizedSiteKey, onTokenChange]);

  useEffect(() => {
    if (!enabled || disabled || !widgetIdRef.current || !window.turnstile) {
      return;
    }
    window.turnstile.reset(widgetIdRef.current);
    onTokenChange("");
  }, [disabled, enabled, onTokenChange, resetKey]);

  if (!enabled || !normalizedSiteKey) {
    return null;
  }

  return (
    <div className="grid gap-2">
      <div
        className="min-h-[65px] overflow-hidden rounded-lg border border-white/15 bg-[#101723]"
        aria-disabled={disabled}
      >
        <div ref={containerRef} className="min-h-[65px]" />
      </div>
      {failed ? (
        <p className="text-xs leading-5 text-rose-200">人机验证加载失败，请刷新页面后重试。</p>
      ) : null}
    </div>
  );
}
