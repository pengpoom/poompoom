import type { ConfigPayload } from "@/lib/api";

export function joinDisplayPath(root: string, relativePath: string) {
  const normalizedRoot = String(root || "")
    .trim()
    .replace(/[\\/]+$/, "");
  const normalizedRelative = String(relativePath || "")
    .trim()
    .replace(/^[\\/]+/, "");
  if (!normalizedRoot) {
    return normalizedRelative;
  }
  if (!normalizedRelative) {
    return normalizedRoot;
  }
  const separator = normalizedRoot.includes("\\") ? "\\" : "/";
  return `${normalizedRoot}${separator}${normalizedRelative.replace(/[\\/]+/g, separator)}`;
}

export function defaultConfigPayload(): ConfigPayload {
  return {
    app: {
      name: "",
      version: "",
      apiKey: "",
      authKey: "",
      imageFormat: "url",
      maxUploadSizeMB: 50,
    },
    server: {
      host: "",
      port: 7000,
      staticDir: "",
      maxImageConcurrency: 8,
      imageQueueLimit: 32,
      imageQueueTimeoutSeconds: 20,
    },
    chatgpt: {
      model: "gpt-image-2",
      sseTimeout: 600,
      pollInterval: 3,
      pollMaxWait: 600,
      requestTimeout: 120,
    },
    accounts: {
      defaultQuota: 5,
      preferRemoteRefresh: true,
      refreshWorkers: 6,
      imageQuotaRefreshTTLSeconds: 120,
    },
    storage: {
      backend: "current",
      configBackend: "file",
      authDir: "",
      stateFile: "",
      syncStateDir: "",
      imageDir: "",
      imageStorage: "browser",
      imageConversationStorage: "browser",
      imageDataStorage: "browser",
      sqlitePath: "",
      redisAddr: "127.0.0.1:6379",
      redisPassword: "",
      redisDb: 0,
      redisPrefix: "imagestudio:studio",
    },
    sync: {
      enabled: false,
      baseUrl: "",
      managementKey: "",
      requestTimeout: 20,
      concurrency: 4,
      providerType: "codex",
    },
    proxy: {
      enabled: false,
      url: "socks5h://127.0.0.1:10808",
      mode: "fixed",
      syncEnabled: false,
    },
    apiAccess: {
      platform: "gpt-image",
      baseUrl: "",
      apiKey: "",
    },
    newapi: {
      baseUrl: "",
      username: "",
      password: "",
      accessToken: "",
      userId: 0,
      sessionCookie: "",
      requestTimeout: 20,
    },
    sub2api: {
      baseUrl: "",
      email: "",
      password: "",
      apiKey: "",
      groupId: "",
      requestTimeout: 20,
    },
    log: {
      logAllRequests: false,
    },
    paths: {
      root: "",
      defaults: "",
      override: "",
    },
  };
}

export function normalizeConfigPayload(
  payload: Partial<ConfigPayload> | null | undefined,
): ConfigPayload {
  const defaults = defaultConfigPayload();
  const next = payload ?? {};
  const chatgpt = {
    ...defaults.chatgpt,
    ...next.chatgpt,
  };
  const storage = {
    ...defaults.storage,
    ...next.storage,
  };
  const apiAccess = {
    ...defaults.apiAccess,
    ...next.apiAccess,
  };
  if (
    apiAccess.platform !== "gpt-image" &&
    apiAccess.platform !== "gemini-banana"
  ) {
    apiAccess.platform = "gpt-image";
  }
  const legacyImageStorage =
    storage.imageStorage === "server" ? "server" : "browser";
  storage.imageConversationStorage =
    storage.imageConversationStorage === "server"
      ? "server"
      : legacyImageStorage;
  storage.imageDataStorage =
    storage.imageDataStorage === "server"
      ? "server"
      : storage.imageConversationStorage;
  storage.imageStorage = storage.imageConversationStorage;

  return {
    ...defaults,
    ...next,
    app: { ...defaults.app, ...next.app },
    server: { ...defaults.server, ...next.server },
    chatgpt,
    accounts: { ...defaults.accounts, ...next.accounts },
    storage,
    sync: { ...defaults.sync, ...next.sync },
    proxy: { ...defaults.proxy, ...next.proxy },
    apiAccess,
    newapi: { ...defaults.newapi, ...next.newapi },
    sub2api: { ...defaults.sub2api, ...next.sub2api },
    log: { ...defaults.log, ...next.log },
    paths: { ...defaults.paths, ...next.paths },
  };
}
