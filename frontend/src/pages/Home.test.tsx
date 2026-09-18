import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Book, TopRated } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));

import { api } from "../api/client";
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
});
