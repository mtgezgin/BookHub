import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminBook, BookStatus, Page } from "../../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// Gerçek ApiError korunur; api nesnesi testte düz fonksiyonlarla doldurulur.
vi.mock("../../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../api/client")>()),
  api: {},
}));

import { api, ApiError } from "../../api/client";
import { Moderation } from "./Moderation";

const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}

const book = (id: number, status: BookStatus): AdminBook => ({
  id, title: `Kitap ${id}`, slug: `kitap-${id}`, author: "Yazar", description: "", cover_url: "",
  publication_year: null, page_count: null, language: "tr", genres: [], average_rating: null, rating_count: 0,
  created_at: "2026-09-01T00:00:00Z", updated_at: "", status, submitted_by: null, rejection_reason: null,
  moderated_by: null, moderated_at: null,
});
const pageOf = (items: AdminBook[]): Page<AdminBook> => ({ items, total: items.length, limit: 20, offset: 0 });

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  stub({ adminBooks: async () => pageOf([book(1, "PENDING")]) });
});

const flush = async () => {
  for (let i = 0; i < 3; i++) await act(async () => {});
};

async function render() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<Moderation />));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return el;
}
const button = (el: ParentNode, text: string) =>
  Array.from(el.querySelectorAll("button")).find((b) => b.textContent === text);
const alertText = (el: HTMLElement) => el.querySelector("[role=alert]")?.textContent ?? null;

describe("Moderation", () => {
  it("red istemi iptal edilirse (prompt → null) api.reject ÇAĞRILMAZ", async () => {
    stub({ reject: async () => book(1, "REJECTED") });
    vi.stubGlobal("prompt", () => null);
    cleanup.push(() => vi.unstubAllGlobals());
    const el = await render();
    await act(async () => button(el, "Reddet")!.click());
    expect(calls.reject).toBeUndefined();
  });

  it("başarısız işlem (409 invalid_transition) mesajını ErrorBox'ta gösterir", async () => {
    stub({ approve: () => Promise.reject(new ApiError(409, "invalid_transition", "APPROVED ✗ approve")) });
    const el = await render();
    await act(async () => button(el, "Onayla")!.click());
    await flush();
    expect(alertText(el)).toBe("APPROVED ✗ approve");
  });

  it("başarılı işlem sayfayı yeniden yükler", async () => {
    stub({ approve: async () => book(1, "APPROVED") });
    const el = await render();
    const before = calls.adminBooks.length;
    await act(async () => button(el, "Onayla")!.click());
    await flush();
    expect(calls.approve).toEqual([[1]]);
    expect(calls.adminBooks.length).toBe(before + 1);
    expect(alertText(el)).toBeNull();
  });

  it("düğmeler statüye göre: PENDING onay/red, APPROVED arşiv, ARCHIVED geri yayın", async () => {
    stub({ adminBooks: async () => pageOf([book(1, "PENDING"), book(2, "APPROVED"), book(3, "ARCHIVED"), book(4, "REJECTED")]) });
    const el = await render();
    const labels = Array.from(el.querySelectorAll("ul.queue > li")).map((li) =>
      Array.from(li.querySelectorAll("button")).map((b) => b.textContent),
    );
    expect(labels).toEqual([["Onayla", "Reddet"], ["Arşivle"], ["Geri yayınla"], []]);
  });

  it("sekme değiştirmek görünen hatayı temizler", async () => {
    stub({ approve: () => Promise.reject(new ApiError(409, "invalid_transition", "geçersiz geçiş")) });
    const el = await render();
    await act(async () => button(el, "Onayla")!.click());
    await flush();
    expect(alertText(el)).toBe("geçersiz geçiş");
    await act(async () => button(el, "Yayında")!.click());
    await flush();
    expect(alertText(el)).toBeNull();
    expect(calls.adminBooks.at(-1)).toEqual(["APPROVED", 0]);
  });

  it.each(["spam", ""])("red istemi %j dönerse api.reject o gerekçeyle çağrılır", async (reason) => {
    stub({ reject: async () => book(1, "REJECTED") });
    vi.stubGlobal("prompt", () => reason);
    cleanup.push(() => vi.unstubAllGlobals());
    const el = await render();
    const before = calls.adminBooks.length;
    await act(async () => button(el, "Reddet")!.click());
    await flush();
    expect(calls.reject).toEqual([[1, reason]]);
    expect(calls.adminBooks.length).toBe(before + 1);
  });

  it("\"Geri yayınla\" api.restoreBook'u çağırır ve listeyi yeniden yükler", async () => {
    stub({ adminBooks: async () => pageOf([book(3, "ARCHIVED")]), restoreBook: async () => book(3, "APPROVED") });
    const el = await render();
    const before = calls.adminBooks.length;
    await act(async () => button(el, "Geri yayınla")!.click());
    await flush();
    expect(calls.restoreBook).toEqual([[3]]);
    expect(calls.adminBooks.length).toBe(before + 1);
  });

  it("red gerekçesi olan kitapta \"Red gerekçesi: …\" satırı çizilir", async () => {
    stub({ adminBooks: async () => pageOf([{ ...book(4, "REJECTED"), rejection_reason: "kopya içerik" }, book(5, "REJECTED")]) });
    const el = await render();
    const items = Array.from(el.querySelectorAll("ul.queue > li"));
    expect(items[0].querySelector("p")!.textContent).toBe("Red gerekçesi: kopya içerik");
    expect(items[1].querySelector("p")).toBeNull();
  });

  it("boş kuyruk \"Bu listede kitap yok.\" gösterir", async () => {
    stub({ adminBooks: async () => pageOf([]) });
    const el = await render();
    expect(el.textContent).toContain("Bu listede kitap yok.");
    expect(el.querySelectorAll("ul.queue > li")).toHaveLength(0);
  });

  async function submitGenre(el: HTMLElement, name: string) {
    const input = el.querySelector<HTMLInputElement>("form.search input")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, name);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => {
      input.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await flush();
    return input;
  }

  it("tür oluşturma başarılıysa girdiyi temizler", async () => {
    stub({ createGenre: async (name: string) => ({ id: 9, name, slug: "bilim-kurgu" }) });
    const el = await render();
    const input = await submitGenre(el, "Bilim Kurgu");
    expect(calls.createGenre).toEqual([["Bilim Kurgu"]]);
    expect(input.value).toBe("");
  });

  it("tür oluşturma hata verirse girdi korunur", async () => {
    stub({ createGenre: () => Promise.reject(new ApiError(409, "genre_conflict", "bu tür zaten var")) });
    const el = await render();
    const input = await submitGenre(el, "Roman");
    expect(input.value).toBe("Roman");
    expect(alertText(el)).toBe("bu tür zaten var");
  });
});
