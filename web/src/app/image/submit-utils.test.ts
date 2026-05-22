import { describe, expect, it } from "vitest";

import { formatImageErrorMessage } from "./submit-utils";

describe("formatImageErrorMessage", () => {
  it("hides upstream provider details from end users", () => {
    const message = `provider_error {"type":"https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-5xx-errors/error-502/","status":502,"cloudflare_error":true}`;

    expect(formatImageErrorMessage(message)).toBe(
      "上游生成失败，可能是服务繁忙或提示词被上游拒绝，点数已退回。",
    );
  });

  it("keeps existing model refusal guidance", () => {
    expect(
      formatImageErrorMessage("No images generated. The model may have refused this request."),
    ).toBe("没有生成图片，模型可能检测到敏感内容，拒绝了这次请求，建议重试或调整提示词。");
  });
});
