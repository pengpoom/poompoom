"use client";

import { useEffect, useState } from "react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import { AuthBrandMark, AuthCard, type AuthMode } from "@/components/auth-card";
import { usePublicSiteSettings } from "@/lib/site-settings";

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
    <div className="relative min-h-screen overflow-hidden bg-black px-4 py-6 text-white sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute -left-[16vw] -top-[22%] h-[72vh] w-[62vw] rotate-[18deg] bg-[radial-gradient(ellipse_at_0%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute -right-[16vw] top-[-18%] h-[72vh] w-[62vw] -rotate-[18deg] bg-[radial-gradient(ellipse_at_100%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:48px_48px] opacity-20 [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,transparent_72%)]" />

      <header className="relative z-10 mx-auto flex max-w-[1200px] items-center justify-between">
        <Link to="/" className="inline-flex items-center gap-3 text-sm font-semibold text-[#c8d1df] transition hover:text-white">
          <ArrowLeft className="size-4" />
          返回首页
        </Link>
        <div className="inline-flex items-center gap-3 text-base font-bold">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} />
          <span>{site.name || "Image Studio"}</span>
        </div>
      </header>

      <main className="relative z-10 mx-auto grid min-h-[calc(100vh-72px)] w-full max-w-[1120px] items-center gap-8 py-10 lg:grid-cols-[minmax(0,0.95fr)_minmax(420px,0.75fr)]">
        <section className="hidden min-h-[560px] overflow-hidden rounded-lg border border-white/15 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.025)),rgba(5,7,12,0.72)] p-8 shadow-[0_34px_110px_rgba(34,131,255,0.18),0_28px_90px_rgba(0,0,0,0.62),inset_0_1px_0_rgba(255,255,255,0.12)] backdrop-blur-2xl lg:block">
          <div className="inline-flex min-h-9 items-center gap-2.5 rounded-full border border-white/15 bg-white/[0.055] px-4 text-[13px] font-medium text-[#c8d1df]">
            <span className="size-2 rounded-full bg-[#5bd6ff] shadow-[0_0_18px_rgba(91,214,255,0.9)]" />
            已服务 1000+ 用户
          </div>
          <h1 className="mt-8 max-w-[520px] bg-[linear-gradient(180deg,#fff,#eef4ff_40%,rgba(163,174,192,0.68))] bg-clip-text text-6xl font-extrabold leading-tight tracking-normal text-transparent">
            引爆你的创意
          </h1>
          <p className="mt-5 flex max-w-[520px] flex-wrap gap-y-2 text-base leading-7 text-[#9aa3b2]">
            {["图片生成", "平面设计", "科研绘图", "PPT制作", "一站式解决平台"].map((item, index) => (
              <span
                key={item}
                className={index > 0 ? "inline-flex items-center before:mx-[18px] before:h-3.5 before:w-px before:bg-white/20 before:content-['']" : "inline-flex items-center"}
              >
                {item}
              </span>
            ))}
          </p>
          <div className="mt-10 grid gap-3">
            {["生成结果自动进入资产库", "管理员可统一查看用量与队列", "保留提示词、模型、比例和清晰度上下文"].map((item) => (
              <div key={item} className="flex items-center gap-3 rounded-lg border border-white/15 bg-white/[0.055] px-4 py-3 text-sm text-[#d4dbea]">
                <span className="size-2 rounded-full bg-[#5bd6ff]" />
                {item}
              </div>
            ))}
          </div>
          <div className="mt-10 rounded-lg border border-white/15 bg-[radial-gradient(circle_at_18%_10%,rgba(53,150,255,0.25),transparent_14rem),radial-gradient(circle_at_90%_18%,rgba(255,108,172,0.20),transparent_12rem),rgba(7,9,14,0.86)] p-5">
            <div className="h-52 rounded-lg border border-white/15 bg-[linear-gradient(110deg,transparent,rgba(111,207,255,0.85)_20%,rgba(255,138,179,0.88)_34%,transparent_52%),linear-gradient(18deg,transparent_18%,rgba(44,114,255,0.92)_42%,rgba(55,218,255,0.72)_54%,transparent_76%),#02030a]" />
          </div>
        </section>

        <AuthCard
          mode={mode}
          onModeChange={setMode}
          from={from}
          syncUrl
        />
      </main>
    </div>
  );
}
