"use client";

import { Activity } from "lucide-react";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import { APIAccessSection } from "@/app/settings/components/api-access-section";
import { ProviderPoolSection } from "@/app/settings/components/provider-pool-section";

export default function AccountsPage() {
  return (
    <AdminPage>
      <AdminHeader
        title="上游管理"
        description="管理图片生成上游、Provider 号池和 API 接入配置。"
        icon={Activity}
      />

      <ProviderPoolSection />
      <APIAccessSection />
    </AdminPage>
  );
}
