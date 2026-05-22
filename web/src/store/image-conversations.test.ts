import { describe, expect, it } from "vitest";

import { businessImageConversationDetailToConversation } from "./image-conversations";

describe("business image history adapter", () => {
  it("maps business image generations to image conversation turns", () => {
    const conversation = businessImageConversationDetailToConversation({
      conversation: {
        id: "conv-1",
        user_id: "dev_user",
        title: "Cat session",
        created_at: "2026-05-13T10:00:00Z",
        updated_at: "2026-05-13T10:01:00Z",
      },
      generations: [
        {
          id: "job-1",
          user_id: "dev_user",
          conversation_id: "conv-1",
          turn_id: "turn-1",
          prompt: "cat",
          model: "gpt-image-2",
          size: "1024x1024",
          quality: "high",
          count: 1,
          status: "succeeded",
          response: {
            data: [
              {
                b64_json: "aW1hZ2U=",
                revised_prompt: "a cat",
              },
            ],
          },
          created_at: "2026-05-13T10:01:00Z",
          finished_at: "2026-05-13T10:02:00Z",
        },
      ],
    });

    expect(conversation.id).toBe("conv-1");
    expect(conversation.title).toBe("cat");
    expect(conversation.prompt).toBe("cat");
    expect(conversation.status).toBe("success");
    expect(conversation.turns).toHaveLength(1);
    expect(conversation.turns?.[0]).toMatchObject({
      id: "turn-1-job-1",
      mode: "generate",
      prompt: "cat",
      model: "gpt-image-2",
      count: 1,
      size: "1024x1024",
      quality: "high",
      status: "success",
      jobId: "job-1",
    });
    expect(conversation.turns?.[0]?.images[0]).toMatchObject({
      id: "turn-1-job-1-0",
      status: "success",
      b64_json: "aW1hZ2U=",
      revised_prompt: "a cat",
    });
  });

  it("keeps failed business image generations visible after refresh", () => {
    const conversation = businessImageConversationDetailToConversation({
      conversation: {
        id: "conv-failed",
        user_id: "dev_user",
        title: "cat",
        created_at: "2026-05-13T10:00:00Z",
        updated_at: "2026-05-13T10:01:00Z",
      },
      generations: [
        {
          id: "job-failed",
          user_id: "dev_user",
          conversation_id: "conv-failed",
          turn_id: "turn-failed",
          prompt: "cat",
          model: "gpt-image-2",
          count: 1,
          status: "failed",
          response: {
            error: {
              message: "Upstream request failed",
            },
          },
          error: "Upstream request failed",
          created_at: "2026-05-13T10:01:00Z",
          finished_at: "2026-05-13T10:02:00Z",
        },
      ],
    });

    expect(conversation.id).toBe("conv-failed");
    expect(conversation.status).toBe("error");
    expect(conversation.turns?.[0]).toMatchObject({
      id: "turn-failed-job-failed",
      status: "error",
      error: "Upstream request failed",
      jobId: "job-failed",
    });
    expect(conversation.turns?.[0]?.images[0]).toMatchObject({
      id: "turn-failed-job-failed-0",
      status: "error",
      error: "Upstream request failed",
    });
  });

  it("keeps non-GPT provider models in business image history", () => {
    const conversation = businessImageConversationDetailToConversation({
      conversation: {
        id: "conv-banana",
        user_id: "dev_user",
        title: "cat",
        created_at: "2026-05-18T10:00:00Z",
        updated_at: "2026-05-18T10:01:00Z",
      },
      generations: [
        {
          id: "job-banana",
          user_id: "dev_user",
          conversation_id: "conv-banana",
          turn_id: "turn-banana",
          prompt: "cat",
          model: "gemini-2.5-flash-image",
          size: "1248x1248",
          quality: "high",
          count: 1,
          status: "succeeded",
          response: {
            platform: "gemini-banana",
            data: [
              {
                url: "/v1/files/image/business-dev_user-conv-banana-job-banana-0.png",
              },
            ],
          },
          created_at: "2026-05-18T10:01:00Z",
          finished_at: "2026-05-18T10:02:00Z",
        },
      ],
    });

    expect(conversation.model).toBe("gemini-2.5-flash-image");
    expect(conversation.turns?.[0]).toMatchObject({
      model: "gemini-2.5-flash-image",
      providerPlatform: "gemini-banana",
    });
  });

  it("uses job failure to override a stale running generation", () => {
    const conversation = businessImageConversationDetailToConversation({
      conversation: {
        id: "conv-job-failed",
        user_id: "dev_user",
        title: "dog",
        created_at: "2026-05-17T10:00:00Z",
        updated_at: "2026-05-17T10:01:00Z",
      },
      generations: [
        {
          id: "gen-1",
          user_id: "dev_user",
          conversation_id: "conv-job-failed",
          turn_id: "turn-1",
          prompt: "dog",
          model: "gpt-image-2",
          count: 1,
          status: "running",
          created_at: "2026-05-17T10:01:00Z",
        },
      ],
      jobs: [
        {
          id: "job-1",
          userId: "dev_user",
          conversationId: "conv-job-failed",
          generationId: "gen-1",
          turnId: "turn-1",
          platform: "gpt-image",
          model: "gpt-image-2",
          requestedCount: 1,
          actualCount: 0,
          status: "failed",
          stage: "admission",
          errorCode: "image_queue_timeout",
          errorMessage: "现在使用人数较多，请稍后使用。",
          queueWaitMs: 5000,
          upstreamDurationMs: 0,
          persistDurationMs: 0,
          totalDurationMs: 5000,
          storageBytes: 0,
          creditReserved: 1,
          creditRefunded: 1,
          createdAt: "2026-05-17T10:01:00Z",
          queuedAt: "2026-05-17T10:01:00Z",
          finishedAt: "2026-05-17T10:01:05Z",
          updatedAt: "2026-05-17T10:01:05Z",
        },
      ],
    });

    expect(conversation.status).toBe("error");
    expect(conversation.turns?.[0]).toMatchObject({
      id: "turn-1-gen-1",
      status: "error",
      error: "现在使用人数较多，请稍后使用。",
      waitingDetail: "admission",
      waitingSince: "2026-05-17T10:01:00Z",
      finishedAt: "2026-05-17T10:01:05Z",
    });
    expect(conversation.turns?.[0]?.images[0]).toMatchObject({
      status: "error",
      error: "现在使用人数较多，请稍后使用。",
    });
  });
});
