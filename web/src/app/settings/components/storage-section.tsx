"use client";

import { AppSelect } from "@/components/app-controls";
import type { ConfigPayload } from "@/lib/api";

import { ConfigSection, Field, TooltipDetails, type SetConfigSection } from "./shared";

type StorageSectionProps = {
  config: ConfigPayload;
  setSection: SetConfigSection;
};

export function StorageSection({ config, setSection }: StorageSectionProps) {
  const accountStorageBackend = config.storage.backend === "redis" ? "redis" : "current";
  const usesRedisAccountStorage = accountStorageBackend === "redis";
  const usesRedisConfigStorage = config.storage.configBackend === "redis";
  const shouldShowRedisFields = usesRedisAccountStorage || usesRedisConfigStorage;
  const imageConversationStorage =
    config.storage.imageConversationStorage === "server" ? "server" : "browser";
  const imageDataStorage =
    config.storage.imageDataStorage === "server" ? "server" : "browser";
  const serverConversationStorageLabel =
    accountStorageBackend === "redis" ? "Redis" : "服务器存储";
  const serverConversationStorageHint =
    accountStorageBackend === "redis"
      ? "服务器侧会话记录会写入当前 Redis。"
      : "服务器侧会话记录会写入当前服务器数据目录。";

  return (
    <ConfigSection
      title="数据存储"
      description="这里分别控制账号池、配置文件、图片会话记录和图片数据的实际落点。本地文件模式兼容旧版目录结构；服务器会话记录会自动配套使用服务器图片目录。"
    >
      <Field
        label="账号池存储后端"
        hint="控制账号池本体、认证文件、额度状态和同步状态写到本地文件还是 Redis。"
        tooltip={
          <TooltipDetails
            items={[
              {
                title: "本地文件",
                body: <>沿用当前版本的 `auths/*.json`、`accounts_state.json`、`sync_state/*.json` 目录结构。</>,
              },
              {
                title: "Redis",
                body: <>账号池相关数据写入 Redis，适合无持久磁盘云容器或多实例共享状态。</>,
              },
            ]}
          />
        }
      >
        <AppSelect
          value={accountStorageBackend}
          onChange={(value) => setSection("storage", { ...config.storage, backend: value })}
          options={[
            { value: "current", label: "本地文件" },
            { value: "redis", label: "Redis" },
          ]}
        />
      </Field>
      <Field label="配置文件存储" hint="决定配置管理页点击保存后，配置写到本地 config.toml 还是 Redis。">
        <AppSelect
          value={config.storage.configBackend === "redis" ? "redis" : "file"}
          onChange={(value) => setSection("storage", { ...config.storage, configBackend: value })}
          options={[
            { value: "file", label: "file" },
            { value: "redis", label: "redis" },
          ]}
        />
      </Field>
      <Field
        label="会话记录存储"
        hint={`切换后保存配置时会自动迁移现有图片会话记录；从${serverConversationStorageLabel}切回浏览器时，需要把历史图片下载回当前浏览器。${serverConversationStorageHint}`}
      >
        <AppSelect
          value={imageConversationStorage}
          onChange={(value) =>
            setSection("storage", {
              ...config.storage,
              imageConversationStorage: value,
              imageDataStorage: value,
              imageStorage: value,
            })
          }
          options={[
            { value: "browser", label: "浏览器存储" },
            { value: "server", label: serverConversationStorageLabel },
          ]}
        />
      </Field>
      <Field
        label="图片数据存储"
        hint={
          imageConversationStorage === "server"
            ? "当前会话记录已使用服务器侧存储，图片数据也必须写入本地/服务器目录；切换后保存配置时会自动迁移。"
            : "当前会话记录使用浏览器存储，图片数据会随会话一起保存在浏览器 local；切换后保存配置时会自动迁移。"
        }
      >
        <AppSelect
          value={imageDataStorage}
          onChange={(value) =>
            setSection("storage", {
              ...config.storage,
              imageConversationStorage: value,
              imageDataStorage: value,
              imageStorage: value,
            })
          }
          options={[
            { value: "browser", label: "浏览器 local", disabled: imageConversationStorage === "server" },
            { value: "server", label: "本地/服务器目录", disabled: imageConversationStorage !== "server" },
          ]}
        />
      </Field>
      {shouldShowRedisFields ? (
        <>
          <Field
            label="Redis 地址"
            hint={
              usesRedisAccountStorage && usesRedisConfigStorage
                ? "账号池和配置都会使用这组 Redis 连接。"
                : usesRedisAccountStorage
                  ? "仅账号池和运行状态使用这组 Redis 连接。"
                  : "仅配置文件存储使用这组 Redis 连接。"
            }
          >
            <input
              className="app-input"
              value={config.storage.redisAddr}
              onChange={(event) => setSection("storage", { ...config.storage, redisAddr: event.target.value })}
            />
          </Field>
          <Field label="Redis 密码" hint="Redis 无密码可留空。">
            <input
              className="app-input"
              type="password"
              value={config.storage.redisPassword}
              onChange={(event) => setSection("storage", { ...config.storage, redisPassword: event.target.value })}
            />
          </Field>
          <Field label="Redis DB" hint="默认 0。">
            <input
              className="app-input"
              type="number"
              value={String(config.storage.redisDb)}
              onChange={(event) =>
                setSection("storage", {
                  ...config.storage,
                  redisDb: Number(event.target.value || 0),
                })
              }
            />
          </Field>
          <Field label="Redis Key 前缀" hint="避免和其他业务共享 Redis 时键名冲突。">
            <input
              className="app-input"
              value={config.storage.redisPrefix}
              onChange={(event) => setSection("storage", { ...config.storage, redisPrefix: event.target.value })}
            />
          </Field>
        </>
      ) : null}
    </ConfigSection>
  );
}
