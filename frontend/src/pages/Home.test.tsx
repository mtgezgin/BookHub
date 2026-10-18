import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Book, Page, TopRated, User } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ user: null as unknown }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ user: h.user, ready: true }) }));

import { api, ApiError } from "../api/client";
import { Home } from "./Home";

const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}

const bookOf = (id: number): Book => ({
  id, title: `Kitap ${id}`, slug: `kitap-${id}`, author: "Yazar", description: "", cover_url: "",
  publication_year: null, page_count: null, language: "tr", genres: [], average_rating: null, rating_count: 0,
  created_at: "", updated_at: "",
});
let total = 30;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  total = 30;
  h.user = null;
  stub({
    genres: async () => [{ id: 1, name: "Roman", slug: "roman" }],
    topRated: async (): Promise<TopRated> => ({ items: [{ ...bookOf(1), weighted_score: 8.1 }], m: 10, c: 7 }),
    books: async (p: { offset: number }) => ({ items: [bookOf(p.offset + 1)], total, limit: 12, offset: p.offset }),
  });
});

const flush = async () => {
  for (let i = 0; i < 3; i++) await act(async () => {});
};

async function render(path: string) {
  const loc = { search: "" };
  const nav: { go?: (to: string) => void } = {};
  function Probe() {
    loc.search = useLocation().search;
    nav.go = useNavigate();
    return null;
  }
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() =>
    root.render(
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/" element={<><Home /><Probe /></>} />
        </Routes>
      </MemoryRouter>,
    ),
  );
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return { el, nav, params: () => new URLSearchParams(loc.search) };
}
// Rol/etiket seçicileri: görünüm (sınıflar) değişince testler kırılmasın (Faz 14).
const LIST = '[aria-label="Kitaplar"]';
const PAGER = 'nav[aria-label="Sayfalar"]';
const COUNT = "[aria-live=polite]";
const TOP = '[aria-labelledby="top-heading"]';
const button = (el: ParentNode, text: string) =>
  Array.from(el.querySelectorAll("button")).find((b) => b.textContent === text)!;
const setValue = (input: HTMLInputElement | HTMLSelectElement, value: string) =>
  act(() => {
    Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), "value")!.set!.call(input, value);
    input.dispatchEvent(new Event(input instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });

describe("Home", () => {
  it.each(["abc", "-5"])("?offset=%s API'ye offset 0 olarak gider (NaN yok)", async (raw) => {
    await render(`/?offset=${raw}`);
    expect(calls.books).toEqual([[{ q: "", genre: "", limit: 12, offset: 0 }]]);
  });

  it("arama değişince offset URL'den düşer", async () => {
    const { el, params } = await render("/?q=eski&offset=24");
    setValue(el.querySelector<HTMLInputElement>("input[type=search]")!, "yeni");
    await act(async () => {
      el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await flush();
    expect(params().get("q")).toBe("yeni");
    expect(params().has("offset")).toBe(false);
  });

  it("tür çipi seçilince offset URL'den düşer", async () => {
    const { el, params } = await render("/?offset=24");
    await act(async () => button(el, "Roman").click());
    await flush();
    expect(params().get("genre")).toBe("roman");
    expect(params().has("offset")).toBe(false);
  });

  it("pager filtreleri korur", async () => {
    const { el, params } = await render("/?q=tolstoy&genre=roman");
    await act(async () => button(el, "Sonraki →").click());
    await flush();
    expect(Object.fromEntries(params())).toEqual({ q: "tolstoy", genre: "roman", offset: "12" });
    expect(calls.books.at(-1)).toEqual([{ q: "tolstoy", genre: "roman", limit: 12, offset: 12 }]);
  });

  it("pager düğmeleri sınırlarda devre dışı", async () => {
    const first = await render("/");
    expect(button(first.el, "← Önceki").disabled).toBe(true);
    expect(button(first.el, "Sonraki →").disabled).toBe(false);
    cleanup.splice(0).forEach((f) => f());

    const last = await render("/?offset=24");
    expect(button(last.el, "← Önceki").disabled).toBe(false);
    expect(button(last.el, "Sonraki →").disabled).toBe(true);
  });

  it("\"en sevilenler\" şeridi sırayı, ağırlıklı puanı ve oy sayısını gösterir; C/m açıklaması açılır kutuda", async () => {
    stub({
      topRated: async (): Promise<TopRated> => ({
        items: [{ ...bookOf(4), average_rating: 9.5, rating_count: 2, weighted_score: 7.456 }],
        m: 10,
        c: 6.9,
      }),
    });
    const { el } = await render("/");
    const strip = el.querySelector(TOP)!;
    const li = strip.querySelector("ol li")!;
    expect(li.querySelector("a")!.getAttribute("href")).toBe("/books/4");
    expect(li.textContent).toContain("Kitap 4");
    expect(li.textContent).toContain("7,46");
    expect(li.textContent).toContain("2 oy");
    expect(strip.querySelector("details")!.textContent).toContain("C = 6,90");
    expect(strip.querySelector("details")!.textContent).toContain("m = 10");
  });

  it("puanlanmış kitap yoksa şerit çizilmez", async () => {
    stub({ topRated: async (): Promise<TopRated> => ({ items: [], m: 10, c: 0 }) });
    const { el } = await render("/");
    expect(el.querySelector(TOP)).toBeNull();
  });

  it("arama ya da tür seçiliyken şerit çizilmez (sonuçların önüne geçmesin)", async () => {
    expect((await render("/?q=dune")).el.querySelector(TOP)).toBeNull();
    cleanup.splice(0).forEach((f) => f());
    expect((await render("/?genre=roman")).el.querySelector(TOP)).toBeNull();
  });

  it("görünür bir sayfa başlığı var", async () => {
    const { el } = await render("/");
    expect(el.querySelector("h1")!.textContent).toBe("Bir sonraki kitabını bul");
  });

  it("tür seçili değilken \"Tümü\" basılı; seçili türün çipi basılı", async () => {
    const all = await render("/");
    expect(button(all.el, "Tümü").getAttribute("aria-pressed")).toBe("true");
    expect(button(all.el, "Roman").getAttribute("aria-pressed")).toBe("false");
    cleanup.splice(0).forEach((f) => f());
    const roman = await render("/?genre=roman");
    expect(button(roman.el, "Roman").getAttribute("aria-pressed")).toBe("true");
  });

  it("\"Tümü\" türü kaldırır, aramayı korur", async () => {
    const { el, params } = await render("/?q=tolstoy&genre=roman");
    await act(async () => button(el, "Tümü").click());
    await flush();
    expect(Object.fromEntries(params())).toEqual({ q: "tolstoy" });
  });

  it("\"En yüksek puan\" sırası URL'ye sort=rating yazar, offset'i düşürür ve API'ye gider", async () => {
    const { el, params } = await render("/?genre=roman&offset=12");
    expect(button(el, "En yeni").getAttribute("aria-pressed")).toBe("true");
    await act(async () => button(el, "En yüksek puan").click());
    await flush();
    expect(Object.fromEntries(params())).toEqual({ genre: "roman", sort: "rating" });
    expect(calls.books.at(-1)).toEqual([{ q: "", genre: "roman", limit: 12, offset: 0, sort: "rating" }]);
    expect(button(el, "En yüksek puan").getAttribute("aria-pressed")).toBe("true");
  });

  it("\"En yeni\" varsayılan sıraya döner: sort URL'den ve API isteğinden kalkar", async () => {
    const { el, params } = await render("/?sort=rating");
    await act(async () => button(el, "En yeni").click());
    await flush();
    expect(params().has("sort")).toBe(false);
    expect(calls.books.at(-1)).toEqual([{ q: "", genre: "", limit: 12, offset: 0 }]);
  });

  it("bilinmeyen sort (?sort=title) varsayılan sıraya düşer: API'ye sort gitmez, \"En yeni\" basılı", async () => {
    const { el } = await render("/?sort=title");
    expect(calls.books).toEqual([[{ q: "", genre: "", limit: 12, offset: 0 }]]);
    expect(button(el, "En yeni").getAttribute("aria-pressed")).toBe("true");
  });

  it("puan sırasındayken \"en sevilenler\" şeridi çizilmez (listenin başıyla aynı şey olurdu)", async () => {
    const { el } = await render("/?sort=rating");
    expect(el.querySelector(TOP)).toBeNull();
  });

  it("zaten seçili sıralama düğmesine basmak URL'yi değiştirmez (sayfa başa dönmez)", async () => {
    const { el, params } = await render("/?sort=rating&offset=12");
    await act(async () => button(el, "En yüksek puan").click());
    await flush();
    expect(Object.fromEntries(params())).toEqual({ sort: "rating", offset: "12" });
    cleanup.splice(0).forEach((f) => f());
    const def = await render("/?offset=12");
    await act(async () => button(def.el, "En yeni").click());
    await flush();
    expect(Object.fromEntries(def.params())).toEqual({ offset: "12" });
    // Elle yazılmış geçersiz sıra varsayılan sayılır ("En yeni" basılı); ona basmak da bir şey yapmaz.
    cleanup.splice(0).forEach((f) => f());
    const bogus = await render("/?sort=foo&offset=12");
    await act(async () => button(bogus.el, "En yeni").click());
    await flush();
    expect(Object.fromEntries(bogus.params())).toEqual({ sort: "foo", offset: "12" });
    cleanup.splice(0).forEach((f) => f());
    const def2 = await render("/?offset=12");
    // Aynı kök neden: zaten seçili tür çipi de sayfayı başa almamalı.
    await act(async () => button(def2.el, "Tümü").click());
    await flush();
    expect(Object.fromEntries(def2.params())).toEqual({ offset: "12" });
  });

  it("yazmayı bırakınca (300 ms) Enter'a basmadan arar", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { el, params } = await render("/?genre=roman");
      setValue(el.querySelector<HTMLInputElement>("input[type=search]")!, " dune ");
      await act(async () => vi.advanceTimersByTime(299));
      expect(params().has("q")).toBe(false);
      await act(async () => vi.advanceTimersByTime(1));
      await flush();
      expect(Object.fromEntries(params())).toEqual({ genre: "roman", q: "dune" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("yazarken 300 ms dolmadan tür seçilirse zamanlayıcı seçimi ezmez: arama da tür de kalır", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { el, params } = await render("/");
      setValue(el.querySelector<HTMLInputElement>("input[type=search]")!, "dune");
      await act(async () => vi.advanceTimersByTime(100));
      await act(async () => button(el, "Roman").click());
      await act(async () => vi.advanceTimersByTime(200));
      await flush();
      expect(Object.fromEntries(params())).toEqual({ genre: "roman", q: "dune" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("Enter'dan hemen sonra sayfa değişirse bekleyen zamanlayıcı sayfayı geri almaz", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { el, params } = await render("/?q=eski");
      setValue(el.querySelector<HTMLInputElement>("input[type=search]")!, "yeni");
      await act(async () => {
        el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      await flush();
      await act(async () => button(el, "Sonraki →").click());
      await act(async () => vi.advanceTimersByTime(300));
      await flush();
      expect(Object.fromEntries(params())).toEqual({ q: "yeni", offset: "12" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("zamanlayıcı URL'yi yazarken yazılan harf yutulmaz; sonraki arama da gider", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { el, params } = await render("/");
      const input = el.querySelector<HTMLInputElement>("input[type=search]")!;
      setValue(input, "dun");
      // Zamanlayıcının URL değişikliği React'e uygulanmadan kullanıcı bir harf daha yazar:
      // ikisi aynı güncellemede işlenir.
      await act(async () => {
        vi.advanceTimersByTime(300);
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, "dune");
        input.dispatchEvent(new Event("input", { bubbles: true }));
      });
      await flush();
      expect(input.value).toBe("dune");
      await act(async () => vi.advanceTimersByTime(300));
      await flush();
      expect(params().get("q")).toBe("dune");
    } finally {
      vi.useRealTimers();
    }
  });

  it("kutunun yazdığı q'dan önce başka bir gezinme gelirse, sonradan o q'ya dönünce kutu yine URL'yi izler", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { el, nav } = await render("/");
      const input = el.querySelector<HTMLInputElement>("input[type=search]")!;
      setValue(input, "dune");
      // Zamanlayıcı ?q=dune yazarken (henüz uygulanmadan) kullanıcı başka bir aramaya geçer.
      await act(async () => {
        vi.advanceTimersByTime(300);
        nav.go!("/?q=vakif");
      });
      await flush();
      expect(input.value).toBe("vakif");
      await act(async () => nav.go!("/?q=dune"));
      await flush();
      expect(input.value).toBe("dune");
    } finally {
      vi.useRealTimers();
    }
  });

  it("URL dışarıdan değişince (geri/ileri, logo) arama kutusu onu gösterir", async () => {
    const { el, nav } = await render("/?q=dune");
    const input = el.querySelector<HTMLInputElement>("input[type=search]")!;
    expect(input.value).toBe("dune");
    await act(async () => nav.go!("/"));
    await flush();
    expect(input.value).toBe("");
  });

  it("liste hatası, filtre değişip yeni istek başarılı olunca kalkar", async () => {
    let fail = true;
    stub({
      books: (p: { offset: number }) =>
        fail
          ? Promise.reject(new ApiError(500, "internal", "arama başarısız"))
          : Promise.resolve({ items: [bookOf(1)], total: 1, limit: 12, offset: p.offset }),
    });
    const { el } = await render("/?q=tolstoy");
    expect(el.querySelector("[role=alert]")!.textContent).toBe("arama başarısız");
    fail = false;
    await act(async () => button(el, "Roman").click());
    await flush();
    expect(el.querySelector("[role=alert]")).toBeNull();
    expect(el.querySelector(LIST)).not.toBeNull();
  });

  it("sıra ya da filtre değişince yeni cevap gelene kadar liste meşgul işaretlenir", async () => {
    const { el } = await render("/");
    expect(el.querySelector(LIST)!.getAttribute("aria-busy")).toBe("false");
    let release!: () => void;
    stub({
      books: (p: { offset: number }) =>
        new Promise((res) => {
          release = () => res({ items: [bookOf(p.offset + 1)], total, limit: 12, offset: p.offset });
        }),
    });
    await act(async () => button(el, "En yüksek puan").click());
    await flush();
    expect(el.querySelector(LIST)!.getAttribute("aria-busy")).toBe("true");
    await act(async () => release());
    await flush();
    expect(el.querySelector(LIST)!.getAttribute("aria-busy")).toBe("false");
  });

  it.each([
    ["genres", () => stub({ genres: () => Promise.reject(new ApiError(500, "internal", "türler yüklenemedi")) }), "türler yüklenemedi"],
    ["topRated", () => stub({ topRated: () => Promise.reject(new ApiError(500, "internal", "liste yüklenemedi")) }), "liste yüklenemedi"],
  ])("açılıştaki %s hatası ErrorBox'ta gösterilir", async (_name, fail, msg) => {
    fail();
    const { el } = await render("/");
    expect(el.querySelector("[role=alert]")!.textContent).toBe(msg);
  });

  it("kitap listesi hatası ErrorBox'ta gösterilir ve liste çizilmez", async () => {
    stub({ books: () => Promise.reject(new ApiError(500, "internal", "arama başarısız")) });
    const { el } = await render("/?q=tolstoy");
    expect(el.querySelector("[role=alert]")!.textContent).toBe("arama başarısız");
    expect(el.querySelector(LIST)).toBeNull();
    expect(el.querySelector(PAGER)).toBeNull();
  });

  it("boş sonuç sayıyı \"0 kitap\" yazar ve ne olduğunu söyleyen boş durumu gösterir", async () => {
    stub({ books: async (p: { offset: number }) => ({ items: [], total: 0, limit: 12, offset: p.offset }) });
    const { el } = await render("/?q=yok");
    expect(el.querySelector(COUNT)!.textContent).toBe("0 kitap");
    expect(el.querySelector("h2")!.textContent).toBe("Sonuç bulunamadı");
    expect(el.querySelector(PAGER)).toBeNull();
  });

  it("boş sonuçta \"Filtreleri temizle\" aramayı ve türü siler", async () => {
    stub({ books: async (p: { offset: number }) => ({ items: [], total: 0, limit: 12, offset: p.offset }) });
    const { el, params } = await render("/?q=yok&genre=roman&offset=12");
    await act(async () => button(el, "Filtreleri temizle").click());
    await flush();
    expect([...params().keys()]).toEqual([]);
    expect(el.querySelector<HTMLInputElement>("input[type=search]")!.value).toBe("");
  });

  it("liste yüklenirken iskelet gösterir; cevap gelince kaldırır", async () => {
    const { pending } = deferredBooks();
    const { el } = await render("/");
    expect(el.querySelector('[role=status][aria-label="Kitaplar yükleniyor"]')).not.toBeNull();
    await act(async () => pending[0]({ items: [bookOf(1)], total: 1, limit: 12, offset: 0 }));
    await flush();
    expect(el.querySelector("[role=status]")).toBeNull();
    expect(el.querySelector(LIST)!.textContent).toContain("Kitap 1");
  });

  it("tek sayfalık sonuçta sayfalayıcı yok", async () => {
    total = 5;
    const { el } = await render("/");
    expect(el.querySelector(LIST)).not.toBeNull();
    expect(el.querySelector(PAGER)).toBeNull();
  });

  async function submitSearch(el: HTMLElement, value: string) {
    setValue(el.querySelector<HTMLInputElement>("input[type=search]")!, value);
    await act(async () => {
      el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await flush();
  }

  it("arama baştaki/sondaki boşlukları kırpar", async () => {
    const { el, params } = await render("/");
    await submitSearch(el, "  tolstoy  ");
    expect(params().get("q")).toBe("tolstoy");
    expect(calls.books.at(-1)).toEqual([{ q: "tolstoy", genre: "", limit: 12, offset: 0 }]);
  });

  it("boş arama q parametresini siler", async () => {
    const { el, params } = await render("/?q=eski&genre=roman");
    await submitSearch(el, "   ");
    expect(params().has("q")).toBe(false);
    expect(params().get("genre")).toBe("roman");
  });

  // Cevapları ELLE çözen bir stub: "hangisi önce döndü" sırasını testin kendisi belirlesin.
  function deferredBooks() {
    const pending: ((p: Page<Book>) => void)[] = [];
    const rejects: ((e: unknown) => void)[] = [];
    stub({
      books: () =>
        new Promise<Page<Book>>((resolve, reject) => {
          pending.push(resolve);
          rejects.push(reject);
        }),
    });
    return { pending, rejects };
  }

  // Gerçek hata: filtresiz sorgu tüm katalogu taradığı için daha YAVAŞ dönüyor; "Dune"
  // araması 47 ms'de gelip ızgarayı doldurduktan sonra 243 ms'lik filtresiz cevap üstüne
  // yazıyordu → URL ?q=Dune, liste ise tüm kitaplar (Playwright catalog senaryosu kırmızı).
  it("geç gelen ESKİ liste cevabı yeni filtrenin üstüne YAZMAZ", async () => {
    const { pending } = deferredBooks();
    const { el } = await render("/");
    await submitSearch(el, "Dune");
    expect(pending).toHaveLength(2); // [0] filtresiz (bayat), [1] q=Dune (güncel)

    // Önce GÜNCEL cevap, sonra bayat olan.
    await act(async () => pending[1]({ items: [bookOf(7)], total: 1, limit: 12, offset: 0 }));
    await act(async () => pending[0]({ items: [bookOf(1), bookOf(2)], total: 30, limit: 12, offset: 0 }));
    await flush();

    expect(el.querySelector(COUNT)!.textContent).toBe("1 kitap");
    expect(el.querySelector(LIST)!.textContent).toContain("Kitap 7");
    expect(el.querySelector(LIST)!.textContent).not.toContain("Kitap 1");
  });

  it("geç gelen ESKİ isteğin HATASI da yutulur", async () => {
    const { pending, rejects } = deferredBooks();
    const { el } = await render("/");
    await submitSearch(el, "Dune");

    await act(async () => pending[1]({ items: [bookOf(7)], total: 1, limit: 12, offset: 0 }));
    await act(async () => rejects[0](new ApiError(500, "internal", "bayat hata")));
    await flush();

    expect(el.querySelector("[role=alert]")).toBeNull();
    expect(el.querySelector(LIST)!.textContent).toContain("Kitap 7");
  });

  it("giriş yapmış okurun kitaplığındaki kitaplar kartta ve şeritte durumuyla görünür", async () => {
    h.user = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" } satisfies User;
    stub({
      topRated: async (): Promise<TopRated> => ({ items: [{ ...bookOf(3), rating_count: 12, weighted_score: 8.1 }], m: 10, c: 7 }),
      books: async () => ({ items: [bookOf(3), bookOf(4)], total: 2, limit: 12, offset: 0 }),
      libraryStatuses: async () => ({ statuses: { "3": "READ" } }),
    });
    const { el } = await render("/");
    const cards = Array.from(el.querySelectorAll(`${LIST} > li`));
    expect(cards.find((c) => c.textContent!.includes("Kitap 3"))!.textContent).toContain("Okudum");
    expect(cards.find((c) => c.textContent!.includes("Kitap 4"))!.textContent).not.toContain("Okudum");
    expect(el.querySelector("section[aria-labelledby=top-heading] li")!.textContent).toContain("Okudum");
    expect(calls.libraryStatuses!.at(-1)).toEqual([[3, 4]]);
  });

  it("giriş yoksa kitaplık durumu sorulmaz", async () => {
    stub({ libraryStatuses: async () => ({ statuses: {} }) });
    await render("/");
    expect(calls.libraryStatuses).toBeUndefined();
  });
});
