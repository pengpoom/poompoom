import { useEffect, useState } from "react";

import { fetchPublicSiteSettings, type PublicSiteSettings } from "@/lib/api";

export const DEFAULT_SITE_SETTINGS: PublicSiteSettings["site"] = {
  name: "ImageStudio",
  subtitle: "图片生成工作台",
  logoUrl: "",
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
