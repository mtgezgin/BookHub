import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { testQueryClient } from "../test/render"; // ayrıca notifyManager.setScheduler(queueMicrotask)

vi.mock("./client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./client")>()),
  api: {},
}));

import { api, ApiError } from "./client";
import { useRemoveFromLibrary, useSetLibraryStatus } from "./libraryQueries";
import { queryKeys, useRateBook } from "./queries";

/*
 * Kitaplık değişikliklerinin ve puanın GEÇERSİZ KILMA kuralları (Faz 16d adım 7). Ekranda OLMAYAN
 * kayıtlara (öteki filtrelerin listeleri, katalog rozetleri, başka kullanıcının kayıtları) ne olduğuna
 * bakıyoruz; kayıt geçersizse bir sonraki açılışta yeniden çekilir. Ekrandaki kaydın yeniden çekildiği
 * LibraryButton.test ve Library.test'te.
 */

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

function mountHook<T>(useHook: () => T, client: QueryClient) {
  const box: { current?: T } = {};
  function Harness() {
    box.current = useHook();
    return null;
  }
  const root = createRoot(document.createElement("div"));
  act(() => root.render(<QueryClientProvider client={client}><Harness /></QueryClientProvider>));
  cleanup.push(() => act(() => root.unmount()));
  return box;
}

const EMPTY_PAGE = { items: [], total: 0, limit: 12, offset: 0 };
const LIST = queryKeys.library(7, { limit: 12, offset: 0 });
const BADGES = queryKeys.libraryStatuses(7, "3,4");
const OTHER_USER = queryKeys.library(8, { limit: 12, offset: 0 });
const CATALOG = queryKeys.books({ limit: 20 });

// Önbelleğe ekranda olmayan (gözlemcisiz) kayıtlar koyar: kullanıcı 7'nin listesi ve rozetleri, kullanıcı
// 8'in listesi, katalog.
function seeded() {
  const client = testQueryClient();
  client.setQueryData(LIST, EMPTY_PAGE);
  client.setQueryData(BADGES, { statuses: {} });
  client.setQueryData(OTHER_USER, EMPTY_PAGE);
  client.setQueryData(CATALOG, { items: [], total: 0, limit: 20, offset: 0 });
  return client;
}
const invalidated = (client: QueryClient, key: readonly unknown[]) => client.getQueryState(key)!.isInvalidated;
const stub = (impls: Record<string, unknown>) => Object.assign(api as unknown as Record<string, unknown>, impls);

describe("kitaplık değişiklikleri", () => {
  it("rafa ekleme bu kullanıcının bütün kitaplık kayıtlarını eskitir; başka kullanıcınınkini ve kataloğu değil", async () => {
    stub({ setLibraryStatus: async () => ({}) });
    const client = seeded();
    const hook = mountHook(() => useSetLibraryStatus(7), client);
    await act(async () => {
      await hook.current!.mutateAsync({ bookId: 3, status: "READ" });
    });
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(true);
    expect(invalidated(client, OTHER_USER)).toBe(false);
    expect(invalidated(client, CATALOG)).toBe(false);
  });

  it("çıkarma da aynı kayıtları eskitir", async () => {
    stub({ removeFromLibrary: async () => undefined });
    const client = seeded();
    const hook = mountHook(() => useRemoveFromLibrary(7), client);
    await act(async () => {
      await hook.current!.mutateAsync(3);
    });
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(true);
    expect(invalidated(client, OTHER_USER)).toBe(false);
  });

  it("başarısız değişiklik hiçbir şeyi eskitmez", async () => {
    stub({ setLibraryStatus: async () => { throw new ApiError(500, "internal", "x"); } });
    const client = seeded();
    const hook = mountHook(() => useSetLibraryStatus(7), client);
    await act(async () => {
      await hook.current!.mutateAsync({ bookId: 3, status: "READ" }).catch(() => {});
    });
    expect(invalidated(client, LIST)).toBe(false);
    expect(invalidated(client, BADGES)).toBe(false);
  });
});

describe("puan ve kitaplık", () => {
  // Kitaplığım her kartta "Puanım"ı gösteriyor ve puana göre süzüp sıralıyor; rozetler puanı göstermiyor.
  it("puan vermek Kitaplığım listelerini eskitir, rozetleri ve başka kullanıcınınkini değil", async () => {
    stub({ rate: async () => ({ score: 8 }), ratingSummary: async () => ({}) });
    const client = seeded();
    const hook = mountHook(() => useRateBook(3, 7), client);
    await act(async () => {
      await hook.current!.mutateAsync(8);
    });
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(false);
    expect(invalidated(client, OTHER_USER)).toBe(false);
  });
});
