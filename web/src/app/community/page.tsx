"use client";

import { Sparkles, Star, Wand2 } from "lucide-react";

import { AdminHeader, AdminPage } from "@/components/admin-layout";

const communityItems = [
  {
    title: "赛博城市海报",
    prompt: "蓝紫霓虹、雨夜街道、电影感光影、品牌主视觉",
    tone: "linear-gradient(135deg, #06b6d4, #2563eb 50%, #c026d3)",
    tag: "平面设计",
  },
  {
    title: "科研封面光束",
    prompt: "粒子束、玻璃网格、论文封面、深色科技背景",
    tone: "linear-gradient(135deg, #6366f1, #0ea5e9 50%, #6ee7b7)",
    tag: "科研绘图",
  },
  {
    title: "产品棚拍构图",
    prompt: "柔光摄影、反射地台、高级商业材质、极简背景",
    tone: "linear-gradient(135deg, #e4e4e7, #78716c 50%, #0f172a)",
    tag: "照片摄影",
  },
  {
    title: "PPT 章节视觉",
    prompt: "抽象曲线、留白排版、企业演示、16:9 横版",
    tone: "linear-gradient(135deg, #8b5cf6, #0ea5e9 50%, #fcd34d)",
    tag: "PPT制作",
  },
];

export default function CommunityPage() {
  return (
    <AdminPage>
      <AdminHeader
        title="社区"
        description="这里先作为创意广场雏形，后续可以接入公开作品、提示词模板、收藏和一键复刻。"
        icon={Sparkles}
        actions={
          <button className="app-btn-primary" type="button">
            <Wand2 className="size-4" />
            发布创意
          </button>
        }
      />

      <section
        style={{
          display: "grid",
          gap: 16,
          gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))",
        }}
      >
        {communityItems.map((item) => (
          <article
            key={item.title}
            className="app-panel"
            style={{ overflow: "hidden", padding: 0 }}
          >
            <div style={{ height: 208, background: item.tone }} />
            <div style={{ padding: 16 }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <span
                  style={{
                    padding: "4px 12px",
                    borderRadius: 999,
                    background: "var(--app-bg-surface)",
                    fontSize: 11.5,
                    color: "var(--app-text-secondary)",
                  }}
                >
                  {item.tag}
                </span>
                <Star className="size-4" style={{ color: "var(--app-text-muted)" }} />
              </div>
              <h2 style={{ marginTop: 14, fontSize: 15, fontWeight: 600, color: "var(--app-text-primary)" }}>{item.title}</h2>
              <p
                style={{
                  marginTop: 6,
                  fontSize: 13,
                  lineHeight: 1.6,
                  color: "var(--app-text-muted)",
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {item.prompt}
              </p>
            </div>
          </article>
        ))}
      </section>
    </AdminPage>
  );
}
