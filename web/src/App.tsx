import { Component, lazy, type ReactNode, Suspense, useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";

import AppShell from "@/app/layout";
import { resetUnauthorizedRedirectState } from "@/lib/request";
import { usePublicSiteSettings } from "@/lib/site-settings";
import { AUTH_STATE_CHANGED_EVENT, clearStoredAuthKey, getStoredAuthRole, type AuthRole } from "@/store/auth";

const AccountsPage = lazy(() => import("@/app/accounts/page"));
const AdminUsagePage = lazy(() => import("@/app/admin-usage/page"));
const AssetsPage = lazy(() => import("@/app/assets/page"));
const CommunityPage = lazy(() => import("@/app/community/page"));
const CreditsPage = lazy(() => import("@/app/credits/page"));
const DashboardPage = lazy(() => import("@/app/dashboard/page"));
const ImagePage = lazy(() => import("@/app/image/page"));
const LoginPage = lazy(() => import("@/app/login/page"));
const OperationsPage = lazy(() => import("@/app/operations/page"));
const HomePage = lazy(() => import("@/app/page"));
const ProfilePage = lazy(() => import("@/app/profile/page"));
const SettingsPage = lazy(() => import("@/app/settings/page"));
const StartupCheckPage = lazy(() => import("@/app/startup-check/page"));
const StoragePage = lazy(() => import("@/app/storage/page"));
const ToolsPage = lazy(() => import("@/app/tools/page"));
const MyUsagePage = lazy(() => import("@/app/usage/page"));
const UserDetailPage = lazy(() => import("@/app/users/detail/page"));
const UsersPage = lazy(() => import("@/app/users/page"));

type RouteErrorBoundaryState = {
  error: Error | null;
};

class RouteErrorBoundary extends Component<
  { children: ReactNode; resetKey: string },
  RouteErrorBoundaryState
> {
  state: RouteErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { error };
  }

  componentDidUpdate(previousProps: { resetKey: string }) {
    if (previousProps.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  render() {
    if (!this.state.error) {
      return this.props.children;
    }

    return <RouteLoadError error={this.state.error} />;
  }
}

function ProtectedRoute({
  role,
  adminOnly = false,
  children,
}: {
  role: AuthRole | null | undefined;
  adminOnly?: boolean;
  children: ReactNode;
}) {
  if (role === undefined) {
    return null;
  }
  if (role === null) {
    return <Navigate to="/login" replace />;
  }
  if (adminOnly && role !== "admin") {
    return <Navigate to="/image/history" replace />;
  }
  return children;
}

function PublicOnlyRoute({ role, children }: { role: AuthRole | null | undefined; children: ReactNode }) {
  if (role === undefined) {
    return null;
  }
  if (role === "admin") {
    return <Navigate to="/admin/dashboard" replace />;
  }
  if (role === "user") {
    return <Navigate to="/image/history" replace />;
  }
  return children;
}

function RouteFallback() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center text-sm text-muted-foreground">
      加载中...
    </div>
  );
}

function RouteLoadError({ error }: { error: Error }) {
  const message = String(error.message || error);
  const isChunkError = /chunk|import|module|preload/i.test(message);

  return (
    <main className="mx-auto flex min-h-[55vh] max-w-md flex-col items-center justify-center px-6 text-center">
      <h1 className="text-xl font-semibold text-foreground">
        {isChunkError ? "页面资源已更新" : "页面加载失败"}
      </h1>
      <p className="mt-3 text-sm leading-6 text-muted-foreground">
        {isChunkError ? "请刷新页面后继续访问。" : "请刷新页面，或返回首页后重新进入。"}
      </p>
      <div className="mt-6 flex flex-wrap justify-center gap-3">
        <button
          type="button"
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          onClick={() => window.location.reload()}
        >
          刷新页面
        </button>
        <button
          type="button"
          className="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-accent"
          onClick={() => {
            window.location.href = "/";
          }}
        >
          返回首页
        </button>
      </div>
    </main>
  );
}

function NotFoundPage({ role }: { role: AuthRole | null | undefined }) {
  const navigate = useNavigate();
  const homePath = role === "admin" ? "/admin/dashboard" : role === "user" ? "/image/history" : "/login";

  return (
    <main className="mx-auto flex min-h-[55vh] max-w-md flex-col items-center justify-center px-6 text-center">
      <p className="text-sm font-medium text-muted-foreground">404</p>
      <h1 className="mt-2 text-xl font-semibold text-foreground">页面不存在</h1>
      <p className="mt-3 text-sm leading-6 text-muted-foreground">当前地址没有对应页面。</p>
      <button
        type="button"
        className="mt-6 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        onClick={() => navigate(homePath, { replace: true })}
      >
        返回首页
      </button>
    </main>
  );
}

export default function App() {
  const location = useLocation();
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const { pathname } = location;
  const [role, setRole] = useState<AuthRole | null | undefined>(undefined);

  useEffect(() => {
    document.title = site.name;
  }, [site.name]);

  useEffect(() => {
    let cancelled = false;
    const loadRole = async () => {
      const storedRole = await getStoredAuthRole();
      if (!cancelled) {
        setRole(storedRole);
      }
    };
    void loadRole();
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  useEffect(() => {
    const handleAuthStateChanged = () => {
      void getStoredAuthRole().then(setRole);
    };
    window.addEventListener(AUTH_STATE_CHANGED_EVENT, handleAuthStateChanged);
    return () => window.removeEventListener(AUTH_STATE_CHANGED_EVENT, handleAuthStateChanged);
  }, []);

  useEffect(() => {
    const handleUnauthorized = () => {
      void clearStoredAuthKey().finally(() => {
        setRole(null);
        if (pathname !== "/login") {
          toast.error("登录已过期，请重新登录");
          navigate("/login", {
            replace: true,
            state: { from: `${location.pathname}${location.search}${location.hash}` },
          });
        }
      });
    };
    window.addEventListener("image-studio:unauthorized", handleUnauthorized);
    return () => window.removeEventListener("image-studio:unauthorized", handleUnauthorized);
  }, [location.hash, location.pathname, location.search, navigate, pathname]);

  useEffect(() => {
    if (pathname === "/login") {
      resetUnauthorizedRedirectState();
    }
  }, [pathname]);

  return (
    <AppShell role={role}>
      <RouteErrorBoundary resetKey={pathname}>
        <Suspense fallback={<RouteFallback />}>
          <Routes>
            <Route path="/" element={<PublicOnlyRoute role={role}><HomePage /></PublicOnlyRoute>} />
            <Route path="/login" element={<PublicOnlyRoute role={role}><LoginPage /></PublicOnlyRoute>} />
            <Route path="/image" element={<ProtectedRoute role={role}><Navigate to="/image/history" replace /></ProtectedRoute>} />
            <Route path="/image/history" element={<ProtectedRoute role={role}><ImagePage /></ProtectedRoute>} />
            <Route path="/image/workspace" element={<ProtectedRoute role={role}><ImagePage /></ProtectedRoute>} />
            <Route path="/community" element={<ProtectedRoute role={role}><CommunityPage /></ProtectedRoute>} />
            <Route path="/assets" element={<ProtectedRoute role={role}><AssetsPage /></ProtectedRoute>} />
            <Route path="/tools" element={<ProtectedRoute role={role}><ToolsPage /></ProtectedRoute>} />
            <Route path="/usage" element={<ProtectedRoute role={role}><MyUsagePage /></ProtectedRoute>} />
            <Route path="/credits" element={<ProtectedRoute role={role}><CreditsPage /></ProtectedRoute>} />
            <Route path="/profile" element={<ProtectedRoute role={role}><ProfilePage /></ProtectedRoute>} />
            <Route path="/admin/dashboard" element={<ProtectedRoute role={role} adminOnly><DashboardPage /></ProtectedRoute>} />
            <Route path="/admin/usage" element={<ProtectedRoute role={role} adminOnly><AdminUsagePage /></ProtectedRoute>} />
            <Route path="/admin/operations" element={<ProtectedRoute role={role} adminOnly><OperationsPage /></ProtectedRoute>} />
            <Route path="/users" element={<ProtectedRoute role={role} adminOnly><UsersPage /></ProtectedRoute>} />
            <Route path="/users/:id" element={<ProtectedRoute role={role} adminOnly><UserDetailPage /></ProtectedRoute>} />
            <Route path="/storage" element={<ProtectedRoute role={role} adminOnly><StoragePage /></ProtectedRoute>} />
            <Route path="/accounts" element={<ProtectedRoute role={role} adminOnly><AccountsPage /></ProtectedRoute>} />
            <Route path="/settings" element={<ProtectedRoute role={role} adminOnly><SettingsPage /></ProtectedRoute>} />
            <Route path="/startup-check" element={<ProtectedRoute role={role} adminOnly><StartupCheckPage /></ProtectedRoute>} />
            <Route path="*" element={<NotFoundPage role={role} />} />
          </Routes>
        </Suspense>
      </RouteErrorBoundary>
    </AppShell>
  );
}
