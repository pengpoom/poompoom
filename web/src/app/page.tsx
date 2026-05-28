"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import {
  ArrowRight,
  Brain,
  FolderOpen,
  ImageIcon,
  Layers,
  LayoutTemplate,
  Monitor,
  ShoppingBag,
  Sparkles,
} from "lucide-react";

import { AuthCard, type AuthMode } from "@/components/auth-card";
import { usePublicSiteSettings } from "@/lib/site-settings";
import { getStoredAuthRole, type AuthRole } from "@/store/auth";
import { cn } from "@/lib/utils";

const featureTags = ["图片生成", "平面设计", "电商运营", "科研绘图", "PPT制作", "一站式解决平台"];

const features = [
  { icon: ImageIcon, title: "图片生成", text: "一句话生成高质量图像，多模型、4K、批量出图，灵感即刻成型。" },
  { icon: LayoutTemplate, title: "平面设计", text: "海报、Banner、品牌视觉，模板即改即用，排版与配色一步到位。" },
  { icon: ShoppingBag, title: "电商运营", text: "商品主图、详情页、营销素材，一键成套，上新节奏快人一步。" },
  { icon: Brain, title: "科研绘图", text: "论文封面、机制示意、figure 风格，规范严谨又不失高级质感。" },
  { icon: Monitor, title: "PPT 制作", text: "演示视觉、配图、版式，几分钟成稿，汇报与提案更有说服力。" },
];

const highlights = [
  { icon: Brain, title: "多模型可选", text: "GPT、Gemini 等多家模型，按场景自由切换。" },
  { icon: ImageIcon, title: "4K 高清", text: "高分辨率出图，细节锐利，可直接用于印刷。" },
  { icon: Layers, title: "批量生成", text: "一次多张、成套产出，把出图效率拉满。" },
  { icon: FolderOpen, title: "一站式管理", text: "提示词、风格、素材统一沉淀，随取随用。" },
];

export default function HomePage() {
  const site = usePublicSiteSettings();
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const mockupRef = useRef<HTMLDivElement | null>(null);
  const [role, setRole] = useState<AuthRole | null | undefined>(undefined);
  const [authModalMode, setAuthModalMode] = useState<AuthMode | null>(null);
  const [headerScrolled, setHeaderScrolled] = useState(false);

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

  useEffect(() => {
    const scrollContainer = wrapperRef.current?.parentElement;
    if (!scrollContainer) {
      return;
    }
    const onScroll = () => {
      setHeaderScrolled(scrollContainer.scrollTop > 16);
    };
    scrollContainer.addEventListener("scroll", onScroll, { passive: true });
    return () => scrollContainer.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    const els = wrapperRef.current?.querySelectorAll(".landing-reveal");
    if (!els?.length) {
      return;
    }
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      els.forEach((el) => el.classList.add("in"));
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add("in");
            observer.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.15, rootMargin: "0px 0px -10% 0px" },
    );
    els.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [role]);

  const handlePointerMove = useCallback((event: React.PointerEvent) => {
    const wrap = mockupRef.current;
    const dashboard = wrap?.querySelector("[data-home-mockup]") as HTMLElement | null;
    if (!wrap || !dashboard) {
      return;
    }
    const b = wrap.getBoundingClientRect();
    const x = (event.clientX - b.left) / b.width - 0.5;
    const y = (event.clientY - b.top) / b.height - 0.5;
    dashboard.style.setProperty("--tilt-x", `${(y * -6).toFixed(2)}deg`);
    dashboard.style.setProperty("--tilt-y", `${(x * 8).toFixed(2)}deg`);
  }, []);

  const handlePointerLeave = useCallback(() => {
    const dashboard = mockupRef.current?.querySelector("[data-home-mockup]") as HTMLElement | null;
    if (dashboard) {
      dashboard.style.setProperty("--tilt-x", "0deg");
      dashboard.style.setProperty("--tilt-y", "0deg");
    }
  }, []);

  const openAuth = useCallback((mode: AuthMode) => setAuthModalMode(mode), []);

  if (role === undefined) {
    return null;
  }

  if (role) {
    return <Navigate to={role === "admin" ? "/admin/dashboard" : "/image/history"} replace />;
  }

  return (
    <div ref={wrapperRef} className="min-h-screen bg-black text-white">
      {/* ── Header ── */}
      <header
        className={cn(
          "sticky top-0 z-30 border-b border-white/12 backdrop-blur-[22px] [backdrop-filter:blur(22px)_saturate(1.35)] [-webkit-backdrop-filter:blur(22px)_saturate(1.35)]",
          "bg-[linear-gradient(180deg,rgba(255,255,255,0.09),rgba(255,255,255,0.025)),rgba(7,9,14,0.62)]",
          "shadow-[0_18px_58px_rgba(0,0,0,0.32),inset_0_1px_0_rgba(255,255,255,0.14)]",
          "transition-[background,box-shadow,min-height] duration-[350ms] [transition-timing-function:cubic-bezier(0.215,0.61,0.355,1)]",
          headerScrolled &&
            "bg-[linear-gradient(180deg,rgba(255,255,255,0.06),rgba(255,255,255,0.02)),rgba(5,7,12,0.82)] shadow-[0_22px_60px_rgba(0,0,0,0.5),inset_0_1px_0_rgba(255,255,255,0.12)]",
        )}
      >
        <div
          className={cn(
            "mx-auto grid w-full max-w-[1440px] grid-cols-[auto_1fr_auto] items-center gap-6 px-5 transition-[min-height] duration-[350ms] [transition-timing-function:cubic-bezier(0.215,0.61,0.355,1)] sm:px-8 lg:px-11",
            headerScrolled ? "min-h-[56px]" : "min-h-[72px]",
          )}
        >
          <Link to="/" className="inline-flex items-center gap-2.5 text-lg font-bold" aria-label={`${site.name} 首页`}>
            <BrandMark logoUrl={site.logoUrl} siteName={site.name} />
            <span>{site.name || "Image Studio"}</span>
          </Link>

          <nav className="hidden justify-center gap-1.5 text-sm font-medium text-[#aeb7c8] md:flex" aria-label="产品导航">
            {["产品介绍", "使用说明", "客服服务", "模板广场"].map((item) => (
              <a
                key={item}
                href="#features"
                className="inline-flex min-h-[38px] items-center rounded-lg px-3.5 transition-[background,color,transform] duration-[180ms] hover:-translate-y-px hover:bg-white/8 hover:text-white"
              >
                {item}
              </a>
            ))}
          </nav>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => openAuth("login")}
              className="inline-flex min-h-[38px] items-center justify-center gap-2 rounded-lg border border-[rgba(191,232,255,0.42)] bg-[linear-gradient(135deg,rgba(255,255,255,0.2),rgba(255,255,255,0.045)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(47,107,255,0.9),rgba(40,214,255,0.78))] px-[18px] text-sm font-bold shadow-[0_14px_34px_rgba(41,152,255,0.28),inset_0_1px_0_rgba(255,255,255,0.42),inset_0_-1px_0_rgba(255,255,255,0.08)] backdrop-blur-[16px] [backdrop-filter:blur(16px)_saturate(1.35)] transition-[transform,box-shadow,border-color] duration-[180ms] hover:-translate-y-px hover:border-[rgba(224,247,255,0.62)] hover:shadow-[0_16px_42px_rgba(41,152,255,0.36),inset_0_1px_0_rgba(255,255,255,0.5),inset_0_-1px_0_rgba(255,255,255,0.12)]"
            >
              立即开始
              <ArrowRight className="size-4" />
            </button>
          </div>
        </div>
      </header>

      {/* ── Hero ── */}
      <section
        className="relative overflow-hidden bg-[radial-gradient(circle_at_50%_22%,rgba(53,88,255,0.16),transparent_36rem),#000] px-6 pb-[72px] pt-[84px]"
        aria-labelledby="hero-title"
      >
        <div className="beam-animate pointer-events-none absolute -left-[12vw] -top-[18%] h-[80vh] w-[60vw] origin-top-left rotate-[20deg] bg-[radial-gradient(ellipse_at_0%_0%,rgba(160,205,255,0.75),rgba(70,135,235,0.4)_42%,rgba(70,135,235,0)_85%)] blur-[64px] mix-blend-screen" />
        <div className="beam-animate pointer-events-none absolute -right-[12vw] -top-[18%] h-[80vh] w-[60vw] origin-top-right -rotate-[20deg] bg-[radial-gradient(ellipse_at_100%_0%,rgba(160,205,255,0.75),rgba(70,135,235,0.4)_42%,rgba(70,135,235,0)_85%)] blur-[64px] mix-blend-screen" />
        <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:48px_48px] opacity-20 [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,transparent_72%)]" />

        <div className="relative z-10 mx-auto flex w-full max-w-[1200px] flex-col items-center text-center">
          <div className="entry-reveal inline-flex min-h-[34px] items-center gap-2.5 rounded-full border border-white/14 bg-white/[0.055] px-4 text-[13px] font-medium leading-none text-[#c8d1df] shadow-[inset_0_1px_0_rgba(255,255,255,0.12),0_0_34px_rgba(47,107,255,0.14)]" style={{ "--reveal-i": 0 } as React.CSSProperties}>
            <span className="size-[7px] rounded-full bg-[#5bd6ff] shadow-[0_0_18px_rgba(91,214,255,0.9)]" />
            <span>AI creative workspace for images, design and research figures</span>
          </div>

          <h1
            id="hero-title"
            className="entry-reveal mt-[42px] max-w-[1020px] bg-[linear-gradient(180deg,#fff,#eef4ff_34%,rgba(163,174,192,0.72)_68%,rgba(116,123,134,0.12))] bg-clip-text pb-[18px] text-[clamp(48px,9vw,112px)] font-extrabold leading-[1.08] tracking-[0.06em] text-transparent"
            style={{ "--reveal-i": 1 } as React.CSSProperties}
          >
            AI Figures
          </h1>

          <p className="entry-reveal mt-7 flex max-w-[940px] flex-wrap justify-center gap-[18px] text-[19px] leading-[1.45] text-[#9aa3b2]" style={{ "--reveal-i": 2 } as React.CSSProperties}>
            {featureTags.map((tag, index) => (
              <span
                key={tag}
                className={cn(
                  "inline-flex items-center",
                  index > 0 && "before:mr-[18px] before:h-3.5 before:w-px before:bg-white/18 before:content-['']",
                )}
              >
                {tag}
              </span>
            ))}
          </p>

          <div className="entry-reveal mt-[46px] flex flex-wrap justify-center gap-7" style={{ "--reveal-i": 3 } as React.CSSProperties}>
            <button
              type="button"
              onClick={() => openAuth("login")}
              className="inline-flex min-h-[50px] min-w-28 items-center justify-center rounded-[10px] border border-[rgba(191,232,255,0.42)] bg-[linear-gradient(135deg,rgba(255,255,255,0.2),rgba(255,255,255,0.04)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(63,105,255,0.82),rgba(31,220,255,0.74))] px-6 text-lg font-bold tracking-[0.22em] shadow-[0_18px_44px_rgba(41,152,255,0.3),inset_0_1px_0_rgba(255,255,255,0.42),inset_0_-1px_0_rgba(255,255,255,0.08)] backdrop-blur-[18px] [backdrop-filter:blur(18px)_saturate(1.35)] transition-[transform,border-color,box-shadow] duration-[220ms] hover:-translate-y-0.5"
            >
              登录
            </button>
            <button
              type="button"
              onClick={() => openAuth("register")}
              className="inline-flex min-h-[50px] min-w-28 items-center justify-center rounded-[10px] border border-white/22 bg-[linear-gradient(135deg,rgba(255,255,255,0.12),rgba(255,255,255,0.035)),rgba(7,10,18,0.56)] px-6 text-lg font-bold tracking-[0.22em] text-[#e6ebf2] shadow-[0_16px_38px_rgba(0,0,0,0.36),inset_0_1px_0_rgba(255,255,255,0.2)] backdrop-blur-[18px] [backdrop-filter:blur(18px)_saturate(1.35)] transition-[transform,border-color,box-shadow] duration-[220ms] hover:-translate-y-0.5"
            >
              注册
            </button>
          </div>

          <div
            ref={mockupRef}
            className="entry-reveal mt-[84px] hidden h-[750px] w-full max-w-[1150px] [perspective:1500px] [transform-style:preserve-3d] lg:block"
            style={{ "--reveal-i": 4 } as React.CSSProperties}
            onPointerMove={handlePointerMove}
            onPointerLeave={handlePointerLeave}
          >
            <ProductMockup />
          </div>

          <div className="mt-14 grid w-full gap-4 text-left sm:grid-cols-3 lg:hidden">
            {[
              { icon: ImageIcon, title: "生成工作台", text: "把提示词、参考图、参数和结果放在同一个创作流里。" },
              { icon: ShoppingBag, title: "电商运营", text: "商品主图、详情页、营销素材，一键成套，上新节奏快人一步。" },
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
        </div>
      </section>

      {/* ── Features ── */}
      <section className="relative z-[1] mx-auto max-w-[1200px] px-8 pb-10 pt-32" id="features" aria-labelledby="features-title">
        <div className="landing-reveal mx-auto mb-14 max-w-[720px] text-center">
          <span className="text-xs font-semibold uppercase tracking-[0.26em] text-[#28d6ff]">CAPABILITIES</span>
          <h2
            id="features-title"
            className="mt-4 bg-[linear-gradient(180deg,#ffffff_40%,#a9c4ff)] bg-clip-text text-[clamp(28px,4.2vw,44px)] font-extrabold leading-[1.08] tracking-[-0.025em] text-transparent"
          >
            一个平台，覆盖你的全部视觉创作
          </h2>
          <p className="mx-auto mt-[18px] max-w-[560px] text-base leading-[1.7] text-[#9aa3b2]">
            从一句提示词到成套视觉资产，AI Figs 把五种创作场景收进同一个工作流。
          </p>
        </div>

        <div className="grid gap-[18px] sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f, i) => {
            const Icon = f.icon;
            return (
              <article
                key={f.title}
                className="landing-reveal relative rounded-[18px] border border-white/10 bg-[linear-gradient(180deg,rgba(255,255,255,0.07),rgba(255,255,255,0.02)),rgba(9,12,20,0.5)] p-[26px] shadow-[0_18px_44px_rgba(0,0,0,0.34),inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-[18px] [backdrop-filter:blur(18px)_saturate(1.2)] transition-[transform,box-shadow,border-color] duration-300 [transition-timing-function:cubic-bezier(0.215,0.61,0.355,1)] hover:-translate-y-[5px] hover:border-[rgba(140,190,255,0.34)] hover:shadow-[0_28px_60px_rgba(0,0,0,0.46),0_0_34px_rgba(47,107,255,0.2),inset_0_1px_0_rgba(255,255,255,0.14)]"
                style={{ "--reveal-i": i } as React.CSSProperties}
              >
                <span className="inline-flex size-[50px] items-center justify-center rounded-[14px] bg-[linear-gradient(135deg,#2f6bff,#8b5cff_55%,#28d6ff)] shadow-[0_10px_24px_rgba(47,107,255,0.4),inset_0_1px_0_rgba(255,255,255,0.45)]">
                  <Icon className="size-6 text-white" />
                </span>
                <h3 className="mt-5 text-lg font-bold tracking-[-0.01em]">{f.title}</h3>
                <p className="mt-2 text-sm leading-[1.65] text-[#9aa3b2]">{f.text}</p>
              </article>
            );
          })}
        </div>
      </section>

      {/* ── Showcase ── */}
      <section className="relative z-[1] mx-auto grid max-w-[1160px] items-center gap-14 px-8 py-[90px] lg:grid-cols-[1fr_1.1fr]" aria-labelledby="showcase-title">
        <div className="landing-reveal">
          <span className="text-xs font-semibold uppercase tracking-[0.26em] text-[#28d6ff]">HOW IT WORKS</span>
          <h2 id="showcase-title" className="mt-4 text-[clamp(26px,3.6vw,38px)] font-extrabold leading-[1.1] tracking-[-0.02em]">
            从一句话，到成套视觉
          </h2>
          <p className="mt-4 max-w-[460px] text-[15.5px] leading-[1.75] text-[#9aa3b2]">
            描述需求、选好参数、一键生成——批量出图、在线微调、导出成套素材，整条创作链路都在一个工作台里完成。
          </p>
          <ol className="mb-[30px] mt-[26px] flex list-none flex-col gap-3.5 p-0">
            {[
              "描述画面，或上传参考图",
              "选模型 / 比例 / 清晰度，一键生成",
              "批量出图、微调、导出成套素材",
            ].map((step, i) => (
              <li key={i} className="flex items-center gap-[13px] text-[14.5px] text-[#d2d9e6]">
                <span className="inline-flex size-[26px] shrink-0 items-center justify-center rounded-lg bg-[linear-gradient(135deg,#2f6bff,#28d6ff)] text-[13px] font-bold shadow-[0_6px_16px_rgba(47,107,255,0.4)]">
                  {i + 1}
                </span>
                {step}
              </li>
            ))}
          </ol>
          <button
            type="button"
            onClick={() => openAuth("register")}
            className="inline-flex min-h-[50px] items-center justify-center rounded-[10px] border border-[rgba(191,232,255,0.42)] bg-[linear-gradient(135deg,rgba(255,255,255,0.2),rgba(255,255,255,0.04)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(63,105,255,0.82),rgba(31,220,255,0.74))] px-6 text-base font-bold tracking-[0.22em] shadow-[0_18px_44px_rgba(41,152,255,0.3),inset_0_1px_0_rgba(255,255,255,0.42),inset_0_-1px_0_rgba(255,255,255,0.08)] backdrop-blur-[18px] [backdrop-filter:blur(18px)_saturate(1.35)] transition-[transform,box-shadow] duration-[220ms] hover:-translate-y-0.5"
          >
            免费开始创作
          </button>
        </div>

        <div className="landing-reveal" style={{ "--reveal-i": 1 } as React.CSSProperties}>
          <div className="grid grid-cols-2 gap-3.5">
            {[
              { label: "电商主图", cls: "bg-[radial-gradient(120%_100%_at_20%_0%,rgba(40,214,255,0.5),transparent_60%),linear-gradient(135deg,#0a1024,#0a0717)]" },
              { label: "科研封面", cls: "bg-[radial-gradient(120%_100%_at_80%_0%,rgba(47,107,255,0.55),transparent_60%),linear-gradient(135deg,#0a0a1e,#070512)]" },
              { label: "品牌海报", cls: "bg-[radial-gradient(120%_100%_at_30%_100%,rgba(139,92,255,0.5),transparent_60%),linear-gradient(135deg,#100a1f,#08060f)]" },
              { label: "PPT 视觉", cls: "bg-[radial-gradient(120%_100%_at_70%_100%,rgba(40,214,255,0.4),rgba(139,92,255,0.3)_50%,transparent_70%),linear-gradient(135deg,#0a0f20,#070611)]" },
            ].map((shot) => (
              <div
                key={shot.label}
                className={cn(
                  "flex aspect-[4/3] items-end overflow-hidden rounded-2xl border border-white/10 p-3 shadow-[0_18px_44px_rgba(0,0,0,0.4),inset_0_1px_0_rgba(255,255,255,0.1)]",
                  shot.cls,
                )}
              >
                <span className="relative z-[1] text-xs font-semibold tracking-[0.04em] text-white [text-shadow:0_2px_8px_rgba(0,0,0,0.5)]">
                  {shot.label}
                </span>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Highlights ── */}
      <section className="relative z-[1] mx-auto grid max-w-[1160px] gap-[22px] px-8 py-[50px] sm:grid-cols-2 lg:grid-cols-4" aria-label="核心能力">
        {highlights.map((h, i) => {
          const Icon = h.icon;
          return (
            <div key={h.title} className="landing-reveal flex items-start gap-3.5" style={{ "--reveal-i": i } as React.CSSProperties}>
              <span className="inline-flex size-[42px] shrink-0 items-center justify-center rounded-xl border border-white/12 bg-white/5 shadow-[inset_0_1px_0_rgba(255,255,255,0.1)]">
                <Icon className="size-5 text-white/92" />
              </span>
              <div>
                <h3 className="text-[15.5px] font-bold">{h.title}</h3>
                <p className="mt-1.5 text-[13px] leading-relaxed text-[#9aa3b2]">{h.text}</p>
              </div>
            </div>
          );
        })}
      </section>

      {/* ── CTA Band ── */}
      <section className="landing-reveal relative z-[1] mx-auto mt-[90px] max-w-[1000px] overflow-hidden rounded-[26px] border border-white/12 bg-[linear-gradient(180deg,rgba(255,255,255,0.06),rgba(255,255,255,0.015)),rgba(8,11,18,0.6)] px-8 py-[70px] text-center shadow-[0_30px_80px_rgba(0,0,0,0.5),inset_0_1px_0_rgba(255,255,255,0.12)]">
        <div className="pointer-events-none absolute left-1/2 top-[-40%] h-[400px] w-[600px] -translate-x-1/2 bg-[radial-gradient(circle,rgba(47,107,255,0.4),rgba(139,92,255,0.18)_45%,transparent_70%)] blur-[20px]" />
        <h2 className="relative z-[1] mx-auto text-[clamp(26px,4vw,40px)] font-extrabold leading-[1.1] tracking-[-0.025em]">
          准备好让创作快人一步了吗？
        </h2>
        <p className="relative z-[1] mx-auto mt-4 max-w-[440px] text-base text-[#9aa3b2]">
          免费开始，几分钟生成你的第一套视觉资产。
        </p>
        <div className="relative z-[1] mt-[30px] flex flex-wrap justify-center gap-3.5">
          <button
            type="button"
            onClick={() => openAuth("register")}
            className="inline-flex min-h-[50px] items-center justify-center rounded-[10px] border border-[rgba(191,232,255,0.42)] bg-[linear-gradient(135deg,rgba(255,255,255,0.2),rgba(255,255,255,0.04)_38%,rgba(255,255,255,0.16)),linear-gradient(135deg,rgba(63,105,255,0.82),rgba(31,220,255,0.74))] px-6 text-base font-bold tracking-[0.22em] shadow-[0_18px_44px_rgba(41,152,255,0.3),inset_0_1px_0_rgba(255,255,255,0.42),inset_0_-1px_0_rgba(255,255,255,0.08)] transition-[transform,box-shadow] duration-[220ms] hover:-translate-y-0.5"
          >
            免费开始创作
          </button>
          <button
            type="button"
            className="inline-flex min-h-[50px] items-center justify-center rounded-[10px] border border-white/22 bg-[linear-gradient(135deg,rgba(255,255,255,0.12),rgba(255,255,255,0.035)),rgba(7,10,18,0.56)] px-6 text-base font-bold tracking-[0.22em] text-[#e6ebf2] shadow-[0_16px_38px_rgba(0,0,0,0.36),inset_0_1px_0_rgba(255,255,255,0.2)] transition-[transform,box-shadow] duration-[220ms] hover:-translate-y-0.5"
          >
            查看模板广场
          </button>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="landing-reveal relative z-[1] mt-[110px] border-t border-white/8 bg-[linear-gradient(180deg,transparent,rgba(255,255,255,0.015))] px-8 pb-9 pt-[60px]">
        <div className="mx-auto flex max-w-[1160px] flex-wrap justify-between gap-10">
          <div className="max-w-[300px]">
            <Link to="/" className="inline-flex items-center gap-2.5 text-lg font-bold">
              <BrandMark logoUrl={site.logoUrl} siteName={site.name} />
              <span>{site.name || "Image Studio"}</span>
            </Link>
            <p className="mt-3 text-[13px] leading-[1.7] text-[#9aa3b2]">
              AI 创意工作台 · 图像 / 设计 / 电商 / 科研图 / PPT 一站式。
            </p>
          </div>
          <div className="flex flex-wrap gap-14">
            {[
              { heading: "产品", links: ["功能", "模板广场", "定价"] },
              { heading: "资源", links: ["使用说明", "更新日志", "帮助中心"] },
              { heading: "公司", links: ["关于我们", "联系", "条款"] },
            ].map((col) => (
              <div key={col.heading}>
                <h4 className="mb-3 text-[13px] font-semibold text-[#e6ebf2]">{col.heading}</h4>
                {col.links.map((link) => (
                  <a key={link} href="#" className="block py-[5px] text-[13px] text-[#9aa3b2] transition-colors duration-[180ms] hover:text-white">
                    {link}
                  </a>
                ))}
              </div>
            ))}
          </div>
        </div>
        <div className="mx-auto mt-11 max-w-[1160px] border-t border-white/6 pt-[22px] text-xs text-[#5d6677]">
          © 2026 AI Figs. All rights reserved.
        </div>
      </footer>

      <AuthModal mode={authModalMode} onModeChange={setAuthModalMode} onClose={() => setAuthModalMode(null)} />
    </div>
  );
}

/* ── Sub-components ── */

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
        "fixed inset-0 z-[60] grid place-items-center p-6 transition-[opacity,visibility,backdrop-filter] duration-[260ms]",
        "bg-[radial-gradient(circle_at_50%_0%,rgba(80,145,255,0.22),transparent_30rem),rgba(0,0,0,0.52)]",
        mode
          ? "pointer-events-auto opacity-100 backdrop-blur-[18px] [backdrop-filter:blur(18px)_saturate(1.35)] [-webkit-backdrop-filter:blur(18px)_saturate(1.35)]"
          : "pointer-events-none opacity-0 backdrop-blur-0 [backdrop-filter:blur(0)_saturate(1)] [-webkit-backdrop-filter:blur(0)_saturate(1)]",
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
            className="absolute right-4 top-4 z-10 grid size-[34px] place-items-center rounded-lg border border-white/14 bg-white/[0.06] text-[#d9e2ef] transition-[background,border-color,transform] duration-[180ms] hover:scale-[1.06] hover:border-white/28 hover:bg-white/12"
          >
            <span className="relative block size-3.5 before:absolute before:left-0 before:top-1/2 before:h-0.5 before:w-3.5 before:-translate-y-1/2 before:rotate-45 before:rounded-full before:bg-[#d9e2ef] after:absolute after:left-0 after:top-1/2 after:h-0.5 after:w-3.5 after:-translate-y-1/2 after:-rotate-45 after:rounded-full after:bg-[#d9e2ef]" />
          </button>
          <AuthCard
            mode={mode}
            onModeChange={onModeChange}
            className="landing-modal-glass-card max-h-[min(760px,calc(100vh-2rem))] overflow-y-auto"
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
    <span className="inline-grid size-[30px] place-items-center rounded-lg bg-[radial-gradient(circle_at_30%_26%,#ffffff_0_8%,transparent_9%),linear-gradient(135deg,#2f6bff,#8b5cff_54%,#28d6ff)] shadow-[0_0_26px_rgba(40,214,255,0.35),inset_0_1px_0_rgba(255,255,255,0.42)]">
      <Sparkles className="size-4" />
    </span>
  );
}

function ProductMockup() {
  return (
    <div
      data-home-mockup
      className="grid h-full w-full grid-cols-[210px_minmax(0,1fr)_292px] gap-4 rounded-lg border border-white/15 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.025)),rgba(5,7,12,0.84)] p-4 text-left shadow-[0_34px_110px_rgba(34,131,255,0.24),0_28px_90px_rgba(0,0,0,0.72),inset_0_1px_0_rgba(255,255,255,0.12)] [transform-origin:center] [transform-style:preserve-3d] [transform:rotateX(var(--tilt-x,0deg))_rotateY(var(--tilt-y,0deg))] [transition:transform_140ms_ease-out]"
    >
      <aside className="flex min-w-0 flex-col rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
        <div className="flex items-center gap-2.5 text-[15px] font-semibold text-[#f4f7fb]">
          <span className="size-[26px] rounded-lg bg-[linear-gradient(135deg,#2f6bff,#28d6ff)] shadow-[0_0_24px_rgba(40,214,255,0.45)]" />
          AI Figs
        </div>
        <nav className="mt-[34px] grid gap-2 text-sm font-medium text-[#8d96a5]">
          {["生成工作台", "电商运营", "科研绘图", "PPT 视觉", "品牌素材"].map((item, index) => (
            <span key={item} className={cn("rounded-lg px-3 py-[11px]", index === 0 && "bg-white/[0.085] text-white")}>
              {item}
            </span>
          ))}
        </nav>
        <div className="mt-auto rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] p-3.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
          <span className="text-xs font-medium text-[#8f98a6]">今日完成作品</span>
          <strong className="mt-2 block text-[32px] text-white">842</strong>
        </div>
      </aside>

      <section className="grid min-w-0 grid-rows-[72px_1fr_160px] gap-4">
        <header className="flex items-center justify-between rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] px-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
          <div>
            <span className="text-xs font-medium text-[#8f98a6]">Creative canvas</span>
            <strong className="mt-1.5 block text-lg text-[#f7f9fc]">科研封面图生成</strong>
          </div>
          <div className="flex gap-2 text-xs text-[#c9d1dc]">
            {["Design", "4K", "Fast"].map((item) => (
              <span key={item} className="rounded-full border border-white/13 bg-white/[0.05] px-2.5 py-[7px]">
                {item}
              </span>
            ))}
          </div>
        </header>

        <div className="relative overflow-hidden rounded-lg border border-white/13 bg-[radial-gradient(circle_at_18%_10%,rgba(53,150,255,0.25),transparent_16rem),radial-gradient(circle_at_90%_18%,rgba(255,108,172,0.22),transparent_13rem),rgba(7,9,14,0.86)] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
          <div className="aurora-card-sweep relative h-full min-h-[360px] overflow-hidden rounded-lg border border-white/12 bg-[linear-gradient(110deg,transparent,rgba(111,207,255,0.85)_20%,rgba(255,138,179,0.88)_34%,transparent_52%),linear-gradient(18deg,transparent_18%,rgba(44,114,255,0.92)_42%,rgba(55,218,255,0.72)_54%,transparent_76%),radial-gradient(ellipse_at_64%_24%,rgba(255,202,136,0.78),transparent_16rem),#02030a]">
            <div className="absolute left-1/2 top-1/2 z-[2] flex -translate-x-1/2 -translate-y-1/2 items-baseline gap-3 whitespace-nowrap text-lg text-white/82">
              <span>WHYSL</span>
              <strong className="text-white">DESIGN</strong>
            </div>
          </div>
          <div className="absolute bottom-[34px] right-[34px] w-[340px] rounded-lg border border-white/13 bg-[rgba(4,6,11,0.72)] p-3.5 backdrop-blur-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
            <span className="text-xs font-medium text-[#8f98a6]">Prompt</span>
            <p className="mt-2 text-[13px] leading-[1.55] text-[#d4dbea]">未来感蓝色光束，科研论文封面，精密网格，玻璃质感，暗黑背景，高级商业海报。</p>
          </div>
        </div>

        <div className="grid grid-cols-4 gap-3">
          {[
            { zh: "平面设计", en: "Brand poster" },
            { zh: "电商运营", en: "Product shot" },
            { zh: "科研绘图", en: "Figure style" },
            { zh: "PPT制作", en: "Slide visual" },
          ].map((item) => (
            <article key={item.zh} className="rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] p-3.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
              <span className="mb-3 block h-[42px] rounded-lg bg-[linear-gradient(135deg,rgba(47,107,255,0.9),rgba(40,214,255,0.55))]" />
              <strong className="block text-sm text-white">{item.zh}</strong>
              <small className="mt-[5px] block text-xs text-[#7f8898]">{item.en}</small>
            </article>
          ))}
        </div>
      </section>

      <aside className="flex min-w-0 flex-col gap-[18px] rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] p-[18px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
        <div>
          <span className="text-xs font-medium text-[#8f98a6]">Generation settings</span>
          <strong className="mt-1.5 block text-lg text-[#f7f9fc]">图像参数</strong>
        </div>
        {[
          ["风格强度", "76%"],
          ["细节密度", "64%"],
        ].map(([label, width]) => (
          <div key={label} className="rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] p-3.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
            <label className="text-xs font-medium text-[#8f98a6]">{label}</label>
            <div className="mt-3.5 h-2 overflow-hidden rounded-full bg-white/10">
              <span className="block h-full rounded-full bg-[linear-gradient(90deg,#2f6bff,#28d6ff)]" style={{ width }} />
            </div>
          </div>
        ))}
        {["专业排版", "高清修复"].map((label) => (
          <div key={label} className="flex items-center justify-between rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] px-3.5 py-[13px] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
            <span className="text-xs font-medium text-[#8f98a6]">{label}</span>
            <span className="relative h-6 w-[42px] rounded-full bg-[linear-gradient(135deg,#2f6bff,#28d6ff)]">
              <span className="absolute right-1 top-1 block size-4 rounded-full bg-white" />
            </span>
          </div>
        ))}
        <div className="mt-auto grid grid-cols-3 gap-2.5">
          {[
            ["尺寸", "4K"],
            ["版本", "V6"],
            ["耗时", "12s"],
          ].map(([label, value]) => (
            <div key={label} className="rounded-lg border border-white/13 bg-[rgba(10,12,18,0.82)] px-2.5 py-3.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
              <span className="text-xs text-[#8f98a6]">{label}</span>
              <strong className="mt-2 block text-[22px] text-white">{value}</strong>
            </div>
          ))}
        </div>
      </aside>
    </div>
  );
}
