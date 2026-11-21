import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryStatus, User } from "../api/types";
import { testQueryClient, withQuery } from "../test/render";

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
import { useLibraryStatuses } from "./useLibraryStatuses";

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
// Sunucudaki kayıt: değişiklikten sonra menü durumu sunucudan YENİDEN okuyor (adım 7), o yüzden sahte uç
// noktalar sabit cevap değil, yazılanı hatırlayan küçük bir sunucu gibi davranıyor.
let shelf: Record<string, LibraryStatus> = {};

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
  shelf = {};
  stub({
    libraryStatuses: async (ids: number[]) => ({
      statuses: Object.fromEntries(ids.filter((id) => shelf[id]).map((id) => [String(id), shelf[id]])),
    }),
    setLibraryStatus: async (id: number, s: LibraryStatus) => {
      shelf[id] = s;
      return entry(s);
    },
    removeFromLibrary: async (id: number) => {
      delete shelf[id];
    },
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

// Aynı sayfada başka bir bileşen: katalog kartlarının rozet kancası (Home ve benzer kitaplar şeridi).
function BadgeProbe() {
  const statuses = useLibraryStatuses([12, 13]);
  return <p data-badges>{`12:${statuses[12] ?? "-"} 13:${statuses[13] ?? "-"}`}</p>;
}

let rerender: () => void = () => {};
async function render(opts: { badges?: boolean } = {}) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  const client = testQueryClient();
  const tree = () => withQuery(
      <ToastProvider>
        <MemoryRouter initialEntries={["/books/12"]}>
          <Routes>
            <Route path="/books/:id" element={<>{opts.badges && <BadgeProbe />}<LibraryButton bookId={12} /></>} />
            <Route path="/login" element={<FromProbe />} />
          </Routes>
        </MemoryRouter>
      </ToastProvider>,
      client,
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
    shelf = { 12: "READING" };
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
    shelf = { 12: "READ" };
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
    shelf = { 12: "READ" };
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
    // Sunucu yazmayı ancak cevap verirken bitirmiş olsun.
    const done = (s: LibraryStatus) => { shelf[12] = s; finish(entry(s)); };
    await render();
    await click(byText("Okumak istiyorum"));
    expect((byText("Okumak istiyorum") as HTMLButtonElement).disabled).toBe(true);
    expect(trigger().disabled).toBe(true);
    // Öznitelik yetmez: Radix menüyü pointerdown ile açıyor ve tarayıcı pasif düğmede de bu
    // olayı tetikliyor. Menü gerçekten AÇILMAMALI, yoksa ikinci bir PUT ilkiyle yarışır.
    await openMenu();
    expect(document.querySelector("[role=menuitemradio]")).toBeNull();
    await act(async () => done("WANT_TO_READ"));
    await flush();
    expect(trigger().disabled).toBe(false);
    expect(trigger().textContent).toContain("Kitaplığında");
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
    shelf = { 12: "READ" };
    stub({ removeFromLibrary: async () => { throw new ApiError(500, "internal", "Çıkarılamadı"); } });
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

  // Adımın asıl dersi: menü ile rozet arasında hiçbir bağlantı yok, ortak noktaları yalnız sorgu anahtarı.
  it("menüden eklenen kitabın rozeti aynı sayfadaki başka bir bileşende de güncellenir", async () => {
    shelf = { 13: "READ" };
    const el = await render({ badges: true });
    expect(el.querySelector("[data-badges]")!.textContent).toBe("12:- 13:READ");
    await click(byText("Okumak istiyorum"));
    expect(el.querySelector("[data-badges]")!.textContent).toBe("12:WANT_TO_READ 13:READ");
  });

  it("değişiklikten sonra durum yeniden okunamazsa eklemeye değil tekrar denemeye dönülür", async () => {
    await render();
    stub({ libraryStatuses: async () => { throw new ApiError(500, "internal", "x"); } });
    await click(byText("Okumak istiyorum"));
    expect(calls.setLibraryStatus).toEqual([[12, "WANT_TO_READ"]]);
    expect(byText("Okumak istiyorum")).toBeUndefined();
    expect(byText("Kitaplık durumu yüklenemedi, tekrar dene")).toBeTruthy();
  });
});
