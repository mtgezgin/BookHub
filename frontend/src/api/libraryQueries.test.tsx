import type { QueryClient } from "@tanstack/react-query";
import { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { mountHook, testQueryClient } from "../test/render"; // ayrıca notifyManager.setScheduler(queueMicrotask)

vi.mock("./client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./client")>()),
  api: {},
}));

import { api, ApiError } from "./client";
import {
  useDeleteReadingLog, useRemoveFromLibrary, useShelf, useShelfCommand, useUpdateReading,
  useUpdateReadingLog,
} from "./libraryQueries";
import { queryKeys, useRateBook } from "./queries";
import type { Shelf } from "./types";

/*
 * Kitaplık değişikliklerinin ve puanın GEÇERSİZ KILMA kuralları (Faz 16d adım 7). Ekranda OLMAYAN
 * kayıtlara (öteki filtrelerin listeleri, katalog rozetleri, başka kullanıcının kayıtları) ne olduğuna
 * bakıyoruz; kayıt geçersizse bir sonraki açılışta yeniden çekilir. Ekrandaki kaydın yeniden çekildiği
 * LibraryButton.test ve Library.test'te.
 */

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
  it("çıkarma bu kullanıcının bütün kitaplık kayıtlarını eskitir; başka kullanıcınınkini ve kataloğu değil", async () => {
    stub({ removeFromLibrary: async () => undefined });
    const client = seeded();
    const hook = mountHook(() => useRemoveFromLibrary(7), client);
    await act(async () => {
      await hook.current!.mutateAsync(3);
    });
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(true);
    expect(invalidated(client, OTHER_USER)).toBe(false);
    expect(invalidated(client, CATALOG)).toBe(false);
  });

  it("başarısız çıkarma hiçbir şeyi eskitmez", async () => {
    stub({ removeFromLibrary: async () => { throw new ApiError(500, "internal", "x"); } });
    const client = seeded();
    const hook = mountHook(() => useRemoveFromLibrary(7), client);
    await act(async () => {
      await hook.current!.mutateAsync(3).catch(() => {});
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

/*
 * RAF değişiklikleri (Faz 17A-1 adım 4): cevap rafa DOĞRUDAN yazılır, öteki görünümler komuta göre eskir,
 * hatada yalnız "raf eskimiş" diyen cevaplar (409, 404) rafı yeniden okutur.
 */
const SHELF = queryKeys.shelf(7, 3);
const OTHER_SHELF = queryKeys.shelf(7, 4);

function shelfAt(version: number, progress = 10): Shelf {
  return {
    book_id: 3,
    entry: {
      status: "READING", version, added_at: "2026-11-01T10:00:00Z", updated_at: "2026-11-01T10:00:00Z",
      reading: { edition_id: 1, unit: "page", progress, total_pages: 310, started_on: "2026-11-01" },
    },
    log: [],
  };
}

function seededShelf() {
  const client = seeded();
  client.setQueryData(SHELF, shelfAt(1));
  client.setQueryData(OTHER_SHELF, { ...shelfAt(1), book_id: 4 });
  return client;
}

describe("raf", () => {
  it("girişsiz kullanıcıda raf sorgusu hiç çalışmaz", async () => {
    const shelf = vi.fn(async () => shelfAt(1));
    stub({ shelf });
    mountHook(() => useShelf(undefined, 3), testQueryClient());
    await act(async () => {});
    expect(shelf).not.toHaveBeenCalled();
  });

  it("komutun cevabı rafa doğrudan yazılır; liste ve rozetler eskir, raflar ve öteki kullanıcı eskimez", async () => {
    const shelfCommand = vi.fn(async () => shelfAt(2));
    stub({ shelfCommand });
    const client = seededShelf();
    const hook = mountHook(() => useShelfCommand(7, 3), client);
    await act(async () => {
      await hook.current!.mutateAsync({ command: "finish", body: { ended_on: "2026-11-24" } });
    });
    expect(shelfCommand).toHaveBeenCalledWith(3, { command: "finish", body: { ended_on: "2026-11-24" } });
    expect(client.getQueryData(SHELF)).toEqual(shelfAt(2));
    expect(invalidated(client, SHELF)).toBe(false); // yeniden sorulmaz: değeri biliyoruz
    expect(invalidated(client, OTHER_SHELF)).toBe(false);
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(true);
    expect(invalidated(client, OTHER_USER)).toBe(false);
    expect(invalidated(client, CATALOG)).toBe(false);
  });

  it("ilerleme düzeltmesi yalnız Kitaplığım listesini eskitir (durum değişmez, rozetler aynı)", async () => {
    stub({ updateReading: async () => shelfAt(2, 42) });
    const client = seededShelf();
    const hook = mountHook(() => useUpdateReading(7, 3), client);
    await act(async () => {
      await hook.current!.mutateAsync({ version: 1, progress: 42 });
    });
    expect(client.getQueryData<Shelf>(SHELF)!.entry!.reading!.progress).toBe(42);
    expect(invalidated(client, LIST)).toBe(true);
    expect(invalidated(client, BADGES)).toBe(false);
  });

  it("geçmiş değişiklikleri yalnız rafı yazar; liste ve rozetler eskimez", async () => {
    const withLog = { ...shelfAt(1), log: [] };
    stub({ updateReadingLog: async () => withLog, deleteReadingLog: async () => withLog });
    const client = seededShelf();
    const hook = mountHook(() => ({ dates: useUpdateReadingLog(7, 3), del: useDeleteReadingLog(7, 3) }), client);
    await act(async () => {
      await hook.current!.dates.mutateAsync({ logId: 9, dates: { ended_on: "2026-11-12" } });
      await hook.current!.del.mutateAsync(9);
    });
    expect(client.getQueryData(SHELF)).toEqual(withLog);
    expect(invalidated(client, LIST)).toBe(false);
    expect(invalidated(client, BADGES)).toBe(false);
  });

  it("cevap, cevabın söylediği kitabın rafına yazılır", async () => {
    stub({ deleteReadingLog: async () => ({ ...shelfAt(5), book_id: 4 }) });
    const client = seededShelf();
    const hook = mountHook(() => useDeleteReadingLog(7, 4), client);
    await act(async () => {
      await hook.current!.mutateAsync(9);
    });
    expect(client.getQueryData<Shelf>(OTHER_SHELF)!.entry!.version).toBe(5);
    expect(client.getQueryData<Shelf>(SHELF)!.entry!.version).toBe(1);
  });

  // Spec § Kararlar "409 cevabı": gövdede güncel raf yok, istemci bir kez GET yapar. Arayüz hatayı
  // gördüğünde raf zaten yeni olmalı ki mesajı ("sayfa 57") yeni veriden kursun.
  it.each([
    [409, "version_conflict"],
    [404, "not_found"],
  ])("%i %s: raf yeniden okunur ve hata ancak raf geldikten sonra yayılır", async (status, code) => {
    // İkinci GET bir zamanlayıcı turu sonra döner: hemen çözülseydi, kanca onu beklemese de hata
    // yayılmadan biterdi ve test beklemeyi kanıtlamazdı (bozma denemesinde öyle oldu).
    const shelf = vi.fn()
      .mockResolvedValueOnce(shelfAt(1))
      .mockImplementationOnce(() => new Promise<Shelf>((r) => setTimeout(() => r(shelfAt(2, 57)), 0)));
    stub({ shelf, updateReading: async () => { throw new ApiError(status, code, "x"); } });
    const client = testQueryClient();
    const hook = mountHook(() => ({ shelf: useShelf(7, 3), patch: useUpdateReading(7, 3) }), client);
    await act(async () => {});
    expect(hook.current!.shelf.data?.entry?.version).toBe(1);

    let seenAtError: Shelf | undefined;
    await act(async () => {
      await hook.current!.patch.mutateAsync({ version: 1, progress: 10 }).catch(() => {
        seenAtError = client.getQueryData<Shelf>(SHELF);
      });
    });
    expect(shelf).toHaveBeenCalledTimes(2);
    expect(seenAtError?.entry?.reading?.progress).toBe(57);
    expect(hook.current!.patch.error).toMatchObject({ code });
  });

  it.each([
    [400, "invalid_progress"],
    [500, "internal"],
  ])("%i %s rafı ve öteki görünümleri eskitmez", async (status, code) => {
    stub({ updateReading: async () => { throw new ApiError(status, code, "x"); } });
    const client = seededShelf();
    const hook = mountHook(() => useUpdateReading(7, 3), client);
    await act(async () => {
      await hook.current!.mutateAsync({ version: 1, progress: -1 }).catch(() => {});
    });
    expect(invalidated(client, SHELF)).toBe(false);
    expect(invalidated(client, LIST)).toBe(false);
    expect(client.getQueryData(SHELF)).toEqual(shelfAt(1));
  });

  // Komuttan ÖNCE başlamış bir GET (ör. pencereye dönünce tazeleme) komuttan SONRA dönerse eski rafı
  // yazmamalı: kanca yazmadan önce süren okumayı iptal eder.
  it("komuttan önce başlamış raf okuması komutun cevabını ezmez", async () => {
    let finishOld: (s: Shelf) => void = () => {};
    const shelf = vi.fn()
      .mockResolvedValueOnce(shelfAt(1))
      .mockImplementationOnce(() => new Promise<Shelf>((r) => (finishOld = r)));
    stub({ shelf, shelfCommand: async () => shelfAt(3) });
    const client = testQueryClient();
    const hook = mountHook(() => ({ shelf: useShelf(7, 3), cmd: useShelfCommand(7, 3) }), client);
    await act(async () => {});

    await act(async () => {
      void client.refetchQueries({ queryKey: SHELF });
      await hook.current!.cmd.mutateAsync({ command: "start" });
    });
    await act(async () => finishOld(shelfAt(2)));
    expect(shelf).toHaveBeenCalledTimes(2);
    expect(client.getQueryData<Shelf>(SHELF)!.entry!.version).toBe(3);
  });
});
