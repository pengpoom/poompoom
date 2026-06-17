import { ApiError, httpRequest } from "@/lib/request";
import webConfig from "@/constants/common-env";
import type { AuthRole } from "@/store/auth";
import {
  buildImageAccountPolicyHeader,
  normalizeImageAccountPolicy,
  type StoredImageAccountPolicy,
} from "@/store/image-account-policy";

export type AccountType = "Free" | "Plus" | "Pro" | "Team";
export type AccountStatus = "正常" | "限流" | "异常" | "禁用";
export type SyncStatus =
  | "synced"
  | "pending_upload"
  | "remote_only"
  | "remote_deleted";
export type SyncSource = "cpa" | "newapi" | "sub2api";
export type AccountSourceKind = "auth_file" | "token";
export type ImageModel = string;
export type ImageModelId = string;
export type ImageQuality = "low" | "medium" | "high";
export type ImageResolutionAccess = "free" | "paid";
export type APIAccessPlatform =
  | "gpt-image"
  | "gemini-banana"
  | "doubao"
  | "qwen"
  | "baidu"
  | "z-ai"
  | "tencent"
  | "kling"
  | "grok";
export type ImageModelCapabilities = {
  generate: boolean;
  edit: boolean;
  referenceImage: boolean;
  mask: boolean;
  sizes?: string[];
  qualities?: ImageQuality[];
  maxImages: number;
  maxReferenceImages: number;
};
export type BusinessImageModel = {
  id: ImageModelId;
  vendor: string;
  vendorLabel: string;
  displayName: string;
  adapter: string;
  platform: APIAccessPlatform | string;
  upstreamModel: ImageModel;
  enabled: boolean;
  preview?: boolean;
  compareEnabled: boolean;
  capabilities: ImageModelCapabilities;
  creditCost: number;
  isDefault?: boolean;
  sortOrder: number;
  availability?: BusinessImageModelAvailability;
  createdAt?: string;
  updatedAt?: string;
};
export type BusinessImageModelAvailability = {
  status: string;
  available: boolean;
  message: string;
  issues?: string[];
  apiProviderAvailable: boolean;
  apiProviderName?: string;
  apiProviderDefaultModel?: string;
  apiProviderModelMismatch?: boolean;
  poolAvailable: boolean;
  poolName?: string;
  poolMemberId?: string;
  poolMemberName?: string;
  poolMemberDefaultModel?: string;
  poolMemberModelMismatch?: boolean;
};
export type BusinessImageModelInput = {
  id?: ImageModelId;
  vendor: string;
  vendorLabel: string;
  displayName: string;
  adapter: string;
  platform: APIAccessPlatform;
  upstreamModel: ImageModel;
  enabled: boolean;
  preview: boolean;
  compareEnabled: boolean;
  capabilities: ImageModelCapabilities;
  creditCost: number;
  isDefault: boolean;
  sortOrder: number;
};
export type BusinessAPIProvider = {
  id: string;
  name: string;
  platform: APIAccessPlatform;
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
  enabled: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
};
export type BusinessAPIProviderInput = {
  name: string;
  platform: APIAccessPlatform;
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
  enabled: boolean;
  isDefault: boolean;
};
export type BusinessProviderMemberStatus = "active" | "limited" | "unavailable";
export type BusinessProviderGroup = {
  id: string;
  name: string;
  platform: APIAccessPlatform;
  description: string;
  tags: string;
  matchMode: "fallback" | "any" | "all";
  enabled: boolean;
  isDefault: boolean;
  priority: number;
  createdAt: string;
  updatedAt: string;
};
export type BusinessProviderGroupInput = {
  name: string;
  platform: APIAccessPlatform;
  description: string;
  tags: string;
  matchMode: "fallback" | "any" | "all";
  enabled: boolean;
  isDefault: boolean;
  priority: number;
};
export type BusinessProviderMember = {
  id: string;
  groupId: string;
  name: string;
  platform: APIAccessPlatform;
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
  enabled: boolean;
  priority: number;
  weight: number;
  maxConcurrent: number;
  cooldownSeconds: number;
  failureThreshold: number;
  consecutiveFailures: number;
  status: BusinessProviderMemberStatus;
  cooldownUntil: string;
  successCount: number;
  failCount: number;
  lastUsedAt: string;
  lastError: string;
  lastErrorAt: string;
  createdAt: string;
  updatedAt: string;
};
export type BusinessProviderMemberInput = {
  groupId: string;
  name: string;
  platform: APIAccessPlatform;
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
  enabled: boolean;
  priority: number;
  weight: number;
  maxConcurrent: number;
  cooldownSeconds: number;
  failureThreshold: number;
  status: BusinessProviderMemberStatus;
};
export type BusinessProviderPool = BusinessProviderGroup & {
  members: BusinessProviderMember[];
};
export type BusinessProviderDispatchPreviewInput = {
  platform: APIAccessPlatform;
  role: string;
  subscriptionTag: string;
  walletTag: string;
  mode: string;
  quality: string;
  size: string;
  model: string;
  extraTags?: string[];
};
export type BusinessProviderDispatchPreviewResponse = {
  ok: boolean;
  message: string;
  platform: APIAccessPlatform | string;
  requestTags: string[];
  userTags: string[];
  dispatchTags: string[];
  strategy?: string;
  group?: BusinessProviderGroup;
  member?: BusinessProviderMember;
  fallbackAvailable?: boolean;
  fallbackSource?: string;
  fallbackName?: string;
  trace?: string[];
  pools?: BusinessProviderDispatchPreviewPool[];
  issues?: BusinessProviderDispatchPreviewIssue[];
};
export type BusinessProviderDispatchPreviewPool = {
  id: string;
  name: string;
  platform: APIAccessPlatform | string;
  enabled: boolean;
  isDefault: boolean;
  priority: number;
  matchMode: "fallback" | "any" | "all" | string;
  tags?: string[];
  role: "tagged" | "fallback" | "ignored" | string;
  matched: boolean;
  considered: boolean;
  reasonCode: string;
  reason: string;
  memberTotal: number;
  availableMembers: number;
  disabledMembers: number;
  unavailableMembers: number;
  limitedMembers: number;
  coolingMembers: number;
  concurrencyFullMembers: number;
  platformMismatchMembers: number;
  members?: BusinessProviderDispatchPreviewMember[];
};
export type BusinessProviderDispatchPreviewMember = {
  id: string;
  name: string;
  enabled: boolean;
  status: BusinessProviderMemberStatus | string;
  priority: number;
  weight: number;
  maxConcurrent: number;
  running: number;
  cooldownUntil?: string;
  available: boolean;
  reasonCode: string;
  reason: string;
  lastError?: string;
  lastErrorAt?: string;
};
export type BusinessProviderDispatchPreviewIssue = {
  code: string;
  label: string;
  count: number;
  detail?: string;
};
export type BusinessNotificationLevel = "info" | "warning" | "success";
export type BusinessNotificationStatus = "draft" | "published" | "archived";
export type BusinessNotificationNotifyMode = "silent" | "popup";
export type BusinessNotificationTargeting = {
  mode: "all" | "balance";
  balance?: {
    operator: ">" | ">=" | "<" | "<=" | "=";
    value: number;
  };
};
export type BusinessNotification = {
  id: string;
  title: string;
  body: string;
  level: BusinessNotificationLevel;
  audience: "all";
  status: BusinessNotificationStatus;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string;
  readAt?: string;
  readCount?: number;
  audienceCount?: number;
  notifyMode: BusinessNotificationNotifyMode;
  startsAt?: string;
  endsAt?: string;
  targeting?: BusinessNotificationTargeting;
};
export type BusinessNotificationListResponse = {
  items: BusinessNotification[];
  unreadCount?: number;
};
export type BusinessNotificationInput = {
  title: string;
  body: string;
  level: BusinessNotificationLevel;
  publish?: boolean;
  status?: BusinessNotificationStatus;
  audience?: "all";
  notifyMode?: BusinessNotificationNotifyMode;
  startsAt?: string;
  endsAt?: string;
  targeting?: BusinessNotificationTargeting;
};
export type BusinessSystemSettings = {
  site: {
    name: string;
    subtitle: string;
    logoUrl: string;
    contactInfo: string;
    apiBaseUrl: string;
  };
  user: {
    defaultRole: AuthRole;
    defaultCredits: number;
    registration: boolean;
    registrationCodeRequired: boolean;
    turnstileEnabled: boolean;
    turnstileSiteKey: string;
    turnstileSecretKey: string;
    turnstileLogin: boolean;
    turnstileRegisterCode: boolean;
    turnstileRegisterSubmit: boolean;
    turnstilePasswordReset: boolean;
  };
  email: {
    smtpHost: string;
    smtpPort: number;
    username: string;
    password: string;
    from: string;
    fromName: string;
  };
  generation: {
    defaultPlatform: APIAccessPlatform;
    defaultQuality: ImageQuality;
    defaultSize: string;
    defaultCount: number;
    maxCount: number;
  };
  billing: {
    gptImageCost: number;
    geminiBananaCost: number;
    refundOnFailure: boolean;
    refundPartialCount: boolean;
    subscriptionLevels: BusinessBillingLevel[];
    walletLevels: BusinessBillingLevel[];
  };
  runtime: {
    maxImageConcurrency: number;
    imageQueueLimit: number;
    imageQueueTimeoutSeconds: number;
    maxUserActiveJobs: number;
    maxProviderRunningJobs: number;
    maxQueuedJobs: number;
  };
  security: {
    imageFileAuthRequired: boolean;
  };
  affiliate: {
    enabled: boolean;
    registrationRewardEnabled: boolean;
    registrationRewardCredits: number;
  };
};
export type BusinessBillingLevel = {
  name: string;
  tag: string;
  description: string;
  enabled: boolean;
  sortOrder: number;
};
export type BusinessSystemRuntime = {
  databaseDriver: string;
  imageDir: string;
  imageFileAuthRequired: boolean;
  legacyConfigWritable: boolean;
  businessSettingsTableName: string;
};
export type BusinessSystemSettingsResponse = {
  settings: BusinessSystemSettings;
  runtime: BusinessSystemRuntime;
};
export type PublicSiteSettings = {
  site: Pick<BusinessSystemSettings["site"], "name" | "subtitle" | "logoUrl">;
  turnstile?: {
    enabled: boolean;
    siteKey: string;
    login?: boolean;
    registerCode?: boolean;
    registerSubmit?: boolean;
    passwordReset?: boolean;
  };
};
export type ImageResponseItem = {
  url?: string;
  b64_json?: string;
  revised_prompt?: string;
  file_id?: string;
  gen_id?: string;
  conversation_id?: string;
  parent_message_id?: string;
  source_account_id?: string;
  error?: string;
};

export type InpaintSourceReference = {
  original_file_id: string;
  original_gen_id: string;
  conversation_id?: string;
  parent_message_id?: string;
  source_account_id: string;
};

export type ImageSourcePayload = {
  id: string;
  role: "image" | "mask";
  name: string;
  dataUrl?: string;
  url?: string;
};

export type Account = {
  id: string;
  fileName: string;
  access_token: string;
  sourceKind?: AccountSourceKind | null;
  type: AccountType;
  status: AccountStatus;
  quota: number;
  email?: string | null;
  user_id?: string | null;
  limits_progress?: Array<{
    feature_name?: string;
    remaining?: number;
    reset_after?: string;
  }>;
  default_model_slug?: string | null;
  restoreAt?: string | null;
  success: number;
  fail: number;
  lastUsedAt: string | null;
  provider?: string;
  disabled?: boolean;
  note?: string | null;
  priority?: number;
  syncStatus?: SyncStatus | null;
  syncOrigin?: string | null;
  lastSyncedAt?: string | null;
  remoteDisabled?: boolean | null;
  importedAt?: string | null;
};

export type SyncAccount = {
  name: string;
  status: SyncStatus;
  location: "local" | "remote" | "both";
  localDisabled?: boolean | null;
  remoteDisabled?: boolean | null;
};

export type SyncRunResult = {
  ok: boolean;
  running?: boolean;
  source: SyncSource;
  error?: string;
  direction?: string;
  imported: number;
  exported: number;
  skipped: number;
  failed: number;
  inaccessible: number;
  total?: number;
  processed?: number;
  phase?: string;
  current?: string;
  notes?: string[];
  started_at: string;
  finished_at: string;
  updated_at?: string;
};

export type SyncStatusResponse = {
  source: SyncSource;
  label: string;
  configured: boolean;
  pullSupported: boolean;
  pushSupported: boolean;
  local: number;
  remote: number;
  pendingPush: number;
  pendingPull: number;
  inaccessibleRemote: number;
  notes?: string[];
  lastRun?: SyncRunResult | null;
};

type AccountListResponse = {
  items: Account[];
};

type AccountMutationResponse = {
  items: Account[];
  added?: number;
  skipped?: number;
  removed?: number;
  refreshed?: number;
  errors?: Array<{ access_token: string; error: string }>;
};

export type AccountImportResponse = {
  items: Account[];
  imported?: number;
  imported_files?: number;
  refreshed?: number;
  errors?: Array<{ access_token: string; error: string }>;
  duplicates?: Array<{ name: string; reason: string }>;
  failed?: Array<{ name: string; error: string }>;
};

type AccountRefreshResponse = {
  items: Account[];
  refreshed: number;
  errors: Array<{ access_token: string; error: string }>;
};

export type AccountRefreshProgress = {
  ok: boolean;
  running: boolean;
  error?: string;
  total: number;
  processed: number;
  refreshed: number;
  failed: number;
  current?: string;
  started_at: string;
  finished_at: string;
  updated_at?: string;
};

type AccountRefreshAllResponse = {
  progress: AccountRefreshProgress | null;
  alreadyRunning?: boolean;
};

type AccountUpdateResponse = {
  item: Account;
  items: Account[];
};

export type AccountQuotaResponse = {
  id: string;
  email?: string | null;
  status: AccountStatus;
  type: AccountType;
  quota: number;
  image_gen_remaining?: number | null;
  image_gen_reset_after?: string | null;
  refresh_requested: boolean;
  refreshed: boolean;
  refresh_error?: string;
};

export type ImageMode = "studio" | "cpa";

type ImageResponse = {
  created: number;
  data: ImageResponseItem[];
  job?: BusinessImageJob;
  jobId?: string;
  conversationId?: string;
  turnId?: string;
  status?: string;
};

export type ConfigPayload = {
  app: {
    name: string;
    version: string;
    apiKey: string;
    authKey: string;
    imageFormat: string;
    maxUploadSizeMB: number;
  };
  server: {
    host: string;
    port: number;
    staticDir: string;
    maxImageConcurrency: number;
    imageQueueLimit: number;
    imageQueueTimeoutSeconds: number;
  };
  chatgpt: {
    model: string;
    sseTimeout: number;
    pollInterval: number;
    pollMaxWait: number;
    requestTimeout: number;
  };
  accounts: {
    defaultQuota: number;
    preferRemoteRefresh: boolean;
    refreshWorkers: number;
    imageQuotaRefreshTTLSeconds: number;
  };
  storage: {
    backend: string;
    configBackend: "file" | "redis" | string;
    authDir: string;
    stateFile: string;
    syncStateDir: string;
    imageDir: string;
    imageStorage: "browser" | "server" | string;
    imageConversationStorage: "browser" | "server" | string;
    imageDataStorage: "browser" | "server" | string;
    redisAddr: string;
    redisPassword: string;
    redisDb: number;
    redisPrefix: string;
  };
  sync: {
    enabled: boolean;
    baseUrl: string;
    managementKey: string;
    requestTimeout: number;
    concurrency: number;
    providerType: string;
  };
  proxy: {
    enabled: boolean;
    url: string;
    mode: string;
    syncEnabled: boolean;
  };
  apiAccess: {
    platform: APIAccessPlatform;
    baseUrl: string;
    apiKey: string;
  };
  newapi: {
    baseUrl: string;
    username: string;
    password: string;
    accessToken: string;
    userId: number;
    sessionCookie: string;
    requestTimeout: number;
  };
  sub2api: {
    baseUrl: string;
    email: string;
    password: string;
    apiKey: string;
    groupId: string;
    requestTimeout: number;
  };
  log: {
    logAllRequests: boolean;
  };
  paths: {
    root: string;
    defaults: string;
    override: string;
  };
};

export type RequestLogItem = {
  id: string;
  startedAt: string;
  finishedAt: string;
  endpoint: string;
  operation: string;
  imageMode: ImageMode | string;
  direction: "official" | "cpa" | string;
  route: string;
  cpaSubroute?: "images_api" | "codex_responses" | "auto" | string;
  queueWaitMs?: number;
  inflightCountAtStart?: number;
  leaseAcquired?: boolean;
  errorCode?: string;
  routingPolicyApplied?: boolean;
  routingGroupIndex?: number;
  routingSortMode?: string;
  routingReservePercent?: number;
  accountType?: string;
  accountEmail?: string;
  accountFile?: string;
  requestedModel?: string;
  upstreamModel?: string;
  imageToolModel?: string;
  size?: string;
  quality?: string;
  promptLength?: number;
  preferred: boolean;
  success: boolean;
  error?: string;
};

export type VersionInfo = {
  version: string;
  current_version?: string;
  latest_version?: string;
  has_update?: boolean;
  cached?: boolean;
  warning?: string;
  release_info?: {
    name: string;
    body?: string;
    published_at?: string;
    html_url: string;
  };
  commit?: string;
  buildTime?: string;
};

export type SystemUpdateJob = {
  id?: string;
  status: "idle" | "running" | "succeeded" | "failed" | string;
  phase?: string;
  error?: string;
  logs?: string[];
  started_at?: string;
  finished_at?: string;
  updated_at?: string;
};

export type SystemUpdateStatus = {
  enabled: boolean;
  reason?: string;
  deploy_dir?: string;
  compose_file?: string;
  compose_project?: string;
  service?: string;
  job?: SystemUpdateJob | null;
};

export type SystemUpdateStartResponse = {
  started: boolean;
  job?: SystemUpdateJob | null;
  status?: SystemUpdateStatus | null;
};

export type MaintenanceStatus = {
  enabled: boolean;
  message: string;
  updatedAt?: string;
  updatedBy?: string;
};

export type StartupCheckItem = {
  key: string;
  label: string;
  status: "pass" | "warn" | "fail" | string;
  detail: string;
  hint?: string;
  durationMs: number;
};

export type StartupCheckResponse = {
  startedAt: string;
  finishedAt: string;
  overall: "pass" | "warn" | "fail" | string;
  passCount: number;
  warnCount: number;
  failCount: number;
  checks: StartupCheckItem[];
  summaryText: string;
};

export type RuntimeStatusResponse = {
  timestamp: string;
  admission: {
    maxConcurrency: number;
    queueLimit: number;
    queueTimeoutMs: number;
    inflight: number;
    queued: number;
  };
  capacity?: {
    maxUserActiveJobs: number;
    maxProviderRunningJobs: number;
    maxQueuedJobs: number;
    queuedJobs: number;
    error?: string;
  };
  providerPool?: {
    groups: number;
    members: number;
    availableMembers: number;
    coolingMembers: number;
    unavailableMembers: number;
    limitedMembers: number;
    concurrencyLimitedMembers?: number;
    disabledGroups?: number;
    disabledMembers?: number;
    noFallbackConfigured?: boolean;
    fallbackMissingPlatforms?: string[];
    dispatchIssues?: Array<{
      code: string;
      label: string;
      count: number;
      detail?: string;
    }>;
    lastError?: string;
    lastErrorAt?: string;
    lastErrorMember?: string;
    error?: string;
  };
  recent: {
    windowSeconds: number;
    failureCount: number;
    lastError?: string;
    lastErrorCode?: string;
    lastErrorAt?: string;
    lastErrorAccount?: string;
  };
  system?: {
    cpu: {
      available: boolean;
      usagePercent?: number;
      error?: string;
    };
    memory: {
      available: boolean;
      usedBytes?: number;
      totalBytes?: number;
      usagePercent?: number;
      source?: string;
      error?: string;
    };
    database: {
      ok: boolean;
      status: string;
      driver?: string;
      name?: string;
      dsn?: string;
      path?: string;
      sizeBytes?: number;
      openConns?: number;
      inUseConns?: number;
      idleConns?: number;
      maxOpenConns?: number;
      maxIdleConns?: number;
      connLifetimeSeconds?: number;
      error?: string;
    };
    redis: {
      enabled: boolean;
      ok?: boolean;
      status: string;
      addr?: string;
      error?: string;
    };
    disk?: {
      available: boolean;
      path?: string;
      freeBytes?: number;
      totalBytes?: number;
      usedPercent?: number;
      error?: string;
    };
    runtime: {
      goroutines: number;
      inflight: number;
      queued: number;
    };
  };
};

type ImageAccountPolicyResponse = {
  policy: StoredImageAccountPolicy;
};

type BusinessAPIProviderListResponse = {
  items: BusinessAPIProvider[];
};

type BusinessAPIProviderMutationResponse = {
  item: BusinessAPIProvider;
};

type BusinessProviderPoolListResponse = {
  items: BusinessProviderPool[];
};

type BusinessProviderGroupMutationResponse = {
  item: BusinessProviderGroup;
};

type BusinessProviderMemberMutationResponse = {
  item: BusinessProviderMember;
};

export type BusinessAPIProviderTestResponse = {
  ok: boolean;
  message: string;
  code?: string;
  durationMs: number;
  imageCount: number;
  group?: BusinessProviderGroup;
  member?: BusinessProviderMember;
};

let cachedImageAccountPolicy: StoredImageAccountPolicy | null = null;
let cachedConfig: ConfigPayload | null = null;

function buildDefaultConfig(): ConfigPayload {
  return {
    app: {
      name: "Poom Studio",
      version: "",
      apiKey: "",
      authKey: "",
      imageFormat: "b64_json",
      maxUploadSizeMB: 10,
    },
    server: {
      host: "0.0.0.0",
      port: 7070,
      staticDir: "",
      maxImageConcurrency: 8,
      imageQueueLimit: 32,
      imageQueueTimeoutSeconds: 20,
    },
    chatgpt: {
      model: "gpt-image-2",
      sseTimeout: 180,
      pollInterval: 2,
      pollMaxWait: 180,
      requestTimeout: 180,
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
      authDir: "data/auths",
      stateFile: "data/accounts_state.json",
      syncStateDir: "data/sync_state",
      imageDir: "data/business-images",
      imageStorage: "browser",
      imageConversationStorage: "browser",
      imageDataStorage: "browser",
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

function isMissingCompatEndpoint(error: unknown) {
  return (
    error instanceof ApiError &&
    (error.status === 404 || error.status === 405)
  );
}

export function setCachedImageAccountPolicy(
  policy: StoredImageAccountPolicy | null,
) {
  cachedImageAccountPolicy = policy ? normalizeImageAccountPolicy(policy) : null;
}

function setCachedConfig(config: ConfigPayload | null) {
  cachedConfig = config;
}

export async function fetchImageAccountPolicy() {
  if (isBusinessProxyMode()) {
    const normalized = normalizeImageAccountPolicy(null);
    setCachedImageAccountPolicy(normalized);
    return normalized;
  }
  let data: ImageAccountPolicyResponse;
  try {
    data = await httpRequest<ImageAccountPolicyResponse>(
      "/api/accounts/image-policy",
    );
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    data = { policy: normalizeImageAccountPolicy(null) };
  }
  const normalized = normalizeImageAccountPolicy(data.policy);
  setCachedImageAccountPolicy(normalized);
  return normalized;
}

export async function updateImageAccountPolicy(
  policy: StoredImageAccountPolicy,
) {
  const data = await httpRequest<ImageAccountPolicyResponse>(
    "/api/accounts/image-policy",
    {
      method: "PUT",
      body: { policy: normalizeImageAccountPolicy(policy) },
    },
  );
  const normalized = normalizeImageAccountPolicy(data.policy);
  setCachedImageAccountPolicy(normalized);
  return normalized;
}

async function getImageAccountPolicyForRequest() {
  if (cachedImageAccountPolicy) {
    return cachedImageAccountPolicy;
  }
  try {
    return await fetchImageAccountPolicy();
  } catch {
    return normalizeImageAccountPolicy(null);
  }
}

function resolveImageResponseFormat(config: ConfigPayload | null) {
  return config?.storage.imageDataStorage === "server" ? "url" : "b64_json";
}

async function getImageResponseFormatForRequest() {
  if (cachedConfig) {
    return resolveImageResponseFormat(cachedConfig);
  }
  try {
    return resolveImageResponseFormat(await fetchConfig());
  } catch {
    return "b64_json";
  }
}

export type ProxyTestResult = {
  ok: boolean;
  status: number;
  latency: number;
  error?: string;
};

export type IntegrationTestResult = {
  ok: boolean;
  source: SyncSource | "cpa";
  message: string;
  status: number;
  latency: number;
  userId?: number;
  username?: string;
  email?: string;
  groupCount?: number;
};

export type NewAPITokenDiscoverResult = {
  ok: boolean;
  message: string;
  latency: number;
  accessToken?: string;
  userId?: number;
};

export type Sub2APIGroupOption = {
  id: string;
  name: string;
  description: string;
  platform: string;
  status: string;
};

export type Sub2APIGroupsResult = {
  ok: boolean;
  message: string;
  latency: number;
  groups: Sub2APIGroupOption[];
};

function isBusinessProxyMode() {
  return webConfig.backendMode === "business_proxy";
}

export type LoginResult = {
  ok: boolean;
  token: string;
  role: AuthRole;
  avatarUrl?: string;
  email?: string;
  username?: string;
  userId?: string;
  expiresAt?: string;
  version?: string;
};

export type RegistrationOptions = {
  enabled: boolean;
  registration: boolean;
  registrationCodeRequired: boolean;
  turnstile?: {
    enabled: boolean;
    siteKey: string;
    login?: boolean;
    registerCode?: boolean;
    registerSubmit?: boolean;
    passwordReset?: boolean;
  };
  emailVerificationConfigured: boolean;
  codeTTLSeconds: number;
  codeCooldownSeconds: number;
};

export type BusinessUserRole = "admin" | "user";
export type BusinessUserStatus = "active" | "disabled" | "deleted";

export type BusinessUser = {
  id: string;
  uid: number;
  username: string;
  email: string;
  role: BusinessUserRole;
  status: BusinessUserStatus;
  avatarUrl?: string;
  subscriptionLevelTag?: string;
  walletLevelTag?: string;
  deleted_at?: string;
  created_at: string;
  updated_at: string;
  usage?: {
    user_id: string;
    generation_count: number;
    success_count: number;
    failed_count: number;
    image_count: number;
    storage_bytes: number;
    last_generated_at?: string;
  };
  credit?: BusinessCreditSummary;
  billing?: {
    subscriptionLevel?: BusinessBillingLevelView;
    walletLevel?: BusinessBillingLevelView;
    subscription?: BusinessSubscription;
    subscriptionLevelOverride?: boolean;
    walletLevelOverride?: boolean;
  };
};

export type BusinessMe = {
  user: BusinessUser;
  credit: BusinessCreditSummary;
  apiAccessEnabled?: boolean;
};

export type BusinessCreditSummary = {
  user_id: string;
  balance: number;
  spent: number;
  updated_at?: string;
};
export type BusinessPaymentPackage = {
  id: string;
  packageType: "balance" | "subscription" | "monthly";
  name: string;
  description: string;
  amountCents: number;
  credits: number;
  durationDays?: number;
  levelTag?: string;
  currency: string;
  enabled: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};
export type BusinessPaymentPackageInput = {
  packageType?: "balance" | "subscription";
  name: string;
  description?: string;
  amountCents: number;
  credits: number;
  durationDays?: number;
  levelTag?: string;
  currency?: string;
  enabled: boolean;
  sortOrder?: number;
};
export type BusinessPaymentOrderStatus = "pending" | "paid" | "completed" | "expired" | "cancelled" | "failed" | "refunded";
export type BusinessPaymentOrder = {
  id: string;
  userId: string;
  userEmail: string;
  username: string;
  packageId: string;
  packageType: "balance" | "subscription" | "monthly";
  amountCents: number;
  credits: number;
  durationDays?: number;
  currency: string;
  paymentMethod?: string;
  providerKey: string;
  providerInstanceId: string;
  outTradeNo: string;
  providerTradeNo: string;
  status: BusinessPaymentOrderStatus;
  payUrl: string;
  qrCode: string;
  expiresAt?: string;
  paidAt?: string;
  completedAt?: string;
  failedAt?: string;
  refundedAt?: string;
  creditLedgerId?: string;
  billingAction?: "new" | "renewal" | "upgrade" | string;
  upgradeFromSubscriptionId?: string;
  upgradeCreditCents?: number;
  originalAmountCents?: number;
  createdAt: string;
  updatedAt: string;
};
export type BusinessPaymentCommission = {
  id: string;
  orderId: string;
  referrerUserId: string;
  referredUserId: string;
  baseAmountCents: number;
  rateBps: number;
  credits: number;
  ledgerId?: string;
  status: string;
  createdAt: string;
  settledAt?: string;
  reversedAt?: string;
};
export type BusinessSubscription = {
  id?: string;
  userId?: string;
  userEmail?: string;
  username?: string;
  orderId?: string;
  packageId?: string;
  packageName?: string;
  durationDays?: number;
  creditsTotal?: number;
  creditsUsed?: number;
  creditsLeft?: number;
  status?: "active" | "expired" | "cancelled" | "upgraded" | string;
  active?: boolean;
  startsAt?: string;
  expiresAt?: string;
  coverageExpiresAt?: string;
  cancelledAt?: string;
  createdAt?: string;
  updatedAt?: string;
};
export type BusinessBillingLevelView = {
  name: string;
  tag: string;
  description?: string;
};
export type BusinessBillingLevelsResponse = {
  subscription: BusinessBillingLevelView;
  wallet: BusinessBillingLevelView;
};
export type BusinessPaymentProvider = {
  id: string;
  providerKey: string;
  name: string;
  enabled: boolean;
  supportedMethods: string[];
  config?: Record<string, string>;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};
export type BusinessPaymentProviderInput = {
  providerKey: string;
  name: string;
  enabled: boolean;
  supportedMethods: string[];
  config?: Record<string, string>;
  sortOrder?: number;
};
export type BusinessPaymentMethod = {
  key: string;
  label: string;
  providerKey: string;
};
export type BusinessPaymentAuditLog = {
  id: string;
  orderId: string;
  action: string;
  detail?: unknown;
  operator: string;
  createdAt: string;
};
export type BusinessCodeType = "redeem" | "promo" | "invite";
export type BusinessCodeStatus = "active" | "disabled" | "expired";
export type BusinessCode = {
  id: string;
  codePreview: string;
  type: BusinessCodeType;
  title: string;
  credits: number;
  maxUses: number;
  usedCount: number;
  status: BusinessCodeStatus;
  startsAt?: string;
  expiresAt?: string;
  createdBy: string;
  note: string;
  createdAt: string;
  updatedAt: string;
};
export type BusinessCodeInput = {
  code?: string;
  type: BusinessCodeType;
  title: string;
  credits: number;
  maxUses: number;
  status: BusinessCodeStatus;
  startsAt?: string;
  expiresAt?: string;
  note?: string;
};
export type BusinessCodeMutationResponse = {
  item: BusinessCode;
  code?: string;
};
export type BusinessCodeUsage = {
  id: string;
  codeId: string;
  userId: string;
  uid: number;
  username: string;
  email: string;
  userStatus: string;
  context: string;
  creditsGranted: number;
  ledgerId: string;
  createdAt: string;
};
export type BusinessAffiliateSummary = {
  enabled: boolean;
  profile?: {
    userId: string;
    codePreview: string;
    enabled: boolean;
    createdAt: string;
    updatedAt: string;
  };
  referralCount: number;
  recentReferrals?: BusinessAffiliateReferral[];
  registrationRewardEnabled: boolean;
  registrationRewardCredits: number;
};
export type BusinessAffiliateReferral = {
  id: string;
  referrerUserId: string;
  referrerUid: number;
  referrerUsername: string;
  referrerEmail: string;
  referrerStatus: string;
  referredUserId: string;
  referredUid: number;
  referredUsername: string;
  referredEmail: string;
  referredStatus: string;
  affiliateCodePreview: string;
  status: string;
  rewardLedgerId?: string;
  rewardCredits: number;
  createdAt: string;
};

export type BusinessStorageReport = {
  summary: {
    assetFiles: number;
    diskFiles: number;
    referencedFiles: number;
    missingFiles: number;
    orphanFiles: number;
    legacyReferencedFiles: number;
    brokenAssets: number;
    assetBytes: number;
    diskBytes: number;
    orphanBytes: number;
  };
  missingFiles: Array<{
    fileName: string;
    userId?: string;
    generationId?: string;
    expectedPath?: string;
    sizeBytes?: number;
  }>;
  orphanFiles: Array<{
    fileName: string;
    path: string;
    sizeBytes: number;
  }>;
  legacyReferencedFiles: Array<{
    fileName: string;
    userId: string;
    generationId: string;
    onDisk: boolean;
  }>;
  brokenAssets: Array<{
    fileName: string;
    userId: string;
    generationId: string;
    reason: string;
  }>;
  directories: string[];
};

export type BusinessStorageBackfillResult = {
  matched: number;
  backfilled: number;
  skipped_missing_file: number;
  skipped_ambiguous: number;
};

export type BusinessStorageBackfillResponse = {
  result: BusinessStorageBackfillResult;
  report: BusinessStorageReport;
};

export type BusinessUsageRecord = {
  id: string;
  user_id: string;
  uid?: number;
  username?: string;
  email?: string;
  conversation_id: string;
  generation_id: string;
  turn_id: string;
  prompt: string;
  model: string;
  size?: string;
  quality?: string;
  count: number;
  status: string;
  error?: string;
  created_at: string;
  finished_at?: string;
  duration_ms: number;
  credit_delta: number;
  credits_used: number;
  api_key_id?: string;
};

export type BusinessModelUsage = {
  model: string;
  generation_count: number;
  success_count: number;
  failed_count: number;
  image_count: number;
  credit_delta: number;
  credits_used: number;
  last_generated_at?: string;
};

export type PaginationMeta = {
  page: number;
  pageSize: number;
  total: number;
};

export type BusinessUsageQuery = {
  page?: number;
  pageSize?: number;
  status?: string;
  userId?: string;
  userQuery?: string;
  model?: string;
  source?: string;
  from?: string;
  to?: string;
  timeRange?: string;
  timezone?: string;
};

export type BusinessCreditLedgerEntry = {
  id: string;
  user_id: string;
  delta: number;
  balance_after: number;
  reason: string;
  generation_id?: string;
  source_type?: string;
  source_id?: string;
  created_at: string;
};

export type BusinessImageAsset = {
  id: string;
  user_id: string;
  conversation_id: string;
  generation_id: string;
  file_name: string;
  file_path: string;
  url: string;
  mime_type: string;
  size_bytes: number;
  sha256: string;
  created_at: string;
  conversation_title?: string;
  prompt?: string;
  model?: string;
  size?: string;
  quality?: string;
};

export type BusinessUserDetail = {
  user: BusinessUser;
  usage: NonNullable<BusinessUser["usage"]>;
  credit: BusinessCreditSummary;
  recentUsage: BusinessUsageRecord[];
  recentUsagePage: PaginationMeta;
  recentAssets: BusinessImageAsset[];
  recentAssetsPage: PaginationMeta;
  recentCreditLedger: BusinessCreditLedgerEntry[];
  recentLedgerPage: PaginationMeta;
  conversationCount: number;
};

export type BusinessDashboardSummary = {
  userCount: number;
  adminCount: number;
  activeUserCount: number;
  disabledUserCount: number;
  generationCount: number;
  successCount: number;
  failedCount: number;
  imageCount: number;
  storageBytes: number;
  conversationCount: number;
  creditBalance: number;
  creditSpent: number;
  lastGeneratedAt?: string;
  recentUsageCount: number;
  recentSuccessCount: number;
  recentFailedCount: number;
  recentImageCount: number;
  recentCreditsUsed: number;
};

export type BusinessDashboardTopUser = BusinessUser & {
  usage: NonNullable<BusinessUser["usage"]>;
  credit: BusinessCreditSummary;
};

export type BusinessDashboard = {
  summary: BusinessDashboardSummary;
  recent: BusinessUsageRecord[];
  topUsers: BusinessDashboardTopUser[];
  modelUsage: BusinessModelUsage[];
  page: PaginationMeta;
};

export type BusinessTrackerRecord = {
  id: string;
  userId?: string;
  conversationId?: string;
  generationId?: string;
  turnId?: string;
  platform?: string;
  providerId?: string;
  providerName?: string;
  model?: string;
  status: string;
  stage?: string;
  errorCode?: string;
  errorMessage?: string;
  requestedCount: number;
  actualCount: number;
  queueWaitMs: number;
  upstreamDurationMs: number;
  persistDurationMs: number;
  totalDurationMs: number;
  creditReserved: number;
  creditRefunded: number;
  storageBytes: number;
  createdAt: string;
  admittedAt?: string;
  upstreamStartedAt?: string;
  upstreamFinishedAt?: string;
  finishedAt?: string;
};

export type BusinessImageJob = {
  id: string;
  userId: string;
  apiKeyId?: string;
  conversationId?: string;
  generationId?: string;
  turnId?: string;
  platform?: APIAccessPlatform | string;
  providerId?: string;
  providerName?: string;
  providerSource?: string;
  providerGroupId?: string;
  providerGroupName?: string;
  providerGroupMatchMode?: string;
  providerGroupTags?: string[];
  providerMemberId?: string;
  providerMemberName?: string;
  compareBatchStatus?: BusinessCompareBatchStatus;
  hasAttachment?: boolean;
  dispatchStrategy?: string;
  dispatchTrace?: string[];
  requestDispatchTags?: string[];
  userDispatchTags?: string[];
  dispatchTags?: string[];
  compareBatchId?: string;
  compareModelIndex?: number;
  compareModelCount?: number;
  modelId?: string;
  modelLabel?: string;
  vendor?: string;
  vendorLabel?: string;
  model?: string;
  upstreamModel?: string;
  upstreamStatusCode?: number;
  prompt?: string;
  size?: string;
  quality?: string;
  requestedCount: number;
  actualCount: number;
  status: "queued" | "running" | "succeeded" | "failed" | string;
  stage?: string;
  upstreamSent: boolean;
  upstreamStatus: "pending" | "sent" | string;
  errorCode?: string;
  errorMessage?: string;
  userErrorType?: string;
  userErrorMessage?: string;
  failureReasonCode?: string;
  failureReasonMessage?: string;
  queueWaitMs: number;
  upstreamDurationMs: number;
  persistDurationMs: number;
  totalDurationMs: number;
  storageBytes: number;
  creditReserved: number;
  creditRefunded: number;
  createdAt: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
  updatedAt: string;
  payload?: Record<string, unknown>;
};

type BusinessImageJobListResponse = {
  items: BusinessImageJob[];
};

type BusinessImageJobPageResponse = {
  items: BusinessImageJob[];
  page: PaginationMeta;
};

export type BusinessCompareBatchStatus = {
  id: string;
  total: number;
  queued: number;
  running: number;
  succeeded: number;
  failed: number;
  cancelled: number;
  reserved: number;
  refunded: number;
  status: string;
};

export type BusinessImageJobQuery = {
  page?: number;
  pageSize?: number;
  userId?: string;
  status?: string;
  platform?: string;
  errorType?: string;
  compareBatchId?: string;
  source?: string;
  from?: string;
  to?: string;
  timeRange?: string;
  timezone?: string;
};

export type BusinessDashboardQuery = {
  from?: string;
  to?: string;
  timeRange?: string;
  timezone?: string;
};

type BusinessImageJobResponse = {
  item: BusinessImageJob;
  activeCancelled?: boolean;
};

export type BusinessImageCompareBatchResponse = {
  summary: BusinessCompareBatchStatus;
  items: BusinessImageJob[];
};

type BusinessImageModelListResponse = {
  items: BusinessImageModel[];
};
type BusinessImageModelMutationResponse = {
  item: BusinessImageModel;
};
export type BusinessImageModelTestResponse = {
  ok: boolean;
  message: string;
  code?: string;
  durationMs?: number;
  imageCount?: number;
  model?: BusinessImageModel;
  availability?: BusinessImageModelAvailability;
};

export type BusinessRiskControlMode = "observe" | "pre_block";
export type BusinessRiskControlProvider = "openai" | "aliyun";
export type BusinessRiskControlFailMode = "fail_closed" | "fail_open";
export type BusinessRiskControlPolicyScope = "global" | "plan" | "user" | "api_key";
export type BusinessRiskControlRiskLevel = "" | "low" | "medium" | "high";
export type BusinessRiskControlConfig = {
  enabled: boolean;
  mode: BusinessRiskControlMode;
  provider: BusinessRiskControlProvider;
  providerChain: BusinessRiskControlProvider[];
  failMode: BusinessRiskControlFailMode;
  baseUrl: string;
  model: string;
  apiKeyConfigured: boolean;
  apiKeyMasked: string;
  openaiBaseUrl: string;
  openaiModel: string;
  openaiApiKeyConfigured: boolean;
  openaiApiKeyMasked: string;
  aliyunAccessKeyId: string;
  aliyunAccessKeySecretConfigured: boolean;
  aliyunAccessKeySecretMasked: string;
  aliyunRegionId: string;
  aliyunEndpoint: string;
  aliyunTextService: string;
  aliyunBlockRiskLevel: Exclude<BusinessRiskControlRiskLevel, "">;
  timeoutMs: number;
  recordNonHits: boolean;
  blockMessage: string;
  thresholds: Record<string, number>;
};
export type BusinessRiskControlConfigInput = Partial<{
  enabled: boolean;
  mode: BusinessRiskControlMode;
  provider: BusinessRiskControlProvider;
  providerChain: BusinessRiskControlProvider[];
  failMode: BusinessRiskControlFailMode;
  baseUrl: string;
  apiKey: string;
  clearApiKey: boolean;
  model: string;
  openaiBaseUrl: string;
  openaiApiKey: string;
  clearOpenaiApiKey: boolean;
  openaiModel: string;
  aliyunAccessKeyId: string;
  aliyunAccessKeySecret: string;
  clearAliyunAccessKeySecret: boolean;
  aliyunRegionId: string;
  aliyunEndpoint: string;
  aliyunTextService: string;
  aliyunBlockRiskLevel: Exclude<BusinessRiskControlRiskLevel, "">;
  timeoutMs: number;
  recordNonHits: boolean;
  blockMessage: string;
  thresholds: Record<string, number>;
}>;
export type BusinessRiskControlPolicy = {
  id: string;
  scope: BusinessRiskControlPolicyScope;
  targetId: string;
  enabled: boolean;
  mode?: "" | BusinessRiskControlMode;
  riskLevel?: BusinessRiskControlRiskLevel;
  blockMessage?: string;
  thresholds?: Record<string, number>;
  createdAt: string;
  updatedAt: string;
};
export type BusinessRiskControlPolicyInput = {
  scope: BusinessRiskControlPolicyScope;
  targetId?: string;
  enabled?: boolean;
  mode?: "" | BusinessRiskControlMode;
  riskLevel?: BusinessRiskControlRiskLevel;
  blockMessage?: string;
  thresholds?: Record<string, number>;
};
export type BusinessRiskControlStatus = {
  enabled: boolean;
  mode: BusinessRiskControlMode;
  provider: BusinessRiskControlProvider;
  providerChain: BusinessRiskControlProvider[];
  apiKeyConfigured: boolean;
  last24hTotal: number;
  last24hFlagged: number;
  last24hBlocked: number;
  last24hErrors: number;
};
export type BusinessRiskControlLog = {
  id: string;
  userId: string;
  jobId: string;
  conversationId: string;
  turnId: string;
  platform: string;
  model: string;
  mode: BusinessRiskControlMode | string;
  provider: BusinessRiskControlProvider | string;
  riskLevel: string;
  providerReason: string;
  providerLatencyMs: number;
  action: "allow" | "block" | "error" | string;
  flagged: boolean;
  highestCategory: string;
  highestScore: number;
  categoryScores: Record<string, number>;
  inputExcerpt: string;
  error: string;
  latencyMs: number;
  createdAt: string;
};
export type BusinessRiskControlLogsQuery = {
  page?: number;
  pageSize?: number;
  result?: string;
  platform?: string;
  search?: string;
  from?: string;
  to?: string;
};
export type BusinessRiskControlLogsResponse = {
  items: BusinessRiskControlLog[];
  total: number;
  page: number;
  pageSize: number;
  pages: number;
};
export type BusinessRiskControlDecision = {
  allowed: boolean;
  action: string;
  flagged: boolean;
  highestCategory: string;
  highestScore: number;
  categoryScores: Record<string, number>;
  provider?: BusinessRiskControlProvider | string;
  riskLevel?: string;
  providerReason?: string;
  message?: string;
  error?: string;
};
export type BusinessRiskControlTestResponse = {
  result: {
    decision: BusinessRiskControlDecision;
    latencyMs: number;
    provider: BusinessRiskControlProvider | string;
    providerLatencyMs: number;
  };
};

export type BusinessTrackerPlatformSummary = {
  platform: string;
  total: number;
  succeeded: number;
  failed: number;
  successRate: number;
  avgUpstreamMs: number;
  p95UpstreamMs: number;
  lastErrorCode?: string;
  lastErrorMessage?: string;
  lastErrorCreatedAt?: string;
};

export type BusinessTrackerSummary = {
  windowSeconds: number;
  windowStart: string;
  total: number;
  succeeded: number;
  failed: number;
  successRate: number;
  requestedCount: number;
  actualCount: number;
  avgTotalMs: number;
  p95TotalMs: number;
  avgQueueMs: number;
  avgUpstreamMs: number;
  p95UpstreamMs: number;
  storageBytes: number;
  platforms: BusinessTrackerPlatformSummary[];
  recentFailures: BusinessTrackerRecord[];
};

type BusinessUserListResponse = {
  items: BusinessUser[];
};

type BusinessUsageListResponse = {
  items: BusinessUsageRecord[];
  page: PaginationMeta;
};

export type BusinessAssetListResponse = {
  items: BusinessImageAsset[];
  page: PaginationMeta;
};

type BusinessUserMutationResponse = {
  item: BusinessUser;
};

export async function login(email: string, password: string, turnstileToken?: string): Promise<LoginResult> {
  const normalizedEmail = String(email || "").trim();
  try {
    return await httpRequest<LoginResult>("/auth/login", {
      method: "POST",
      body: { email: normalizedEmail, password, turnstileToken: turnstileToken?.trim() || undefined },
      redirectOnUnauthorized: false,
    });
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    throw error;
  }
}

export async function fetchRegistrationOptions() {
  return httpRequest<RegistrationOptions>("/auth/register/options", {
    redirectOnUnauthorized: false,
  });
}

export async function requestRegistrationCode(email: string, turnstileToken?: string) {
  return httpRequest<{ ok: boolean; expiresIn: number; cooldownSeconds: number }>("/auth/register/code", {
    method: "POST",
    body: { email: String(email || "").trim(), turnstileToken: turnstileToken?.trim() || undefined },
    redirectOnUnauthorized: false,
  });
}

export async function requestPasswordResetCode(email: string, turnstileToken?: string) {
  return httpRequest<{ ok: boolean; expiresIn: number; cooldownSeconds: number }>("/auth/password-reset/code", {
    method: "POST",
    body: { email: String(email || "").trim(), turnstileToken: turnstileToken?.trim() || undefined },
    redirectOnUnauthorized: false,
  });
}

export async function registerBusinessUser(payload: {
  email: string;
  username?: string;
  password: string;
  code: string;
  registerCode?: string;
  affiliateCode?: string;
  turnstileToken?: string;
}): Promise<LoginResult> {
  return httpRequest<LoginResult>("/auth/register", {
    method: "POST",
    body: {
      email: payload.email.trim(),
      username: payload.username?.trim() || undefined,
      password: payload.password,
      code: payload.code.trim(),
      registerCode: payload.registerCode?.trim() || undefined,
      affiliateCode: payload.affiliateCode?.trim() || undefined,
      turnstileToken: payload.turnstileToken?.trim() || undefined,
    },
    redirectOnUnauthorized: false,
  });
}

export async function resetBusinessUserPasswordByEmail(payload: {
  email: string;
  code: string;
  password: string;
}) {
  return httpRequest<{ ok: boolean }>("/auth/password-reset", {
    method: "POST",
    body: {
      email: payload.email.trim(),
      code: payload.code.trim(),
      password: payload.password,
    },
    redirectOnUnauthorized: false,
  });
}

export async function logout() {
  return httpRequest<{ ok: boolean }>("/auth/logout", {
    method: "POST",
    redirectOnUnauthorized: false,
  });
}

export async function fetchAccounts() {
  if (isBusinessProxyMode()) {
    return { items: [] };
  }
  try {
    return await httpRequest<AccountListResponse>("/api/accounts");
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    return { items: [] };
  }
}

export async function fetchBusinessUsers(query: {
  includeDeleted?: boolean;
} = {}) {
  return httpRequest<BusinessUserListResponse>(
    `/api/business/users${buildQuery({
      includeDeleted: query.includeDeleted ? "true" : undefined,
    })}`,
  );
}

export async function fetchBusinessDashboard(query: BusinessDashboardQuery = {}) {
  return httpRequest<BusinessDashboard>(`/api/business/admin/dashboard${buildQuery(query)}`);
}

export async function fetchBusinessTrackerSummary(windowSeconds = 600) {
  return httpRequest<BusinessTrackerSummary>(
    `/api/business/tracker/summary${buildQuery({ windowSeconds })}`,
  );
}

export async function fetchAdminBusinessNotifications(query: {
  status?: string;
  search?: string;
  limit?: number;
} = {}) {
  return httpRequest<BusinessNotificationListResponse>(
    `/api/business/admin/notifications${buildQuery({
      status: query.status && query.status !== "all" ? query.status : undefined,
      search: query.search,
      limit: query.limit,
    })}`,
  );
}

export async function createBusinessNotification(payload: BusinessNotificationInput) {
  return httpRequest<{ item: BusinessNotification }>("/api/business/admin/notifications", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessNotification(id: string, payload: BusinessNotificationInput) {
  return httpRequest<{ item: BusinessNotification }>(
    `/api/business/admin/notifications/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function deleteBusinessNotification(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/notifications/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchBusinessNotifications() {
  return httpRequest<BusinessNotificationListResponse>("/api/business/notifications");
}

export async function markBusinessNotificationsRead(ids: string[]) {
  return httpRequest<{ ok: boolean }>("/api/business/notifications/read", {
    method: "POST",
    body: { ids },
  });
}

function buildQuery(params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === "" || value === "all") {
      return;
    }
    search.set(key, String(value));
  });
  const query = search.toString();
  return query ? `?${query}` : "";
}

export async function fetchBusinessUserDetail(id: string, query: {
  usagePage?: number;
  usagePageSize?: number;
  assetsPage?: number;
  assetsPageSize?: number;
  ledgerPage?: number;
  ledgerPageSize?: number;
  status?: string;
  model?: string;
  from?: string;
  to?: string;
  timeRange?: string;
  timezone?: string;
} = {}) {
  return httpRequest<BusinessUserDetail>(
    `/api/business/users/${encodeURIComponent(id)}${buildQuery(query)}`,
  );
}

export async function fetchBusinessMe() {
  return httpRequest<BusinessMe>("/api/business/me");
}

export async function uploadBusinessMeAvatar(file: File) {
  const body = new FormData();
  body.append("avatar", file);
  return httpRequest<{ user: BusinessUser }>("/api/business/me/avatar", {
    method: "POST",
    body,
  });
}

export async function changeBusinessMePassword(payload: {
  currentPassword: string;
  newPassword: string;
}) {
  return httpRequest<{ user: BusinessUser }>("/api/business/me/password", {
    method: "PATCH",
    body: payload,
  });
}

export async function fetchBusinessCredit() {
  return httpRequest<BusinessCreditSummary>("/api/business/credit");
}

export async function fetchBusinessCreditLedger(query: {
  page?: number;
  pageSize?: number;
} = {}) {
  return httpRequest<{ items: BusinessCreditLedgerEntry[]; page: PaginationMeta }>(
    `/api/business/credit/ledger${buildQuery(query)}`,
  );
}

export async function redeemBusinessCode(code: string) {
  return httpRequest<{
    ok: boolean;
    result: {
      creditsGranted: number;
      ledgerId?: string;
      code: BusinessCode;
    };
    credit: BusinessCreditSummary;
  }>("/api/business/credit/redeem", {
    method: "POST",
    body: { code: String(code || "").trim() },
  });
}

export async function fetchBusinessPaymentPackages() {
  return httpRequest<{ items: BusinessPaymentPackage[] }>("/api/business/payment/packages");
}

export async function fetchBusinessPaymentMethods() {
  return httpRequest<{ items: BusinessPaymentMethod[] }>("/api/business/payment/methods");
}

export async function fetchBusinessPaymentOrders(query: {
  status?: BusinessPaymentOrderStatus | "all";
  kind?: "all" | "balance" | "subscription" | "renewal" | "upgrade";
  search?: string;
  page?: number;
  pageSize?: number;
  limit?: number;
} = {}) {
  return httpRequest<{ items: BusinessPaymentOrder[]; page?: PaginationMeta }>(
    `/api/business/payment/orders${buildQuery(query)}`,
  );
}

export async function createBusinessPaymentOrder(packageId: string, paymentMethod?: string) {
  return httpRequest<{ order: BusinessPaymentOrder }>("/api/business/payment/orders", {
    method: "POST",
    body: { packageId, paymentMethod },
  });
}

export async function fetchBusinessSubscription() {
  return httpRequest<{ subscription: BusinessSubscription }>("/api/business/subscription");
}

export async function fetchBusinessBillingLevels() {
  return httpRequest<BusinessBillingLevelsResponse>("/api/business/billing-levels");
}

export async function fetchBusinessAffiliateSummary() {
  return httpRequest<BusinessAffiliateSummary>("/api/business/affiliate");
}

export async function fetchAdminBusinessAffiliateReferrals(limit = 100) {
  return httpRequest<{ items: BusinessAffiliateReferral[] }>(
    `/api/business/admin/affiliate/referrals${buildQuery({ limit })}`,
  );
}

export async function fetchBusinessUsage(query: BusinessUsageQuery = {}) {
  return httpRequest<BusinessUsageListResponse>(`/api/business/usage${buildQuery(query)}`);
}

export async function fetchBusinessAssets(query: {
  page?: number;
  pageSize?: number;
} = {}) {
  return httpRequest<BusinessAssetListResponse>(
    `/api/business/assets${buildQuery(query)}`,
  );
}

export async function fetchBusinessImageJobs(query: {
  conversationId?: string;
  limit?: number;
} = {}) {
  return httpRequest<BusinessImageJobListResponse>(
    `/api/business/jobs${buildQuery(query)}`,
  );
}

export async function fetchBusinessImageJob(id: string) {
  return httpRequest<BusinessImageJobResponse>(
    `/api/business/jobs/${encodeURIComponent(id)}`,
  );
}

export async function fetchBusinessImageModels() {
  return httpRequest<BusinessImageModelListResponse>("/api/business/image-models");
}

export async function fetchAdminBusinessImageModels() {
  return httpRequest<BusinessImageModelListResponse>("/api/business/admin/image-models");
}

export async function createBusinessImageModel(payload: BusinessImageModelInput) {
  return httpRequest<BusinessImageModelMutationResponse>("/api/business/admin/image-models", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessImageModel(id: string, payload: BusinessImageModelInput) {
  return httpRequest<BusinessImageModelMutationResponse>(
    `/api/business/admin/image-models/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function setDefaultBusinessImageModel(id: string) {
  return httpRequest<BusinessImageModelMutationResponse>(
    `/api/business/admin/image-models/${encodeURIComponent(id)}/default`,
    { method: "POST" },
  );
}

export async function testBusinessImageModel(id: string) {
  return httpRequest<BusinessImageModelTestResponse>(
    `/api/business/admin/image-models/${encodeURIComponent(id)}/test`,
    { method: "POST" },
  );
}

export async function deleteBusinessImageModel(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/image-models/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchAdminBusinessImageJobs(query: BusinessImageJobQuery = {}) {
  return httpRequest<BusinessImageJobPageResponse>(
    `/api/business/admin/jobs${buildQuery(query)}`,
  );
}

export async function fetchAdminBusinessImageCompareBatch(id: string) {
  return httpRequest<BusinessImageCompareBatchResponse>(
    `/api/business/admin/compare-batches/${encodeURIComponent(id)}`,
  );
}

export async function fetchBusinessRiskControlConfig() {
  return httpRequest<{ config: BusinessRiskControlConfig }>(
    "/api/business/admin/risk-control/config",
  );
}

export async function updateBusinessRiskControlConfig(payload: BusinessRiskControlConfigInput) {
  return httpRequest<{ config: BusinessRiskControlConfig }>(
    "/api/business/admin/risk-control/config",
    { method: "PUT", body: payload },
  );
}

export async function fetchBusinessRiskControlStatus() {
  return httpRequest<{ status: BusinessRiskControlStatus }>(
    "/api/business/admin/risk-control/status",
  );
}

export async function fetchBusinessRiskControlLogs(query: BusinessRiskControlLogsQuery = {}) {
  return httpRequest<BusinessRiskControlLogsResponse>(
    `/api/business/admin/risk-control/logs${buildQuery(query)}`,
  );
}

export async function testBusinessRiskControl(payload: {
  prompt: string;
  mode?: BusinessRiskControlMode;
  providerChain?: BusinessRiskControlProvider[];
  failMode?: BusinessRiskControlFailMode;
  baseUrl?: string;
  apiKey?: string;
  clearApiKey?: boolean;
  model?: string;
  openaiBaseUrl?: string;
  openaiApiKey?: string;
  clearOpenaiApiKey?: boolean;
  openaiModel?: string;
  aliyunAccessKeyId?: string;
  aliyunAccessKeySecret?: string;
  clearAliyunAccessKeySecret?: boolean;
  aliyunRegionId?: string;
  aliyunEndpoint?: string;
  aliyunTextService?: string;
  aliyunBlockRiskLevel?: Exclude<BusinessRiskControlRiskLevel, "">;
  timeoutMs?: number;
  blockMessage?: string;
  thresholds?: Record<string, number>;
}) {
  return httpRequest<BusinessRiskControlTestResponse>(
    "/api/business/admin/risk-control/test",
    { method: "POST", body: payload },
  );
}

export async function fetchBusinessRiskControlPolicies(query: { scope?: string; targetId?: string } = {}) {
  return httpRequest<{ items: BusinessRiskControlPolicy[] }>(
    `/api/business/admin/risk-control/policies${buildQuery(query)}`,
  );
}

export async function upsertBusinessRiskControlPolicy(payload: BusinessRiskControlPolicyInput) {
  return httpRequest<{ item: BusinessRiskControlPolicy }>(
    "/api/business/admin/risk-control/policies",
    { method: "PUT", body: payload },
  );
}

export async function deleteBusinessRiskControlPolicy(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/risk-control/policies/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function cancelBusinessImageJob(id: string) {
  return httpRequest<BusinessImageJobResponse>(
    `/api/business/jobs/${encodeURIComponent(id)}/cancel`,
    { method: "POST" },
  );
}

export async function fetchAllBusinessUsage(query: BusinessUsageQuery = {}) {
  return httpRequest<BusinessUsageListResponse>(`/api/business/admin/usage${buildQuery(query)}`);
}

export async function fetchBusinessStorageReport() {
  return httpRequest<BusinessStorageReport>("/api/business/storage/report");
}

export async function backfillBusinessStorageAssets() {
  return httpRequest<BusinessStorageBackfillResponse>(
    "/api/business/storage/backfill-assets",
    { method: "POST" },
  );
}

export async function fetchBusinessAPIProviders() {
  return httpRequest<BusinessAPIProviderListResponse>("/api/business/api-providers");
}

export async function createBusinessAPIProvider(payload: BusinessAPIProviderInput) {
  return httpRequest<BusinessAPIProviderMutationResponse>("/api/business/api-providers", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessAPIProvider(id: string, payload: BusinessAPIProviderInput) {
  return httpRequest<BusinessAPIProviderMutationResponse>(
    `/api/business/api-providers/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function setDefaultBusinessAPIProvider(id: string) {
  return httpRequest<BusinessAPIProviderMutationResponse>(
    `/api/business/api-providers/${encodeURIComponent(id)}/default`,
    { method: "POST" },
  );
}

export async function deleteBusinessAPIProvider(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/api-providers/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function testBusinessAPIProvider(id: string) {
  return httpRequest<BusinessAPIProviderTestResponse>(
    `/api/business/api-providers/${encodeURIComponent(id)}/test`,
    { method: "POST" },
  );
}

export async function fetchBusinessProviderPools() {
  return httpRequest<BusinessProviderPoolListResponse>("/api/business/provider-pools");
}

export async function createBusinessProviderGroup(payload: BusinessProviderGroupInput) {
  return httpRequest<BusinessProviderGroupMutationResponse>("/api/business/provider-pools", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessProviderGroup(id: string, payload: BusinessProviderGroupInput) {
  return httpRequest<BusinessProviderGroupMutationResponse>(
    `/api/business/provider-pools/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function setDefaultBusinessProviderGroup(id: string) {
  return httpRequest<BusinessProviderGroupMutationResponse>(
    `/api/business/provider-pools/${encodeURIComponent(id)}/default`,
    { method: "POST" },
  );
}

export async function deleteBusinessProviderGroup(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/provider-pools/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function testBusinessProviderGroup(id: string) {
  return httpRequest<BusinessAPIProviderTestResponse>(
    `/api/business/provider-pools/${encodeURIComponent(id)}/test`,
    { method: "POST" },
  );
}

export async function previewBusinessProviderDispatch(payload: BusinessProviderDispatchPreviewInput) {
  return httpRequest<BusinessProviderDispatchPreviewResponse>("/api/business/provider-pools/preview", {
    method: "POST",
    body: payload,
  });
}

export async function createBusinessProviderMember(payload: BusinessProviderMemberInput) {
  return httpRequest<BusinessProviderMemberMutationResponse>("/api/business/provider-pool-members", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessProviderMember(id: string, payload: BusinessProviderMemberInput) {
  return httpRequest<BusinessProviderMemberMutationResponse>(
    `/api/business/provider-pool-members/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function deleteBusinessProviderMember(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/provider-pool-members/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function testBusinessProviderMember(id: string) {
  return httpRequest<BusinessAPIProviderTestResponse>(
    `/api/business/provider-pool-members/${encodeURIComponent(id)}/test`,
    { method: "POST" },
  );
}

export async function recoverBusinessProviderMember(id: string) {
  return httpRequest<BusinessProviderMemberMutationResponse>(
    `/api/business/provider-pool-members/${encodeURIComponent(id)}/recover`,
    { method: "POST" },
  );
}

export async function fetchBusinessSystemSettings() {
  return httpRequest<BusinessSystemSettingsResponse>("/api/business/system-settings");
}

export async function fetchPublicSiteSettings() {
  return httpRequest<PublicSiteSettings>("/api/site", { redirectOnUnauthorized: false });
}

export async function updateBusinessSystemSettings(settings: BusinessSystemSettings) {
  return httpRequest<BusinessSystemSettingsResponse>("/api/business/system-settings", {
    method: "PUT",
    body: { settings },
  });
}

export async function updateBusinessAffiliateSettings(affiliate: BusinessSystemSettings["affiliate"]) {
  const current = await fetchBusinessSystemSettings();
  return updateBusinessSystemSettings({
    ...current.settings,
    affiliate: {
      ...current.settings.affiliate,
      ...affiliate,
    },
  });
}

export async function fetchBusinessCodes(query: {
  type?: BusinessCodeType | "registration" | "all";
  status?: BusinessCodeStatus | "all";
  search?: string;
} = {}) {
  return httpRequest<{ items: BusinessCode[] }>(
    `/api/business/admin/codes${buildQuery(query)}`,
  );
}

export async function createBusinessCode(payload: BusinessCodeInput) {
  return httpRequest<BusinessCodeMutationResponse>("/api/business/admin/codes", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessCode(id: string, payload: Omit<BusinessCodeInput, "code" | "type">) {
  return httpRequest<BusinessCodeMutationResponse>(
    `/api/business/admin/codes/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function updateBusinessCodeStatusBatch(ids: string[], status: BusinessCodeStatus) {
  return httpRequest<{ ok: boolean; updated: number }>("/api/business/admin/codes/batch-status", {
    method: "POST",
    body: { ids, status },
  });
}

export async function fetchBusinessCodeUsages(id: string) {
  return httpRequest<{ items: BusinessCodeUsage[] }>(
    `/api/business/admin/codes/${encodeURIComponent(id)}/usages`,
  );
}

export async function deleteBusinessCode(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/codes/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchAdminBusinessPaymentPackages() {
  return httpRequest<{ items: BusinessPaymentPackage[] }>("/api/business/admin/payment/packages");
}

export async function createAdminBusinessPaymentPackage(payload: BusinessPaymentPackageInput) {
  return httpRequest<{ item: BusinessPaymentPackage }>("/api/business/admin/payment/packages", {
    method: "POST",
    body: payload,
  });
}

export async function updateAdminBusinessPaymentPackage(id: string, payload: BusinessPaymentPackageInput) {
  return httpRequest<{ item: BusinessPaymentPackage }>(
    `/api/business/admin/payment/packages/${encodeURIComponent(id)}`,
    { method: "PUT", body: payload },
  );
}

export async function deleteAdminBusinessPaymentPackage(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/payment/packages/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchAdminBusinessPaymentProviders() {
  return httpRequest<{ items: BusinessPaymentProvider[] }>("/api/business/admin/payment/providers");
}

export async function createAdminBusinessPaymentProvider(payload: BusinessPaymentProviderInput) {
  return httpRequest<{ item: BusinessPaymentProvider }>("/api/business/admin/payment/providers", {
    method: "POST",
    body: payload,
  });
}

export async function updateAdminBusinessPaymentProvider(id: string, payload: BusinessPaymentProviderInput) {
  return httpRequest<{ item: BusinessPaymentProvider }>(
    `/api/business/admin/payment/providers/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function deleteAdminBusinessPaymentProvider(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/admin/payment/providers/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchAdminBusinessPaymentOrders(query: {
  status?: BusinessPaymentOrderStatus | "all";
  kind?: "all" | "balance" | "subscription" | "renewal" | "upgrade";
  search?: string;
  page?: number;
  pageSize?: number;
  limit?: number;
} = {}) {
  return httpRequest<{ items: BusinessPaymentOrder[]; page?: PaginationMeta }>(
    `/api/business/admin/payment/orders${buildQuery(query)}`,
  );
}

export async function fetchAdminBusinessSubscriptions(query: {
  status?: "active" | "expired" | "cancelled" | "upgraded" | "all";
  activeWindow?: "current" | "future" | "history" | "all";
  search?: string;
  page?: number;
  pageSize?: number;
  limit?: number;
} = {}) {
  return httpRequest<{ items: BusinessSubscription[]; page?: PaginationMeta }>(
    `/api/business/admin/payment/subscriptions${buildQuery(query)}`,
  );
}

export async function completeAdminBusinessPaymentOrder(id: string, payload: {
  providerTradeNo?: string;
  commissionRateBps?: number;
} = {}) {
  return httpRequest<{ order: BusinessPaymentOrder; commission?: BusinessPaymentCommission }>(
    `/api/business/admin/payment/orders/${encodeURIComponent(id)}/complete`,
    { method: "POST", body: payload },
  );
}

export async function cancelAdminBusinessPaymentOrder(id: string) {
  return httpRequest<{ order: BusinessPaymentOrder }>(
    `/api/business/admin/payment/orders/${encodeURIComponent(id)}/cancel`,
    { method: "POST" },
  );
}

export async function refundAdminBusinessPaymentOrder(id: string) {
  return httpRequest<{ order: BusinessPaymentOrder; commission?: BusinessPaymentCommission }>(
    `/api/business/admin/payment/orders/${encodeURIComponent(id)}/refund`,
    { method: "POST" },
  );
}

export async function fetchAdminBusinessPaymentOrderAuditLogs(id: string) {
  return httpRequest<{ items: BusinessPaymentAuditLog[] }>(
    `/api/business/admin/payment/orders/${encodeURIComponent(id)}/audit`,
  );
}

export async function createBusinessUser(payload: {
  password: string;
  email: string;
  username?: string;
  balance?: number;
}) {
  return httpRequest<BusinessUserMutationResponse>("/api/business/users", {
    method: "POST",
    body: payload,
  });
}

export async function updateBusinessUser(id: string, payload: {
  username: string;
  password?: string;
}) {
  return httpRequest<BusinessUserMutationResponse>(
    `/api/business/users/${encodeURIComponent(id)}`,
    {
      method: "PATCH",
      body: payload,
    },
  );
}

export async function updateBusinessUserBillingLevels(id: string, payload: {
  subscriptionLevelTag?: string;
  walletLevelTag?: string;
}) {
  return httpRequest<BusinessUserMutationResponse>(
    `/api/business/users/${encodeURIComponent(id)}/billing-levels`,
    {
      method: "PATCH",
      body: payload,
    },
  );
}

export async function updateBusinessUserStatus(id: string, status: BusinessUserStatus) {
  return httpRequest<BusinessUserMutationResponse>(
    `/api/business/users/${encodeURIComponent(id)}/status`,
    {
      method: "PATCH",
      body: { status },
    },
  );
}

export async function deleteBusinessUser(id: string) {
  return httpRequest<{ ok: boolean; item: BusinessUser }>(
    `/api/business/users/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function restoreBusinessUser(id: string) {
  return httpRequest<BusinessUserMutationResponse>(
    `/api/business/users/${encodeURIComponent(id)}/restore`,
    { method: "POST" },
  );
}

export async function purgeBusinessUser(id: string) {
  return httpRequest<{ ok: boolean; item: BusinessUser }>(
    `/api/business/users/${encodeURIComponent(id)}/purge`,
    { method: "DELETE" },
  );
}

export async function clearBusinessUserData(id: string) {
  return httpRequest<{ ok: boolean; item: BusinessUser }>(
    `/api/business/users/${encodeURIComponent(id)}/data`,
    { method: "DELETE" },
  );
}

export async function resetBusinessUserPassword(id: string, password: string) {
  return httpRequest<BusinessUserMutationResponse>(
    `/api/business/users/${encodeURIComponent(id)}/password`,
    {
      method: "PATCH",
      body: { password },
    },
  );
}

export async function setBusinessUserCredit(id: string, balance: number) {
  return httpRequest<{ credit: BusinessCreditSummary }>(
    `/api/business/users/${encodeURIComponent(id)}/credit`,
    {
      method: "PUT",
      body: { balance, reason: "admin_adjustment" },
    },
  );
}

export async function adjustBusinessUserCredit(id: string, payload: {
  operation: "recharge" | "refund";
  amount: number;
}) {
  return httpRequest<{ credit: BusinessCreditSummary }>(
    `/api/business/users/${encodeURIComponent(id)}/credit`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function createAccounts(tokens: string[]) {
  return httpRequest<AccountMutationResponse>("/api/accounts", {
    method: "POST",
    body: { tokens },
  });
}

export async function importAccountFiles(files: File[]) {
  const formData = new FormData();
  files.forEach((file) => formData.append("file", file));
  return httpRequest<AccountImportResponse>("/api/accounts/import", {
    method: "POST",
    body: formData,
  });
}

export async function deleteAccounts(tokens: string[]) {
  return httpRequest<AccountMutationResponse>("/api/accounts", {
    method: "DELETE",
    body: { tokens },
  });
}

export async function refreshAccounts(accessTokens: string[]) {
  return httpRequest<AccountRefreshResponse>("/api/accounts/refresh", {
    method: "POST",
    body: { access_tokens: accessTokens },
  });
}

export async function refreshAllAccounts() {
  return httpRequest<AccountRefreshAllResponse>("/api/accounts/refresh-all", {
    method: "POST",
    body: {},
  });
}

export async function fetchAccountRefreshProgress() {
  return httpRequest<AccountRefreshAllResponse>(
    "/api/accounts/refresh-progress",
  );
}

export async function updateAccount(
  accessToken: string,
  updates: {
    type?: AccountType;
    status?: AccountStatus;
    quota?: number;
    note?: string;
  },
) {
  return httpRequest<AccountUpdateResponse>("/api/accounts/update", {
    method: "POST",
    body: {
      access_token: accessToken,
      ...updates,
    },
  });
}

export async function fetchAccountQuota(
  accountId: string,
  options: { refresh?: boolean } = {},
) {
  const refresh = options.refresh ?? true;
  const suffix = refresh ? "" : "?refresh=false";
  return httpRequest<AccountQuotaResponse>(
    `/api/accounts/${encodeURIComponent(accountId)}/quota${suffix}`,
  );
}

export async function fetchSyncStatus(
  source: SyncSource = "cpa",
  options: { progressOnly?: boolean } = {},
) {
  const params = new URLSearchParams({ source });
  if (options.progressOnly) {
    params.set("progress_only", "1");
  }
  return httpRequest<SyncStatusResponse>(
    `/api/sync/status?${params.toString()}`,
  );
}

export async function fetchConfig() {
  if (isBusinessProxyMode()) {
    const config = buildDefaultConfig();
    setCachedConfig(config);
    return config;
  }
  let config: ConfigPayload;
  try {
    config = await httpRequest<ConfigPayload>("/api/config");
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    config = buildDefaultConfig();
  }
  setCachedConfig(config);
  return config;
}

export async function testProxy(url?: string) {
  return httpRequest<ProxyTestResult>("/api/proxy/test", {
    method: "POST",
    body: { url: url ?? "" },
  });
}

export async function testIntegration(
  source: "newapi" | "sub2api",
  payload: {
    newapi?: ConfigPayload["newapi"];
    sub2api?: ConfigPayload["sub2api"];
  },
) {
  return httpRequest<IntegrationTestResult>("/api/integration/test", {
      method: "POST",
      body: {
        source,
        newapi: payload.newapi,
        sub2api: payload.sub2api,
    },
  });
}

export async function discoverNewAPIToken(newapi: ConfigPayload["newapi"]) {
  return httpRequest<NewAPITokenDiscoverResult>(
    "/api/integration/newapi/token",
    {
      method: "POST",
      body: { newapi },
    },
  );
}

export async function fetchSub2APIGroups(sub2api: ConfigPayload["sub2api"]) {
  return httpRequest<Sub2APIGroupsResult>("/api/integration/sub2api/groups", {
    method: "POST",
    body: { sub2api },
  });
}

export async function fetchDefaultConfig() {
  if (isBusinessProxyMode()) {
    return buildDefaultConfig();
  }
  try {
    return await httpRequest<ConfigPayload>("/api/config/defaults");
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    return buildDefaultConfig();
  }
}

export async function updateConfig(config: ConfigPayload) {
  const result = await httpRequest<{ status: string; config: ConfigPayload }>("/api/config", {
    method: "PUT",
    body: config,
  });
  setCachedConfig(result.config);
  return result;
}

export async function fetchRequestLogs() {
  return httpRequest<{ items: RequestLogItem[] }>("/api/requests");
}

export async function fetchVersionInfo(force = false) {
  try {
    return await httpRequest<VersionInfo>(force ? "/version?force=true" : "/version", {
      redirectOnUnauthorized: false,
    });
  } catch (error) {
    if (!isMissingCompatEndpoint(error)) {
      throw error;
    }
    return { version: "sub2api-dev" };
  }
}

export async function fetchSystemUpdateStatus() {
  return httpRequest<SystemUpdateStatus>("/api/system/update/status", {
    redirectOnUnauthorized: false,
  });
}

export async function startSystemUpdate() {
  return httpRequest<SystemUpdateStartResponse>("/api/system/update/start", {
    method: "POST",
    timeoutMs: 30000,
  });
}

export async function fetchMaintenanceStatus() {
  return httpRequest<MaintenanceStatus>("/api/system/maintenance");
}

export async function updateMaintenanceStatus(enabled: boolean) {
  return httpRequest<MaintenanceStatus>("/api/system/maintenance", {
    method: "PUT",
    body: { enabled },
  });
}

export async function fetchStartupCheck() {
  return httpRequest<StartupCheckResponse>("/api/startup/check");
}

export async function fetchRuntimeStatus() {
  return httpRequest<RuntimeStatusResponse>("/api/runtime/status");
}

export async function downloadDiagnosticsExport() {
  const response = await fetch(
    `${webConfig.apiUrl.replace(/\/$/, "")}/api/diagnostics/export`,
    {
      method: "GET",
      credentials: "include",
    },
  );
  if (!response.ok) {
    let message = `download failed (${response.status})`;
    try {
      const payload = (await response.json()) as {
        error?: string;
        message?: string;
        detail?: { message?: string };
      };
      message =
        payload?.detail?.message || payload?.message || payload?.error || message;
    } catch {
      // ignore json parse errors
    }
    throw new Error(message);
  }
  const blob = await response.blob();
  const disposition = response.headers.get("content-disposition") || "";
  const match = disposition.match(/filename="([^"]+)"/i);
  const fileName =
    match?.[1] || `poom-studio-diagnostics-${Date.now()}.json`;
  return { blob, fileName };
}

export async function runSync(
  direction: "pull" | "push",
  source: SyncSource = "cpa",
) {
  return httpRequest<{ result: SyncRunResult; status?: SyncStatusResponse }>(
    "/api/sync/run",
    {
      method: "POST",
      body: { direction, source },
    },
  );
}

export async function generateImage(
  prompt: string,
  model: ImageModel = "gpt-image-2",
  count = 1,
) {
  return generateImageWithOptions(prompt, { model, count });
}

function buildImageDispatchTags(options: {
  mode?: "generate" | "edit";
  modelId?: ImageModelId;
  model?: ImageModel;
  size?: string;
  quality?: ImageQuality;
  vendor?: string;
  adapter?: string;
  dispatchTags?: string[];
}) {
  const tags = new Set<string>();
  const add = (value: string | undefined) => {
    const normalized = value?.trim().toLowerCase();
    if (normalized) {
      tags.add(normalized);
    }
  };
  options.dispatchTags?.forEach(add);
  add(`mode:${options.mode || "generate"}`);
  add(options.quality ? `quality:${options.quality}` : undefined);
  add(options.size ? `size:${options.size}` : undefined);
  add(options.modelId ? `modelId:${options.modelId}` : undefined);
  add(options.model ? `model:${options.model}` : undefined);
  add(options.vendor ? `vendor:${options.vendor}` : undefined);
  add(options.adapter ? `adapter:${options.adapter}` : undefined);
  return Array.from(tags);
}

export async function generateImageWithOptions(
  prompt: string,
  options: {
    mode?: "generate" | "edit";
    modelId?: ImageModelId;
    model?: ImageModel;
    modelLabel?: string;
    vendor?: string;
    vendorLabel?: string;
    adapter?: string;
    count?: number;
    size?: string;
    quality?: ImageQuality;
    platform?: APIAccessPlatform;
    jobId?: string;
    conversationId?: string;
    turnId?: string;
    title?: string;
    compareBatchId?: string;
    compareGroupId?: string;
    compareModelLabel?: string;
    compareModelIndex?: number;
    compareModelCount?: number;
    sourceImages?: ImageSourcePayload[];
    hasAttachment?: boolean;
    sourceReference?: InpaintSourceReference;
    dispatchTags?: string[];
  } = {},
) {
  const { model = "gpt-image-2", count = 1, size, quality = "high" } = options;
  const [policy, responseFormat] = await Promise.all([
    getImageAccountPolicyForRequest(),
    getImageResponseFormatForRequest(),
  ]);
  const policyHeader = buildImageAccountPolicyHeader(policy);
  const normalizedCount = Math.max(1, count);
  const body: Record<string, unknown> = {
    prompt,
    modelId: options.modelId?.trim() || undefined,
    model,
    modelLabel: options.modelLabel?.trim() || undefined,
    vendor: options.vendor?.trim() || undefined,
    vendorLabel: options.vendorLabel?.trim() || undefined,
    adapter: options.adapter?.trim() || undefined,
    n: normalizedCount,
    size: size?.trim() || undefined,
    quality,
    response_format: responseFormat,
    mode: options.mode || "generate",
    platform: options.platform?.trim() || undefined,
    jobId: options.jobId?.trim() || undefined,
    conversationId: options.conversationId?.trim() || undefined,
    turnId: options.turnId?.trim() || undefined,
    title: options.title?.trim() || undefined,
    compareBatchId: options.compareBatchId?.trim() || options.compareGroupId?.trim() || undefined,
    compareGroupId: options.compareGroupId?.trim() || undefined,
    compareModelLabel: options.compareModelLabel?.trim() || undefined,
    compareModelIndex: options.compareModelIndex,
    compareModelCount: options.compareModelCount,
    dispatchTags: buildImageDispatchTags(options),
  };
  if (options.sourceImages?.length) {
    body.sourceImages = options.sourceImages;
  }
  if (options.hasAttachment) {
    body.hasAttachment = true;
  }
  if (options.sourceReference) {
    body.sourceReference = options.sourceReference;
  }
  return httpRequest<ImageResponse>("/api/image/generate", {
    method: "POST",
    headers: policyHeader
      ? { "X-Studio-Account-Policy": policyHeader }
      : undefined,
    body,
  });
}

export type BusinessAPIKeyStatus = "active" | "disabled" | "revoked";

export type BusinessAPIKey = {
  id: string;
  userId: string;
  name: string;
  keyPrefix: string;
  keyLast4: string;
  status: BusinessAPIKeyStatus;
  creditLimit: number;
  usedCredits: number;
  rateLimitPerMinute: number;
  concurrencyLimit: number;
  allowedModels: string[];
  lastUsedAt?: string;
  createdAt: string;
  updatedAt: string;
  revokedAt?: string;
  plaintext?: string;
};

export type AdminCreateAPIKeyInput = {
  userId: string;
  name: string;
  env: "live" | "test";
  creditLimit?: number;
  rateLimitPerMinute?: number;
  concurrencyLimit?: number;
  allowedModels?: string[];
};

export type AdminUpdateAPIKeyInput = {
  name?: string;
  status?: BusinessAPIKeyStatus;
  creditLimit?: number;
  rateLimitPerMinute?: number;
  concurrencyLimit?: number;
  allowedModels?: string[];
};

export type BusinessAPIKeyCreateResult = {
  item: BusinessAPIKey;
  secret: string;
};

// 管理员侧（requireAdminAuth）
export async function fetchAdminAPIKeys(userId?: string) {
  const suffix = userId ? `?userId=${encodeURIComponent(userId)}` : "";
  return httpRequest<{ items: BusinessAPIKey[] }>(`/api/business/admin/api-keys${suffix}`);
}

export async function createAdminAPIKey(payload: AdminCreateAPIKeyInput) {
  return httpRequest<BusinessAPIKeyCreateResult>("/api/business/admin/api-keys", {
    method: "POST",
    body: payload,
  });
}

export async function updateAdminAPIKey(id: string, payload: AdminUpdateAPIKeyInput) {
  return httpRequest<{ item: BusinessAPIKey }>(
    `/api/business/admin/api-keys/${encodeURIComponent(id)}`,
    { method: "PATCH", body: payload },
  );
}

export async function revokeAdminAPIKey(id: string) {
  return httpRequest<{ item: BusinessAPIKey }>(
    `/api/business/admin/api-keys/${encodeURIComponent(id)}/revoke`,
    { method: "POST" },
  );
}

export async function fetchAdminUsersAPIAccess() {
  return httpRequest<{ enabledUserIds: string[] }>("/api/business/admin/users-api-access");
}

export async function updateBusinessUserAPIAccess(userId: string, enabled: boolean) {
  return httpRequest<{ ok: boolean; apiAccessEnabled: boolean }>(
    `/api/business/users/${encodeURIComponent(userId)}/api-access`,
    { method: "PATCH", body: { enabled } },
  );
}

// 用户自助侧（requireUIAuth + 本人已开通）
export async function fetchMyAPIKeys() {
  return httpRequest<{ items: BusinessAPIKey[]; baseUrl: string }>("/api/business/api-keys");
}

export async function createMyAPIKey(name: string) {
  return httpRequest<BusinessAPIKeyCreateResult>("/api/business/api-keys", {
    method: "POST",
    body: { name },
  });
}

export async function revokeMyAPIKey(id: string) {
  return httpRequest<{ item: BusinessAPIKey }>(
    `/api/business/api-keys/${encodeURIComponent(id)}/revoke`,
    { method: "POST" },
  );
}

export async function updateMyAPIKeyStatus(id: string, status: "active" | "disabled") {
  return httpRequest<{ item: BusinessAPIKey }>(
    `/api/business/api-keys/${encodeURIComponent(id)}`,
    { method: "PATCH", body: { status } },
  );
}

export async function deleteMyAPIKey(id: string) {
  return httpRequest<{ ok: boolean }>(
    `/api/business/api-keys/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}
