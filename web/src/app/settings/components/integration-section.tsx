"use client";

import { useMemo, useState } from "react";
import { Link2, LoaderCircle, ListTree } from "lucide-react";
import { toast } from "sonner";

import { AppSelect } from "@/components/app-controls";
import {
  fetchSub2APIGroups,
  testIntegration,
  type ConfigPayload,
  type IntegrationTestResult,
  type Sub2APIGroupOption,
} from "@/lib/api";

import { ConfigSection, Field, TooltipDetails, type SetConfigSection } from "./shared";

type IntegrationSectionProps = {
  config: ConfigPayload;
  setSection: SetConfigSection;
};

function buildIntegrationToastMessage(result: IntegrationTestResult) {
  const segments = [result.message];
  if (result.status > 0) {
    segments.push(`HTTP ${result.status}`);
  }
  if (result.latency >= 0) {
    segments.push(`${result.latency} ms`);
  }
  return segments.filter(Boolean).join(" / ");
}

function normalizeSub2APIGroupStatus(status: string) {
  const normalized = String(status || "").trim().toLowerCase();
  if (normalized === "active") {
    return "启用";
  }
  if (normalized === "inactive") {
    return "停用";
  }
  return String(status || "").trim();
}

function buildSub2APIGroupLabel(group: Sub2APIGroupOption) {
  const parts = [group.name || `分组 ${group.id}`];
  if (group.platform) {
    parts.push(group.platform);
  }
  const statusLabel = normalizeSub2APIGroupStatus(group.status);
  if (statusLabel) {
    parts.push(statusLabel);
  }
  parts.push(`ID ${group.id}`);
  return parts.join(" · ");
}

export function IntegrationSection({ config, setSection }: IntegrationSectionProps) {
  const [testingSource, setTestingSource] = useState<"newapi" | "sub2api" | null>(null);
  const [isLoadingSub2APIGroups, setIsLoadingSub2APIGroups] = useState(false);
  const [sub2apiGroups, setSub2apiGroups] = useState<Sub2APIGroupOption[]>([]);

  const sub2apiGroupHint = useMemo(() => {
    if (sub2apiGroups.length === 0) {
      return "先点击“拉取分组”，再从下拉框里选择目标分组。";
    }
    return `已拉取 ${sub2apiGroups.length} 个分组；可直接从下拉框选择，也可选“不限制分组”。`;
  }, [sub2apiGroups.length]);

  const sub2apiGroupOptions = useMemo(() => {
    const items = [...sub2apiGroups];
    const currentGroupId = String(config.sub2api.groupId || "").trim();
    if (currentGroupId && !items.some((group) => group.id === currentGroupId)) {
      items.unshift({
        id: currentGroupId,
        name: "当前已保存分组",
        description: "",
        platform: "",
        status: "",
      });
    }
    return items;
  }, [config.sub2api.groupId, sub2apiGroups]);

  const handleTestNewAPI = async () => {
    setTestingSource("newapi");
    try {
      const result = await testIntegration("newapi", { newapi: config.newapi });
      if (result.ok) {
        if (result.userId && result.userId > 0 && result.userId !== config.newapi.userId) {
          setSection("newapi", {
            ...config.newapi,
            userId: result.userId,
          });
        }
        toast.success(buildIntegrationToastMessage(result));
      } else {
        toast.error(buildIntegrationToastMessage(result));
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "NewAPI 测试失败");
    } finally {
      setTestingSource(null);
    }
  };

  const handleTestSub2API = async () => {
    setTestingSource("sub2api");
    try {
      const result = await testIntegration("sub2api", { sub2api: config.sub2api });
      if (result.ok) {
        toast.success(buildIntegrationToastMessage(result));
      } else {
        toast.error(buildIntegrationToastMessage(result));
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Sub2API 测试失败");
    } finally {
      setTestingSource(null);
    }
  };

  const handleLoadSub2APIGroups = async () => {
    setIsLoadingSub2APIGroups(true);
    try {
      const result = await fetchSub2APIGroups(config.sub2api);
      setSub2apiGroups(result.groups || []);
      toast.success(`${result.message} / ${result.latency} ms`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Sub2API 分组拉取失败");
    } finally {
      setIsLoadingSub2APIGroups(false);
    }
  };

  return (
    <>
      <ConfigSection
        title="NewAPI 接入"
        description="上游管理页里的 NewAPI 来源和推送会走这里。推荐顺序是先填写“地址 + 用户名 + 密码”完成测试连接，再按需补“系统访问令牌 + 用户 ID”；Session Cookie 只保留给高级兼容场景。"
        actions={
          <button
            type="button"
            className="app-btn"
            onClick={() => void handleTestNewAPI()}
            disabled={testingSource === "newapi"}
          >
            {testingSource === "newapi" ? <LoaderCircle className="size-4 animate-spin" /> : <Link2 className="size-4" />}
            测试连接
          </button>
        }
      >
        <Field
          label="NewAPI 地址"
          hint="填写你的 NewAPI 站点根地址，例如 http://127.0.0.1:3000。"
        >
          <input className="app-input"
            value={config.newapi.baseUrl}
            onChange={(event) => setSection("newapi", { ...config.newapi, baseUrl: event.target.value })}
           
          />
        </Field>

        <Field label="NewAPI 超时（秒）" hint="配置页测试连接和账号同步时使用。">
          <input className="app-input"
            type="number"
            value={String(config.newapi.requestTimeout)}
            onChange={(event) =>
              setSection("newapi", {
                ...config.newapi,
                requestTimeout: Number(event.target.value || 0),
              })
            }
           
          />
        </Field>

        <Field
          label="用户名"
          hint="对应 NewAPI 登录页里的“用户名”。页面入口：站点登录页。"
        >
          <input className="app-input"
            value={config.newapi.username}
            onChange={(event) => setSection("newapi", { ...config.newapi, username: event.target.value })}
           
          />
        </Field>

        <Field
          label="密码"
          hint="对应 NewAPI 登录页里的“密码”。页面入口：站点登录页。"
        >
          <input className="app-input"
            type="password"
            value={config.newapi.password}
            onChange={(event) => setSection("newapi", { ...config.newapi, password: event.target.value })}
           
          />
        </Field>

        <Field
          label="系统访问令牌"
          hint="对应 NewAPI 个人设置里的“系统访问令牌”。页面路径：右上角头像 -> 个人设置 -> 安全设置 -> 系统访问令牌。"
          tooltip={
            <TooltipDetails
              items={[
                {
                  title: "在哪里看",
                  body: <>在 NewAPI 页面右上角点头像，进入“个人设置”，再切到“安全设置”，就能看到“系统访问令牌”卡片。</>,
                },
                {
                  title: "注意",
                  body: <>这里建议填写你已经确认好的现成值；如果你在 NewAPI 页面里点了“重新生成”，旧令牌会失效。</>,
                },
              ]}
            />
          }
        >
          <input className="app-input"
            type="password"
            value={config.newapi.accessToken}
            onChange={(event) => setSection("newapi", { ...config.newapi, accessToken: event.target.value })}
           
          />
        </Field>

        <Field
          label="用户 ID"
          hint="测试连接成功后会自动回填；如果你已经知道自己的用户 ID，也可以直接填写。"
        >
          <input className="app-input"
            type="number"
            value={String(config.newapi.userId)}
            onChange={(event) =>
              setSection("newapi", {
                ...config.newapi,
                userId: Number(event.target.value || 0),
              })
            }
           
          />
        </Field>

        <Field
          label="Session Cookie（高级）"
          hint="高级备用项，不对应 NewAPI 页面里的常规表单。通常优先使用“用户名/密码”或“系统访问令牌/用户 ID”。"
          fullWidth
        >
          <input className="app-input"
            value={config.newapi.sessionCookie}
            onChange={(event) => setSection("newapi", { ...config.newapi, sessionCookie: event.target.value })}
           
          />
        </Field>
      </ConfigSection>

      <ConfigSection
        title="Sub2API 接入"
        description="用于 Sub2API 来源同步和推送。字段名称尽量和 Sub2API 页面里的叫法一致，同时在提示里给出对应页面路径。"
        actions={
          <>
            <button
              type="button"
              className="app-btn"
              onClick={() => void handleTestSub2API()}
              disabled={testingSource === "sub2api"}
            >
              {testingSource === "sub2api" ? <LoaderCircle className="size-4 animate-spin" /> : <Link2 className="size-4" />}
              测试连接
            </button>
            <button
              type="button"
              className="app-btn"
              onClick={() => void handleLoadSub2APIGroups()}
              disabled={isLoadingSub2APIGroups}
            >
              {isLoadingSub2APIGroups ? <LoaderCircle className="size-4 animate-spin" /> : <ListTree className="size-4" />}
              拉取分组
            </button>
          </>
        }
      >
        <Field
          label="Sub2API 地址"
          hint="填写你的 Sub2API 站点根地址，例如 http://127.0.0.1:8080。"
        >
          <input className="app-input"
            value={config.sub2api.baseUrl}
            onChange={(event) => setSection("sub2api", { ...config.sub2api, baseUrl: event.target.value })}
           
          />
        </Field>

        <Field
          label="邮箱"
          hint="对应 Sub2API 登录页里的“邮箱”。页面入口：站点登录页。"
        >
          <input className="app-input"
            value={config.sub2api.email}
            onChange={(event) => setSection("sub2api", { ...config.sub2api, email: event.target.value })}
           
          />
        </Field>

        <Field
          label="密码"
          hint="对应 Sub2API 登录页里的“密码”。页面入口：站点登录页。"
        >
          <input className="app-input"
            type="password"
            value={config.sub2api.password}
            onChange={(event) => setSection("sub2api", { ...config.sub2api, password: event.target.value })}
           
          />
        </Field>

        <Field
          label="API 密钥"
          hint="对应 Sub2API 用户侧的“API 密钥”页面。页面路径：左侧菜单 -> API 密钥。如果你使用邮箱密码登录，这里可以留空。"
        >
          <input className="app-input"
            type="password"
            value={config.sub2api.apiKey}
            onChange={(event) => setSection("sub2api", { ...config.sub2api, apiKey: event.target.value })}
           
          />
        </Field>

        <Field
          label="分组"
          hint={sub2apiGroupHint}
          tooltip={
            <TooltipDetails
              items={[
                {
                  title: "不填写",
                  body: <>表示不限制拉取分组，推送时也不强制绑定指定分组。</>,
                },
                {
                  title: "页面路径",
                  body: <>对应 Sub2API 管理后台左侧菜单里的“分组”页面。</>,
                },
                {
                  title: "选择方式",
                  body: <>点击“拉取分组”后，会按名称、平台、状态和 ID 组合展示成下拉选项，避免只看数字难分辨。</>,
                },
              ]}
            />
          }
        >
          <AppSelect
            value={config.sub2api.groupId ? config.sub2api.groupId : "__none__"}
            onChange={(value) =>
              setSection("sub2api", {
                ...config.sub2api,
                groupId: value === "__none__" ? "" : value,
              })
            }
            placeholder="先点击“拉取分组”获取可选项"
            options={[
              { value: "__none__", label: "不限制分组" },
              ...sub2apiGroupOptions.map((group) => ({ value: group.id, label: buildSub2APIGroupLabel(group) })),
            ]}
          />
        </Field>

        <Field label="Sub2API 超时（秒）" hint="配置页测试连接、拉取分组和账号同步时使用。">
          <input className="app-input"
            type="number"
            value={String(config.sub2api.requestTimeout)}
            onChange={(event) =>
              setSection("sub2api", {
                ...config.sub2api,
                requestTimeout: Number(event.target.value || 0),
              })
            }
           
          />
        </Field>
      </ConfigSection>

    </>
  );
}
