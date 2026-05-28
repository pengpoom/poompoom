"use client";

import { useEffect, useState } from "react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import { AuthBrandMark, AuthCard, type AuthMode } from "@/components/auth-card";
import { usePublicSiteSettings } from "@/lib/site-settings";
import { cn } from "@/lib/utils";

const galleryShots = [
  { label: "电商主图", cls: "bg-[radial-gradient(120%_100%_at_20%_0%,rgba(40,214,255,0.5),transparent_60%),linear-gradient(135deg,#0a1024,#0a0717)]" },
  { label: "科研封面", cls: "bg-[radial-gradient(120%_100%_at_80%_0%,rgba(47,107,255,0.55),transparent_60%),linear-gradient(135deg,#0a0a1e,#070512)]" },
  { label: "品牌海报", cls: "bg-[radial-gradient(120%_100%_at_30%_100%,rgba(139,92,255,0.5),transparent_60%),linear-gradient(135deg,#100a1f,#08060f)]" },
  { label: "PPT 视觉", cls: "bg-[radial-gradient(120%_100%_at_70%_100%,rgba(40,214,255,0.4),rgba(139,92,255,0.3)_50%,transparent_70%),linear-gradient(135deg,#0a0f20,#070611)]" },
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
    <div className="relative min-h-screen overflow-hidden bg-black px-6 py-6 text-white">
      <div className="pointer-events-none absolute -left-[16vw] -top-[22%] h-[72vh] w-[62vw] rotate-[18deg] bg-[radial-gradient(ellipse_at_0%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute -right-[16vw] top-[-18%] h-[72vh] w-[62vw] -rotate-[18deg] bg-[radial-gradient(ellipse_at_100%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:48px_48px] opacity-20 [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,transparent_72%)]" />

      <header className="entry-reveal relative z-10 mx-auto flex max-w-[1200px] items-center justify-between" style={{ "--reveal-i": 0 } as React.CSSProperties}>
        <Link to="/" className="inline-flex items-center gap-2 text-sm font-semibold text-[#c8d1df] transition hover:text-white">
          <ArrowLeft className="size-4" />
          返回首页
        </Link>
        <div className="inline-flex items-center gap-3 text-base font-bold">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} />
          <span>{site.name || "Image Studio"}</span>
        </div>
      </header>

      <main className="relative z-10 mx-auto grid min-h-[calc(100vh-96px)] w-full max-w-[1120px] items-center gap-8 py-9 lg:grid-cols-[minmax(0,0.95fr)_minmax(420px,0.75fr)]">
        <aside className="entry-reveal hidden min-h-[560px] overflow-hidden rounded-[18px] border border-white/15 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.025)),rgba(5,7,12,0.72)] p-9 shadow-[0_34px_110px_rgba(34,131,255,0.18),0_28px_90px_rgba(0,0,0,0.62),inset_0_1px_0_rgba(255,255,255,0.12)] backdrop-blur-[28px] lg:block" style={{ "--reveal-i": 1 } as React.CSSProperties}>
          <div className="inline-flex min-h-9 items-center gap-2.5 rounded-full border border-white/15 bg-white/[0.055] px-4 text-[13px] font-medium text-[#c8d1df]">
            <span className="size-2 rounded-full bg-[#5bd6ff] shadow-[0_0_18px_rgba(91,214,255,0.9)]" />
            已服务 1000+ 用户
          </div>

          <h1 className="mt-8 max-w-[520px] bg-[linear-gradient(180deg,#fff,#eef4ff_40%,rgba(163,174,192,0.7))] bg-clip-text text-[clamp(40px,5vw,56px)] font-extrabold leading-tight tracking-tight text-transparent">
            引爆你的创意
          </h1>
          <p className="mt-5 max-w-[440px] text-[17px] leading-[1.7] text-[#c2cad8]">
            从图片生成到电商、科研图、PPT——一个 AI 工作台全搞定。
          </p>

          <div className="mt-10 grid max-w-[460px] grid-cols-2 gap-3.5">
            {galleryShots.map((shot) => (
              <div
                key={shot.label}
                className={cn(
                  "flex aspect-[4/3] items-end overflow-hidden rounded-[14px] border border-white/12 p-3 shadow-[0_16px_40px_rgba(0,0,0,0.4),inset_0_1px_0_rgba(255,255,255,0.1)]",
                  shot.cls,
                )}
              >
                <span className="relative z-[1] text-xs font-semibold text-white drop-shadow-[0_2px_8px_rgba(0,0,0,0.5)]">
                  {shot.label}
                </span>
              </div>
            ))}
          </div>
        </aside>

        <div className="entry-reveal" style={{ "--reveal-i": 2 } as React.CSSProperties}>
          <AuthCard
            mode={mode}
            onModeChange={setMode}
            from={from}
            syncUrl
          />
        </div>
      </main>
    </div>
  );
}
