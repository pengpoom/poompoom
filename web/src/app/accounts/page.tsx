"use client";

import { useEffect, useMemo, useState } from "react";
import { Activity, LoaderCircle, RefreshCcw, RefreshCw, Save } from "lucide-react";
import { toast } from "sonner";

import { AdminHeader, AdminPage } from "@/components/admin-layout";
import { Button } from "@/components/ui/button";
import {
  fetchConfig,
  fetchDefaultConfig,
  updateConfig,
  type ConfigPayload,
} from "@/lib/api";
import {
  defaultConfigPayload,
  normalizeConfigPayload,
} from "@/app/settings/config-utils";
import { clearCachedSyncStatus } from "@/store/sync-status-cache";
import { APIAccessSection } from "@/app/settings/components/api-access-section";
import { IntegrationSection } from "@/app/settings/components/integration-section";

export default function AccountsPage() {
  const [config, setConfig] = useState<ConfigPayload>(defaultConfigPayload);
  const [defaultConfig, setDefaultConfig] = useState<ConfigPayload>(defaultConfigPayload);
  const [savedConfig, setSavedConfig] = useState<ConfigPayload | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const isDirty = useMemo(() => {
    if (!savedConfig) {
      return false;
    }
    return JSON.stringify(config) !== JSON.stringify(savedConfig);
  }, [config, savedConfig]);

  const loadConfig = async () => {
    setIsLoading(true);
    try {
      const [currentConfig, defaults] = await Promise.all([
        fetchConfig(),
        fetchDefaultConfig(),
      ]);
      const normalizedConfig = normalizeConfigPayload(currentConfig);
      setConfig(normalizedConfig);
      setSavedConfig(normalizedConfig);
      setDefaultConfig(normalizeConfigPayload(defaults));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取上游配置失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadConfig();
  }, []);

  const setSection = <K extends keyof ConfigPayload>(
    section: K,
    nextValue: ConfigPayload[K],
  ) => {
    setConfig((current) => ({
      ...current,
      [section]: nextValue,
    }));
  };

  const saveConfig = async () => {
    setIsSaving(true);
    try {
      const result = await updateConfig(normalizeConfigPayload(config));
      const normalizedConfig = normalizeConfigPayload(result.config);
      clearCachedSyncStatus();
      setConfig(normalizedConfig);
      setSavedConfig(normalizedConfig);
      toast.success("上游配置已保存并立即生效");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存上游配置失败");
    } finally {
      setIsSaving(false);
    }
  };

  const restoreDefaults = () => {
    setConfig(defaultConfig);
    toast.success("已恢复为默认配置草稿，点击“保存配置”后才会真正生效");
  };

  return (
    <AdminPage>
        <AdminHeader
          title="上游管理"
          description="管理图片生成上游、兼容服务和同步接入配置。"
          actions={
            <>
              <Button
                type="button"
                variant="outline"
                className="h-10 w-full justify-center px-3 text-[13px] sm:w-auto"
                onClick={() => void loadConfig()}
                disabled={isLoading || isSaving}
              >
                {isLoading ? (
                  <LoaderCircle className="size-4 animate-spin" />
                ) : (
                  <RefreshCw className="size-4" />
                )}
                重新读取
              </Button>
              <Button
                type="button"
                variant="outline"
                className="h-10 w-full justify-center px-3 text-[13px] sm:w-auto"
                onClick={restoreDefaults}
                disabled={isLoading || isSaving}
              >
                <RefreshCcw className="size-4" />
                恢复默认
              </Button>
              <Button
                type="button"
                className="h-10 w-full justify-center px-3 text-[13px] sm:w-auto"
                onClick={() => void saveConfig()}
                disabled={!isDirty || isLoading || isSaving}
              >
                {isSaving ? (
                  <LoaderCircle className="size-4 animate-spin" />
                ) : (
                  <Save className="size-4" />
                )}
                保存配置
              </Button>
            </>
          }
        >
          <div className="mb-3 inline-flex size-12 items-center justify-center rounded-[var(--app-radius-lg)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
            <Activity className="size-5" />
          </div>
        </AdminHeader>

        <div className="space-y-5">
          <APIAccessSection />

          <IntegrationSection config={config} setSection={setSection} />
        </div>
    </AdminPage>
  );
}
