import type { APIAccessPlatform } from "@/lib/api";

export type ProviderPlatformOption = {
  label: string;
  value: APIAccessPlatform;
  vendor: string;
  vendorLabel: string;
  adapter: "openai-images" | "gemini";
  defaultModel: string;
};

export const providerPlatformOptions: ProviderPlatformOption[] = [
  {
    label: "OpenAI",
    value: "gpt-image",
    vendor: "openai",
    vendorLabel: "OpenAI",
    adapter: "openai-images",
    defaultModel: "gpt-image-2",
  },
  {
    label: "Google",
    value: "gemini-banana",
    vendor: "google",
    vendorLabel: "Google",
    adapter: "gemini",
    defaultModel: "gemini-2.5-flash-image",
  },
  {
    label: "Doubao",
    value: "doubao",
    vendor: "doubao",
    vendorLabel: "Doubao",
    adapter: "openai-images",
    defaultModel: "doubao-seedream-image",
  },
  {
    label: "Qwen",
    value: "qwen",
    vendor: "qwen",
    vendorLabel: "Qwen",
    adapter: "openai-images",
    defaultModel: "qwen-image",
  },
  {
    label: "Baidu",
    value: "baidu",
    vendor: "baidu",
    vendorLabel: "Baidu",
    adapter: "openai-images",
    defaultModel: "baidu-image",
  },
  {
    label: "Z.AI",
    value: "z-ai",
    vendor: "z-ai",
    vendorLabel: "Z.AI",
    adapter: "openai-images",
    defaultModel: "z-ai-image",
  },
  {
    label: "Tencent",
    value: "tencent",
    vendor: "tencent",
    vendorLabel: "Tencent",
    adapter: "openai-images",
    defaultModel: "tencent-image",
  },
  {
    label: "Kling",
    value: "kling",
    vendor: "kling",
    vendorLabel: "Kling",
    adapter: "openai-images",
    defaultModel: "kling-image",
  },
  {
    label: "Grok",
    value: "grok",
    vendor: "grok",
    vendorLabel: "Grok",
    adapter: "openai-images",
    defaultModel: "grok-image",
  },
];

export function providerPlatformLabel(value: string) {
  return providerPlatformOptions.find((item) => item.value === value)?.label ?? value;
}

export function providerPlatformMeta(value: APIAccessPlatform) {
  return providerPlatformOptions.find((item) => item.value === value) ?? providerPlatformOptions[0];
}

export function defaultModelForPlatform(platform: APIAccessPlatform) {
  return providerPlatformMeta(platform).defaultModel;
}

export function isGeminiPlatform(platform: string) {
  return platform === "gemini-banana";
}

export function normalizeAPIAccessPlatform(value: unknown): APIAccessPlatform | undefined {
  const normalized = String(value || "").trim().toLowerCase();
  if (normalized === "zai" || normalized === "z.ai") {
    return "z-ai";
  }
  return providerPlatformOptions.some((item) => item.value === normalized)
    ? (normalized as APIAccessPlatform)
    : undefined;
}
