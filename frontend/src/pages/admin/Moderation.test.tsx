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
import { ToastProvider } from "../../components/toast";
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
  act(() => root.render(<ToastProvider><Moderation /></ToastProvider>));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return el;
}
const button = (el: ParentNode, text: string) =>
  Array.from(el.querySelectorAll("button")).find((b) => b.textContent === text);
const QUEUE = 'ul[aria-label="Moderasyon kuyruğu"] > li';
const alertText = (el: HTMLElement) => el.querySelector("[role=alert]")?.textContent ?? null;
// Diyalog document.body'ye "portal"la çizilir (sayfanın kutusunun dışında).
const dialog = () => document.querySelector<HTMLElement>("[role=dialog]");
const dialogButton = (text: string) => Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === text)!;

describe("Moderation", () => {
  it("\"Reddet\" gerekçe soran bir diyalog açar; \"Vazgeç\" api.reject'i ÇAĞIRMAZ", async () => {
    stub({ reject: async () => book(1, "REJECTED") });
    const el = await render();
    await act(async () => button(el, "Reddet")!.click());
    expect(dialog()!.textContent).toContain("geri alınamaz");
    await act(async () => dialogButton("Vazgeç").click());
    expect(dialog()).toBeNull();
    expect(calls.reject).toBeUndefined();
  });

  it("başarılı işlem kısa bir bildirimle onaylanır", async () => {
    stub({ approve: async () => book(1, "APPROVED") });
    const el = await render();
    await act(async () => button(el, "Onayla")!.click());
    await flush();
    expect(el.querySelector('[aria-label="Bildirimler"]')!.textContent).toContain("“Kitap 1” yayına alındı");
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
    const labels = Array.from(el.querySelectorAll(QUEUE)).map((li) =>
      Array.from(li.querySelectorAll("button")).map((b) => b.textContent),
    );
    expect(labels).toEqual([["Onayla", "Reddet"], ["Arşivle"], ["Geri yayınla"], []]);
  });

  it("seçili sekme aria-pressed=true, diğerleri false", async () => {
    const el = await render();
    expect(button(el, "Bekleyen")!.getAttribute("aria-pressed")).toBe("true");
    expect(button(el, "Yayında")!.getAttribute("aria-pressed")).toBe("false");
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

  it.each(["spam", ""])("gerekçe %j ile onaylanınca api.reject o gerekçeyle çağrılır, diyalog kapanır", async (reason) => {
    stub({ reject: async () => book(1, "REJECTED") });
    const el = await render();
    const before = calls.adminBooks.length;
    await act(async () => button(el, "Reddet")!.click());
    const ta = dialog()!.querySelector("textarea")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(ta, reason);
      ta.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => dialogButton("Reddet").click());
    await flush();
    expect(calls.reject).toEqual([[1, reason]]);
    expect(calls.adminBooks.length).toBe(before + 1);
    expect(dialog()).toBeNull();
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

  it("\"Arşivle\" api.archive'ı çağırır ve listeyi yeniden yükler", async () => {
    stub({ adminBooks: async () => pageOf([book(2, "APPROVED")]), archive: async () => book(2, "ARCHIVED") });
    const el = await render();
    const before = calls.adminBooks.length;
    await act(async () => button(el, "Arşivle")!.click());
    await flush();
    expect(calls.archive).toEqual([[2]]);
    expect(calls.adminBooks.length).toBe(before + 1);
  });

  it("red gerekçesi olan kitapta \"Red gerekçesi: …\" satırı çizilir", async () => {
    stub({ adminBooks: async () => pageOf([{ ...book(4, "REJECTED"), rejection_reason: "kopya içerik" }, book(5, "REJECTED")]) });
    const el = await render();
    const items = Array.from(el.querySelectorAll(QUEUE));
    expect(items[0].querySelector("p")!.textContent).toBe("Red gerekçesi: kopya içerik");
    expect(items[1].querySelector("p")).toBeNull();
  });

  it("boş kuyruk \"Bu listede kitap yok.\" gösterir", async () => {
    stub({ adminBooks: async () => pageOf([]) });
    const el = await render();
    expect(el.textContent).toContain("Bu listede kitap yok.");
    expect(el.querySelectorAll(QUEUE)).toHaveLength(0);
  });

  async function submitGenre(el: HTMLElement, name: string) {
    const input = el.querySelector<HTMLInputElement>('form[aria-label="Yeni tür"] input')!;
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

  it("işlem sürerken aynı düğmeye ikinci tıklama ikinci isteği göndermez; hata sonrası düğme yeniden kullanılır", async () => {
    let fail!: () => void;
    stub({ approve: () => new Promise((_, rej) => { fail = () => rej(new ApiError(500, "internal", "onaylanamadı")); }) });
    const el = await render();
    await act(async () => {
      button(el, "Onayla")!.click();
      button(el, "Onayla")!.click();
    });
    expect(calls.approve).toHaveLength(1);
    expect(button(el, "Onayla")!.disabled).toBe(true);
    await act(async () => fail());
    await flush();
    expect(alertText(el)).toBe("onaylanamadı");
    expect(button(el, "Onayla")!.disabled).toBe(false);
  });

  it("tür formu iki kez gönderilse de tek istek gider", async () => {
    let done!: () => void;
    stub({ createGenre: (name: string) => new Promise((res) => { done = () => res({ id: 9, name, slug: "roman" }); }) });
    const el = await render();
    const input = el.querySelector<HTMLInputElement>('form[aria-label="Yeni tür"] input')!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, "Roman");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => {
      input.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      input.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(calls.createGenre).toHaveLength(1);
    await act(async () => done());
    await flush();
  });

  it("başarılı işlem bittiğinde düğmeler yeniden kullanılır: sonraki tıklama yeni istek gönderir", async () => {
    let succeed!: () => void;
    stub({ approve: () => new Promise((res) => { succeed = () => res(book(1, "APPROVED")); }) });
    const el = await render();
    await act(async () => button(el, "Onayla")!.click());
    expect(button(el, "Onayla")!.disabled).toBe(true);
    await act(async () => succeed());
    await flush();
    expect(button(el, "Onayla")!.disabled).toBe(false);
    expect(button(el, "Reddet")!.disabled).toBe(false);
    await act(async () => button(el, "Onayla")!.click());
    expect(calls.approve).toHaveLength(2);
    await act(async () => succeed());
    await flush();
  });

  it("koruma liste yeniden yüklenene kadar sürer: istek dönse de satır güncellenmeden ikinci onay gitmez", async () => {
    stub({ approve: async () => book(1, "APPROVED") });
    const el = await render();
    let reloaded!: () => void;
    stub({ adminBooks: () => new Promise((res) => { reloaded = () => res(pageOf([])); }) });
    await act(async () => button(el, "Onayla")!.click());
    await flush();
    await act(async () => button(el, "Onayla")?.click());
    expect(calls.approve).toHaveLength(1);
    expect(button(el, "Onayla")!.disabled).toBe(true);
    await act(async () => reloaded());
    await flush();
    expect(button(el, "Onayla")).toBeUndefined();
  });
});
