import axios, { AxiosError, type AxiosRequestConfig } from "axios";

import webConfig from "@/constants/common-env";
import { clearStoredAuthKey, isIntentionalLogoutInProgress } from "@/store/auth";

type RequestConfig = AxiosRequestConfig & {
  redirectOnUnauthorized?: boolean;
  unauthorizedRetryDone?: boolean;
};

type ErrorPayload = {
  detail?: { error?: string; code?: string; message?: string };
  error?: string | { message?: string; code?: string };
  message?: string;
  reason?: string;
  code?: string;
};

export class ApiError extends Error {
  code?: string;
  status?: number;

  constructor(message: string, options: { code?: string; status?: number } = {}) {
    super(message);
    this.name = "ApiError";
    this.code = options.code;
    this.status = options.status;
  }
}

const request = axios.create({
  baseURL: webConfig.apiUrl.replace(/\/$/, ""),
  withCredentials: true,
});

let unauthorizedEventDispatched = false;

export function resetUnauthorizedRedirectState() {
  unauthorizedEventDispatched = false;
}

function dispatchUnauthorizedEvent() {
  if (typeof window === "undefined" || unauthorizedEventDispatched) {
    return;
  }
  unauthorizedEventDispatched = true;
  window.dispatchEvent(new CustomEvent("image-studio:unauthorized"));
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function canRetryUnauthorized(config?: RequestConfig) {
  if (!config || config.unauthorizedRetryDone) {
    return false;
  }
  const method = String(config.method || "GET").toUpperCase();
  return method === "GET" || method === "HEAD" || method === "OPTIONS";
}

request.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ErrorPayload>) => {
    const status = error.response?.status;
    if (status === 401 && isIntentionalLogoutInProgress()) {
      return new Promise<never>(() => {
        // 主动退出时，页面卸载中的请求可能晚返回 401；交给路由跳转处理，不再把错误抛给页面 toast。
      });
    }
    const shouldRedirect =
      (error.config as RequestConfig | undefined)?.redirectOnUnauthorized !== false;
    if (status === 401 && shouldRedirect && typeof window !== "undefined") {
      const retryConfig = error.config as RequestConfig | undefined;
      if (retryConfig && canRetryUnauthorized(retryConfig)) {
        await sleep(800);
        return request.request({
          ...retryConfig,
          unauthorizedRetryDone: true,
        });
      }
      await clearStoredAuthKey();
      dispatchUnauthorizedEvent();
    }

    const payload = error.response?.data;
    const nestedError =
      payload && typeof payload.error === "object" && payload.error
        ? (payload.error as { message?: string; code?: string })
        : null;
    const code =
      payload?.detail?.code ||
      nestedError?.code ||
      payload?.code;
    const message =
      payload?.detail?.error ||
      payload?.detail?.message ||
      nestedError?.message ||
      (typeof payload?.error === "string" ? payload.error : "") ||
      payload?.reason ||
      payload?.message ||
      error.message ||
      `请求失败 (${status || 500})`;
    return Promise.reject(new ApiError(message, { code, status }));
  },
);

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  timeoutMs?: number;
  redirectOnUnauthorized?: boolean;
};

export async function httpRequest<T>(path: string, options: RequestOptions = {}) {
  const {
    method = "GET",
    body,
    headers,
    timeoutMs,
    redirectOnUnauthorized = true,
  } = options;
  const config: RequestConfig = {
    url: path,
    method,
    data: body,
    headers,
    timeout: timeoutMs,
    redirectOnUnauthorized,
  };
  const response = await request.request<T>(config);
  return response.data;
}
