import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Book, Page, TopRated } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));

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
  function Probe() {
    loc.search = useLocation().search;
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
  return { el, params: () => new URLSearchParams(loc.search) };
}
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
    setValue(el.querySelector<HTMLInputElement>("form.search input")!, "yeni");
    await act(async () => {
      el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await flush();
    expect(params().get("q")).toBe("yeni");
    expect(params().has("offset")).toBe(false);
  });

  it("tür değişince offset URL'den düşer", async () => {
    const { el, params } = await render("/?offset=24");
    setValue(el.querySelector("select")!, "roman");
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

  it("boş top-rated listesi \"Henüz puanlanmış kitap yok.\" gösterir", async () => {
    stub({ topRated: async (): Promise<TopRated> => ({ items: [], m: 10, c: 0 }) });
    const { el } = await render("/");
    expect(el.querySelector("aside")!.textContent).toContain("Henüz puanlanmış kitap yok.");
  });

  it("top-rated satırı ağırlıklı puanı, oy sayısını, ortalamayı ve C/m açıklamasını gösterir", async () => {
    stub({
      topRated: async (): Promise<TopRated> => ({
        items: [{ ...bookOf(4), average_rating: 9.5, rating_count: 2, weighted_score: 7.456 }],
        m: 10,
        c: 6.9,
      }),
    });
    const { el } = await render("/");
    const aside = el.querySelector("aside")!;
    expect(aside.textContent).toContain("(C = 6.90, m = 10)");
    const li = aside.querySelector("ol.top li")!;
    expect(li.querySelector("a")!.getAttribute("href")).toBe("/books/4");
    expect(li.textContent).toBe("Kitap 4 — 7.46 (2 oy, ort. 9.50)");
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
    expect(el.querySelector(".grid")).toBeNull();
    expect(el.querySelector(".pager")).toBeNull();
  });

  it("boş sonuç \"Sonuç yok.\" ve \"0 kitap\" gösterir", async () => {
    stub({ books: async (p: { offset: number }) => ({ items: [], total: 0, limit: 12, offset: p.offset }) });
    const { el } = await render("/?q=yok");
    expect(el.querySelector("section p.muted")!.textContent).toBe("0 kitap");
    expect(el.querySelector("section")!.textContent).toContain("Sonuç yok.");
  });

  async function submitSearch(el: HTMLElement, value: string) {
    setValue(el.querySelector<HTMLInputElement>("form.search input")!, value);
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

    expect(el.querySelector("section p.muted")!.textContent).toBe("1 kitap");
    expect(el.querySelector(".grid")!.textContent).toContain("Kitap 7");
    expect(el.querySelector(".grid")!.textContent).not.toContain("Kitap 1");
  });

  it("geç gelen ESKİ isteğin HATASI da yutulur", async () => {
    const { pending, rejects } = deferredBooks();
    const { el } = await render("/");
    await submitSearch(el, "Dune");

    await act(async () => pending[1]({ items: [bookOf(7)], total: 1, limit: 12, offset: 0 }));
    await act(async () => rejects[0](new ApiError(500, "internal", "bayat hata")));
    await flush();

    expect(el.querySelector("[role=alert]")).toBeNull();
    expect(el.querySelector(".grid")!.textContent).toContain("Kitap 7");
  });
});
