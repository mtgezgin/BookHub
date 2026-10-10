import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import type { Book } from "../api/types";
import { BookCard } from "./BookCard";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

const book = (over: Partial<Book> = {}): Book => ({
  id: 3, title: "Tutunamayanlar", slug: "tutunamayanlar", author: "Oğuz Atay", description: "",
  cover_url: "", publication_year: 1972, page_count: null, language: "tr",
  genres: [{ id: 1, name: "Roman", slug: "roman" }, { id: 2, name: "Klasik", slug: "klasik" }],
  average_rating: null, rating_count: 0, created_at: "", updated_at: "", ...over,
});

function render(b: Book) {
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(<MemoryRouter><BookCard book={b} /></MemoryRouter>));
  cleanup.push(() => act(() => root.unmount()));
  return el;
}

describe("BookCard", () => {
  it("cover_url yoksa başlığı ve yazarı yazan üretilmiş kapak çizer, img çizmez", () => {
    const el = render(book());
    expect(el.querySelector("img")).toBeNull();
    expect(el.querySelector("[aria-hidden=true]")!.textContent).toContain("Tutunamayanlar");
  });

  it("cover_url varsa <img src> çizer", () => {
    const el = render(book({ cover_url: "https://example.com/kapak.jpg" }));
    expect(el.querySelector("img")!.getAttribute("src")).toBe("https://example.com/kapak.jpg");
  });

  it("başlık /books/{id} adresine bağlanır; bağlantının adı başlığın kendisi", () => {
    const link = render(book()).querySelector("h2 a")!;
    expect(link.getAttribute("href")).toBe("/books/3");
    expect(link.textContent).toBe("Tutunamayanlar");
  });

  it("yıl varsa yazarın yanında görünür, yoksa ayraç da yok", () => {
    expect(render(book()).textContent).toContain("Oğuz Atay · 1972");
    expect(render(book({ publication_year: null })).textContent).not.toContain("·");
  });

  it("tür rozetleri /?genre=<slug> adresine bağlanır", () => {
    const hrefs = Array.from(render(book()).querySelectorAll('a[href^="/?genre="]')).map((a) => a.getAttribute("href"));
    expect(hrefs).toEqual(["/?genre=roman", "/?genre=klasik"]);
  });

  it("puanı Türkçe biçimde gösterir", () => {
    expect(render(book({ average_rating: 8.09, rating_count: 11 })).textContent).toContain("8,1 (11 oy)");
  });
});
