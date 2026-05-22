"use client";

import { Sparkles, Star, Wand2 } from "lucide-react";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import { Button } from "@/components/ui/button";

const communityItems = [
  {
    title: "赛博城市海报",
    prompt: "蓝紫霓虹、雨夜街道、电影感光影、品牌主视觉",
    tone: "from-cyan-500 via-blue-700 to-fuchsia-600",
    tag: "平面设计",
  },
  {
    title: "科研封面光束",
    prompt: "粒子束、玻璃网格、论文封面、深色科技背景",
    tone: "from-indigo-500 via-sky-500 to-emerald-300",
    tag: "科研绘图",
  },
  {
    title: "产品棚拍构图",
    prompt: "柔光摄影、反射地台、高级商业材质、极简背景",
    tone: "from-zinc-200 via-stone-500 to-slate-900",
    tag: "照片摄影",
  },
  {
    title: "PPT 章节视觉",
    prompt: "抽象曲线、留白排版、企业演示、16:9 横版",
    tone: "from-violet-500 via-sky-500 to-amber-300",
    tag: "PPT制作",
  },
];

export default function CommunityPage() {
  return (
    <AdminPage>
        <AdminHeader
          title="社区"
          description="这里先作为创意广场雏形，后续可以接入公开作品、提示词模板、收藏和一键复刻。"
          actions={
            <Button type="button" className="h-11 px-5">
              <Wand2 className="size-4" />
              发布创意
            </Button>
          }
        >
          <div className="mb-3 inline-flex items-center gap-2 rounded-full bg-[var(--app-bg-surface)] px-3 py-1.5 text-xs font-semibold text-[var(--app-text-secondary)]">
            <Sparkles className="size-4 text-[var(--app-accent-cyan)]" />
            Creative community
          </div>
        </AdminHeader>

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {communityItems.map((item) => (
            <article key={item.title} className="overflow-hidden rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)]">
              <div className={`h-52 bg-gradient-to-br ${item.tone}`} />
              <div className="p-4">
                <div className="flex items-center justify-between gap-3">
                  <span className="rounded-full bg-[var(--app-bg-surface)] px-3 py-1 text-xs text-[var(--app-text-secondary)]">{item.tag}</span>
                  <Star className="size-4 text-[var(--app-text-muted)]" />
                </div>
                <h2 className="mt-4 text-base font-semibold text-[var(--app-text-primary)]">{item.title}</h2>
                <p className="mt-2 line-clamp-2 text-sm leading-6 text-[var(--app-text-muted)]">{item.prompt}</p>
              </div>
            </article>
          ))}
        </section>
    </AdminPage>
  );
}
