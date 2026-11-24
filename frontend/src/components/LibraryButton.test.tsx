import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { Edition, LibraryStatus, Reading, ReadingLogEntry, Shelf, User } from "../api/types";
import { today } from "../lib/day";
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

/*
 * Kitap sayfasındaki RAF KONTROLÜ (Faz 17A-1 adım 5; LibraryButton tembel yükler). Her test bir raftan
 * başlar ve iki şeye bakar: hangi isteğin (komut + gövde) gittiği ve ekranda ne kaldığı. Durum makinesinin
 * kendisi sunucuda (commands_test.go); burada sahte sunucu yalnız "komuttan sonra raf şu olsun" der.
 */

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const EDITIONS: Edition[] = [
  { id: 1, title: "Dune", language: "tr", translator: "", publisher: "Ithaki", publication_year: 2019, isbn: "", page_count: 310, cover_url: "", is_primary: true },
  { id: 2, title: "Dune", language: "en", translator: "", publisher: "Ace", publication_year: 1990, isbn: "", page_count: 412, cover_url: "", is_primary: false },
];

const readingOf = (over: Partial<Reading> = {}): Reading => ({
  edition_id: 1, unit: "page", progress: 42, total_pages: 310, started_on: "2026-11-01", ...over,
});
const logOf = (over: Partial<ReadingLogEntry> = {}): ReadingLogEntry => ({
  id: 9, edition_id: 1, outcome: "FINISHED", unit: "page", progress: 310, total_pages: 310,
  started_on: "2026-11-01", ended_on: "2026-11-12", created_at: "", ...over,
});
function shelfOf(status: LibraryStatus | null, over: { reading?: Reading | null; log?: ReadingLogEntry[]; version?: number } = {}): Shelf {
  return {
    book_id: 12,
    entry: status && {
      status, version: over.version ?? 3, added_at: "", updated_at: "",
      reading: status === "READING" ? (over.reading === undefined ? readingOf() : over.reading) : null,
    },
    log: over.log ?? [],
  };
}

const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}

// Sahte sunucu: `server` şu anki raf; bir değişiklikten sonra raf `after` olur (verilmişse) ve cevap odur.
let server: Shelf;
let after: Shelf | undefined;
const change = async () => {
  if (after) [server, after] = [after, undefined];
  return server;
};

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
// Kontrol tembel yüklenen bir parça: modül önceden yüklenince lazy bir mikro görevde çözülür.
beforeAll(async () => {
  await import("./ShelfControl");
});
beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  h.user = USER;
  server = shelfOf(null);
  after = undefined;
  stub({
    shelf: async () => server,
    shelfCommand: change,
    updateReading: change,
    updateReadingLog: change,
    deleteReadingLog: change,
    libraryStatuses: async (ids: number[]) => ({
      statuses: Object.fromEntries(ids.filter((id) => id === 12 && server.entry).map((id) => [String(id), server.entry!.status])),
    }),
    myRating: async () => { throw new ApiError(404, "not_rated", "puan yok"); },
  });
});

const errorToast = () => document.querySelector("section[aria-label=Bildirimler] .text-destructive")?.textContent;
const toastText = () => document.querySelector("section[aria-label=Bildirimler]")?.textContent ?? "";

const flush = async () => {
  for (let i = 0; i < 5; i++) await act(async () => {});
};

function FromProbe() {
  const from = (useLocation().state as { from?: { pathname: string } } | null)?.from?.pathname;
  return <p data-from={from}>login</p>;
}

// Aynı sayfada başka bir bileşen: katalog kartlarının rozet kancası (benzer kitaplar şeridi).
function BadgeProbe() {
  const statuses = useLibraryStatuses([12, 13]);
  return <p data-badges>{`12:${statuses[12] ?? "-"}`}</p>;
}

let rerender: () => void = () => {};
async function render(opts: { badges?: boolean; onRate?: () => void } = {}) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  const client = testQueryClient();
  const tree = () => withQuery(
    <ToastProvider>
      <MemoryRouter initialEntries={["/books/12"]}>
        <Routes>
          <Route
            path="/books/:id"
            element={<>{opts.badges && <BadgeProbe />}<LibraryButton bookId={12} editions={EDITIONS} onRate={opts.onRate} /></>}
          />
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
const menuItems = () => Array.from(document.querySelectorAll("[role=menuitem]")).map((i) => i.textContent);
const trigger = () => document.querySelector<HTMLButtonElement>("button[aria-label='Diğer seçenekler']")!;
const group = () => document.querySelector("[role=group][aria-label=Kitaplık]")!;
const dialog = () => document.querySelector("[role=dialog]");
async function openMenu() {
  await act(async () => {
    trigger().dispatchEvent(new PointerEvent("pointerdown", { bubbles: true, button: 0 }));
  });
}
async function click(target: Element | null | undefined) {
  expect(target, "tıklanacak öğe bulunamadı").toBeTruthy();
  await act(async () => (target as HTMLElement).click());
  await flush();
}
async function choose(label: string) {
  await openMenu();
  await click(byText(label));
}
// React kontrollü alanı: değer prototipteki setter'la yazılır, yoksa React değişikliği görmez.
async function type(el: Element | null, value: string) {
  expect(el, "alan bulunamadı").toBeTruthy();
  const proto = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el!.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}
const field = (form: string, label: string) =>
  Array.from(document.querySelectorAll(`form[aria-label='${form}'] label`))
    .find((l) => l.textContent?.startsWith(label))
    ?.querySelector("input, select") ?? null;
async function submit(form: string, button = "Kaydet") {
  await click(Array.from(document.querySelectorAll(`form[aria-label='${form}'] button`)).find((b) => b.textContent === button));
}

describe("raf kontrolü — durumlar ve komutlar", () => {
  it("giriş yoksa giriş bağlantısı, dönüş adresi bu kitap", async () => {
    h.user = null;
    const el = await render();
    expect(calls.shelf).toBeUndefined();
    await click(byText("Kitaplığa eklemek için giriş yap"));
    expect(el.querySelector("[data-from]")?.getAttribute("data-from")).toBe("/books/12");
  });

  it("kitaplıkta değilken: tek tıkla Okumak istiyorum (gövdesiz); menüde başla ve zaten okudum", async () => {
    after = shelfOf("WANT_TO_READ");
    await render();
    expect(calls.shelf).toEqual([[12]]);
    await openMenu();
    expect(menuItems()).toEqual(["Okumaya başla", "Zaten okudum"]);
    await act(async () => document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true })));
    await click(byText("Okumak istiyorum"));
    expect(calls.shelfCommand).toEqual([[12, { command: "want" }]]);
    expect(group().textContent).toContain("Kitaplığında: Okumak istiyorum");
    expect(toastText()).toContain("Okumak istediklerine eklendi");
  });

  it("Okumaya başla bugünün yerel gününü gönderir; Zaten okudum tarihsiz", async () => {
    server = shelfOf("WANT_TO_READ");
    await render();
    await click(byText("Okumaya başla"));
    await choose("Zaten okudum");
    expect(calls.shelfCommand).toEqual([
      [12, { command: "start", body: { started_on: today() } }],
      [12, { command: "mark-read" }],
    ]);
  });

  it("okurken: ilerleme çubuğu ve metni; Bitirdim ve Yarım bıraktım bugünle kapatır", async () => {
    server = shelfOf("READING");
    await render();
    const bar = document.querySelector("[role=progressbar]")!;
    expect(bar.getAttribute("aria-valuenow")).toBe("14"); // 42 / 310
    expect(bar.getAttribute("aria-valuetext")).toBe("42 / 310 sayfa");
    await openMenu();
    expect(menuItems()).toEqual(["Yarım bıraktım", "Ertele", "Okumayı düzenle", "Kitaplığımdan çıkar"]);
    await click(byText("Yarım bıraktım"));
    await click(byText("Bitirdim"));
    expect(calls.shelfCommand).toEqual([
      [12, { command: "abandon", body: { ended_on: today() } }],
      [12, { command: "finish", body: { ended_on: today() } }],
    ]);
  });

  // Spec Doğrulama 8: Faz 15'ten kalan "Şu an okuyorum" (birimsiz) ve toplamı bilinmeyen 0 sayfa.
  it.each([
    ["birimsiz (Faz 15)", readingOf({ unit: null, progress: null, total_pages: null })],
    ["toplamı bilinmeyen 0 sayfa", readingOf({ progress: 0, total_pages: null })],
  ])("%s: çubuk ve '0 sayfa' yok, İlerlemeni gir", async (_name, reading) => {
    server = shelfOf("READING", { reading });
    await render();
    expect(document.querySelector("[role=progressbar]")).toBeNull();
    expect(group().textContent).toContain("İlerlemeni henüz girmedin");
    expect(group().textContent).not.toMatch(/0\. sayfa|0 sayfa/);
    expect(byText("İlerlemeni gir")).toBeTruthy();
  });

  it("okumuşken: bitiş günü ve Yeniden oku; tarihsiz okumada tarih ekle", async () => {
    server = shelfOf("READ", { log: [logOf()] });
    await render();
    expect(group().textContent).toContain("Bitirdin · 12 Kasım 2026");
    expect(byText("tarihi değiştir")).toBeTruthy();
    await click(byText("Yeniden oku"));
    expect(calls.shelfCommand).toEqual([[12, { command: "start", body: { started_on: today() } }]]);
  });

  it("tarihsiz Okudum: 'Okudun · tarih ekle', düğme o kaydın günlerini açar", async () => {
    server = shelfOf("READ", { log: [logOf({ ended_on: null, started_on: null })] });
    await render();
    expect(group().textContent).toContain("Okudun");
    await click(byText("tarih ekle"));
    await type(field("Okumanın günleri", "Bitiş günü"), "2026-11-20");
    await submit("Okumanın günleri");
    expect(calls.updateReadingLog).toEqual([[9, { ended_on: "2026-11-20" }]]);
  });

  it("Faz 15'ten kalan Okudum (geçmişi yok): tarih düzeltme önerilmez", async () => {
    server = shelfOf("READ");
    await render();
    expect(group().textContent).toContain("Okudun");
    expect(byText("tarih ekle")).toBeUndefined();
  });

  it("yarım bırakılmışken: nerede kalındığı; Devam et kaldığı yerden, Baştan başla bugünle", async () => {
    server = shelfOf("ABANDONED", { log: [logOf({ outcome: "ABANDONED", progress: 42, ended_on: "2026-11-10" })] });
    await render();
    expect(group().textContent).toContain("Yarım bıraktın · 42 / 310 sayfa");
    await openMenu();
    expect(menuItems()).toEqual(["Baştan başla", "Zaten okudum", "Kitaplığımdan çıkar"]);
    await click(byText("Baştan başla"));
    await click(byText("Devam et"));
    expect(calls.shelfCommand).toEqual([
      [12, { command: "start", body: { mode: "restart", started_on: today() } }],
      // Devam et başlangıç gününü göndermez: sunucu eski okumanın gününü geri getirir.
      [12, { command: "start", body: { mode: "resume" } }],
    ]);
  });
});

describe("raf kontrolü — onaylar", () => {
  it("ilerleme varken Ertele önce sorar; vazgeçince istek yok", async () => {
    server = shelfOf("READING");
    await render();
    await choose("Ertele");
    expect(dialog()?.textContent).toContain("42 / 310 sayfa");
    await click(byText("Vazgeç"));
    expect(calls.shelfCommand).toBeUndefined();
    await choose("Ertele");
    await click(Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === "Ertele"));
    expect(calls.shelfCommand).toEqual([[12, { command: "postpone" }]]);
  });

  it("ilerleme yokken Ertele sormaz (yan etkisi yok)", async () => {
    server = shelfOf("READING", { reading: readingOf({ progress: 0 }) });
    await render();
    await choose("Ertele");
    expect(dialog()).toBeNull();
    expect(calls.shelfCommand).toEqual([[12, { command: "postpone" }]]);
  });

  it("çıkarma her zaman sorar; ilerleme varsa geçmişe yazılacağını söyler", async () => {
    server = shelfOf("READING");
    after = shelfOf(null, { log: [logOf({ outcome: "ABANDONED", progress: 42, ended_on: null })] });
    await render();
    await choose("Kitaplığımdan çıkar");
    expect(dialog()?.textContent).toContain("Okuman (42 / 310 sayfa) geçmişine “yarım bıraktın” olarak kaydedilecek");
    await click(byText("Çıkar"));
    expect(calls.shelfCommand).toEqual([[12, { command: "remove" }]]);
    // Çıkarılan kitabın geçmişi kalır ve görünür (spec Doğrulama 5).
    expect(byText("Okumak istiyorum")).toBeTruthy();
    expect(document.querySelector("#reading-history-heading")?.textContent).toBe("Okuma geçmişin");
  });

  it("ilerleme yokken çıkarma onayı geçmişin silinmediğini söyler", async () => {
    server = shelfOf("WANT_TO_READ");
    await render();
    await choose("Kitaplığımdan çıkar");
    expect(dialog()?.textContent).toContain("Puanın, yorumların ve okuma geçmişin silinmez");
  });
});

describe("raf kontrolü — ilerleme ve 409", () => {
  it("ilerleme sürümle gönderilir; toplamı aşan değer gönderilmez", async () => {
    server = shelfOf("READING");
    after = shelfOf("READING", { reading: readingOf({ progress: 57 }), version: 4 });
    await render();
    await click(byText("İlerlemeyi güncelle"));
    await type(field("İlerleme", "Kaçıncı sayfadasın?"), "400");
    await submit("İlerleme");
    expect(calls.updateReading).toBeUndefined();
    expect(document.querySelector("form[aria-label='İlerleme'] [role=alert]")?.textContent).toContain("0 ile 310 arasında");
    await type(field("İlerleme", "Kaçıncı sayfadasın?"), "57");
    await submit("İlerleme");
    expect(calls.updateReading).toEqual([[12, { version: 3, progress: 57 }]]);
    expect(document.querySelector("form[aria-label='İlerleme']")).toBeNull();
    expect(document.querySelector("[role=progressbar]")?.getAttribute("aria-valuetext")).toBe("57 / 310 sayfa");
  });

  it("birimsiz okumada ilk girişle birim de gönderilir (varsayılan: toplam biliniyorsa sayfa)", async () => {
    server = shelfOf("READING", { reading: readingOf({ unit: null, progress: null, total_pages: null }) });
    await render();
    await click(byText("İlerlemeni gir"));
    await type(field("İlerleme", "Kaçıncı sayfadasın?"), "12");
    await submit("İlerleme");
    expect(calls.updateReading).toEqual([[12, { version: 3, progress: 12, unit: "page" }]]);
  });

  // Spec 17A § 5: raf yeniden okunur, mesaj YENİ veriden; girilen değer korunur; kullanıcı yeni sürümle
  // yeniden gönderir. Sessiz yeniden deneme yok.
  it("409: başka yerde yazılan değer görünür, girilen korunur, Benimkini kaydet yeni sürümle gönderir", async () => {
    server = shelfOf("READING");
    let first = true;
    stub({
      updateReading: async () => {
        if (first) {
          first = false;
          server = shelfOf("READING", { reading: readingOf({ progress: 80 }), version: 4 }); // öteki sekme yazdı
          throw new ApiError(409, "version_conflict", "okuman başka bir yerde güncellenmiş");
        }
        return (server = shelfOf("READING", { reading: readingOf({ progress: 57 }), version: 5 }));
      },
    });
    await render();
    await click(byText("İlerlemeyi güncelle"));
    await type(field("İlerleme", "Kaçıncı sayfadasın?"), "57");
    await submit("İlerleme");
    expect(calls.shelf).toHaveLength(2); // 409'dan sonra bir kez
    const alert = document.querySelector("form[aria-label='İlerleme'] [role=alert]")!;
    expect(alert.textContent).toContain("İlerlemen başka bir yerde güncellenmiş: 80 / 310 sayfa");
    expect((field("İlerleme", "Kaçıncı sayfadasın?") as HTMLInputElement).value).toBe("57");
    expect(calls.updateReading).toHaveLength(1);

    await submit("İlerleme", "Benimkini kaydet");
    expect(calls.updateReading).toEqual([[12, { version: 3, progress: 57 }], [12, { version: 4, progress: 57 }]]);
    expect(document.querySelector("[role=progressbar]")?.getAttribute("aria-valuetext")).toBe("57 / 310 sayfa");
  });

  it("409'dan sonra Vazgeç formu kapatır, öteki sekmenin değeri kalır", async () => {
    server = shelfOf("READING");
    stub({
      updateReading: async () => {
        server = shelfOf("READING", { reading: readingOf({ progress: 80 }), version: 4 });
        throw new ApiError(409, "version_conflict", "x");
      },
    });
    await render();
    await click(byText("İlerlemeyi güncelle"));
    await type(field("İlerleme", "Kaçıncı sayfadasın?"), "57");
    await submit("İlerleme");
    await submit("İlerleme", "Vazgeç");
    expect(document.querySelector("form[aria-label='İlerleme']")).toBeNull();
    expect(document.querySelector("[role=progressbar]")?.getAttribute("aria-valuetext")).toBe("80 / 310 sayfa");
    expect(calls.updateReading).toHaveLength(1);
  });

  it("Okumayı düzenle: baskı değişince sıfırlanacağını söyler, yalnız değişeni gönderir", async () => {
    server = shelfOf("READING");
    await render();
    await choose("Okumayı düzenle");
    expect(document.querySelector("form[aria-label='Okumayı düzenle'] [role=note]")).toBeNull();
    await type(field("Okumayı düzenle", "Baskı"), "2");
    expect(document.querySelector("form[aria-label='Okumayı düzenle'] [role=note]")?.textContent).toContain("42 / 310 sayfa");
    await submit("Okumayı düzenle");
    expect(calls.updateReading).toEqual([[12, { version: 3, edition_id: 2 }]]);
  });
});

describe("raf kontrolü — geçmiş", () => {
  it("geçmiş satırı: sonuç, günler, (birden çok baskıda) baskı", async () => {
    server = shelfOf("READ", { log: [logOf({ edition_id: 2 })] });
    await render();
    const row = document.querySelector("section[aria-labelledby=reading-history-heading] li")!;
    expect(row.textContent).toContain("Bitirdin · 1 Kasım 2026 – 12 Kasım 2026");
    expect(row.textContent).toContain("İngilizce · Ace · 1990 · 412 sayfa");
  });

  it("bitiş başlangıçtan önceyse istek gitmez", async () => {
    server = shelfOf("READ", { log: [logOf()] });
    await render();
    await click(byText("Günleri düzelt"));
    await type(field("Okumanın günleri", "Bitiş günü"), "2026-10-01");
    await submit("Okumanın günleri");
    expect(calls.updateReadingLog).toBeUndefined();
    expect(document.querySelector("form[aria-label='Okumanın günleri'] [role=alert]")?.textContent).toContain("önce olamaz");
  });

  it("geçmişten silme sorar; onaylayınca siler", async () => {
    server = shelfOf("READ", { log: [logOf()] });
    after = shelfOf("READ");
    await render();
    await click(byText("Sil"));
    expect(dialog()?.textContent).toContain("Kitaplığındaki durumu değişmez");
    await click(Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === "Sil"));
    expect(calls.deleteReadingLog).toEqual([[9]]);
    expect(document.querySelector("#reading-history-heading")).toBeNull();
  });
});

describe("raf kontrolü — davranış", () => {
  it("bitirince puanı yoksa Puan ver daveti; tıklayınca sayfaya haber verir", async () => {
    server = shelfOf("READING");
    after = shelfOf("READ", { log: [logOf({ ended_on: today() })] });
    const onRate = vi.fn();
    await render({ onRate });
    await click(byText("Bitirdim"));
    expect(group().textContent).toContain("Bugün bitirdin");
    await click(byText("Puan ver"));
    expect(onRate).toHaveBeenCalledTimes(1);
  });

  it("açılışta zaten okunmuş kitapta davet yok", async () => {
    server = shelfOf("READ", { log: [logOf()] });
    await render({ onRate: () => {} });
    expect(byText("Puan ver")).toBeUndefined();
  });

  it("komut sürerken bütün kontroller pasif ve menü açılmaz", async () => {
    let finish: (s: Shelf) => void = () => {};
    stub({ shelfCommand: () => new Promise<Shelf>((r) => (finish = r)) });
    server = shelfOf("WANT_TO_READ");
    await render();
    await click(byText("Okumaya başla"));
    expect((byText("Okumaya başla") as HTMLButtonElement).disabled).toBe(true);
    expect(trigger().disabled).toBe(true);
    await openMenu();
    expect(document.querySelector("[role=menuitem]")).toBeNull();
    await act(async () => finish(shelfOf("READING")));
    await flush();
    expect((byText("Bitirdim") as HTMLButtonElement).disabled).toBe(false);
  });

  it("komut başarısızsa hata bildirimi, raf olduğu gibi", async () => {
    stub({ shelfCommand: async () => { throw new ApiError(500, "internal", "Sunucu hatası oluştu"); } });
    server = shelfOf("WANT_TO_READ");
    await render();
    await click(byText("Okumaya başla"));
    expect(errorToast()).toContain("Sunucu hatası oluştu");
    expect(group().textContent).toContain("Kitaplığında: Okumak istiyorum");
  });

  // Raf bilinmiyorken "Okumak istiyorum" gösterilseydi tek tık kayıtlı bir durumu değiştirirdi.
  it("raf okunamazsa komut yok, tekrar dene var; tekrar başarılıysa gerçek durum görünür", async () => {
    let fail = true;
    stub({
      shelf: async () => {
        if (fail) throw new ApiError(500, "internal", "x");
        return shelfOf("READ");
      },
    });
    await render();
    expect(byText("Okumak istiyorum")).toBeUndefined();
    fail = false;
    await click(byText("Kitaplık durumu yüklenemedi, tekrar dene"));
    expect(group().textContent).toContain("Kitaplığında: Okudum");
    expect(calls.shelfCommand).toBeUndefined();
  });

  it("token yenilenince (aynı kullanıcı, yeni nesne) raf yeniden okunmaz", async () => {
    server = shelfOf("READ");
    await render();
    h.user = { ...USER };
    rerender();
    await flush();
    expect(calls.shelf).toEqual([[12]]);
  });

  // Faz 16d adım 7'nin dersi rafta da geçerli: kontrol ile rozet arasında bağlantı yok, ortak noktaları anahtar.
  it("komuttan sonra aynı sayfadaki başka bir bileşenin rozeti de güncellenir", async () => {
    after = shelfOf("WANT_TO_READ");
    const el = await render({ badges: true });
    expect(el.querySelector("[data-badges]")!.textContent).toBe("12:-");
    await click(byText("Okumak istiyorum"));
    expect(el.querySelector("[data-badges]")!.textContent).toBe("12:WANT_TO_READ");
  });
});
