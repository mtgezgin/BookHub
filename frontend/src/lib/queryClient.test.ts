import { describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import { createQueryClient, shouldRetry } from "./queryClient";

describe("shouldRetry", () => {
  it("4xx hiç yeniden denenmez (istek yanlış; 401'i client.ts kendisi ele alır)", () => {
    for (const status of [400, 401, 403, 404, 409, 429]) {
      expect(shouldRetry(0, new ApiError(status, "x", "m"))).toBe(false);
    }
  });

  it("5xx ve ağ hatası yalnız bir kez yeniden denenir", () => {
    for (const err of [new ApiError(502, "x", "m"), new TypeError("Failed to fetch")]) {
      expect(shouldRetry(0, err)).toBe(true);
      expect(shouldRetry(1, err)).toBe(false);
    }
  });
});

describe("createQueryClient", () => {
  it("okumalar 30 sn taze kalır, değişiklikler hiç yeniden denenmez", () => {
    const opts = createQueryClient().getDefaultOptions();
    expect(opts.queries?.staleTime).toBe(30_000);
    expect(opts.queries?.retry).toBe(shouldRetry);
    expect(opts.mutations?.retry).toBe(false);
  });
});
