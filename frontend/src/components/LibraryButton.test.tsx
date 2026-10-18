import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryStatus, User } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
// Radix menüsünün konumlandırıcısı ResizeObserver ister; jsdom'da yok.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ user: null as unknown }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ user: h.user, ready: true }) }));

import { api, ApiError } from "../api/client";
import { LibraryButton } from "./LibraryButton";
import { ToastProvider } from "./toast";

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}
const entry = (status: LibraryStatus) => ({ book_id: 12, status, added_at: "", updated_at: "" });
function statusesOf(status?: LibraryStatus) {
  return async () => ({ statuses: status ? { "12": status } : {} });
}

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
// Menü tembel yüklenen bir parça (LibraryButton → lazy LibraryMenu): modül önceden yüklenince
// lazy bir mikro görevde çözülür ve testler ilk render'da modül derlenmesini beklemez.
beforeAll(async () => {
  await import("./LibraryMenu");
});
beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  h.user = USER;
  stub({
    libraryStatuses: statusesOf(),
    setLibraryStatus: async (_id: number, s: LibraryStatus) => entry(s),
    removeFromLibrary: async () => undefined,
  });
});

const errorToast = () =>
  document.querySelector("section[aria-label=Bildirimler] .text-destructive")?.textContent;

const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => {});
};

function FromProbe() {
  const from = (useLocation().state as { from?: { pathname: string } } | null)?.from?.pathname;
  return <p data-from={from}>login</p>;
}

let rerender: () => void = () => {};
async function render() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  const tree = () => (
      <ToastProvider>
        <MemoryRouter initialEntries={["/books/12"]}>
          <Routes>
            <Route path="/books/:id" element={<LibraryButton bookId={12} />} />
            <Route path="/login" element={<FromProbe />} />
          </Routes>
        </MemoryRouter>
      </ToastProvider>
  );
  act(() => root.render(tree()));
  rerender = () => act(() => root.render(tree()));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return el;
}

const byText = (text: string) =>
  Array.from(document.querySelectorAll<HTMLElement>("button, a, [role^=menuitem]")).find((b) => b.textContent?.trim() === text);
const trigger = () => document.querySelector<HTMLButtonElement>("button[aria-haspopup=menu]")!;
async function openMenu() {
  await act(async () => {
    trigger().dispatchEvent(new PointerEvent("pointerdown", { bubbles: true, button: 0 }));
  });
}
async function click(target: HTMLElement | undefined) {
  expect(target, "tıklanacak öğe bulunamadı").toBeTruthy();
  await act(async () => target!.click());
  await flush();
}

describe("LibraryButton", () => {
  it("giriş yoksa giriş bağlantısı, dönüş adresi bu kitap", async () => {
    h.user = null;
    const el = await render();
    expect(calls.libraryStatuses).toBeUndefined();
    await click(byText("Kitaplığa eklemek için giriş yap"));
    expect(el.querySelector("[data-from]")?.getAttribute("data-from")).toBe("/books/12");
  });

  it("kitaplıkta yoksa tek tıkla Okumak istiyorum", async () => {
    await render();
    expect(calls.libraryStatuses).toEqual([[[12]]]);
    await click(byText("Okumak istiyorum"));
    expect(calls.setLibraryStatus).toEqual([[12, "WANT_TO_READ"]]);
    expect(trigger().textContent).toContain("Kitaplığında");
    expect(trigger().textContent).toContain("Okumak istiyorum");
  });

  it("menüden durum değiştirilir", async () => {
    stub({ libraryStatuses: statusesOf("READING") });
    await render();
    expect(trigger().textContent).toContain("Şu an okuyorum");
    await openMenu();
    const current = Array.from(document.querySelectorAll("[role=menuitemradio]")).find((i) => i.textContent?.includes("Şu an okuyorum"));
    expect(current?.getAttribute("aria-checked")).toBe("true");
    await click(byText("Okudum"));
    expect(calls.setLibraryStatus).toEqual([[12, "READ"]]);
    expect(trigger().textContent).toContain("Okudum");
  });

  it("çıkarmadan önce onay; vazgeçince çağrı yok, onaylayınca çıkar", async () => {
    stub({ libraryStatuses: statusesOf("READ") });
    await render();
    await openMenu();
    await click(byText("Kitaplığımdan çıkar"));
    expect(document.querySelector("[role=dialog]")?.textContent).toContain("Puanın ve yorumların silinmez");
    await click(byText("Vazgeç"));
    expect(calls.removeFromLibrary).toBeUndefined();

    await openMenu();
    await click(byText("Kitaplığımdan çıkar"));
    await click(byText("Çıkar"));
    expect(calls.removeFromLibrary).toEqual([[12]]);
    expect(byText("Okumak istiyorum")).toBeTruthy();
  });

  it("token yenilenince (aynı kullanıcı, yeni nesne) durum yeniden okunmaz", async () => {
    stub({ libraryStatuses: statusesOf("READ") });
    await render();
    h.user = { ...USER };
    rerender();
    await flush();
    expect(calls.libraryStatuses).toEqual([[[12]]]);
    expect(trigger().textContent).toContain("Okudum");
  });

  it("istek sürerken menü pasif", async () => {
    let finish: (v: unknown) => void = () => {};
    stub({ setLibraryStatus: () => new Promise((r) => (finish = r)) });
    await render();
    await click(byText("Okumak istiyorum"));
    expect((byText("Okumak istiyorum") as HTMLButtonElement).disabled).toBe(true);
    expect(trigger().disabled).toBe(true);
    // Öznitelik yetmez: Radix menüyü pointerdown ile açıyor ve tarayıcı pasif düğmede de bu
    // olayı tetikliyor. Menü gerçekten AÇILMAMALI, yoksa ikinci bir PUT ilkiyle yarışır.
    await openMenu();
    expect(document.querySelector("[role=menuitemradio]")).toBeNull();
    await act(async () => finish(entry("WANT_TO_READ")));
    await flush();
    expect(trigger().disabled).toBe(false);
  });

  it("ekleme başarısızsa hata bildirimi, bölünmüş düğme kalır", async () => {
    stub({ setLibraryStatus: async () => { throw new ApiError(500, "internal", "Sunucu hatası oluştu"); } });
    await render();
    await click(byText("Okumak istiyorum"));
    expect(errorToast()).toContain("Sunucu hatası oluştu");
    expect(byText("Okumak istiyorum")).toBeTruthy();
    expect(trigger().textContent).not.toContain("Kitaplığında");
  });

  it("çıkarma başarısızsa hata bildirimi, Kitaplığında düğmesi kalır", async () => {
    stub({
      libraryStatuses: statusesOf("READ"),
      removeFromLibrary: async () => { throw new ApiError(500, "internal", "Çıkarılamadı"); },
    });
    await render();
    await openMenu();
    await click(byText("Kitaplığımdan çıkar"));
    await click(byText("Çıkar"));
    expect(calls.removeFromLibrary).toEqual([[12]]);
    expect(errorToast()).toContain("Çıkarılamadı");
    expect(trigger().textContent).toContain("Kitaplığında");
    expect(trigger().textContent).toContain("Okudum");
  });

  // Durum bilinmiyorken "Okumak istiyorum" gösterilseydi tek tıklama kayıtlı bir "Okudum"un üstüne
  // yazardı (PUT, ON CONFLICT ile durumu değiştirir). Bilinmeyen durum, ekleme değil tekrar dene demek.
  it("durum okunamazsa ekleme yok, tekrar dene var; tekrar başarılıysa gerçek durum görünür", async () => {
    let fail = true;
    stub({
      libraryStatuses: async () => {
        if (fail) throw new ApiError(500, "internal", "x");
        return { statuses: { "12": "READ" } };
      },
    });
    await render();
    expect(calls.libraryStatuses).toEqual([[[12]]]);
    expect(byText("Okumak istiyorum")).toBeUndefined();
    fail = false;
    await click(byText("Kitaplık durumu yüklenemedi, tekrar dene"));
    expect(calls.libraryStatuses).toEqual([[[12]], [[12]]]);
    expect(trigger().textContent).toContain("Okudum");
    expect(calls.setLibraryStatus).toBeUndefined();
  });
});
