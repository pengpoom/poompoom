"use client";

import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  Activity,
  BarChart3,
  Bell,
  Boxes,
  Check,
  Database,
  FileText,
  Gauge,
  History,
  ImageIcon,
  LogOut,
  Menu,
  Moon,
  RefreshCw,
  ScrollText,
  Settings,
  Share2,
  Sparkles,
  Sun,
  SwatchBook,
  Wrench,
  UsersRound,
  type LucideIcon,
} from "lucide-react";

import { VersionUpdateDialog } from "@/components/version-update-dialog";
import { useTheme, type ThemeMode } from "@/components/theme-provider";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { fetchBusinessCredit, fetchVersionInfo, logout } from "@/lib/api";
import { usePublicSiteSettings } from "@/lib/site-settings";
import {
  AUTH_STATE_CHANGED_EVENT,
  beginIntentionalLogout,
  clearStoredAuthKey,
  finishIntentionalLogout,
  getStoredAuthUsername,
  type AuthRole,
} from "@/store/auth";
import { cn } from "@/lib/utils";

type ShellNavItem = {
  href: string;
  matchPrefix: string;
  label: string;
  icon: LucideIcon;
};

const adminItems: readonly ShellNavItem[] = [
  { href: "/admin/dashboard", matchPrefix: "/admin/dashboard", label: "仪表盘", icon: BarChart3 },
  { href: "/admin/operations", matchPrefix: "/admin/operations", label: "运维", icon: Gauge },
  { href: "/admin/usage", matchPrefix: "/admin/usage", label: "记录", icon: History },
  { href: "/users", matchPrefix: "/users", label: "用户", icon: UsersRound },
  { href: "/accounts", matchPrefix: "/accounts", label: "接入", icon: Activity },
  { href: "/storage", matchPrefix: "/storage", label: "存储", icon: Database },
];

const userItems: readonly ShellNavItem[] = [
  { href: "/image/history", matchPrefix: "/image", label: "生图", icon: ImageIcon },
  { href: "/tools", matchPrefix: "/tools", label: "工具", icon: Wrench },
];

const libraryItems: readonly ShellNavItem[] = [
  { href: "/assets", matchPrefix: "/assets", label: "资产", icon: Boxes },
  { href: "/community", matchPrefix: "/community", label: "社区", icon: Share2 },
];

const usageItems: readonly ShellNavItem[] = [
  { href: "/usage", matchPrefix: "/usage", label: "用量", icon: History },
];

function isActive(pathname: string, item: ShellNavItem) {
  return pathname === item.matchPrefix || pathname.startsWith(`${item.matchPrefix}/`);
}

function navItemsForRole(role: AuthRole | null) {
  if (role === "admin") {
    return [...adminItems, ...userItems, ...libraryItems, ...usageItems];
  }
  return [...userItems, ...libraryItems, ...usageItems];
}

function formatVersionLabel(value: string) {
  const normalized = String(value || "").trim();
  if (!normalized) {
    return "读取中";
  }

  const semanticMatch = normalized.match(/v?(\d+\.\d+\.\d+)/i);
  if (semanticMatch?.[1]) {
    return `v${semanticMatch[1]}`;
  }

  return normalized;
}

function avatarText(username: string, role: AuthRole | null) {
  const fallback = role === "admin" ? "管理员" : "用户";
  const normalized = String(username || fallback).trim();
  if (!normalized) {
    return "U";
  }
  if (/^[a-z0-9_-]+$/i.test(normalized)) {
    return normalized.slice(0, 4);
  }
  return Array.from(normalized).slice(0, 3).join("");
}

function BrandMark({
  logoUrl,
  siteName,
  className,
  iconClassName = "size-5",
}: {
  logoUrl?: string;
  siteName: string;
  className?: string;
  iconClassName?: string;
}) {
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [logoUrl]);

  const imageClassName = cn(
    "app-logo-image-frame grid place-items-center overflow-hidden rounded-[12px] border border-[var(--app-border-strong)] text-white shadow-[0_0_28px_rgba(40,214,255,0.16),inset_0_1px_0_rgba(255,255,255,0.16)]",
    className,
  );
  const fallbackClassName = cn(
    "grid place-items-center overflow-hidden rounded-[12px] border border-[var(--app-border-strong)] bg-[linear-gradient(135deg,rgba(47,107,255,0.86),rgba(40,214,255,0.62))] text-white shadow-[0_0_28px_rgba(40,214,255,0.20),inset_0_1px_0_rgba(255,255,255,0.22)]",
    className,
  );

  if (logoUrl && !failed) {
    return (
      <span className={imageClassName}>
        <img
          src={logoUrl}
          alt={siteName}
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      </span>
    );
  }

  return (
    <span className={fallbackClassName}>
      <Sparkles className={iconClassName} />
    </span>
  );
}

const railActionClass =
  "grid h-[48px] w-[58px] place-items-center rounded-[16px] text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]";
const railUtilityActionClass =
  "grid h-[38px] w-[58px] place-items-center rounded-[14px] text-[var(--app-text-muted)] transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]";
const railActionIconClass = "size-5 stroke-[1.9]";
const menuPanelClass =
  "w-[184px] rounded-[18px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-1.5 text-[13px] font-semibold text-[var(--app-text-secondary)] shadow-[var(--app-shadow-floating)] backdrop-blur-2xl";
const menuItemClass =
  "flex h-10 w-full items-center gap-2 rounded-[12px] px-3 text-left transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]";
const themeOptions: Array<{ mode: ThemeMode; label: string; icon: LucideIcon }> = [
  { mode: "dark", label: "深色模式", icon: Moon },
  { mode: "light", label: "浅色模式", icon: Sun },
  { mode: "graphite", label: "灰色模式", icon: SwatchBook },
  { mode: "system", label: "跟随系统", icon: Sparkles },
];

export function AppShellNav({ role = null }: { role?: AuthRole | null }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const { themeMode, setThemeMode } = useTheme();
  const moreMenuRef = useRef<HTMLDivElement | null>(null);
  const moreMenuButtonRef = useRef<HTMLButtonElement | null>(null);
  const moreMenuPanelRef = useRef<HTMLDivElement | null>(null);
  const railNavRef = useRef<HTMLElement | null>(null);
  const [balance, setBalance] = useState<number | null>(null);
  const [username, setUsername] = useState("");
  const [moreMenuOpen, setMoreMenuOpen] = useState(false);
  const [moreMenuStyle, setMoreMenuStyle] = useState<CSSProperties | null>(null);
  const [mounted, setMounted] = useState(false);
  const [versionDialogOpen, setVersionDialogOpen] = useState(false);
  const [notificationDialogOpen, setNotificationDialogOpen] = useState(false);
  const [hasUnreadNotifications, setHasUnreadNotifications] = useState(true);
  const [infoDialog, setInfoDialog] = useState<"terms" | "changelog" | null>(null);
  const [versionLabel, setVersionLabel] = useState("读取中");

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void fetchBusinessCredit()
      .then((credit) => {
        if (!cancelled) {
          setBalance(credit.balance);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setBalance(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const loadUsername = async () => {
      const storedUsername = await getStoredAuthUsername();
      if (!cancelled) {
        setUsername(storedUsername);
      }
    };
    void loadUsername();
    const handleAuthChange = () => {
      void loadUsername();
    };
    window.addEventListener(AUTH_STATE_CHANGED_EVENT, handleAuthChange);
    return () => {
      cancelled = true;
      window.removeEventListener(AUTH_STATE_CHANGED_EVENT, handleAuthChange);
    };
  }, [role]);

  useEffect(() => {
    if (role !== "admin") {
      return;
    }
    let cancelled = false;
    const loadVersion = async () => {
      try {
        const payload = await fetchVersionInfo();
        if (!cancelled) {
          setVersionLabel(formatVersionLabel(payload.version));
        }
      } catch {
        if (!cancelled) {
          setVersionLabel("未知版本");
        }
      }
    };
    void loadVersion();
    return () => {
      cancelled = true;
    };
  }, [role]);

  useEffect(() => {
    if (!moreMenuOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const anchor = moreMenuRef.current;
      const panel = moreMenuPanelRef.current;
      if (anchor?.contains(event.target as Node) || panel?.contains(event.target as Node)) {
        return;
      }
      setMoreMenuOpen(false);
      setMoreMenuStyle(null);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMoreMenuOpen(false);
        setMoreMenuStyle(null);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [moreMenuOpen]);

  useLayoutEffect(() => {
    if (!moreMenuOpen) {
      return;
    }

    const updateMenuStyle = () => {
      const button = moreMenuButtonRef.current;
      if (!button) {
        return;
      }
      const rect = button.getBoundingClientRect();
      const panelHeight = role === "admin" ? 252 : 172;
      const panelWidth = 184;
      const left = Math.min(window.innerWidth - panelWidth - 12, rect.right + 8);
      const top = Math.max(12, Math.min(window.innerHeight - panelHeight - 12, rect.bottom - panelHeight));
      setMoreMenuStyle({
        position: "fixed",
        left,
        top,
        zIndex: 70,
      });
    };

    updateMenuStyle();
    window.addEventListener("resize", updateMenuStyle);
    window.addEventListener("scroll", updateMenuStyle, true);
    return () => {
      window.removeEventListener("resize", updateMenuStyle);
      window.removeEventListener("scroll", updateMenuStyle, true);
    };
  }, [moreMenuOpen, role]);

  useEffect(() => {
    const activeItem = railNavRef.current?.querySelector<HTMLElement>("[data-active-rail-item='true']");
    activeItem?.scrollIntoView({ block: "nearest" });
  }, [pathname, role]);

  const handleLogout = async () => {
    beginIntentionalLogout();
    await clearStoredAuthKey();
    navigate("/login", { replace: true });
    try {
      await logout();
    } catch {
      // 本地登出已完成，服务端会话清理失败不阻塞用户退出。
    }
    finishIntentionalLogout();
  };

  const navItems = navItemsForRole(role);
  const displayUsername = username || (role === "admin" ? "管理员" : "用户");
  const infoDialogTitle = infoDialog === "terms" ? "平台协议" : "更新日志";
  const closeMoreMenu = () => {
    setMoreMenuOpen(false);
    setMoreMenuStyle(null);
  };

  return (
    <>
      <aside className="hidden min-h-0 border-r border-[var(--app-border)] bg-[var(--app-bg-rail)] px-0 pb-5 pt-4 shadow-[inset_-1px_0_0_var(--app-border)] backdrop-blur-2xl lg:flex lg:w-[82px] lg:flex-col lg:items-center">
        <Link
          to={role === "admin" ? "/admin/dashboard" : "/image/history"}
          className="grid justify-items-center gap-1.5 text-[12px] font-extrabold leading-none text-[var(--app-accent-cyan)] drop-shadow-[0_0_18px_rgba(91,214,255,0.26)]"
          aria-label={`${site.name} 首页`}
        >
          <BrandMark logoUrl={site.logoUrl} siteName={site.name} className="size-9" />
          <span>{site.name || "IMAGE STUDIO"}</span>
        </Link>

        <div className="relative mt-5 min-h-0 w-full flex-1">
          <nav ref={railNavRef} className="app-rail-scrollbar grid h-full min-h-0 content-start justify-items-center gap-2 overflow-y-auto px-2 py-3 text-[11px] font-semibold text-[var(--app-text-muted)]" aria-label="主导航">
            {navItems.map((item) => {
              const Icon = item.icon;
              const active = isActive(pathname, item);
              return (
                <Link
                  key={`${item.href}-${item.label}`}
                  to={item.href}
                  aria-label={item.label}
                  data-active-rail-item={active ? "true" : undefined}
                  className={cn(
                    "grid h-[58px] w-[58px] scroll-mb-3 scroll-mt-3 place-items-center rounded-[16px] px-1 transition hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                    active && "border border-[var(--app-border-strong)] bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)] shadow-[var(--app-shadow-floating)]",
                  )}
                >
                  <Icon className="size-5" />
                  <span>{item.label}</span>
                </Link>
              );
            })}
          </nav>
          <div className="pointer-events-none absolute inset-x-0 top-0 h-3 bg-gradient-to-b from-[var(--app-rail-fade-top)] to-transparent" aria-hidden="true" />
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-4 bg-gradient-to-t from-[var(--app-rail-fade-bottom)] to-transparent" aria-hidden="true" />
        </div>

        <div className="mt-5 grid w-full shrink-0 justify-items-center gap-2">
          <div className="grid justify-items-center gap-2">
            <Link
              to="/profile"
              aria-label={`账号信息：${displayUsername}`}
              className={cn(
                "grid size-11 place-items-center rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] shadow-[var(--app-shadow-floating)] transition hover:bg-[var(--app-bg-surface-hover)]",
                isActive(pathname, { href: "/profile", matchPrefix: "/profile", label: "账号", icon: UsersRound }) && "border-[var(--app-border-strong)] bg-[var(--app-bg-surface-hover)]",
              )}
            >
              <span className="grid size-10 place-items-center overflow-hidden rounded-full border border-transparent bg-[linear-gradient(rgba(7,10,18,0.9),rgba(7,10,18,0.9))_padding-box,conic-gradient(#e8ed48,#28d9ef,#6257f8,#57f08b,#e8ed48)_border-box] px-0.5 text-[10px] font-bold leading-none text-white shadow-[0_0_24px_rgba(40,214,255,0.14)]">
                {avatarText(username, role)}
              </span>
            </Link>
            <Link
              to="/credits"
              className="inline-flex h-7 items-center gap-1 rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] px-2.5 text-[12px] font-bold text-[var(--app-text-primary)]"
              aria-label="剩余积分"
            >
              <span className="size-3.5 rounded-full bg-[conic-gradient(#58d7ff,#ffd65c,#ff703e,#58d7ff)]" />
              {balance === null ? "888" : balance.toLocaleString()}
            </Link>
          </div>
          <div className="grid justify-items-center gap-0.5">
            <div className="group relative">
              <button
                type="button"
                className={cn(railUtilityActionClass, "relative")}
                aria-label="消息通知"
                onClick={() => {
                  setHasUnreadNotifications(false);
                  setNotificationDialogOpen(true);
                }}
                data-notification-action
              >
                <Bell className={railActionIconClass} />
                {hasUnreadNotifications ? (
                  <span className="absolute right-[17px] top-[8px] size-1.5 rounded-full bg-[var(--app-accent-cyan)] shadow-[0_0_12px_rgba(91,214,255,0.9)]" />
                ) : null}
              </button>
              <div className="pointer-events-none absolute bottom-2 left-[66px] whitespace-nowrap rounded-[12px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-3 py-1.5 text-xs font-semibold text-[var(--app-text-secondary)] opacity-0 shadow-[var(--app-shadow-floating)] backdrop-blur-xl transition-opacity delay-0 duration-150 group-hover:delay-1000 group-hover:opacity-100">
                消息通知
              </div>
            </div>
            <div ref={moreMenuRef} className="group relative">
              <button
                ref={moreMenuButtonRef}
                type="button"
                className={railUtilityActionClass}
                aria-label="更多设置"
                aria-expanded={moreMenuOpen}
                onClick={() => {
                  setMoreMenuOpen((current) => {
                    if (current) {
                      setMoreMenuStyle(null);
                      return false;
                    }
                    return true;
                  });
                }}
                data-rail-action
              >
                <Menu className={railActionIconClass} />
              </button>
              {!moreMenuOpen ? (
                <div
                  data-more-menu-tooltip
                  className="pointer-events-none absolute bottom-2 left-[66px] whitespace-nowrap rounded-[12px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] px-3 py-1.5 text-xs font-semibold text-[var(--app-text-secondary)] opacity-0 shadow-[var(--app-shadow-floating)] backdrop-blur-xl transition-opacity delay-0 duration-150 group-hover:delay-1000 group-hover:opacity-100"
                >
                  更多设置
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </aside>

      {mounted && moreMenuOpen && moreMenuStyle ? createPortal(
        <div
          ref={moreMenuPanelRef}
          style={moreMenuStyle}
          data-more-menu-panel
          className={cn(menuPanelClass, "transition")}
        >
          <button type="button" className={menuItemClass} onClick={() => { setInfoDialog("terms"); closeMoreMenu(); }}>
            <FileText className="size-4" />
            平台协议
          </button>
          <button type="button" className={menuItemClass} onClick={() => { setInfoDialog("changelog"); closeMoreMenu(); }}>
            <ScrollText className="size-4" />
            更新日志
          </button>
          <div className="group/theme relative before:absolute before:-right-3 before:bottom-0 before:h-full before:w-4 before:content-['']">
            <button type="button" className={cn(menuItemClass, "justify-between")} data-theme-menu-trigger onClick={() => setThemeMode("dark")}>
              <span className="inline-flex items-center gap-2">
                <Moon className="size-4" />
                深色模式
              </span>
              <span className="text-[var(--app-text-muted)]">{">"}</span>
            </button>
            <div
              data-theme-submenu
              className="pointer-events-none absolute bottom-0 left-[calc(100%+8px)] w-[154px] translate-x-1 rounded-[16px] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-1.5 opacity-0 shadow-[var(--app-shadow-floating)] backdrop-blur-2xl transition group-hover/theme:pointer-events-auto group-hover/theme:translate-x-0 group-hover/theme:opacity-100 group-focus-within/theme:pointer-events-auto group-focus-within/theme:translate-x-0 group-focus-within/theme:opacity-100"
            >
              {themeOptions.map((option) => {
                const Icon = option.icon;
                const active = themeMode === option.mode;
                return (
                  <button
                    key={option.mode}
                    type="button"
                    className={cn(menuItemClass, "h-9 justify-between px-2.5")}
                    onClick={() => setThemeMode(option.mode)}
                  >
                    <span className="inline-flex items-center gap-2">
                      <Icon className="size-4" />
                      {option.label}
                    </span>
                    {active ? <Check className="size-4 text-[var(--app-accent-cyan)]" /> : null}
                  </button>
                );
              })}
            </div>
          </div>
          {role === "admin" ? (
            <>
              <Link to="/settings" className={menuItemClass} onClick={closeMoreMenu}>
                <Settings className="size-4" />
                系统设置
              </Link>
              <button type="button" className={menuItemClass} onClick={() => { setVersionDialogOpen(true); closeMoreMenu(); }}>
                <RefreshCw className="size-4" />
                版本更新
              </button>
            </>
          ) : null}
          <button type="button" className={cn(menuItemClass, "text-rose-500 hover:text-rose-600")} onClick={() => void handleLogout()}>
            <LogOut className="size-4" />
            退出
          </button>
        </div>,
        document.body,
      ) : null}

      {role === "admin" ? (
        <VersionUpdateDialog
          open={versionDialogOpen}
          onOpenChange={setVersionDialogOpen}
          versionLabel={versionLabel}
          onVersionChange={(version) => setVersionLabel(formatVersionLabel(version))}
        />
      ) : null}
      <Dialog open={notificationDialogOpen} onOpenChange={setNotificationDialogOpen}>
        <DialogContent className="w-[min(92vw,520px)]">
          <DialogHeader>
            <DialogTitle>消息通知</DialogTitle>
            <DialogDescription>
              后续会接入系统通知、任务结果、额度变动和活动消息。
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-4">
              <div className="flex items-start gap-3">
                <span className="mt-1 size-2 rounded-full bg-[var(--app-accent-cyan)] shadow-[0_0_14px_rgba(91,214,255,0.68)]" />
                <div>
                  <p className="text-sm font-semibold text-[var(--app-text-primary)]">通知中心准备中</p>
                  <p className="mt-1 text-sm leading-6 text-[var(--app-text-secondary)]">
                    这里会用于展示生成任务、系统维护、版本更新和积分相关提醒。
                  </p>
                </div>
              </div>
            </div>
            <div className="rounded-[var(--app-radius-md)] border border-dashed border-[var(--app-border)] bg-[var(--app-bg-surface)] px-4 py-6 text-center text-sm text-[var(--app-text-muted)]">
              暂无新的通知
            </div>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={infoDialog !== null} onOpenChange={(open) => !open && setInfoDialog(null)}>
        <DialogContent className="w-[min(92vw,520px)]">
          <DialogHeader>
            <DialogTitle>{infoDialogTitle}</DialogTitle>
            <DialogDescription>
              {infoDialog === "terms" ? "平台协议会在正式发布前接入完整内容。" : "更新日志会按版本整理最近功能与修复。"}
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-4 text-sm leading-6 text-[var(--app-text-secondary)]">
            {infoDialog === "terms" ? (
              <p>使用平台时请遵守账号安全、内容合规和资源使用规则。这里后续可以接入正式服务条款、隐私说明和使用限制。</p>
            ) : (
              <p>最近更新包含首页/登录页迁移、统一后台壳、生图工作区修复、时间筛选浮层修复、侧栏菜单整合和主题模式补充。</p>
            )}
          </div>
        </DialogContent>
      </Dialog>

      <header className="border-b border-[var(--app-border)] bg-[var(--app-bg-rail)] px-3 py-3 shadow-[var(--app-shadow-floating)] backdrop-blur-2xl lg:hidden">
        <div className="flex items-center gap-3">
          <Link
            to={role === "admin" ? "/admin/dashboard" : "/image/history"}
            className="shrink-0"
            aria-label={`${site.name} 首页`}
          >
            <BrandMark logoUrl={site.logoUrl} siteName={site.name} className="size-10" />
          </Link>
          <nav className="hide-scrollbar flex flex-1 gap-2 overflow-x-auto" aria-label="移动端导航">
            {navItems.map((item) => {
              const Icon = item.icon;
              const active = isActive(pathname, item);
              return (
                <Link
                  key={`${item.href}-${item.label}`}
                  to={item.href}
                  className={cn(
                    "inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full px-3 text-sm font-semibold text-[var(--app-text-secondary)]",
                    active ? "border border-[var(--app-border-strong)] bg-[var(--app-bg-surface-hover)] text-[var(--app-text-primary)]" : "hover:bg-[var(--app-bg-surface-hover)] hover:text-[var(--app-text-primary)]",
                  )}
                >
                  <Icon className="size-4" />
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <button
            type="button"
            className="grid size-10 shrink-0 place-items-center rounded-full border border-[var(--app-border)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]"
            aria-label="退出登录"
            onClick={() => void handleLogout()}
          >
            <LogOut className="size-5" />
          </button>
        </div>
      </header>
    </>
  );
}
