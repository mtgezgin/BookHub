import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { BookCover, COVER_PALETTE, coverPalette } from "./BookCover";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

function render(node: React.ReactNode) {
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(node));
  cleanup.push(() => act(() => root.unmount()));
  return el;
}

describe("coverPalette", () => {
  it("aynı kitap her seferinde aynı rengi alır (sayfa yenilenince kapak değişmesin)", () => {
    expect(coverPalette("Huzur", "Ahmet Hamdi Tanpınar")).toBe(coverPalette("Huzur", "Ahmet Hamdi Tanpınar"));
    expect(COVER_PALETTE).toContain(coverPalette("Huzur", "Ahmet Hamdi Tanpınar"));
  });

  it("farklı kitaplar paletin geneline dağılır (hepsi aynı renk olmasın)", () => {
    const titles = ["Huzur", "1984", "Dune", "Vakıf", "Yabancı", "Kozmos", "Hobbit", "Simyacı",
      "Martin Eden", "Aylak Adam", "Küçük Prens", "Fahrenheit 451"];
    const used = new Set(titles.map((t) => coverPalette(t, "Yazar")));
    expect(used.size).toBeGreaterThanOrEqual(5);
  });
});

describe("BookCover", () => {
  it("resim yoksa başlık ve yazarı yazan üretilmiş kapak çizer", () => {
    const el = render(<BookCover title="Huzur" author="Ahmet Hamdi Tanpınar" />);
    expect(el.querySelector("img")).toBeNull();
    expect(el.textContent).toContain("Huzur");
    expect(el.textContent).toContain("Ahmet Hamdi Tanpınar");
  });

  it("kapak dekoratiftir: ekran okuyucu başlığı kartta zaten duyuyor", () => {
    const el = render(<BookCover title="Huzur" author="Tanpınar" />);
    expect(el.firstElementChild!.getAttribute("aria-hidden")).toBe("true");
  });

  it("resim varsa onu çizer", () => {
    const el = render(<BookCover title="Huzur" author="Tanpınar" src="https://example.com/k.jpg" />);
    expect(el.querySelector("img")!.getAttribute("src")).toBe("https://example.com/k.jpg");
  });

  it("resim yüklenemezse üretilmiş kapağa düşer", () => {
    const el = render(<BookCover title="Huzur" author="Tanpınar" src="https://example.com/yok.jpg" />);
    act(() => {
      el.querySelector("img")!.dispatchEvent(new Event("error"));
    });
    expect(el.querySelector("img")).toBeNull();
    expect(el.textContent).toContain("Huzur");
  });
});
