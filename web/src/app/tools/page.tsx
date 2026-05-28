"use client";

import { FileText, Hammer, Presentation, Sparkles } from "lucide-react";

import { AdminHeader, AdminPage } from "@/components/admin-layout";

const tools = [
  { title: "Skills 工具箱", desc: "后续可接入提示词增强、批量改写、风格拆解等技能。", icon: Sparkles },
  { title: "PPT 制作", desc: "计划把生成图、结构大纲和页面视觉组合成演示文稿。", icon: Presentation },
  { title: "素材整理", desc: "面向资产库做批量命名、分组和导出。", icon: FileText },
];

export default function ToolsPage() {
  return (
    <AdminPage>
        <AdminHeader
          title="工具"
          description="先放一个工具入口框架，后续可以加入 Skills、PPT 制作、批量资产处理等能力。"
          icon={Hammer}
        />

        <section className="grid gap-4 md:grid-cols-3">
          {tools.map((tool) => {
            const Icon = tool.icon;
            return (
              <article key={tool.title} className="rounded-[var(--app-radius-lg)] border border-[var(--app-border)] bg-[var(--app-bg-elevated)] p-5">
                <div className="grid size-11 place-items-center rounded-full bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
                  <Icon className="size-5" />
                </div>
                <h2 className="mt-5 text-lg font-semibold text-[var(--app-text-primary)]">{tool.title}</h2>
                <p className="mt-2 text-sm leading-6 text-[var(--app-text-secondary)]">{tool.desc}</p>
                <button type="button" className="app-btn" style={{ marginTop: 20 }} disabled>
                  即将开放
                </button>
              </article>
            );
          })}
        </section>
    </AdminPage>
  );
}
