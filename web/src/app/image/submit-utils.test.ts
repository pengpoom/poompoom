import { describe, expect, it } from "vitest";

import { formatImageErrorMessage } from "./submit-utils";

describe("formatImageErrorMessage", () => {
  it("hides upstream provider details from end users", () => {
    const message = `provider_error {"type":"https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-5xx-errors/error-502/","status":502,"cloudflare_error":true}`;

    expect(formatImageErrorMessage(message)).toBe(
      "服务繁忙，请稍后重试。",
    );
  });

  it("hides provider pool and api access details from end users", () => {
    expect(
      formatImageErrorMessage("当前平台没有可用号池成员，请恢复或启用 active 成员，或配置 API 接入作为兜底"),
    ).toBe("服务繁忙，请稍后重试。");

    expect(
      formatImageErrorMessage("api_access.base_url or IMAGE_BASE_URL is required"),
    ).toBe("服务繁忙，请稍后重试。");
  });

  it("keeps actionable user errors visible", () => {
    expect(formatImageErrorMessage("点数余额不足")).toBe("点数余额不足，请充值后再试。");
    expect(formatImageErrorMessage("prompt is required")).toBe("请输入提示词后再生成。");
    expect(formatImageErrorMessage("image_queue_full")).toBe("当前生成请求较多，请稍后再试。");
  });

  it("keeps existing model refusal guidance", () => {
    expect(
      formatImageErrorMessage("No images generated. The model may have refused this request."),
    ).toBe("内容可能未通过模型安全检查，请调整提示词后重试。");
  });

  it("normalizes prompt overload, timeout and unknown errors", () => {
    expect(
      formatImageErrorMessage("An error occurred while processing your request. request id abc"),
    ).toBe("提示词较长或当前规格较高，请简化描述或降低规格后重试。");
    expect(
      formatImageErrorMessage("timed out waiting for async image generation"),
    ).toBe("生成等待超时，请稍后重试或降低规格。");
    expect(formatImageErrorMessage("database internal error")).toBe("生成失败，请稍后重试。");
  });
});
