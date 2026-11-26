import type { QueryClient } from "@tanstack/react-query";
import { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { mountHook, testQueryClient } from "../test/render";

vi.mock("./client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./client")>()),
  api: {},
}));

import { api, ApiError } from "./client";
import { queryKeys } from "./queries";
import { useDeleteReview, useModerateReview, useMyReview, useSaveReview } from "./reviewQueries";
import type { Review } from "./types";

/*
 * İnceleme kancalarının önbellek kuralları (Faz 17B adım 4): neyi doğrudan yazdıkları, neyi eskittikleri,
 * hangi hatada yeniden okuttukları. Ekrandaki davranış adım 5'in bileşen testlerinde.
 */

const USER = 7;
const BOOK = 5;
const MINE = queryKeys.myReview(USER, BOOK);
const PAGE_1 = queryKeys.reviews(BOOK, 0);
const PAGE_2 = queryKeys.reviews(BOOK, 20);
const OTHER_BOOK = queryKeys.reviews(6, 0);
const SUMMARY = queryKeys.ratingSummary(BOOK);
const EMPTY_PAGE = { items: [], total: 0, limit: 20, offset: 0 };

const review = (over: Partial<Review> = {}): Review => ({
  id: 3, book_id: BOOK, body: "Güzel", has_spoiler: false, version: 1,
  created_at: "2026-11-25T10:00:00Z", edited_at: null, hidden: null, ...over,
});

// Ekranda olmayan (gözlemcisiz) kayıtlar: benim incelemem, bu kitabın iki liste sayfası, başka kitabın
// listesi ve puan özeti.
function seeded() {
  const client = testQueryClient();
  client.setQueryData(MINE, review());
  for (const key of [PAGE_1, PAGE_2, OTHER_BOOK]) client.setQueryData(key, EMPTY_PAGE);
  client.setQueryData(SUMMARY, {});
  return client;
}
const invalidated = (client: QueryClient, key: readonly unknown[]) => client.getQueryState(key)!.isInvalidated;
const stub = (impls: Record<string, unknown>) => Object.assign(api as unknown as Record<string, unknown>, impls);
const fail = (status: number, code: string) => async () => {
  throw new ApiError(status, code, "x");
};

async function settle(run: () => Promise<unknown>) {
  await act(async () => {
    await run().catch(() => {});
  });
}

describe("benim incelemem", () => {
  it("no_review hata değil, null", async () => {
    stub({ myReview: fail(404, "no_review") });
    const client = testQueryClient();
    const hook = mountHook(() => useMyReview(USER, BOOK), client);
    await act(async () => {});
    expect(hook.current!.isError).toBe(false);
    expect(client.getQueryData(MINE)).toBeNull();
  });

  it("kitap yoksa (not_found) hata kalır", async () => {
    stub({ myReview: fail(404, "not_found") });
    const hook = mountHook(() => useMyReview(USER, BOOK), testQueryClient());
    await act(async () => {});
    expect(hook.current!.isError).toBe(true);
  });

  it("girişsiz kullanıcıda istek gitmez", async () => {
    const myReview = vi.fn();
    stub({ myReview });
    mountHook(() => useMyReview(undefined, BOOK), testQueryClient());
    await act(async () => {});
    expect(myReview).not.toHaveBeenCalled();
  });
});

describe("yazma", () => {
  it("cevap benim incelememe yazılır; bu kitabın bütün liste sayfaları eskir, başka kitabınki ve puan değil", async () => {
    stub({ saveReview: async () => review({ body: "Yeni", version: 2 }) });
    const client = seeded();
    const hook = mountHook(() => useSaveReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync({ body: "Yeni", has_spoiler: false, version: 1 }));
    expect(client.getQueryData<Review>(MINE)).toMatchObject({ body: "Yeni", version: 2 });
    expect(invalidated(client, MINE)).toBe(false);
    expect([invalidated(client, PAGE_1), invalidated(client, PAGE_2)]).toEqual([true, true]);
    expect(invalidated(client, OTHER_BOOK)).toBe(false);
    expect(invalidated(client, SUMMARY)).toBe(false);
  });

  // Düzenleyici "benim incelemem"i izler (gözlemci var); eskitilen kayıt ancak öyle yeniden çekilir.
  it("409'da benim incelemem yeniden okunur, hata SONRA yayılır; liste eskimez", async () => {
    const fresh = review({ body: "Öteki sekme", version: 2 });
    const myReview = vi.fn(async () => review());
    stub({ saveReview: fail(409, "version_conflict"), myReview });
    const client = seeded();
    const hook = mountHook(() => ({ mine: useMyReview(USER, BOOK), save: useSaveReview(USER, BOOK) }), client);
    await act(async () => {}); // açılıştaki okuma biter
    myReview.mockImplementation(async () => fresh);
    let seenOnError: unknown;
    await settle(() =>
      hook.current!.save.mutateAsync({ body: "Benim", has_spoiler: false, version: 1 }).catch((e) => {
        seenOnError = client.getQueryData(MINE);
        throw e;
      }),
    );
    expect(seenOnError).toEqual(fresh);
    expect(invalidated(client, PAGE_1)).toBe(false);
  });

  it.each([
    ["400", 400, "validation_error"],
    ["kitap yayında değil (404)", 404, "not_found"],
  ])("%s hiçbir şeyi eskitmez", async (_name, status, code) => {
    stub({ saveReview: fail(status, code) });
    const client = seeded();
    const hook = mountHook(() => useSaveReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync({ body: "x", has_spoiler: false }));
    expect([invalidated(client, MINE), invalidated(client, PAGE_1)]).toEqual([false, false]);
  });
});

describe("silme", () => {
  it.each([
    ["başarı", async () => undefined],
    ["başka sekmede zaten silinmiş (no_review)", fail(404, "no_review")],
  ])("%s: benim incelemem null, liste eskir", async (_name, deleteReview) => {
    stub({ deleteReview });
    const client = seeded();
    const hook = mountHook(() => useDeleteReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync());
    expect(client.getQueryData(MINE)).toBeNull();
    expect(invalidated(client, PAGE_1)).toBe(true);
  });

  it("beklenmeyen hata hiçbir şeyi değiştirmez", async () => {
    stub({ deleteReview: fail(500, "internal") });
    const client = seeded();
    const hook = mountHook(() => useDeleteReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync());
    expect(client.getQueryData(MINE)).toEqual(review());
    expect(invalidated(client, PAGE_1)).toBe(false);
  });
});

describe("yönetici", () => {
  it("gizleme doğru ucu gerekçeyle çağırır; liste ve yöneticinin kendi incelemesi eskir", async () => {
    const hideReview = vi.fn(async () => review({ hidden: { at: "2026-11-25T11:00:00Z", reason: "Spam" } }));
    stub({ hideReview });
    const client = seeded();
    const hook = mountHook(() => useModerateReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync({ id: 3, action: "hide", reason: "Spam" }));
    expect(hideReview).toHaveBeenCalledWith(3, "Spam");
    expect([invalidated(client, PAGE_1), invalidated(client, MINE)]).toEqual([true, true]);
    expect(invalidated(client, OTHER_BOOK)).toBe(false);
  });

  it("geri açma hata verse de liste eskir (hayalet satır kalmasın)", async () => {
    const unhideReview = vi.fn(fail(404, "not_found"));
    stub({ unhideReview });
    const client = seeded();
    const hook = mountHook(() => useModerateReview(USER, BOOK), client);
    await settle(() => hook.current!.mutateAsync({ id: 3, action: "unhide" }));
    expect(unhideReview).toHaveBeenCalledWith(3);
    expect(invalidated(client, PAGE_1)).toBe(true);
  });
});
