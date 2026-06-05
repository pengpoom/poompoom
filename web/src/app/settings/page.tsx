"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  Database,
  ImageIcon,
  LoaderCircle,
  Mail,
  RefreshCw,
  Save,
  Settings2,
  ShieldCheck,
  Sparkles,
  Share2,
  UserRound,
} from "lucide-react";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
} from "@/components/admin-layout";
import { AppSelect } from "@/components/app-controls";
import {
  fetchBusinessSystemSettings,
  updateBusinessSystemSettings,
  type APIAccessPlatform,
  type BusinessBillingLevel,
  type BusinessSystemRuntime,
  type BusinessSystemSettings,
  type ImageQuality,
} from "@/lib/api";
import { providerPlatformOptions } from "@/lib/provider-platforms";
import { dispatchSiteSettingsChanged } from "@/lib/site-settings";
import { cn } from "@/lib/utils";
import { isAuthErrorDuringIntentionalLogout } from "@/store/auth";
import { ImageModelsSection } from "./components/image-models-section";

type SectionProps = {
  title: string;
  description: string;
  icon: typeof Settings2;
  children: React.ReactNode;
};

type SettingsTab = "basic" | "email";

const qualityOptions: Array<{ label: string; value: ImageQuality }> = [
  { label: "Low", value: "low" },
  { label: "Medium", value: "medium" },
  { label: "High", value: "high" },
];

const sizeOptions = [
  { label: "1:1 1024 x 1024", value: "1024x1024" },
  { label: "横版 1536 x 1024", value: "1536x1024" },
  { label: "竖版 1024 x 1536", value: "1024x1536" },
];

const defaultSubscriptionLevels: BusinessBillingLevel[] = [
  { name: "Free", tag: "tier:free", description: "免费体验", enabled: true, sortOrder: 0 },
  { name: "Lumen", tag: "tier:lumen", description: "入门创作订阅", enabled: true, sortOrder: 10 },
  { name: "Prism", tag: "tier:prism", description: "标准创作订阅", enabled: true, sortOrder: 20 },
  { name: "Atelier", tag: "tier:atelier", description: "专业创作订阅", enabled: true, sortOrder: 30 },
  { name: "Meridian", tag: "tier:meridian", description: "旗舰创作订阅", enabled: true, sortOrder: 40 },
];

const defaultWalletLevels: BusinessBillingLevel[] = [
  { name: "None", tag: "wallet:none", description: "未充值", enabled: true, sortOrder: 0 },
  { name: "Ember", tag: "wallet:ember", description: "小额充值", enabled: true, sortOrder: 10 },
  { name: "Glow", tag: "wallet:glow", description: "中等充值", enabled: true, sortOrder: 20 },
  { name: "Flare", tag: "wallet:flare", description: "高价值充值", enabled: true, sortOrder: 30 },
  { name: "Radiant", tag: "wallet:radiant", description: "重度充值", enabled: true, sortOrder: 40 },
  { name: "Zenith", tag: "wallet:zenith", description: "顶级充值用户", enabled: true, sortOrder: 50 },
];


function defaultSystemSettings(): BusinessSystemSettings {
  return {
    site: {
      name: "ImageStudio",
      subtitle: "图片生成工作台",
      logoUrl: "",
      contactInfo: "",
      apiBaseUrl: "",
    },
    user: {
      defaultRole: "user",
      defaultCredits: 20,
      registration: false,
      registrationCodeRequired: false,
      turnstileEnabled: false,
      turnstileSiteKey: "",
      turnstileSecretKey: "",
      turnstileLogin: false,
      turnstileRegisterCode: true,
      turnstileRegisterSubmit: false,
      turnstilePasswordReset: true,
    },
    email: {
      smtpHost: "",
      smtpPort: 587,
      username: "",
      password: "",
      from: "",
      fromName: "ImageStudio",
    },
    generation: {
      defaultPlatform: "gpt-image",
      defaultQuality: "high",
      defaultSize: "1024x1024",
      defaultCount: 1,
      maxCount: 8,
    },
    billing: {
      gptImageCost: 1,
      geminiBananaCost: 1,
      refundOnFailure: true,
      refundPartialCount: true,
      subscriptionLevels: defaultSubscriptionLevels,
      walletLevels: defaultWalletLevels,
    },
    runtime: {
      maxImageConcurrency: 8,
      imageQueueLimit: 32,
      imageQueueTimeoutSeconds: 20,
      maxUserActiveJobs: 8,
      maxProviderRunningJobs: 4,
      maxQueuedJobs: 1000,
    },
    security: {
      imageFileAuthRequired: true,
    },
    affiliate: {
      enabled: false,
      registrationRewardEnabled: false,
      registrationRewardCredits: 0,
    },
  };
}

function defaultRuntime(): BusinessSystemRuntime {
  return {
    databaseDriver: "",
    imageDir: "",
    imageFileAuthRequired: true,
    legacyConfigWritable: false,
    businessSettingsTableName: "business_system_settings",
  };
}

function normalizePositiveInt(value: number, fallback: number, max?: number) {
  const normalized = Math.floor(Number(value) || fallback);
  const clamped = Math.max(1, normalized);
  return max ? Math.min(max, clamped) : clamped;
}

function normalizeNonNegativeInt(value: number) {
  return Math.max(0, Math.floor(Number(value) || 0));
}

function normalizeBillingLevels(levels: BusinessBillingLevel[] | undefined, defaults: BusinessBillingLevel[]) {
  const source = levels?.length ? levels : defaults;
  return source
    .map((level, index) => ({
      name: level.name?.trim() || level.tag?.trim() || "Level",
      tag: level.tag?.trim().toLowerCase() || "",
      description: level.description?.trim() || "",
      enabled: Boolean(level.enabled),
      sortOrder: Math.max(0, Math.floor(Number(level.sortOrder) || index * 10)),
    }))
    .filter((level) => level.tag);
}

function normalizeSettings(settings: BusinessSystemSettings): BusinessSystemSettings {
  const defaults = defaultSystemSettings();
  const next = {
    ...defaults,
    ...settings,
    site: { ...defaults.site, ...settings.site },
    user: { ...defaults.user, ...settings.user },
    email: { ...defaults.email, ...settings.email },
    generation: { ...defaults.generation, ...settings.generation },
    billing: { ...defaults.billing, ...settings.billing },
    runtime: { ...defaults.runtime, ...settings.runtime },
    security: { ...defaults.security, ...settings.security },
    affiliate: { ...defaults.affiliate, ...settings.affiliate },
  };
  const maxCount = normalizePositiveInt(next.generation.maxCount, 8, 8);
  return {
    ...next,
    site: {
      ...next.site,
      name: next.site.name.trim() || "ImageStudio",
      subtitle: next.site.subtitle.trim() || "图片生成工作台",
      logoUrl: next.site.logoUrl.trim(),
      contactInfo: next.site.contactInfo.trim(),
      apiBaseUrl: next.site.apiBaseUrl.trim(),
    },
    user: {
      ...next.user,
      defaultRole: next.user.defaultRole === "admin" ? "admin" : "user",
      defaultCredits: normalizeNonNegativeInt(next.user.defaultCredits),
      registrationCodeRequired: Boolean(next.user.registration) && Boolean(next.user.registrationCodeRequired),
      turnstileEnabled: Boolean(next.user.turnstileEnabled && next.user.turnstileSiteKey.trim() && next.user.turnstileSecretKey.trim()),
      turnstileSiteKey: next.user.turnstileSiteKey.trim(),
      turnstileSecretKey: next.user.turnstileSecretKey.trim(),
      turnstileLogin: Boolean(next.user.turnstileEnabled && next.user.turnstileLogin),
      turnstileRegisterCode: Boolean(next.user.turnstileEnabled && next.user.turnstileRegisterCode),
      turnstileRegisterSubmit: Boolean(next.user.turnstileEnabled && next.user.turnstileRegisterSubmit),
      turnstilePasswordReset: Boolean(next.user.turnstileEnabled && next.user.turnstilePasswordReset),
    },
    email: {
      ...next.email,
      smtpHost: next.email.smtpHost.trim(),
      smtpPort: normalizePositiveInt(next.email.smtpPort, 587, 65535),
      username: next.email.username.trim(),
      password: next.email.password.trim(),
      from: next.email.from.trim(),
      fromName: next.email.fromName.trim() || "ImageStudio",
    },
    generation: {
      ...next.generation,
      defaultCount: Math.min(
        maxCount,
        normalizePositiveInt(next.generation.defaultCount, 1, 8),
      ),
      maxCount,
    },
    billing: {
      ...next.billing,
      gptImageCost: normalizeNonNegativeInt(next.billing.gptImageCost),
      geminiBananaCost: normalizeNonNegativeInt(next.billing.geminiBananaCost),
      subscriptionLevels: normalizeBillingLevels(next.billing.subscriptionLevels, defaultSubscriptionLevels),
      walletLevels: normalizeBillingLevels(next.billing.walletLevels, defaultWalletLevels),
    },
    runtime: {
      ...next.runtime,
      maxImageConcurrency: normalizePositiveInt(next.runtime.maxImageConcurrency, 8, 128),
      imageQueueLimit: Math.min(10000, normalizeNonNegativeInt(next.runtime.imageQueueLimit)),
      imageQueueTimeoutSeconds: normalizePositiveInt(next.runtime.imageQueueTimeoutSeconds, 20, 3600),
      maxUserActiveJobs: Math.min(10000, normalizeNonNegativeInt(next.runtime.maxUserActiveJobs)),
      maxProviderRunningJobs: Math.min(10000, normalizeNonNegativeInt(next.runtime.maxProviderRunningJobs)),
      maxQueuedJobs: Math.min(1000000, normalizeNonNegativeInt(next.runtime.maxQueuedJobs)),
    },
    affiliate: {
      ...next.affiliate,
      enabled: Boolean(next.affiliate.enabled),
      registrationRewardEnabled: Boolean(next.affiliate.registrationRewardEnabled),
      registrationRewardCredits: Math.min(1000000000, normalizeNonNegativeInt(next.affiliate.registrationRewardCredits)),
    },
  };
}

function formatPath(value: string) {
  return value.trim() || "-";
}

function SettingSection({ title, children }: Omit<SectionProps, "description" | "icon"> & { description?: string; icon?: SectionProps["icon"] }) {
  return (
    <AdminPanel>
      <div className="panel-title">
        <h3>{title}</h3>
      </div>
      <div className="app-form-grid">{children}</div>
    </AdminPanel>
  );
}

function Field({
  label,
  hint,
  children,
  fullWidth = false,
}: {
  label: string;
  hint: string;
  children: React.ReactNode;
  fullWidth?: boolean;
}) {
  return (
    <label className={cn("app-fld", fullWidth && "full")}>
      <span className="fl">{label}</span>
      {children}
      <span className="fd">{hint}</span>
    </label>
  );
}

function ReadonlyField({
  label,
  value,
  hint,
  fullWidth = false,
}: {
  label: string;
  value: string;
  hint: string;
  fullWidth?: boolean;
}) {
  return (
    <div className={cn("app-fld", fullWidth && "full")}>
      <span className="fl">{label}</span>
      <div
        style={{
          minHeight: 36,
          padding: "8px 12px",
          borderRadius: "var(--app-radius-md)",
          border: "1px solid var(--app-border)",
          background: "var(--app-bg-surface)",
          fontSize: 13,
          lineHeight: 1.5,
          color: "var(--app-text-secondary)",
          wordBreak: "break-all",
        }}
      >
        {value}
      </div>
      <span className="fd">{hint}</span>
    </div>
  );
}

function BillingLevelEditor({
  title,
  levels,
  onChange,
}: {
  title: string;
  levels: BusinessBillingLevel[];
  onChange: (levels: BusinessBillingLevel[]) => void;
}) {
  const updateLevel = (index: number, patch: Partial<BusinessBillingLevel>) => {
    onChange(levels.map((level, currentIndex) => (currentIndex === index ? { ...level, ...patch } : level)));
  };
  return (
    <div className="app-fld full">
      <span className="fl">{title}</span>
      <div className="space-y-2">
        {levels.map((level, index) => (
          <div key={`${level.tag}-${index}`} className="grid gap-2 rounded-[var(--app-radius-md)] border border-[var(--app-border)] bg-[var(--app-bg-surface)] p-3 md:grid-cols-[minmax(110px,0.8fr)_minmax(150px,1fr)_minmax(180px,1.2fr)_90px_88px]">
            <input
              className="app-input"
              value={level.name}
              onChange={(event) => updateLevel(index, { name: event.target.value })}
              placeholder="名称"
            />
            <input
              className="app-input"
              value={level.tag}
              onChange={(event) => updateLevel(index, { tag: event.target.value })}
              placeholder="tier:lumen"
            />
            <input
              className="app-input"
              value={level.description}
              onChange={(event) => updateLevel(index, { description: event.target.value })}
              placeholder="说明"
            />
            <input
              className="app-input"
              type="number"
              min={0}
              value={level.sortOrder}
              onChange={(event) => updateLevel(index, { sortOrder: Math.max(0, Number(event.target.value) || 0) })}
            />
            <button
              type="button"
              className={level.enabled ? "app-btn-primary" : "app-btn"}
              onClick={() => updateLevel(index, { enabled: !level.enabled })}
            >
              {level.enabled ? "启用" : "禁用"}
            </button>
          </div>
        ))}
      </div>
      <span className="fd">展示名可改；标签会用于套餐、充值等级和后续号池调度。</span>
    </div>
  );
}

function ToggleRow({
  label,
  hint,
  checked,
  onCheckedChange,
  disabled = false,
}: {
  label: string;
  hint: string;
  checked: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="app-fld switch-row full">
      <div className="fl-wrap">
        <span className="fl">{label}</span>
        <span className="fd">{hint}</span>
      </div>
      <button
        type="button"
        className={cn("app-switch", checked && "on")}
        aria-label={label}
        disabled={disabled}
        onClick={() => onCheckedChange?.(!checked)}
      />
    </div>
  );
}

function CompactToggle({
  label,
  hint,
  checked,
  disabled = false,
  onCheckedChange,
}: {
  label: string;
  hint: string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 12,
        minHeight: 76,
        padding: "12px 14px",
        borderRadius: 10,
        border: `1px solid ${checked ? "rgba(165, 243, 252, 0.32)" : "var(--app-border)"}`,
        background: checked ? "rgba(63, 175, 255, 0.08)" : "var(--app-bg-surface)",
        opacity: disabled ? 0.5 : 1,
        cursor: disabled ? "not-allowed" : "pointer",
        textAlign: "left",
        transition: "background 0.2s ease, border-color 0.2s ease",
      }}
    >
      <span
        className={cn("app-switch", checked && "on")}
        style={{ flexShrink: 0, marginTop: 2, pointerEvents: "none" }}
        aria-hidden
      />
      <span style={{ minWidth: 0 }}>
        <span style={{ display: "block", fontSize: 13, fontWeight: 500, color: "var(--app-text-primary)" }}>{label}</span>
        <span style={{ display: "block", marginTop: 4, fontSize: 11.5, lineHeight: 1.6, color: "var(--app-text-muted)" }}>{hint}</span>
      </span>
    </button>
  );
}

function TurnstileSettingsPanel({
  settings,
  setSettings,
}: {
  settings: BusinessSystemSettings;
  setSettings: React.Dispatch<React.SetStateAction<BusinessSystemSettings>>;
}) {
  const keysReady = Boolean(settings.user.turnstileSiteKey.trim() && settings.user.turnstileSecretKey.trim());
  const enabled = Boolean(settings.user.turnstileEnabled);
  return (
    <div
      className="full"
      style={{
        gridColumn: "1 / -1",
        padding: 16,
        borderRadius: 12,
        border: "1px solid var(--app-border)",
        background: "var(--app-bg-surface)",
        display: "grid",
        gap: 14,
      }}
    >
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "flex-start", justifyContent: "space-between" }}>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--app-text-primary)" }}>人机验证</div>
          <div style={{ marginTop: 4, fontSize: 12, lineHeight: 1.6, color: "var(--app-text-muted)" }}>
            使用 Cloudflare Turnstile。默认只保护发送验证码，避免登录和注册提交重复打扰用户。
          </div>
        </div>
        <button
          type="button"
          className={cn("app-switch", enabled && "on")}
          aria-label="启用人机验证"
          disabled={!keysReady}
          onClick={() =>
            setSettings((current) => {
              const checked = !current.user.turnstileEnabled;
              return {
                ...current,
                user: {
                  ...current.user,
                  turnstileEnabled: checked,
                  turnstileLogin: checked ? current.user.turnstileLogin : false,
                  turnstileRegisterCode: checked ? current.user.turnstileRegisterCode : false,
                  turnstileRegisterSubmit: checked ? current.user.turnstileRegisterSubmit : false,
                  turnstilePasswordReset: checked ? current.user.turnstilePasswordReset : false,
                },
              };
            })
          }
        />
      </div>

      <div style={{ display: "grid", gap: 10, gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))" }}>
        <CompactToggle
          label="注册验证码"
          hint="推荐开启，防止刷邮件。"
          checked={enabled && settings.user.turnstileRegisterCode}
          disabled={!enabled}
          onCheckedChange={(checked) =>
            setSettings((current) => ({
              ...current,
              user: {
                ...current.user,
                turnstileRegisterCode: current.user.turnstileEnabled ? checked : false,
              },
            }))
          }
        />
        <CompactToggle
          label="忘记密码验证码"
          hint="推荐开启，防止刷重置邮件。"
          checked={enabled && settings.user.turnstilePasswordReset}
          disabled={!enabled}
          onCheckedChange={(checked) =>
            setSettings((current) => ({
              ...current,
              user: {
                ...current.user,
                turnstilePasswordReset: current.user.turnstileEnabled ? checked : false,
              },
            }))
          }
        />
        <CompactToggle
          label="登录"
          hint="默认关闭，后续可改成失败多次触发。"
          checked={enabled && settings.user.turnstileLogin}
          disabled={!enabled}
          onCheckedChange={(checked) =>
            setSettings((current) => ({
              ...current,
              user: {
                ...current.user,
                turnstileLogin: current.user.turnstileEnabled ? checked : false,
              },
            }))
          }
        />
        <CompactToggle
          label="注册提交"
          hint="默认关闭，邮箱验证码已兜底。"
          checked={enabled && settings.user.turnstileRegisterSubmit}
          disabled={!enabled}
          onCheckedChange={(checked) =>
            setSettings((current) => ({
              ...current,
              user: {
                ...current.user,
                turnstileRegisterSubmit: current.user.turnstileEnabled ? checked : false,
              },
            }))
          }
        />
      </div>
      {!keysReady ? (
        <div style={{ fontSize: 12, lineHeight: 1.6, color: "#fbbf24" }}>填写 Site Key 和 Secret Key 后才能启用。</div>
      ) : null}
    </div>
  );
}

export default function SettingsPage() {
  const mountedRef = useRef(false);
  const [settings, setSettings] = useState<BusinessSystemSettings>(defaultSystemSettings);
  const [savedSettings, setSavedSettings] = useState<BusinessSystemSettings | null>(null);
  const [runtime, setRuntime] = useState<BusinessSystemRuntime>(defaultRuntime);
  const [activeTab, setActiveTab] = useState<SettingsTab>("basic");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const isDirty = useMemo(() => {
    if (!savedSettings) {
      return false;
    }
    return JSON.stringify(normalizeSettings(settings)) !== JSON.stringify(normalizeSettings(savedSettings));
  }, [savedSettings, settings]);

  const loadSettings = async () => {
    setIsLoading(true);
    try {
      const result = await fetchBusinessSystemSettings();
      if (!mountedRef.current) {
        return;
      }
      const normalized = normalizeSettings(result.settings);
      setSettings(normalized);
      setSavedSettings(normalized);
      setRuntime(result.runtime);
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      if (isAuthErrorDuringIntentionalLogout(error)) {
        return;
      }
      toast.error(error instanceof Error ? error.message : "读取系统设置失败");
    } finally {
      if (mountedRef.current) {
        setIsLoading(false);
      }
    }
  };

  useEffect(() => {
    mountedRef.current = true;
    void loadSettings();
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const saveSettings = async () => {
    setIsSaving(true);
    try {
      const result = await updateBusinessSystemSettings(normalizeSettings(settings));
      if (!mountedRef.current) {
        return;
      }
      const normalized = normalizeSettings(result.settings);
      setSettings(normalized);
      setSavedSettings(normalized);
      setRuntime(result.runtime);
      dispatchSiteSettingsChanged(normalized.site);
      toast.success("系统设置已保存");
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      if (isAuthErrorDuringIntentionalLogout(error)) {
        return;
      }
      toast.error(error instanceof Error ? error.message : "保存系统设置失败");
    } finally {
      if (mountedRef.current) {
        setIsSaving(false);
      }
    }
  };

  return (
    <AdminPage>
        <AdminHeader
          title="系统设置"
          description="管理站点展示、用户默认值、点数规则和关键运行信息。"
          icon={Settings2}
          actions={
            <>
              <button className="app-btn" type="button" onClick={() => void loadSettings()} disabled={isLoading || isSaving}>
                {isLoading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                更新连接
              </button>
              <button className="app-btn-primary" type="button" onClick={() => void saveSettings()} disabled={!isDirty || isLoading || isSaving}>
                {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
                保存设置
              </button>
            </>
          }
        />

        <div className="app-tabs">
          {([
            { value: "basic" as SettingsTab, label: "基础设置", icon: Settings2 },
            { value: "email" as SettingsTab, label: "邮件设置", icon: Mail },
          ]).map((item) => {
            const Icon = item.icon;
            const active = activeTab === item.value;
            return (
              <button
                key={item.value}
                type="button"
                className={active ? "on" : undefined}
                onClick={() => setActiveTab(item.value)}
              >
                <Icon className="size-4" />
                {item.label}
              </button>
            );
          })}
        </div>

        {isLoading ? (
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, padding: "64px 0", fontSize: 13, color: "var(--app-text-muted)" }}>
            <LoaderCircle className="size-4 animate-spin" />
            正在读取系统设置
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
            {activeTab === "basic" ? (
              <>
              <SettingSection
              title="站点信息"
              description="这些字段用于后续前台展示和后台识别，不影响后端启动参数。"
              icon={Sparkles}
            >
              <Field label="站点名称" hint="显示在前台和后台的产品名称。">
                <input className="app-input"
                  value={settings.site.name}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, name: event.target.value },
                    }))
                  }
                />
              </Field>
              <Field label="站点副标题" hint="一句简短说明，后续可显示在登录页或导航栏。">
                <input className="app-input"
                  value={settings.site.subtitle}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, subtitle: event.target.value },
                    }))
                  }
                />
              </Field>
              <Field label="Logo URL" hint="先保存远程或本地可访问 URL，后续再做上传。">
                <input className="app-input"
                  value={settings.site.logoUrl}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, logoUrl: event.target.value },
                    }))
                  }
                  placeholder="https://..."
                />
              </Field>
              <Field label="联系方式" hint="可填写邮箱、微信或客服说明。">
                <input className="app-input"
                  value={settings.site.contactInfo}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, contactInfo: event.target.value },
                    }))
                  }
                  placeholder="support@example.com"
                />
              </Field>
              <Field label="对外 API 地址" hint="对外分发 API 的公开访问地址，终端用户填入客户端 base_url；留空则使用部署默认值。">
                <input className="app-input"
                  value={settings.site.apiBaseUrl}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, apiBaseUrl: event.target.value },
                    }))
                  }
                  placeholder="https://your-domain.com"
                />
              </Field>
            </SettingSection>

            <SettingSection
              title="用户默认值"
              description="影响管理员新建用户时的默认业务规则。已有用户不会被自动覆盖。"
              icon={UserRound}
            >
              <Field label="新用户默认点数" hint="管理员创建新用户后，系统会自动写入这笔初始余额。">
                <input className="app-input"
                  type="number"
                  min="0"
                  step="1"
                  value={settings.user.defaultCredits}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      user: {
                        ...current.user,
                        defaultCredits: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="新用户默认角色" hint="目前管理员仍可在用户管理里手动选择角色。">
                <AppSelect
                  value={settings.user.defaultRole}
                  onChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      user: { ...current.user, defaultRole: value === "admin" ? "admin" : "user" },
                    }))
                  }
                  options={[
                    { value: "user", label: "普通用户" },
                    { value: "admin", label: "管理员" },
                  ]}
                />
              </Field>
              <ToggleRow
                label="开放注册"
                hint="开启后，用户可从登录页进入邮箱验证码注册。邮件服务请在邮件设置中配置。"
                checked={settings.user.registration}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    user: {
                      ...current.user,
                      registration: checked,
                      registrationCodeRequired: checked ? current.user.registrationCodeRequired : false,
                    },
                  }))
                }
              />
              <ToggleRow
                label="注册需要注册码"
                hint="需要先开启开放注册。开启后，注册页必须填写有效邀请码或优惠码。"
                checked={settings.user.registration && settings.user.registrationCodeRequired}
                disabled={!settings.user.registration}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    user: {
                      ...current.user,
                      registrationCodeRequired: current.user.registration ? checked : false,
                    },
                  }))
                }
              />
              <TurnstileSettingsPanel settings={settings} setSettings={setSettings} />
              <Field label="Turnstile Site Key" hint="Cloudflare Turnstile 的公开 site key，会下发到登录页。">
                <input className="app-input"
                  value={settings.user.turnstileSiteKey}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      user: {
                        ...current.user,
                        turnstileSiteKey: event.target.value,
                      },
                    }))
                  }
                  placeholder="0x4AAAA..."
                />
              </Field>
              <Field label="Turnstile Secret Key" hint="Cloudflare Turnstile 的服务端密钥，只用于后端校验。">
                <input className="app-input"
                  type="password"
                  value={settings.user.turnstileSecretKey}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      user: {
                        ...current.user,
                        turnstileSecretKey: event.target.value,
                      },
                    }))
                  }
                  placeholder="0x4AAAA..."
                  autoComplete="new-password"
                />
              </Field>
            </SettingSection>

            <SettingSection
              title="邀请返利"
              description="控制用户邀请码入口，以及邀请注册成功后的固定奖励。"
              icon={Share2}
            >
              <ToggleRow
                label="开启邀请入口"
                hint="开启后，用户积分中心会展示邀请码、邀请链接和邀请人数。"
                checked={settings.affiliate.enabled}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    affiliate: {
                      ...current.affiliate,
                      enabled: checked,
                      registrationRewardEnabled: checked ? current.affiliate.registrationRewardEnabled : false,
                    },
                  }))
                }
              />
              <ToggleRow
                label="邀请注册奖励"
                hint="开启后，被邀请用户注册成功并绑定关系时，系统会给邀请人增加固定点数。"
                checked={settings.affiliate.enabled && settings.affiliate.registrationRewardEnabled}
                disabled={!settings.affiliate.enabled}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    affiliate: {
                      ...current.affiliate,
                      registrationRewardEnabled: current.affiliate.enabled ? checked : false,
                    },
                  }))
                }
              />
              <Field label="邀请注册奖励点数" hint="每成功邀请一个新用户注册，给邀请人增加的点数。填 0 表示不奖励。">
                <input className="app-input"
                  type="number"
                  min="0"
                  step="1"
                  value={settings.affiliate.registrationRewardCredits}
                  disabled={!settings.affiliate.enabled || !settings.affiliate.registrationRewardEnabled}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      affiliate: {
                        ...current.affiliate,
                        registrationRewardCredits: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
            </SettingSection>

            <SettingSection
              title="生图默认值"
              description="控制工作台默认生成参数。模型扣点以“模型目录”为准，平台扣点仅作为旧请求兜底。"
              icon={ImageIcon}
            >
              <Field label="默认平台" hint="后续工作台首次打开会优先使用这个平台。">
                <AppSelect
                  value={settings.generation.defaultPlatform}
                  onChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        defaultPlatform: value as APIAccessPlatform,
                      },
                    }))
                  }
                  options={providerPlatformOptions}
                />
              </Field>
              <Field label="默认质量" hint="后续工作台首次打开会优先使用这个质量。">
                <AppSelect
                  value={settings.generation.defaultQuality}
                  onChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        defaultQuality: value as ImageQuality,
                      },
                    }))
                  }
                  options={qualityOptions}
                />
              </Field>
              <Field label="默认尺寸" hint="保存业务默认值，具体工作台尺寸映射后续继续细化。">
                <AppSelect
                  value={settings.generation.defaultSize}
                  onChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: { ...current.generation, defaultSize: value },
                    }))
                  }
                  options={sizeOptions}
                />
              </Field>
              <Field label="默认张数" hint="后续工作台首次打开会使用这个张数。">
                <input className="app-input"
                  type="number"
                  min="1"
                  max="8"
                  step="1"
                  value={settings.generation.defaultCount}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        defaultCount: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="单次最大张数" hint="后端已接入该限制，超过后会拒绝请求。">
                <input className="app-input"
                  type="number"
                  min="1"
                  max="8"
                  step="1"
                  value={settings.generation.maxCount}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        maxCount: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="OpenAI 兜底每张扣点" hint="仅在请求没有匹配到模型目录时使用。正常生图按模型目录扣点。">
                <input className="app-input"
                  type="number"
                  min="0"
                  step="1"
                  value={settings.billing.gptImageCost}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      billing: {
                        ...current.billing,
                        gptImageCost: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="Google 兜底每张扣点" hint="仅在请求没有匹配到模型目录时使用。正常生图按模型目录扣点。">
                <input className="app-input"
                  type="number"
                  min="0"
                  step="1"
                  value={settings.billing.geminiBananaCost}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      billing: {
                        ...current.billing,
                        geminiBananaCost: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <ToggleRow
                label="生成失败自动返还点数"
                hint="上游报错、超时或无图时，后端会按本次预扣金额返还。"
                checked={settings.billing.refundOnFailure}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    billing: { ...current.billing, refundOnFailure: checked },
                  }))
                }
              />
              <ToggleRow
                label="少出图时按实际张数结算"
                hint="请求 4 张但只返回 2 张时，后端会返还缺少图片对应的点数。"
                checked={settings.billing.refundPartialCount}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    billing: { ...current.billing, refundPartialCount: checked },
                  }))
                }
              />
              <BillingLevelEditor
                title="订阅等级"
                levels={settings.billing.subscriptionLevels}
                onChange={(levels) =>
                  setSettings((current) => ({
                    ...current,
                    billing: { ...current.billing, subscriptionLevels: levels },
                  }))
                }
              />
              <BillingLevelEditor
                title="充值等级"
                levels={settings.billing.walletLevels}
                onChange={(levels) =>
                  setSettings((current) => ({
                    ...current,
                    billing: { ...current.billing, walletLevels: levels },
                  }))
                }
              />
            </SettingSection>

            <ImageModelsSection />

            <SettingSection
              title="运行限制"
              description="控制新版业务生图入口的并发和排队规则，保存后立即对后端当前进程生效。"
              icon={RefreshCw}
            >
              <Field label="最大并发" hint="同时请求上游生图接口的最大数量。当前默认 8。">
                <input className="app-input"
                  type="number"
                  min="1"
                  max="128"
                  step="1"
                  value={settings.runtime.maxImageConcurrency}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        maxImageConcurrency: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="最大排队" hint="并发满时允许等待的请求数量。填 0 表示不允许排队。">
                <input className="app-input"
                  type="number"
                  min="0"
                  max="10000"
                  step="1"
                  value={settings.runtime.imageQueueLimit}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        imageQueueLimit: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="排队时长（秒）" hint="请求排队超过这个时间后，会提示用户稍后使用。">
                <input className="app-input"
                  type="number"
                  min="1"
                  max="3600"
                  step="1"
                  value={settings.runtime.imageQueueTimeoutSeconds}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        imageQueueTimeoutSeconds: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="单用户活跃任务" hint="同一用户 queued/running job 的上限。填 0 表示不限制。">
                <input className="app-input"
                  type="number"
                  min="0"
                  max="10000"
                  step="1"
                  value={settings.runtime.maxUserActiveJobs}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        maxUserActiveJobs: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="单接入运行任务" hint="同一 provider 同时 running job 的上限。填 0 表示不限制。">
                <input className="app-input"
                  type="number"
                  min="0"
                  max="10000"
                  step="1"
                  value={settings.runtime.maxProviderRunningJobs}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        maxProviderRunningJobs: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <Field label="全站数据库排队" hint="PostgreSQL job 表 queued 状态总上限。填 0 表示不限制。">
                <input className="app-input"
                  type="number"
                  min="0"
                  max="1000000"
                  step="1"
                  value={settings.runtime.maxQueuedJobs}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      runtime: {
                        ...current.runtime,
                        maxQueuedJobs: Number(event.target.value),
                      },
                    }))
                  }
                />
              </Field>
              <div
                className="full"
                style={{
                  gridColumn: "1 / -1",
                  padding: "12px 16px",
                  borderRadius: 10,
                  border: "1px solid var(--app-border)",
                  background: "var(--app-bg-surface)",
                  fontSize: 13,
                  lineHeight: 1.6,
                  color: "var(--app-text-secondary)",
                }}
              >
                运维监控负责看实时状态，系统设置负责改运行规则。排队满、排队超时或容量超限不会扣点。
              </div>
            </SettingSection>

            <SettingSection
              title="存储与安全"
              description="这里展示当前运行中的关键路径和安全状态，部署级路径暂时不允许从前端修改。"
              icon={Database}
            >
              <ReadonlyField
                label="数据库"
                value={runtime.databaseDriver || "-"}
                hint="业务用户、历史记录、图片资产、系统设置等结构化数据保存在当前数据库。"
                fullWidth
              />
              <ReadonlyField
                label="图片目录"
                value={formatPath(runtime.imageDir)}
                hint="持久化图片文件保存位置。正式部署时建议迁移到对象存储。"
                fullWidth
              />
              <ReadonlyField
                label="系统设置表"
                value={runtime.businessSettingsTableName}
                hint="当前系统设置使用独立业务表，不再写旧 config.toml。"
              />
              <ReadonlyField
                label="旧配置页写入"
                value={runtime.legacyConfigWritable ? "允许" : "关闭"}
                hint="旧运行配置不再从前端修改，避免误改端口、数据库等部署参数。"
              />
              <ToggleRow
                label="图片文件访问鉴权"
                hint="业务图片 URL 需要登录态或 Authorization，管理员可访问全部图片，普通用户只能访问自己的图片。"
                checked={runtime.imageFileAuthRequired && settings.security.imageFileAuthRequired}
                disabled
              />
              <div
                className="full"
                style={{
                  gridColumn: "1 / -1",
                  padding: "12px 16px",
                  borderRadius: 10,
                  border: "1px solid rgba(52, 211, 153, 0.32)",
                  background: "rgba(52, 211, 153, 0.10)",
                  color: "#a7f3d0",
                  fontSize: 13,
                  lineHeight: 1.6,
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8, fontWeight: 500 }}>
                  <ShieldCheck className="size-4" />
                  当前图片鉴权已在后端强制启用
                </div>
                <div style={{ marginTop: 4, fontSize: 11.5, lineHeight: 1.6, color: "rgba(167, 243, 208, 0.75)" }}>
                  这个状态暂时不做关闭开关。后续如果接入对象存储，可以改成签名 URL 或 CDN 私有访问。
                </div>
              </div>
            </SettingSection>
              </>
            ) : (
              <SettingSection
                title="邮件设置"
                description="用于邮箱验证码注册。SMTP 密码会保存在服务器数据库，不会写入仓库。"
                icon={Mail}
              >
                <Field label="SMTP 主机" hint="例如 smtp.gmail.com、smtp.qq.com。">
                  <input className="app-input"
                    value={settings.email.smtpHost}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, smtpHost: event.target.value },
                      }))
                    }
                    placeholder="smtp.example.com"
                  />
                </Field>
                <Field label="SMTP 端口" hint="常见端口为 587、465 或 25。">
                  <input className="app-input"
                    type="number"
                    min="1"
                    max="65535"
                    step="1"
                    value={settings.email.smtpPort}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, smtpPort: Number(event.target.value) },
                      }))
                    }
                  />
                </Field>
                <Field label="SMTP 用户名" hint="多数服务商要求填写完整邮箱，也有服务商可留空。">
                  <input className="app-input"
                    value={settings.email.username}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, username: event.target.value },
                      }))
                    }
                    autoComplete="off"
                  />
                </Field>
                <Field label="SMTP 密码" hint="建议使用邮箱服务商生成的应用专用密码。">
                  <input className="app-input"
                    type="password"
                    value={settings.email.password}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, password: event.target.value },
                      }))
                    }
                    autoComplete="new-password"
                  />
                </Field>
                <Field label="发件邮箱" hint="验证码邮件显示的发件地址。">
                  <input className="app-input"
                    type="email"
                    value={settings.email.from}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, from: event.target.value },
                      }))
                    }
                    placeholder="noreply@example.com"
                  />
                </Field>
                <Field label="发件名称" hint="验证码邮件显示的发件人名称。">
                  <input className="app-input"
                    value={settings.email.fromName}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, fromName: event.target.value },
                      }))
                    }
                    placeholder="ImageStudio"
                  />
                </Field>
                <div
                  className="full"
                  style={{
                    gridColumn: "1 / -1",
                    padding: "12px 16px",
                    borderRadius: 10,
                    border: "1px solid rgba(251, 191, 36, 0.32)",
                    background: "rgba(251, 191, 36, 0.10)",
                    color: "#fcd34d",
                    fontSize: 13,
                    lineHeight: 1.6,
                  }}
                >
                  开放注册需要同时满足：基础设置中开启“开放注册”，并填写 SMTP 主机和发件邮箱。保存后登录页注册入口即可发送验证码。
                </div>
              </SettingSection>
            )}
          </div>
        )}
    </AdminPage>
  );
}
