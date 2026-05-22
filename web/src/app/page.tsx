"use client";

import { useEffect, useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { ArrowRight, Boxes, ImageIcon, LayoutTemplate, Sparkles } from "lucide-react";

import { AuthCard, type AuthMode } from "@/components/auth-card";
import { usePublicSiteSettings } from "@/lib/site-settings";
import { getStoredAuthRole, type AuthRole } from "@/store/auth";
import { cn } from "@/lib/utils";

const featureTags = ["图片生成", "平面设计", "科研绘图", "PPT制作", "一站式解决平台"];

export default function HomePage() {
  const site = usePublicSiteSettings();
  const mockupRef = useRef<HTMLDivElement | null>(null);
  const [role, setRole] = useState<AuthRole | null | undefined>(undefined);
  const [tilt, setTilt] = useState({ x: 0, y: 0 });
  const [authModalMode, setAuthModalMode] = useState<AuthMode | null>(null);

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
  }, []);

  if (role === undefined) {
    return null;
  }

  if (role) {
    return <Navigate to={role === "admin" ? "/admin/dashboard" : "/image/history"} replace />;
  }

  return (
    <div className="min-h-screen bg-black text-white">
      <header className="relative z-30 border-b border-white/10 bg-white/[0.045] shadow-[0_18px_58px_rgba(0,0,0,0.32)] backdrop-blur-2xl">
        <div className="mx-auto grid min-h-[72px] w-full max-w-[1440px] grid-cols-[auto_1fr_auto] items-center gap-6 px-5 sm:px-8 lg:px-11">
          <Link to="/" className="inline-flex items-center gap-3 text-lg font-bold" aria-label={`${site.name} 首页`}>
            <BrandMark logoUrl={site.logoUrl} siteName={site.name} />
            <span>{site.name || "Image Studio"}</span>
          </Link>

          <nav className="hidden justify-center gap-1 text-sm font-medium text-[#aeb7c8] md:flex" aria-label="产品导航">
            {["产品介绍", "使用说明", "客服服务", "模板广场"].map((item) => (
              <a key={item} href="#features" className="rounded-lg px-3 py-2.5 transition hover:bg-white/10 hover:text-white">
                {item}
              </a>
            ))}
          </nav>

          <div className="flex items-center gap-2">
            <button
              type="button"
              data-auth-open="login"
              onClick={() => setAuthModalMode("login")}
              className="inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border border-cyan-200/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.20),rgba(255,255,255,0.04)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(47,107,255,0.9),rgba(40,214,255,0.78))] px-4 text-sm font-bold shadow-[0_14px_34px_rgba(41,152,255,0.28)] transition hover:-translate-y-0.5 hover:border-cyan-100/60"
            >
              立即开始
              <ArrowRight className="size-4" />
            </button>
          </div>
        </div>
      </header>

      <main className="relative min-h-[calc(100vh-72px)] overflow-hidden bg-[radial-gradient(circle_at_50%_22%,rgba(53,88,255,0.16),transparent_36rem),#000] px-5 pb-24 pt-16 sm:px-8 lg:px-10">
        <div className="pointer-events-none absolute -left-[12vw] -top-[18%] h-[80vh] w-[60vw] rotate-[20deg] bg-[radial-gradient(ellipse_at_0%_0%,rgba(160,205,255,0.75),rgba(70,135,235,0.4)_42%,rgba(70,135,235,0)_85%)] opacity-70 blur-[64px] mix-blend-screen" />
        <div className="pointer-events-none absolute -right-[12vw] -top-[18%] h-[80vh] w-[60vw] -rotate-[20deg] bg-[radial-gradient(ellipse_at_100%_0%,rgba(160,205,255,0.75),rgba(70,135,235,0.4)_42%,rgba(70,135,235,0)_85%)] opacity-70 blur-[64px] mix-blend-screen" />
        <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:48px_48px] opacity-20 [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,transparent_72%)]" />

        <section className="relative z-10 mx-auto flex w-full max-w-[1200px] flex-col items-center text-center" aria-labelledby="hero-title">
          <div className="inline-flex min-h-9 items-center gap-2.5 rounded-full border border-white/15 bg-white/[0.055] px-4 text-[13px] font-medium text-[#c8d1df] shadow-[inset_0_1px_0_rgba(255,255,255,0.12),0_0_34px_rgba(47,107,255,0.14)]">
            <span className="size-2 rounded-full bg-[#5bd6ff] shadow-[0_0_18px_rgba(91,214,255,0.9)]" />
            <span>已服务 1000+ 用户</span>
          </div>

          <h1
            id="hero-title"
            className="mt-10 max-w-[1020px] bg-[linear-gradient(180deg,#fff,#eef4ff_34%,rgba(163,174,192,0.72)_68%,rgba(116,123,134,0.12))] bg-clip-text pb-4 text-[clamp(48px,9vw,112px)] font-extrabold leading-[1.08] tracking-normal text-transparent"
          >
            引爆你的创意
          </h1>

          <p className="mt-5 flex max-w-[940px] flex-wrap justify-center gap-y-2 text-[15px] leading-7 text-[#9aa3b2] sm:text-[19px]">
            {featureTags.map((tag, index) => (
              <span
                key={tag}
                className={cn(
                  "inline-flex items-center",
                  index > 0 && "before:mx-[18px] before:h-3.5 before:w-px before:bg-white/20 before:content-['']",
                )}
              >
                {tag}
              </span>
            ))}
          </p>

          <div className="mt-10 flex flex-wrap justify-center gap-5">
            <button
              type="button"
              data-auth-open="login"
              onClick={() => setAuthModalMode("login")}
              className="inline-flex min-h-[50px] min-w-28 items-center justify-center rounded-[10px] border border-cyan-200/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.2),rgba(255,255,255,0.04)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(63,105,255,0.82),rgba(31,220,255,0.74))] px-6 text-lg font-bold tracking-[0.12em] shadow-[0_18px_44px_rgba(41,152,255,0.3)] transition hover:-translate-y-0.5"
            >
              登录
            </button>
            <button
              type="button"
              data-auth-open="register"
              onClick={() => setAuthModalMode("register")}
              className="inline-flex min-h-[50px] min-w-28 items-center justify-center rounded-[10px] border border-white/25 bg-white/[0.055] px-6 text-lg font-bold tracking-[0.12em] text-[#e6ebf2] shadow-[0_16px_38px_rgba(0,0,0,0.36)] transition hover:-translate-y-0.5 hover:bg-white/10"
            >
              注册
            </button>
          </div>

          <div
            ref={mockupRef}
            data-home-mockup-stage
            className="mt-20 hidden h-[750px] w-full max-w-[1150px] [perspective:1500px] lg:block"
            onPointerMove={(event) => {
              const bounds = mockupRef.current?.getBoundingClientRect();
              if (!bounds) {
                return;
              }
              const x = (event.clientX - bounds.left) / bounds.width - 0.5;
              const y = (event.clientY - bounds.top) / bounds.height - 0.5;
              setTilt({ x: y * -6, y: x * 8 });
            }}
            onPointerLeave={() => setTilt({ x: 0, y: 0 })}
          >
            <ProductMockup tiltX={tilt.x} tiltY={tilt.y} />
          </div>

          <div className="mt-14 grid w-full gap-4 text-left sm:grid-cols-3 lg:hidden" id="features">
            {[
              { icon: ImageIcon, title: "生成工作台", text: "把提示词、参考图、参数和结果放在同一个创作流里。" },
              { icon: Boxes, title: "资产沉淀", text: "保留常用图像、风格和项目素材，方便继续编辑和复用。" },
              { icon: LayoutTemplate, title: "多场景输出", text: "覆盖设计海报、照片摄影、科研图和演示视觉。" },
            ].map((item) => {
              const Icon = item.icon;
              return (
                <article key={item.title} className="rounded-lg border border-white/15 bg-white/[0.055] p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
                  <Icon className="size-6 text-cyan-200" />
                  <h2 className="mt-4 text-base font-semibold">{item.title}</h2>
                  <p className="mt-2 text-sm leading-6 text-[#9aa3b2]">{item.text}</p>
                </article>
              );
            })}
          </div>
        </section>
      </main>

      <AuthModal
        mode={authModalMode}
        onModeChange={setAuthModalMode}
        onClose={() => setAuthModalMode(null)}
      />
    </div>
  );
}

function AuthModal({
  mode,
  onModeChange,
  onClose,
}: {
  mode: AuthMode | null;
  onModeChange: (mode: AuthMode) => void;
  onClose: () => void;
}) {
  useEffect(() => {
    if (!mode) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    document.body.classList.add("overflow-hidden");
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.classList.remove("overflow-hidden");
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [mode, onClose]);

  return (
    <div
      data-auth-modal={mode ? "true" : "false"}
      className={cn(
        "fixed inset-0 z-[80] grid place-items-center bg-[radial-gradient(circle_at_50%_0%,rgba(80,145,255,0.10),transparent_30rem),rgba(0,0,0,0.94)] p-4 opacity-0 transition duration-300 sm:p-6",
        mode ? "pointer-events-auto opacity-100" : "pointer-events-none",
      )}
      aria-hidden={!mode}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      {mode ? (
        <div className="relative w-full max-w-[470px] animate-[modalIn_320ms_cubic-bezier(0.215,0.61,0.355,1)_both]">
          <button
            type="button"
            aria-label="关闭弹窗"
            onClick={onClose}
            className="absolute right-4 top-4 z-10 grid size-[34px] place-items-center rounded-lg border border-white/15 bg-white/[0.065] text-[#d9e2ef] transition hover:bg-white/10"
          >
            <span className="relative block size-4 before:absolute before:left-0 before:top-1/2 before:h-0.5 before:w-4 before:-translate-y-1/2 before:rotate-45 before:rounded-full before:bg-current after:absolute after:left-0 after:top-1/2 after:h-0.5 after:w-4 after:-translate-y-1/2 after:-rotate-45 after:rounded-full after:bg-current" />
          </button>
          <AuthCard
            mode={mode}
            onModeChange={onModeChange}
            className="max-h-[min(760px,calc(100vh-2rem))] overflow-y-auto"
          />
        </div>
      ) : null}
    </div>
  );
}

function BrandMark({ logoUrl, siteName }: { logoUrl?: string; siteName: string }) {
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [logoUrl]);

  if (logoUrl && !failed) {
    return (
      <span className="app-logo-image-frame inline-grid size-[30px] place-items-center overflow-hidden rounded-lg">
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
    <span className="inline-grid size-[30px] place-items-center rounded-lg bg-[linear-gradient(135deg,#2f6bff,#8b5cff_54%,#28d6ff)] shadow-[0_0_26px_rgba(40,214,255,0.35)]">
      <Sparkles className="size-4" />
    </span>
  );
}

function ProductMockup({ tiltX, tiltY }: { tiltX: number; tiltY: number }) {
  return (
    <div
      data-home-mockup
      className="grid h-full w-full grid-cols-[210px_minmax(0,1fr)_292px] gap-4 rounded-lg border border-white/15 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.025)),rgba(5,7,12,0.84)] p-4 text-left shadow-[0_34px_110px_rgba(34,131,255,0.24),0_28px_90px_rgba(0,0,0,0.72),inset_0_1px_0_rgba(255,255,255,0.12)] transition-transform duration-150"
      style={{ transform: `rotateX(${tiltX.toFixed(2)}deg) rotateY(${tiltY.toFixed(2)}deg)` }}
    >
      <aside className="flex min-w-0 flex-col rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-5">
        <div className="flex items-center gap-3 text-sm font-semibold">
          <span className="size-7 rounded-lg bg-[linear-gradient(135deg,#2f6bff,#28d6ff)] shadow-[0_0_24px_rgba(40,214,255,0.45)]" />
          Image Studio
        </div>
        <nav className="mt-9 grid gap-2 text-sm font-medium text-[#8d96a5]">
          {["生成工作台", "科研绘图", "照片摄影", "PPT 视觉", "品牌素材"].map((item, index) => (
            <span key={item} className={index === 0 ? "rounded-lg bg-white/[0.085] px-3 py-3 text-white" : "px-3 py-3"}>
              {item}
            </span>
          ))}
        </nav>
        <div className="mt-auto rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-4">
          <span className="text-xs font-medium text-[#8f98a6]">今日生成额度</span>
          <strong className="mt-2 block text-3xl text-white">842</strong>
          <small className="text-[#5bd6ff]">剩余 92%</small>
        </div>
      </aside>

      <section className="grid min-w-0 grid-rows-[72px_1fr_160px] gap-4">
        <header className="flex items-center justify-between rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] px-5">
          <div>
            <span className="text-xs font-medium text-[#8f98a6]">Creative canvas</span>
            <strong className="mt-1.5 block text-lg text-white">科研封面图生成</strong>
          </div>
          <div className="flex gap-2 text-xs text-[#c9d1dc]">
            {["Design", "4K", "Fast"].map((item) => (
              <span key={item} className="rounded-full border border-white/15 bg-white/[0.05] px-3 py-2">
                {item}
              </span>
            ))}
          </div>
        </header>

        <div className="relative overflow-hidden rounded-lg border border-white/15 bg-[radial-gradient(circle_at_18%_10%,rgba(53,150,255,0.25),transparent_16rem),radial-gradient(circle_at_90%_18%,rgba(255,108,172,0.22),transparent_13rem),rgba(7,9,14,0.86)] p-5">
          <div className="relative h-full min-h-[360px] overflow-hidden rounded-lg border border-white/15 bg-[linear-gradient(110deg,transparent,rgba(111,207,255,0.85)_20%,rgba(255,138,179,0.88)_34%,transparent_52%),linear-gradient(18deg,transparent_18%,rgba(44,114,255,0.92)_42%,rgba(55,218,255,0.72)_54%,transparent_76%),radial-gradient(ellipse_at_64%_24%,rgba(255,202,136,0.78),transparent_16rem),#02030a]">
            <div className="absolute left-1/2 top-1/2 flex -translate-x-1/2 -translate-y-1/2 items-baseline gap-3 text-lg text-white/85">
              <span>WHYSL</span>
              <strong>DESIGN</strong>
            </div>
          </div>
          <div className="absolute bottom-8 right-8 w-[340px] rounded-lg border border-white/15 bg-black/70 p-4 backdrop-blur-xl">
            <span className="text-xs font-medium text-[#8f98a6]">Prompt</span>
            <p className="mt-2 text-[13px] leading-6 text-[#d4dbea]">未来感蓝色光束，科研论文封面，精密网格，玻璃质感，暗黑背景，高级商业海报。</p>
          </div>
        </div>

        <div className="grid grid-cols-4 gap-3">
          {["平面设计", "照片摄影", "科研绘图", "PPT制作"].map((item) => (
            <article key={item} className="rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-4">
              <span className="mb-3 block h-11 rounded-lg bg-[linear-gradient(135deg,rgba(47,107,255,0.9),rgba(40,214,255,0.55))]" />
              <strong className="block text-sm text-white">{item}</strong>
              <small className="mt-1 block text-xs text-[#7f8898]">Visual asset</small>
            </article>
          ))}
        </div>
      </section>

      <aside className="flex min-w-0 flex-col gap-5 rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-5">
        <div>
          <span className="text-xs font-medium text-[#8f98a6]">Generation settings</span>
          <strong className="mt-1.5 block text-lg text-white">图像参数</strong>
        </div>
        {[
          ["风格强度", "76%"],
          ["细节密度", "64%"],
        ].map(([label, width]) => (
          <div key={label} className="rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-4">
            <label className="text-xs font-medium text-[#8f98a6]">{label}</label>
            <div className="mt-3 h-2 overflow-hidden rounded-full bg-white/10">
              <span className="block h-full rounded-full bg-[linear-gradient(90deg,#2f6bff,#28d6ff)]" style={{ width }} />
            </div>
          </div>
        ))}
        <div className="mt-auto grid grid-cols-3 gap-2">
          {[
            ["尺寸", "4K"],
            ["版本", "V6"],
            ["耗时", "12s"],
          ].map(([label, value]) => (
            <div key={label} className="rounded-lg border border-white/15 bg-[rgba(10,12,18,0.82)] p-3">
              <span className="text-xs text-[#8f98a6]">{label}</span>
              <strong className="mt-2 block text-xl text-white">{value}</strong>
            </div>
          ))}
        </div>
      </aside>
    </div>
  );
}
