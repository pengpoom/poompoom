"use client";

import { useEffect, useState, type CSSProperties } from "react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import { AuthBrandMark, AuthCard, type AuthMode } from "@/components/auth-card";
import { usePublicSiteSettings } from "@/lib/site-settings";

const galleryShots = [
  { label: "电商主图", cls: "shot-a" },
  { label: "科研封面", cls: "shot-b" },
  { label: "品牌海报", cls: "shot-c" },
  { label: "PPT 视觉", cls: "shot-d" },
] as const;

export default function LoginPage() {
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const site = usePublicSiteSettings();
  const initialMode = searchParams.get("mode") === "register" ? "register" : "login";
  const [mode, setMode] = useState<AuthMode>(initialMode);
  const from = typeof location.state?.from === "string" ? location.state.from : "";

  useEffect(() => {
    setMode(initialMode);
  }, [initialMode]);

  return (
    <div className="auth-shell">
      <div className="auth-bg-beam left" aria-hidden="true" />
      <div className="auth-bg-beam right" aria-hidden="true" />
      <div className="auth-bg-grid" aria-hidden="true" />

      <header className="auth-top auth-lr" style={{ "--auth-i": 0 } as CSSProperties}>
        <Link to="/" className="auth-back">
          <ArrowLeft className="size-4" />
          返回首页
        </Link>
        <div className="auth-brand">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} />
          <span>{site.name || "Poom Studio"}</span>
        </div>
      </header>

      <main className="auth-grid">
        <aside className="auth-aside auth-lr" style={{ "--auth-i": 1 } as CSSProperties}>
          <div className="auth-badge">
            <i />
            已服务 1000+ 用户
          </div>
          <h1>引爆你的创意</h1>
          <p className="auth-lead">从图片生成到电商、科研图、PPT——一个 AI 工作台全搞定。</p>
          <div className="auth-gallery">
            {galleryShots.map((shot) => (
              <div key={shot.label} className={`shot ${shot.cls}`}>
                <span>{shot.label}</span>
              </div>
            ))}
          </div>
        </aside>

        <div className="auth-lr" style={{ "--auth-i": 2 } as CSSProperties}>
          <AuthCard mode={mode} onModeChange={setMode} from={from} syncUrl />
        </div>
      </main>
    </div>
  );
}
