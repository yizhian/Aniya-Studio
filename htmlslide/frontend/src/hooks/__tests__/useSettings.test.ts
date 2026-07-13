import { describe, it, expect } from "vitest";
import { providerPayload, type ProviderKey, type ProviderConfig } from "../useSettings";

describe("providerPayload", () => {
  const cfg: ProviderConfig = {
    apiKey: "sk-test-key",
    baseUrl: "https://api.example.com",
    modelName: "claude-opus-4-5",
  };

  it("converts anthropic provider config to API payload", () => {
    const result = providerPayload("anthropic", cfg);
    expect(result).toEqual({
      provider: "anthropic",
      api_key: "sk-test-key",
      base_url: "https://api.example.com",
      model_name: "claude-opus-4-5",
    });
  });

  it("converts openai provider config to API payload", () => {
    const result = providerPayload("openai", cfg);
    expect(result.provider).toBe("openai");
    expect(result.api_key).toBe("sk-test-key");
  });

  it("handles empty strings", () => {
    const emptyCfg: ProviderConfig = {
      apiKey: "",
      baseUrl: "",
      modelName: "",
    };
    const result = providerPayload("anthropic", emptyCfg);
    expect(result).toEqual({
      provider: "anthropic",
      api_key: "",
      base_url: "",
      model_name: "",
    });
  });

  it("uses camelCase to snake_case mapping", () => {
    const result = providerPayload("anthropic", {
      apiKey: "key",
      baseUrl: "https://example.com",
      modelName: "gpt-4o",
    });
    // Verify the keys are snake_case (not camelCase)
    expect(Object.keys(result)).toEqual([
      "provider",
      "api_key",
      "base_url",
      "model_name",
    ]);
  });
});
