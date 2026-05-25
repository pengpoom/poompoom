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
export type ImageQuality = "low" | "medium" | "high";
export type ImageResolutionAccess = "free" | "paid";
export type APIAccessPlatform = "gpt-image" | "gemini-banana";
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
  };
  user: {
    defaultRole: AuthRole;
    defaultCredits: number;
    registration: boolean;
    registrationCodeRequired: boolean;
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

export type BusinessAPIProviderTestResponse = {
  ok: boolean;
  message: string;
  code?: string;
  durationMs: number;
  imageCount: number;
};

let cachedImageAccountPolicy: StoredImageAccountPolicy | null = null;
let cachedConfig: ConfigPayload | null = null;

function buildDefaultConfig(): ConfigPayload {
  return {
    app: {
      name: "Image Studio",
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
};

export type BusinessMe = {
  user: BusinessUser;
  credit: BusinessCreditSummary;
};

export type BusinessCreditSummary = {
  user_id: string;
  balance: number;
  spent: number;
  updated_at?: string;
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
  registrationRewardEnabled: boolean;
  registrationRewardCredits: number;
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
  conversationId?: string;
  generationId?: string;
  turnId?: string;
  platform?: APIAccessPlatform | string;
  providerId?: string;
  providerName?: string;
  model?: string;
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

export type BusinessImageJobQuery = {
  page?: number;
  pageSize?: number;
  userId?: string;
  status?: string;
  platform?: string;
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

export async function login(email: string, password: string): Promise<LoginResult> {
  const normalizedEmail = String(email || "").trim();
  try {
    return await httpRequest<LoginResult>("/auth/login", {
      method: "POST",
      body: { email: normalizedEmail, password },
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

export async function requestRegistrationCode(email: string) {
  return httpRequest<{ ok: boolean; expiresIn: number; cooldownSeconds: number }>("/auth/register/code", {
    method: "POST",
    body: { email: String(email || "").trim() },
    redirectOnUnauthorized: false,
  });
}

export async function requestPasswordResetCode(email: string) {
  return httpRequest<{ ok: boolean; expiresIn: number; cooldownSeconds: number }>("/auth/password-reset/code", {
    method: "POST",
    body: { email: String(email || "").trim() },
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

export async function fetchBusinessAffiliateSummary() {
  return httpRequest<BusinessAffiliateSummary>("/api/business/affiliate");
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

export async function fetchAdminBusinessImageJobs(query: BusinessImageJobQuery = {}) {
  return httpRequest<BusinessImageJobPageResponse>(
    `/api/business/admin/jobs${buildQuery(query)}`,
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
    match?.[1] || `image-studio-diagnostics-${Date.now()}.json`;
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

export async function generateImageWithOptions(
  prompt: string,
  options: {
    mode?: "generate" | "edit";
    model?: ImageModel;
    count?: number;
    size?: string;
    quality?: ImageQuality;
    platform?: APIAccessPlatform;
    jobId?: string;
    conversationId?: string;
    turnId?: string;
    title?: string;
    sourceImages?: ImageSourcePayload[];
    sourceReference?: InpaintSourceReference;
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
    model,
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
  };
  if (options.sourceImages?.length) {
    body.sourceImages = options.sourceImages;
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
