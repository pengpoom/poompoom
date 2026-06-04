import { useEffect, useState } from "react";

import { fetchPublicSiteSettings, type PublicSiteSettings } from "@/lib/api";

export const DEFAULT_SITE_SETTINGS: PublicSiteSettings["site"] = {
  name: "ImageStudio",
  subtitle: "图片生成工作台",
  logoUrl: "",
};
export const DEFAULT_TURNSTILE_SETTINGS = {
  enabled: false,
  siteKey: "",
  login: false,
  registerCode: false,
  registerSubmit: false,
  passwordReset: false,
};

export const SITE_SETTINGS_CHANGED_EVENT = "image-studio:site-settings-changed";

type SiteSettingsChangedDetail = PublicSiteSettings["site"];

function normalizeSiteSettings(value: Partial<PublicSiteSettings["site"]> | undefined | null) {
  return {
    name: String(value?.name || "").trim() || DEFAULT_SITE_SETTINGS.name,
    subtitle: String(value?.subtitle || "").trim() || DEFAULT_SITE_SETTINGS.subtitle,
    logoUrl: String(value?.logoUrl || "").trim(),
  };
}

export function usePublicSiteSettings() {
  const [site, setSite] = useState(DEFAULT_SITE_SETTINGS);

  useEffect(() => {
    let cancelled = false;

    const loadSiteSettings = async () => {
      try {
        const result = await fetchPublicSiteSettings();
        if (!cancelled) {
          setSite(normalizeSiteSettings(result.site));
        }
      } catch {
        if (!cancelled) {
          setSite(DEFAULT_SITE_SETTINGS);
        }
      }
    };

    void loadSiteSettings();
    const handleSiteSettingsChanged = (event: Event) => {
      const detail = (event as CustomEvent<SiteSettingsChangedDetail>).detail;
      setSite(normalizeSiteSettings(detail));
    };
    window.addEventListener(SITE_SETTINGS_CHANGED_EVENT, handleSiteSettingsChanged as EventListener);
    return () => {
      cancelled = true;
      window.removeEventListener(SITE_SETTINGS_CHANGED_EVENT, handleSiteSettingsChanged as EventListener);
    };
  }, []);

  return site;
}

export function usePublicTurnstileSettings() {
  const [turnstile, setTurnstile] = useState(DEFAULT_TURNSTILE_SETTINGS);

  useEffect(() => {
    let cancelled = false;

    const loadSettings = async () => {
      try {
        const result = await fetchPublicSiteSettings();
        if (!cancelled) {
          setTurnstile(normalizeTurnstileSettings(result.turnstile));
        }
      } catch {
        if (!cancelled) {
          setTurnstile(DEFAULT_TURNSTILE_SETTINGS);
        }
      }
    };

    void loadSettings();
    return () => {
      cancelled = true;
    };
  }, []);

  return turnstile;
}

export function dispatchSiteSettingsChanged(site: PublicSiteSettings["site"]) {
  window.dispatchEvent(new CustomEvent<SiteSettingsChangedDetail>(SITE_SETTINGS_CHANGED_EVENT, {
    detail: normalizeSiteSettings(site),
  }));
}

export function siteInitials(name: string) {
  const normalized = String(name || "").trim();
  if (!normalized) {
    return "PI";
  }
  const asciiWords = normalized.match(/[A-Za-z0-9]+/g);
  if (asciiWords?.length) {
    return asciiWords.slice(0, 2).map((word) => word[0]).join("").toUpperCase();
  }
  return Array.from(normalized).slice(0, 2).join("");
}

function normalizeTurnstileSettings(value: PublicSiteSettings["turnstile"] | undefined | null) {
  const siteKey = String(value?.siteKey || "").trim();
  const enabled = Boolean(value?.enabled && siteKey);
  return {
    enabled,
    siteKey,
    login: enabled && Boolean(value?.login),
    registerCode: enabled && Boolean(value?.registerCode),
    registerSubmit: enabled && Boolean(value?.registerSubmit),
    passwordReset: enabled && Boolean(value?.passwordReset),
  };
}
