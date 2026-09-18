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
  it("cover_url yoksa yer tutucu gösterir, img çizmez", () => {
    const el = render(book());
    expect(el.querySelector(".cover.placeholder")).not.toBeNull();
    expect(el.querySelector("img")).toBeNull();
  });

  it("tür etiketleri /?genre=<slug> adresine bağlanır", () => {
    const hrefs = Array.from(render(book()).querySelectorAll("a.tag")).map((a) => a.getAttribute("href"));
    expect(hrefs).toEqual(["/?genre=roman", "/?genre=klasik"]);
  });
});
