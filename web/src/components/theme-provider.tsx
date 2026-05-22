"use client";

import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { Toaster } from "sonner";

export type ThemeMode = "light" | "graphite" | "dark" | "system";
type ResolvedThemeMode = Exclude<ThemeMode, "system">;

type ThemeContextValue = {
  themeMode: ThemeMode;
  resolvedThemeMode: ResolvedThemeMode;
  isDark: boolean;
  isDarkLike: boolean;
  setThemeMode: (mode: ThemeMode) => void;
  toggleTheme: () => void;
};

const THEME_STORAGE_KEY = "image-studio:theme";

const ThemeContext = createContext<ThemeContextValue | null>(null);

function normalizeThemeMode(value: string | null | undefined, fallback: ThemeMode = "dark"): ThemeMode {
  if (value === "light" || value === "dark" || value === "graphite" || value === "system") {
    return value;
  }
  return fallback;
}

function readStoredThemeMode(): ThemeMode {
  if (typeof window === "undefined") {
    return "dark";
  }
  try {
    return normalizeThemeMode(window.localStorage.getItem(THEME_STORAGE_KEY), "dark");
  } catch {
    return "dark";
  }
}

function readSystemDarkPreference() {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function applyThemeMode(mode: ResolvedThemeMode, sourceMode: ThemeMode) {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  const isDarkLike = mode !== "light";
  root.classList.toggle("dark", isDarkLike);
  root.classList.toggle("theme-dark", mode === "dark");
  root.classList.toggle("theme-graphite", mode === "graphite");
  root.dataset.theme = sourceMode;
  root.dataset.resolvedTheme = mode;
  root.style.colorScheme = isDarkLike ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [themeMode, setThemeModeState] = useState<ThemeMode>(() =>
    readStoredThemeMode(),
  );
  const [systemDark, setSystemDark] = useState(() => readSystemDarkPreference());
  const resolvedThemeMode: ResolvedThemeMode = themeMode === "system" ? (systemDark ? "dark" : "light") : themeMode;

  useLayoutEffect(() => {
    applyThemeMode(resolvedThemeMode, themeMode);
  }, [resolvedThemeMode, themeMode]);

  useEffect(() => {
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, themeMode);
    } catch {
      // 忽略本地存储失败，保持当前会话内主题可用。
    }
  }, [themeMode]);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const handleChange = () => setSystemDark(media.matches);
    handleChange();
    media.addEventListener("change", handleChange);
    return () => {
      media.removeEventListener("change", handleChange);
    };
  }, []);

  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key !== THEME_STORAGE_KEY) {
        return;
      }
      setThemeModeState(normalizeThemeMode(event.newValue));
    };

    window.addEventListener("storage", handleStorage);
    return () => {
      window.removeEventListener("storage", handleStorage);
    };
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({
      themeMode,
      resolvedThemeMode,
      isDark: resolvedThemeMode === "dark",
      isDarkLike: resolvedThemeMode !== "light",
      setThemeMode: (mode) => {
        setThemeModeState((current) => (current === mode ? current : mode));
      },
      toggleTheme: () => {
        setThemeModeState((current) => {
          if (current === "light") {
            return "graphite";
          }
          if (current === "graphite") {
            return "dark";
          }
          if (current === "dark") {
            return "system";
          }
          return "light";
        });
      },
    }),
    [resolvedThemeMode, themeMode],
  );

  return (
    <ThemeContext.Provider value={value}>
      <Toaster
        position="top-center"
        richColors
        theme={resolvedThemeMode === "light" ? "light" : "dark"}
      />
      {children}
    </ThemeContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
}
