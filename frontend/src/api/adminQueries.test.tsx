import type { QueryClient } from "@tanstack/react-query";
import { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { mountHook, testQueryClient } from "../test/render"; // ayrıca notifyManager.setScheduler(queueMicrotask)

vi.mock("./client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./client")>()),
  api: {},
}));

import { api, ApiError } from "./client";
import { useCreateGenre, useModerateBook } from "./adminQueries";
import { queryKeys } from "./queries";

/*
 * Değişikliklerin GEÇERSİZ KILMA kuralları (Faz 16d adım 6). Sayfa testleri "liste yeniden çekildi mi"
 * sorusunu ekrandaki listeyle cevaplıyor; burada ekranda OLMAYAN kayıtlara (öteki sekmeler, katalog, tür
 * listesi) ne olduğuna bakıyoruz. Kayıt geçersizse bir sonraki açılışta yeniden çekilir.
 */

// Önbelleğe ekranda olmayan (gözlemcisiz) birkaç kayıt koyar.
function seeded() {
  const client = testQueryClient();
  client.setQueryData(queryKeys.genres, []);
  client.setQueryData(queryKeys.books({ limit: 20 }), { items: [], total: 0, limit: 20, offset: 0 });
  client.setQueryData(queryKeys.adminBooks("APPROVED", 0), { items: [], total: 0, limit: 20, offset: 0 });
  client.setQueryData(queryKeys.comments(1, 0), { items: [], total: 0, limit: 10, offset: 0 });
  return client;
}
const invalidated = (client: QueryClient, key: readonly unknown[]) => client.getQueryState(key)!.isInvalidated;

describe("useModerateBook", () => {
  it("başarıda öteki moderasyon sekmeleri ve katalog bayat olur; türler ve yorumlar değişmez", async () => {
    (api as unknown as Record<string, unknown>).approve = async () => ({});
    const client = seeded();
    const hook = mountHook(() => useModerateBook(), client);
    await act(async () => {
      await hook.current!.mutateAsync({ kind: "approve", id: 1 });
    });
    expect(invalidated(client, queryKeys.adminBooks("APPROVED", 0))).toBe(true);
    expect(invalidated(client, queryKeys.books({ limit: 20 }))).toBe(true);
    expect(invalidated(client, queryKeys.genres)).toBe(false);
    expect(invalidated(client, queryKeys.comments(1, 0))).toBe(false);
  });

  it("hata olursa hiçbir şey geçersiz kılınmaz (kitabın durumu değişmedi)", async () => {
    (api as unknown as Record<string, unknown>).approve = () =>
      Promise.reject(new ApiError(409, "invalid_transition", "geçersiz geçiş"));
    const client = seeded();
    const hook = mountHook(() => useModerateBook(), client);
    await act(async () => {
      await hook.current!.mutateAsync({ kind: "approve", id: 1 }).catch(() => {});
    });
    expect(invalidated(client, queryKeys.adminBooks("APPROVED", 0))).toBe(false);
    expect(invalidated(client, queryKeys.books({ limit: 20 }))).toBe(false);
  });

  it("red, gerekçeyi api.reject'e iletir", async () => {
    const calls: unknown[][] = [];
    (api as unknown as Record<string, unknown>).reject = async (...a: unknown[]) => calls.push(a);
    const hook = mountHook(() => useModerateBook(), seeded());
    await act(async () => {
      await hook.current!.mutateAsync({ kind: "reject", id: 3, reason: "spam" });
    });
    expect(calls).toEqual([[3, "spam"]]);
  });
});

describe("useCreateGenre", () => {
  // Türlerin staleTime'ı 5 dk (useGenres): geçersiz kılınmasaydı yeni tür katalog filtresinde 5 dk görünmezdi.
  it("başarıda tür listesi bayat olur, katalog değişmez", async () => {
    (api as unknown as Record<string, unknown>).createGenre = async (name: string) => ({ id: 9, name, slug: "roman" });
    const client = seeded();
    const hook = mountHook(() => useCreateGenre(), client);
    await act(async () => {
      await hook.current!.mutateAsync("Roman");
    });
    expect(invalidated(client, queryKeys.genres)).toBe(true);
    expect(invalidated(client, queryKeys.books({ limit: 20 }))).toBe(false);
  });
});
