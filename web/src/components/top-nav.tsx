"use client";

import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Activity, BarChart3, ChevronLeft, Database, Gauge, History, ImageIcon, LogOut, Moon, PanelLeftClose, PanelLeftOpen, RefreshCw, Settings2, Sparkles, Sun, UserRound, UsersRound, type LucideIcon } from "lucide-react";

import { fetchVersionInfo, logout } from "@/lib/api";
import { siteInitials, usePublicSiteSettings } from "@/lib/site-settings";
import { beginIntentionalLogout, clearStoredAuthKey, finishIntentionalLogout, getStoredAuthRole, type AuthRole } from "@/store/auth";
import { cn } from "@/lib/utils";
import { ThemeToggleButton } from "@/components/theme-toggle-button";
import { useTheme } from "@/components/theme-provider";
import { VersionUpdateDialog } from "@/components/version-update-dialog";

type NavItem = {
  href: string;
  matchPrefix: string;
  label: string;
  description: string;
  icon: LucideIcon;
};

type NavSection = {
  key: string;
  label?: string;
  items: readonly NavItem[];
};

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

const adminManagementNavItems: readonly NavItem[] = [
  { href: "/admin/dashboard", matchPrefix: "/admin/dashboard", label: "仪表盘", description: "业务指标与趋势", icon: BarChart3 },
  { href: "/admin/operations", matchPrefix: "/admin/operations", label: "运维监控", description: "队列、并发与错误", icon: Gauge },
  { href: "/admin/usage", matchPrefix: "/admin/usage", label: "使用记录", description: "所有用户生成与扣点", icon: History },
  { href: "/users", matchPrefix: "/users", label: "用户管理", description: "业务用户与密码", icon: UsersRound },
  { href: "/accounts", matchPrefix: "/accounts", label: "上游管理", description: "上游 API 与号池", icon: Activity },
  { href: "/storage", matchPrefix: "/storage", label: "存储检查", description: "图片资产一致性", icon: Database },
  { href: "/settings", matchPrefix: "/settings", label: "系统设置", description: "站点、点数与生成规则", icon: Settings2 },
];

const userWorkspaceNavItems: readonly NavItem[] = [
  { href: "/image/history", matchPrefix: "/image", label: "生图", description: "生成与编辑", icon: ImageIcon },
  { href: "/usage", matchPrefix: "/usage", label: "使用记录", description: "个人生成与扣点", icon: History },
  { href: "/profile", matchPrefix: "/profile", label: "账号信息", description: "账号与密码", icon: UserRound },
];

const userWorkspaceSection: NavSection = {
  key: "workspace",
  label: "个人工作区",
  items: userWorkspaceNavItems,
};

const adminManagementSection: NavSection = {
  key: "admin",
  label: "管理后台",
  items: adminManagementNavItems,
};

function navSectionsForRole(role: AuthRole | null): NavSection[] {
  if (role === "admin") {
    return [adminManagementSection, userWorkspaceSection];
  }
  return [{ ...userWorkspaceSection, label: undefined }];
}

function navItemsForRole(role: AuthRole | null) {
  return navSectionsForRole(role).flatMap((section) => section.items);
}

function BrandLogo({ logoUrl, name }: { logoUrl: string; name: string }) {
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [logoUrl]);

  if (logoUrl && !failed) {
    return (
      <span className="app-logo-image-frame flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-2xl shadow-sm">
        <img
          src={logoUrl}
          alt={name}
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      </span>
    );
  }

  return (
    <span className="flex size-8 shrink-0 items-center justify-center rounded-2xl bg-white text-xs font-semibold text-stone-900 shadow-sm dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text-strong)]">
      {siteInitials(name)}
    </span>
  );
}

function BrandCopy({ name, subtitle }: { name: string; subtitle: string }) {
  return (
    <span className="min-w-0">
      <span className="block truncate text-sm font-semibold tracking-tight text-stone-900 dark:text-[var(--studio-text-strong)]">
        {name}
      </span>
      <span className="block truncate text-xs text-stone-500 dark:text-[var(--studio-text-muted)]">
        {subtitle}
      </span>
    </span>
  );
}

function isNavItemActive(pathname: string, href: string, matchPrefix?: string) {
  if (matchPrefix) {
    return pathname === matchPrefix || pathname.startsWith(`${matchPrefix}/`);
  }
  return pathname === href;
}

function desktopNavButtonClass(collapsed: boolean, active = false) {
  return cn(
    "flex h-12 rounded-2xl transition",
    collapsed ? "mx-auto w-12 items-center justify-center px-0" : "w-full items-center gap-3 px-3",
    active
      ? "bg-white text-stone-950 shadow-sm dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text-strong)]"
      : "text-stone-600 hover:bg-white/65 hover:text-stone-900 dark:text-[var(--studio-text-muted)] dark:hover:bg-[var(--studio-panel-soft)] dark:hover:text-[var(--studio-text-strong)]",
  );
}

function desktopNavIconClass(collapsed: boolean, active = false) {
  return cn(
    "flex items-center justify-center rounded-2xl",
    "size-8",
    active
      ? "bg-stone-950 text-white dark:bg-[var(--studio-accent-strong)] dark:text-[var(--studio-accent-foreground)]"
      : "bg-white/80 text-stone-600 dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text-muted)]",
  );
}

function MobileNavSections({ pathname, role }: { pathname: string; role: AuthRole | null }) {
  const sections = navSectionsForRole(role);

  return (
    <div className="flex min-w-full flex-col gap-2 rounded-[20px] bg-white/55 p-1 dark:bg-[var(--studio-panel-soft)]/40">
      {sections.map((section, sectionIndex) => (
        <div
          key={section.key}
          className={cn(
            "min-w-0",
            sectionIndex > 0 ? "border-t border-stone-200/80 pt-2 dark:border-[var(--studio-border)]" : null,
          )}
        >
          {section.label ? (
            <div className="px-2 pb-1 text-[11px] font-semibold text-stone-400 dark:text-[var(--studio-text-muted)]">
              {section.label}
            </div>
          ) : null}
          <div className="hide-scrollbar flex gap-2 overflow-x-auto">
            {section.items.map((item) => {
              const active = isNavItemActive(pathname, item.href, item.matchPrefix);
              const Icon = item.icon;
              return (
                <Link
                  key={item.href}
                  to={item.href}
                  className={cn(
                    "flex min-w-[104px] shrink-0 items-center justify-center gap-2 rounded-2xl px-3 py-2.5 text-sm font-medium transition",
                    active
                      ? "bg-white text-stone-950 shadow-sm dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text-strong)]"
                      : "text-stone-600 hover:bg-white/75 hover:text-stone-900 dark:text-[var(--studio-text-muted)] dark:hover:bg-[var(--studio-panel-soft)] dark:hover:text-[var(--studio-text-strong)]",
                  )}
                >
                  <Icon className="size-4 shrink-0" />
                  <span className="truncate">{item.label}</span>
                </Link>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

type DesktopTopNavProps = {
  pathname: string;
  role: AuthRole | null;
  siteName: string;
  siteLogoUrl: string;
  versionLabel: string;
  onVersionChange: (version: string) => void;
  onLogout: () => Promise<void>;
};

function DesktopTopNav({ pathname, role, siteName, siteLogoUrl, versionLabel, onVersionChange, onLogout }: DesktopTopNavProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [versionDialogOpen, setVersionDialogOpen] = useState(false);
  const visibleNavSections = navSectionsForRole(role);
  const { themeMode, toggleTheme } = useTheme();
  const ThemeIcon = themeMode === "light" ? Sun : themeMode === "graphite" ? Moon : Sparkles;

  return (
    <aside className={cn("hidden min-h-0 shrink-0 transition-[width] duration-200 lg:flex", collapsed ? "w-[84px]" : "w-[228px]")}>
      <div className="flex h-full min-h-0 w-full flex-col overflow-hidden rounded-[28px] border border-stone-200 bg-[#f0f0ed] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] transition-colors duration-200 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
        <Link
          to={role === "admin" ? "/admin/dashboard" : "/image/history"}
          className={cn(
            "flex h-12 rounded-2xl transition hover:bg-white/70 dark:hover:bg-[var(--studio-panel-soft)]",
            collapsed ? "mx-auto w-12 items-center justify-center px-0" : "w-full items-center gap-3 px-3",
          )}
          title={collapsed ? siteName : undefined}
        >
          <BrandLogo logoUrl={siteLogoUrl} name={siteName} />
          {!collapsed ? (
            <BrandCopy name={siteName} subtitle={role === "admin" ? "管理后台 / 个人工作区" : "图片生成工作区"} />
          ) : null}
        </Link>

        <nav className="hide-scrollbar mt-4 min-h-0 flex-1 overflow-y-auto">
          <div className="space-y-3">
            {visibleNavSections.map((section, sectionIndex) => (
              <div
                key={section.key}
                className="space-y-1"
              >
                {section.label ? (
                  <div
                    className={cn(
                      "flex h-6 items-center px-3 text-[11px] font-semibold text-stone-400 dark:text-[var(--studio-text-muted)]",
                      collapsed ? "justify-center" : null,
                    )}
                    aria-hidden={collapsed}
                  >
                    {collapsed ? (
                      <span className="h-px w-10 bg-stone-200 dark:bg-[var(--studio-border)]" />
                    ) : section.label}
                  </div>
                ) : null}
                {section.items.map((item) => {
                  const active = isNavItemActive(pathname, item.href, item.matchPrefix);
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.href}
                      to={item.href}
                      className={desktopNavButtonClass(collapsed, active)}
                      title={collapsed ? item.label : undefined}
                    >
                      <span className={desktopNavIconClass(collapsed, active)}>
                        <Icon className="size-4" />
                      </span>
                      {!collapsed ? (
                        <span className="min-w-0">
                          <span className="block truncate text-sm font-medium">{item.label}</span>
                        </span>
                      ) : null}
                    </Link>
                  );
                })}
              </div>
            ))}
            <div className="space-y-1 border-t border-stone-200/80 pt-3 dark:border-[var(--studio-border)]">
              {role === "admin" ? (
                <button
                  type="button"
                  className={desktopNavButtonClass(collapsed)}
                  title={collapsed ? `版本 ${versionLabel}` : "检查更新"}
                  onClick={() => setVersionDialogOpen(true)}
                >
                  <span className={desktopNavIconClass(collapsed)}>
                    <RefreshCw className="size-4" />
                  </span>
                  {!collapsed ? (
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium">版本</span>
                      <span className="block truncate text-xs text-stone-500 dark:text-[var(--studio-text-muted)]">{versionLabel}</span>
                    </span>
                  ) : null}
                </button>
              ) : null}
              <button
                type="button"
                className={desktopNavButtonClass(collapsed)}
                onClick={() => void onLogout()}
                title={collapsed ? "退出登录" : undefined}
              >
                <span className={desktopNavIconClass(collapsed)}>
                  <LogOut className="size-4" />
                </span>
                  {!collapsed ? (
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium">退出登录</span>
                    </span>
                  ) : null}
              </button>
            </div>
          </div>
        </nav>

        <div className="shrink-0 space-y-2 border-t border-stone-200/80 pt-3 dark:border-[var(--studio-border)]">
          <div className="space-y-2">
            <button
              type="button"
              className={desktopNavButtonClass(collapsed)}
              onClick={toggleTheme}
              aria-label="主题切换"
              title={collapsed ? "主题切换" : undefined}
            >
              <span className={desktopNavIconClass(collapsed)}>
                <ThemeIcon className="size-4" />
              </span>
              {!collapsed ? (
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">主题切换</span>
                </span>
              ) : null}
            </button>
            <button
              type="button"
              className={desktopNavButtonClass(collapsed)}
              onClick={() => setCollapsed((current) => !current)}
              aria-label={collapsed ? "展开侧边栏" : "收起侧边栏"}
              title={collapsed ? "展开侧边栏" : undefined}
            >
              <span className={desktopNavIconClass(collapsed)}>
                {collapsed ? <PanelLeftOpen className="size-4" /> : <PanelLeftClose className="size-4" />}
              </span>
              {!collapsed ? (
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">收起侧边栏</span>
                </span>
              ) : null}
            </button>
          </div>
        </div>
      </div>
      {role === "admin" ? (
        <VersionUpdateDialog
          open={versionDialogOpen}
          onOpenChange={setVersionDialogOpen}
          versionLabel={versionLabel}
          onVersionChange={onVersionChange}
        />
      ) : null}
    </aside>
  );
}

export function TopNav() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const isImageRoute = pathname === "/image" || pathname?.startsWith("/image/");
  const isMobileWorkspaceRoute = pathname === "/image/workspace";
  const [versionLabel, setVersionLabel] = useState("读取中");
  const [authRole, setAuthRole] = useState<AuthRole | null>(null);
  const [mobileNavExpanded, setMobileNavExpanded] = useState(false);
  const [mobileHeaderHeight, setMobileHeaderHeight] = useState(0);
  const [mobileWorkspaceHeaderHeight, setMobileWorkspaceHeaderHeight] = useState(0);
  const [mobileWorkspaceTitle, setMobileWorkspaceTitle] = useState<string | null>(null);
  const mobileHeaderRef = useRef<HTMLElement | null>(null);
  const mobileWorkspaceHeaderRef = useRef<HTMLDivElement | null>(null);
  const setMobileHeaderRef = (node: HTMLElement | null) => {
    mobileHeaderRef.current = node;
  };

  useEffect(() => {
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
  }, []);

  useEffect(() => {
    let cancelled = false;
    const loadRole = async () => {
      const role = await getStoredAuthRole();
      if (!cancelled) {
        setAuthRole(role);
      }
    };
    void loadRole();
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  useEffect(() => {
    setMobileNavExpanded(false);
  }, [pathname]);

  useEffect(() => {
    const element = mobileHeaderRef.current;
    if (!element) {
      return;
    }

    const updateHeight = () => {
      setMobileHeaderHeight(element.offsetHeight);
    };

    updateHeight();
    const observer = new ResizeObserver(() => updateHeight());
    observer.observe(element);
    window.addEventListener("resize", updateHeight);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateHeight);
    };
  }, [mobileNavExpanded]);

  useEffect(() => {
    if (!isMobileWorkspaceRoute) {
      setMobileWorkspaceHeaderHeight(0);
      return;
    }

    const element = mobileWorkspaceHeaderRef.current;
    if (!element) {
      return;
    }

    const updateHeight = () => {
      setMobileWorkspaceHeaderHeight(element.offsetHeight);
    };

    updateHeight();
    const observer = new ResizeObserver(() => updateHeight());
    observer.observe(element);
    window.addEventListener("resize", updateHeight);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateHeight);
    };
  }, [isMobileWorkspaceRoute, mobileWorkspaceTitle]);

  useEffect(() => {
    if (!isImageRoute) {
      setMobileWorkspaceTitle(null);
      return;
    }

    const handleWorkspaceTitle = (event: Event) => {
      const detail = (event as CustomEvent<{ title?: string | null }>).detail;
      setMobileWorkspaceTitle(detail?.title ? String(detail.title) : null);
    };

    window.addEventListener("image-studio:mobile-workspace-title", handleWorkspaceTitle as EventListener);
    return () => {
      window.removeEventListener("image-studio:mobile-workspace-title", handleWorkspaceTitle as EventListener);
    };
  }, [isImageRoute]);

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

  if (pathname === "/login" || pathname === "/login.html" || pathname.startsWith("/login/")) {
    return null;
  }

  return (
    <>
      <div
        className="lg:hidden"
        style={{
          height: isMobileWorkspaceRoute
            ? mobileWorkspaceHeaderHeight + (mobileNavExpanded ? mobileHeaderHeight : 0)
            : mobileHeaderHeight,
        }}
      />
      {!isMobileWorkspaceRoute ? (
        <header ref={setMobileHeaderRef} className="fixed inset-x-0 top-0 z-40 px-3 lg:hidden">
        <div className="rounded-[26px] border border-stone-200 bg-[#f0f0ed] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] transition-colors duration-200 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
          <div className="flex items-center justify-between gap-3">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <ThemeToggleButton className="size-10 shrink-0" />
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center rounded-2xl px-1 py-1 text-left transition hover:bg-white/70 dark:hover:bg-[var(--studio-panel-soft)]"
                onClick={() => setMobileNavExpanded((current) => !current)}
                aria-label={mobileNavExpanded ? "收起导航" : "展开导航"}
              >
                <BrandCopy name={site.name} subtitle={mobileNavExpanded ? "点击收起导航" : "点击展开导航"} />
              </button>
            </div>
            <div className="flex items-center gap-2">
              <Link
                to="/image/history"
                className="hidden rounded-2xl border border-stone-200 bg-white px-3 py-2 text-xs font-medium text-stone-600 shadow-sm dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)] sm:inline-flex"
              >
                {navItemsForRole(authRole).find((item) => isNavItemActive(pathname, item.href, item.matchPrefix))?.label ?? "导航"}
              </Link>
              <button
                type="button"
                className="inline-flex h-10 shrink-0 items-center justify-center rounded-2xl border border-stone-200 bg-white px-3 text-sm font-medium text-stone-700 transition hover:bg-stone-50 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)] dark:hover:bg-[var(--studio-panel-muted)]"
                onClick={() => void handleLogout()}
              >
                <LogOut className="size-4" />
              </button>
            </div>
          </div>

          {mobileNavExpanded ? (
            <nav className="hide-scrollbar mt-3 -mx-1 overflow-x-auto px-1">
              <MobileNavSections pathname={pathname} role={authRole} />
            </nav>
          ) : null}
        </div>
        </header>
      ) : null}
      {isMobileWorkspaceRoute ? (
        <div
          ref={mobileWorkspaceHeaderRef}
          className="fixed inset-x-0 top-0 z-40 px-3 lg:hidden"
          style={{ top: mobileNavExpanded ? mobileHeaderHeight : 0 }}
        >
          <div className="min-w-0 rounded-[26px] border border-stone-200 bg-[#f0f0ed] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] transition-colors duration-200 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => navigate("/image/history")}
                className="inline-flex h-10 items-center gap-2 rounded-full border border-stone-200 bg-white px-4 text-stone-700 shadow-none dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)]"
              >
                <ChevronLeft className="size-4" />
                会话历史
              </button>
              <h1 className="text-xl font-semibold tracking-tight text-stone-950 dark:text-[var(--studio-text-strong)]">生图</h1>
              <ThemeToggleButton className="ml-auto size-9 shrink-0 rounded-full" />
              <button
                type="button"
                onClick={() => setMobileNavExpanded((current) => !current)}
                className="inline-flex size-9 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-600 shadow-none dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)]"
                aria-label={mobileNavExpanded ? "收起导航" : "显示导航"}
                title={mobileNavExpanded ? "收起导航" : "显示导航"}
              >
                {mobileNavExpanded ? <PanelLeftClose className="size-4" /> : <PanelLeftOpen className="size-4" />}
              </button>
            </div>
            {mobileWorkspaceTitle ? (
              <div className="mt-3">
                <span className="inline-flex max-w-full truncate rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-stone-600 dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)]">
                  {mobileWorkspaceTitle}
                </span>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
      {isMobileWorkspaceRoute && mobileNavExpanded ? (
        <div
          ref={setMobileHeaderRef}
          className="fixed inset-x-0 top-0 z-40 px-3 lg:hidden"
        >
          <div className="rounded-[24px] border border-stone-200 bg-[#f0f0ed] p-3 shadow-[0_14px_40px_rgba(15,23,42,0.05)] transition-colors duration-200 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel)] dark:shadow-[0_14px_40px_rgba(0,0,0,0.5)]">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 flex-1 items-center gap-3">
                <ThemeToggleButton className="size-10 shrink-0" />
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center rounded-2xl px-1 py-1 text-left transition hover:bg-white/70 dark:hover:bg-[var(--studio-panel-soft)]"
                  onClick={() => setMobileNavExpanded(false)}
                  aria-label="收起导航"
                >
                  <BrandCopy name={site.name} subtitle="点击收起导航" />
                </button>
              </div>
              <button
                type="button"
                className="inline-flex h-10 shrink-0 items-center justify-center rounded-2xl border border-stone-200 bg-white px-3 text-sm font-medium text-stone-700 transition hover:bg-stone-50 dark:border-[var(--studio-border)] dark:bg-[var(--studio-panel-soft)] dark:text-[var(--studio-text)] dark:hover:bg-[var(--studio-panel-muted)]"
                onClick={() => void handleLogout()}
              >
                <LogOut className="size-4" />
              </button>
            </div>
            <nav className="hide-scrollbar mt-3 -mx-1 overflow-x-auto px-1">
              <MobileNavSections pathname={pathname} role={authRole} />
            </nav>
          </div>
        </div>
      ) : null}
      <DesktopTopNav
        pathname={pathname}
        role={authRole}
        siteName={site.name}
        siteLogoUrl={site.logoUrl}
        versionLabel={versionLabel}
        onVersionChange={(version) => setVersionLabel(formatVersionLabel(version))}
        onLogout={handleLogout}
      />
    </>
  );
}
