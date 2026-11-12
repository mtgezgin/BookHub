import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { Edition } from "../api/types";
import { EditionStrip, PrimaryEditionNote } from "./EditionStrip";

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

const ed = (over: Partial<Edition>): Edition => ({
  id: 1, title: "Suç ve Ceza", language: "tr", translator: "", publisher: "", publication_year: null,
  isbn: "", page_count: null, cover_url: "", is_primary: false, ...over,
});
const PRIMARY = ed({ id: 1, is_primary: true, publisher: "Yediveren Yayınları", cover_url: "https://example.test/tr.jpg" });
const RUSSIAN = ed({ id: 2, title: "Преступление и наказание", language: "ru", publication_year: 2012, cover_url: "https://example.test/ru.jpg" });

describe("EditionStrip", () => {
  it("kitabın tek baskısı varsa hiç çizilmez", () => {
    const el = render(<EditionStrip editions={[PRIMARY]} author="Dostoyevski" />);
    expect(el.innerHTML).toBe("");
  });

  it("varsayılan baskıyı göstermez (o sayfanın ana kapağı); diğerleri dil adı ve yılla", () => {
    const el = render(<EditionStrip editions={[PRIMARY, RUSSIAN]} author="Dostoyevski" />);
    const list = el.querySelector('ul[aria-labelledby="editions-heading"]')!;
    expect(el.querySelector("#editions-heading")?.textContent).toBe("Diğer baskılar");
    const items = list.querySelectorAll("li");
    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Rusça · 2012");
    expect(items[0].querySelector("img")?.getAttribute("src")).toBe("https://example.test/ru.jpg");
  });

  it("baskının adı ekran okuyucuya metin olarak gider (kapak dekoratif)", () => {
    const el = render(<EditionStrip editions={[PRIMARY, RUSSIAN]} author="Dostoyevski" />);
    expect(el.querySelector("li .sr-only")?.textContent).toContain("Преступление и наказание");
    expect(el.querySelector("li [aria-hidden=true]")).not.toBeNull();
  });

  it("çevirmen varsa 'çev.' satırı; yıl yoksa yalnız dil", () => {
    const el = render(
      <EditionStrip editions={[PRIMARY, ed({ id: 3, language: "en", translator: "Constance Garnett" })]} author="Dostoyevski" />,
    );
    const li = el.querySelector("li")!;
    expect(li.textContent).toContain("İngilizce");
    expect(li.textContent).not.toContain("·");
    expect(li.textContent).toContain("çev. Constance Garnett");
  });

  it("kapağı olmayan baskıya üretilmiş kapak çizilir", () => {
    const el = render(<EditionStrip editions={[PRIMARY, ed({ id: 4, title: "Crime and Punishment", language: "en" })]} author="Dostoyevski" />);
    const li = el.querySelector("li")!;
    expect(li.querySelector("img")).toBeNull();
    expect(li.textContent).toContain("Crime and Punishment");
  });

  it("liste klavyeyle odaklanabilir (yatay kaydırma için)", () => {
    const el = render(<EditionStrip editions={[PRIMARY, RUSSIAN]} author="Dostoyevski" />);
    expect(el.querySelector("ul")?.getAttribute("tabindex")).toBe("0");
  });
});

describe("PrimaryEditionNote", () => {
  it("birden çok baskıda varsayılanın dili ve yayınevi", () => {
    const el = render(<PrimaryEditionNote editions={[PRIMARY, RUSSIAN]} />);
    expect(el.textContent).toBe("Türkçe baskı · Yediveren Yayınları");
  });

  it("yayınevi bilinmiyorsa yalnız dil", () => {
    const el = render(<PrimaryEditionNote editions={[{ ...PRIMARY, publisher: "" }, RUSSIAN]} />);
    expect(el.textContent).toBe("Türkçe baskı");
  });

  it("tek baskıda ya da baskı yokken çizilmez", () => {
    expect(render(<PrimaryEditionNote editions={[PRIMARY]} />).innerHTML).toBe("");
    expect(render(<PrimaryEditionNote editions={[]} />).innerHTML).toBe("");
  });
});
