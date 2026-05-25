import "./globals.css";
import { AppShellNav } from "@/components/app-shell-nav";
import { ThemeProvider } from "@/components/theme-provider";
import type { AuthRole } from "@/store/auth";
import { useLocation } from "react-router-dom";

export default function AppShell({
  children,
  role,
}: Readonly<{
  children: React.ReactNode;
  role?: AuthRole | null;
}>) {
  const { pathname } = useLocation();
  const isLoginRoute = pathname === "/login" || pathname === "/login.html" || pathname.startsWith("/login/");
  const isPublicLandingRoute = pathname === "/";
  const isStandaloneRoute = isLoginRoute || isPublicLandingRoute;

  return (
    <ThemeProvider>
      {isStandaloneRoute ? (
        <main
          className="box-border h-screen min-h-screen overflow-y-auto bg-[var(--app-bg-root)] text-[var(--app-text-primary)]"
          style={{
            fontFamily:
              '"SF Pro Display","SF Pro Text","PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif',
          }}
        >
          {children}
        </main>
      ) : (
        <main
          className="app-app-shell box-border min-h-screen overflow-hidden bg-[var(--app-bg-root)] text-[var(--app-text-primary)]"
          style={{
            fontFamily:
              '"SF Pro Display","SF Pro Text","PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif',
          }}
        >
          <div className="relative z-10 grid h-screen min-h-screen grid-cols-1 grid-rows-[auto_minmax(0,1fr)] overflow-hidden md:grid-cols-[82px_minmax(0,1fr)] md:grid-rows-1">
            <AppShellNav role={role} />
            <div className="app-content min-h-0 min-w-0 overflow-y-auto bg-transparent">
              {children}
            </div>
          </div>
        </main>
      )}
    </ThemeProvider>
  );
}
