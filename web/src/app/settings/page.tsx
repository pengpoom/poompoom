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
  UserRound,
} from "lucide-react";
import { toast } from "sonner";

import {
  AdminHeader,
  AdminPage,
  AdminPanel,
  AdminToolbar,
} from "@/components/admin-layout";
import {
  adminInputClass,
  adminSubPanelClass,
} from "@/components/admin-styles";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  fetchBusinessSystemSettings,
  updateBusinessSystemSettings,
  type APIAccessPlatform,
  type BusinessSystemRuntime,
  type BusinessSystemSettings,
  type ImageQuality,
} from "@/lib/api";
import { dispatchSiteSettingsChanged } from "@/lib/site-settings";
import { cn } from "@/lib/utils";
import { isAuthErrorDuringIntentionalLogout } from "@/store/auth";

type SectionProps = {
  title: string;
  description: string;
  icon: typeof Settings2;
  children: React.ReactNode;
};

type SettingsTab = "basic" | "email";

const platformOptions: Array<{ label: string; value: APIAccessPlatform }> = [
  { label: "gpt-image", value: "gpt-image" },
  { label: "gemini-banana", value: "gemini-banana" },
];

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

const settingsInputClass = cn("h-11 rounded-[var(--app-radius-md)]", adminInputClass);
const settingsSelectClass = cn(settingsInputClass, "focus-visible:ring-0");

function defaultSystemSettings(): BusinessSystemSettings {
  return {
    site: {
      name: "ImageStudio",
      subtitle: "图片生成工作台",
      logoUrl: "",
      contactInfo: "",
    },
    user: {
      defaultRole: "user",
      defaultCredits: 20,
      registration: false,
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
    },
    runtime: {
      maxImageConcurrency: 8,
      imageQueueLimit: 32,
      imageQueueTimeoutSeconds: 20,
    },
    security: {
      imageFileAuthRequired: true,
    },
  };
}

function defaultRuntime(): BusinessSystemRuntime {
  return {
    sqlitePath: "",
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
    },
    user: {
      ...next.user,
      defaultRole: next.user.defaultRole === "admin" ? "admin" : "user",
      defaultCredits: normalizeNonNegativeInt(next.user.defaultCredits),
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
    },
    runtime: {
      ...next.runtime,
      maxImageConcurrency: normalizePositiveInt(next.runtime.maxImageConcurrency, 8, 128),
      imageQueueLimit: Math.min(10000, normalizeNonNegativeInt(next.runtime.imageQueueLimit)),
      imageQueueTimeoutSeconds: normalizePositiveInt(next.runtime.imageQueueTimeoutSeconds, 20, 3600),
    },
  };
}

function formatPath(value: string) {
  return value.trim() || "-";
}

function SettingSection({ title, description, icon: Icon, children }: SectionProps) {
  return (
    <AdminPanel className="p-5">
      <div className="mb-5 flex items-start gap-3">
        <div className="inline-flex size-10 shrink-0 items-center justify-center rounded-[var(--app-radius-md)] bg-[var(--app-bg-surface)] text-[var(--app-text-primary)]">
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <h2 className="text-base font-semibold tracking-tight text-[var(--app-text-primary)]">{title}</h2>
          <p className="mt-1 text-sm leading-6 text-[var(--app-text-muted)]">{description}</p>
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-2">{children}</div>
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
    <label className={cn("space-y-2", fullWidth && "md:col-span-2")}>
      <div className="text-sm font-medium text-[var(--app-text-secondary)]">{label}</div>
      {children}
      <div className="text-xs leading-5 text-[var(--app-text-muted)]">{hint}</div>
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
    <div className={cn("space-y-2", fullWidth && "md:col-span-2")}>
      <div className="text-sm font-medium text-[var(--app-text-secondary)]">{label}</div>
      <div className={cn(adminSubPanelClass, "min-h-11 break-all px-3 py-3 text-sm leading-5 text-[var(--app-text-secondary)]")}>
        {value}
      </div>
      <div className="text-xs leading-5 text-[var(--app-text-muted)]">{hint}</div>
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
    <div className={cn(adminSubPanelClass, "p-4 md:col-span-2")}>
      <div className="flex items-start gap-3">
        <Checkbox
          checked={checked}
          disabled={disabled}
          onCheckedChange={(value) => onCheckedChange?.(Boolean(value))}
          className="mt-0.5"
        />
        <div className="min-w-0">
          <div className="text-sm font-medium text-[var(--app-text-secondary)]">{label}</div>
          <div className="mt-1 text-xs leading-5 text-[var(--app-text-muted)]">{hint}</div>
        </div>
      </div>
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
        <div className="space-y-4">
          <AdminHeader
            title="系统设置"
            description="管理站点展示、用户默认值、点数规则和关键运行信息。"
            actions={
              <>
              <Button
                type="button"
                variant="outline"
                className="h-10 px-3 text-[13px]"
                onClick={() => void loadSettings()}
                disabled={isLoading || isSaving}
              >
                {isLoading ? <LoaderCircle className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                重新读取
              </Button>
              <Button
                type="button"
                className="h-10 px-3 text-[13px]"
                onClick={() => void saveSettings()}
                disabled={!isDirty || isLoading || isSaving}
              >
                {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
                保存设置
              </Button>
              </>
            }
          >
            <div className="mb-3 inline-flex size-10 items-center justify-center rounded-[var(--app-radius-md)] border border-white/10 bg-white/[0.045] text-[var(--app-text-primary)]">
              <Settings2 className="size-5" />
            </div>
          </AdminHeader>
          <AdminToolbar className="flex flex-wrap gap-2">
            {([
              { value: "basic", label: "基础设置", icon: Settings2 },
              { value: "email", label: "邮件设置", icon: Mail },
            ] as Array<{ value: SettingsTab; label: string; icon: typeof Settings2 }>).map((item) => {
              const Icon = item.icon;
              const active = activeTab === item.value;
              return (
                <Button
                  key={item.value}
                  type="button"
                  variant={active ? "default" : "outline"}
                  className="h-9 px-3 text-[13px]"
                  onClick={() => setActiveTab(item.value)}
                >
                  <Icon className="size-4" />
                  {item.label}
                </Button>
              );
            })}
          </AdminToolbar>
        </div>

        {isLoading ? (
          <div className="mt-16 flex items-center justify-center gap-2 text-sm text-[var(--app-text-muted)]">
            <LoaderCircle className="size-4 animate-spin" />
            正在读取系统设置
          </div>
        ) : (
          <div className="mt-5 space-y-5">
            {activeTab === "basic" ? (
              <>
              <SettingSection
              title="站点信息"
              description="这些字段用于后续前台展示和后台识别，不影响后端启动参数。"
              icon={Sparkles}
            >
              <Field label="站点名称" hint="显示在前台和后台的产品名称。">
                <Input
                  value={settings.site.name}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, name: event.target.value },
                    }))
                  }
                  className={settingsInputClass}
                />
              </Field>
              <Field label="站点副标题" hint="一句简短说明，后续可显示在登录页或导航栏。">
                <Input
                  value={settings.site.subtitle}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, subtitle: event.target.value },
                    }))
                  }
                  className={settingsInputClass}
                />
              </Field>
              <Field label="Logo URL" hint="先保存远程或本地可访问 URL，后续再做上传。">
                <Input
                  value={settings.site.logoUrl}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, logoUrl: event.target.value },
                    }))
                  }
                  placeholder="https://..."
                  className={settingsInputClass}
                />
              </Field>
              <Field label="联系方式" hint="可填写邮箱、微信或客服说明。">
                <Input
                  value={settings.site.contactInfo}
                  onChange={(event) =>
                    setSettings((current) => ({
                      ...current,
                      site: { ...current.site, contactInfo: event.target.value },
                    }))
                  }
                  placeholder="support@example.com"
                  className={settingsInputClass}
                />
              </Field>
            </SettingSection>

            <SettingSection
              title="用户默认值"
              description="影响管理员新建用户时的默认业务规则。已有用户不会被自动覆盖。"
              icon={UserRound}
            >
              <Field label="新用户默认点数" hint="管理员创建新用户后，系统会自动写入这笔初始余额。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="新用户默认角色" hint="目前管理员仍可在用户管理里手动选择角色。">
                <Select
                  value={settings.user.defaultRole}
                  onValueChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      user: { ...current.user, defaultRole: value === "admin" ? "admin" : "user" },
                    }))
                  }
                >
                  <SelectTrigger className={settingsSelectClass}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="user">普通用户</SelectItem>
                    <SelectItem value="admin">管理员</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <ToggleRow
                label="开放注册"
                hint="开启后，用户可从登录页进入邮箱验证码注册。邮件服务请在邮件设置中配置。"
                checked={settings.user.registration}
                onCheckedChange={(checked) =>
                  setSettings((current) => ({
                    ...current,
                    user: { ...current.user, registration: checked },
                  }))
                }
              />
            </SettingSection>

            <SettingSection
              title="生图与点数"
              description="控制工作台默认生成参数和不同平台的基础扣点规则。"
              icon={ImageIcon}
            >
              <Field label="默认平台" hint="后续工作台首次打开会优先使用这个平台。">
                <Select
                  value={settings.generation.defaultPlatform}
                  onValueChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        defaultPlatform: value as APIAccessPlatform,
                      },
                    }))
                  }
                >
                  <SelectTrigger className={settingsSelectClass}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {platformOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="默认质量" hint="后续工作台首次打开会优先使用这个质量。">
                <Select
                  value={settings.generation.defaultQuality}
                  onValueChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: {
                        ...current.generation,
                        defaultQuality: value as ImageQuality,
                      },
                    }))
                  }
                >
                  <SelectTrigger className={settingsSelectClass}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {qualityOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="默认尺寸" hint="保存业务默认值，具体工作台尺寸映射后续继续细化。">
                <Select
                  value={settings.generation.defaultSize}
                  onValueChange={(value) =>
                    setSettings((current) => ({
                      ...current,
                      generation: { ...current.generation, defaultSize: value },
                    }))
                  }
                >
                  <SelectTrigger className={settingsSelectClass}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {sizeOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="默认张数" hint="后续工作台首次打开会使用这个张数。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="单次最大张数" hint="后端已接入该限制，超过后会拒绝请求。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="gpt-image 每张扣点" hint="后端已按平台读取该规则扣点。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="gemini-banana 每张扣点" hint="后端已按平台读取该规则扣点。">
                <Input
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
                  className={settingsInputClass}
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
            </SettingSection>

            <SettingSection
              title="运行限制"
              description="控制新版业务生图入口的并发和排队规则，保存后立即对后端当前进程生效。"
              icon={RefreshCw}
            >
              <Field label="最大并发" hint="同时请求上游生图接口的最大数量。当前默认 8。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="最大排队" hint="并发满时允许等待的请求数量。填 0 表示不允许排队。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <Field label="排队时长（秒）" hint="请求排队超过这个时间后，会提示用户稍后使用。">
                <Input
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
                  className={settingsInputClass}
                />
              </Field>
              <div className={cn(adminSubPanelClass, "px-4 py-3 text-sm leading-6 text-[var(--app-text-secondary)]")}>
                运维监控负责看实时状态，系统设置负责改运行规则。排队满或排队超时不会扣点。
              </div>
            </SettingSection>

            <SettingSection
              title="存储与安全"
              description="这里展示当前运行中的关键路径和安全状态，部署级路径暂时不允许从前端修改。"
              icon={Database}
            >
              <ReadonlyField
                label="数据库路径"
                value={formatPath(runtime.sqlitePath)}
                hint="业务用户、历史记录、图片资产、系统设置都保存在这个 SQLite 文件里。"
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
              <div className="rounded-[var(--app-radius-md)] border border-emerald-400/20 bg-emerald-400/10 px-4 py-3 text-sm leading-6 text-emerald-100 md:col-span-2">
                <div className="flex items-center gap-2 font-medium">
                  <ShieldCheck className="size-4" />
                  当前图片鉴权已在后端强制启用
                </div>
                <div className="mt-1 text-xs leading-5 text-emerald-100/70">
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
                  <Input
                    value={settings.email.smtpHost}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, smtpHost: event.target.value },
                      }))
                    }
                    placeholder="smtp.example.com"
                    className={settingsInputClass}
                  />
                </Field>
                <Field label="SMTP 端口" hint="常见端口为 587、465 或 25。">
                  <Input
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
                    className={settingsInputClass}
                  />
                </Field>
                <Field label="SMTP 用户名" hint="多数服务商要求填写完整邮箱，也有服务商可留空。">
                  <Input
                    value={settings.email.username}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, username: event.target.value },
                      }))
                    }
                    autoComplete="off"
                    className={settingsInputClass}
                  />
                </Field>
                <Field label="SMTP 密码" hint="建议使用邮箱服务商生成的应用专用密码。">
                  <Input
                    type="password"
                    value={settings.email.password}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, password: event.target.value },
                      }))
                    }
                    autoComplete="new-password"
                    className={settingsInputClass}
                  />
                </Field>
                <Field label="发件邮箱" hint="验证码邮件显示的发件地址。">
                  <Input
                    type="email"
                    value={settings.email.from}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, from: event.target.value },
                      }))
                    }
                    placeholder="noreply@example.com"
                    className={settingsInputClass}
                  />
                </Field>
                <Field label="发件名称" hint="验证码邮件显示的发件人名称。">
                  <Input
                    value={settings.email.fromName}
                    onChange={(event) =>
                      setSettings((current) => ({
                        ...current,
                        email: { ...current.email, fromName: event.target.value },
                      }))
                    }
                    placeholder="ImageStudio"
                    className={settingsInputClass}
                  />
                </Field>
                <div className="rounded-[var(--app-radius-md)] border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm leading-6 text-amber-100 md:col-span-2">
                  开放注册需要同时满足：基础设置中开启“开放注册”，并填写 SMTP 主机和发件邮箱。保存后登录页注册入口即可发送验证码。
                </div>
              </SettingSection>
            )}
          </div>
        )}
    </AdminPage>
  );
}
