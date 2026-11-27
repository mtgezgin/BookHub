import { act } from "react";
import { describe, expect, it } from "vitest";
import type { DocNode } from "../lib/reviewDoc";
import { render } from "../test/render";
import { ReviewDoc } from "./ReviewDoc";

/*
 * İnceleme belgesinin çizimi (Faz 17C adım 5). Bakılanlar: yapı doğru öğelere çevriliyor mu, metin her
 * zaman METİN olarak mı basılıyor, spoiler parçasının metni açılana kadar DOM'da yok mu.
 */

const text = (t: string, ...marks: string[]): DocNode =>
  marks.length ? { type: "text", text: t, marks: marks.map((type) => ({ type })) } : { type: "text", text: t };
const p = (...content: DocNode[]): DocNode => ({ type: "paragraph", content });
const doc = (...content: DocNode[]): DocNode => ({ type: "doc", content });

const buttons = (el: Element, label: string) => Array.from(el.querySelectorAll("button")).filter((b) => b.textContent === label);
async function click(target: Element | undefined) {
  expect(target, "tıklanacak düğme bulunamadı").toBeTruthy();
  await act(async () => (target as HTMLElement).click());
}

describe("yapı", () => {
  it("paragraf, kalın, italik, alıntı, sıralı ve sırasız liste", () => {
    const el = render(<ReviewDoc doc={doc(
      p(text("düz "), text("kalın", "bold"), text(" "), text("ikisi", "bold", "italic")),
      { type: "blockquote", content: [p(text("alıntı"))] },
      { type: "bulletList", content: [{ type: "listItem", content: [p(text("madde")), { type: "orderedList", content: [{ type: "listItem", content: [p(text("iç"))] }] }] }] },
    )} />);
    expect(el.querySelector("p")!.textContent).toBe("düz kalın ikisi");
    expect(el.querySelector("p > span > strong")!.textContent).toBe("kalın");
    expect(el.querySelector("strong > em")!.textContent).toBe("ikisi");
    expect(el.querySelector("blockquote > p")!.textContent).toBe("alıntı");
    expect(el.querySelector("ul > li p")!.textContent).toBe("madde");
    expect(el.querySelector("ul > li ol > li p")!.textContent).toBe("iç");
    expect(el.querySelectorAll("button").length).toBe(0);
  });

  it("boş paragraf bir satır yer tutar", () => {
    const el = render(<ReviewDoc doc={doc(p(text("bir")), { type: "paragraph" }, p(text("iki")))} />);
    const paragraphs = el.querySelectorAll("p");
    expect(paragraphs.length).toBe(3);
    expect(paragraphs[1].querySelector("br")).toBeTruthy();
  });

  it("HTML metindir: etiket öğeye dönüşmez", () => {
    const el = render(<ReviewDoc doc={doc(p(text(`<img src=x onerror="alert(1)"><script>alert(1)</script>`)))} />);
    expect(el.querySelector("img, script")).toBeNull();
    expect(el.querySelector("p")!.textContent).toBe(`<img src=x onerror="alert(1)"><script>alert(1)</script>`);
  });

  it("tanınmayan düğüm ve işaret çizilmez, gerisi çizilir", () => {
    const el = render(<ReviewDoc doc={doc(
      { type: "heading", content: [text("başlık")] },
      { type: "image" },
      p(text("kalan", "link"), { type: "hardBreak" }),
    )} />);
    expect(el.textContent).toBe("kalan");
    expect(el.querySelector("a, h1, h2, img")).toBeNull();
  });
});

describe("spoiler parçaları", () => {
  // Adım 1 deneyinin çıktısı: kalın, spoiler'ın ortasında; belgede üç ayrı metin düğümü.
  const overlap = doc(p(
    text("Sonu "), text("katil ", "spoiler"), text("uşak", "bold", "spoiler"), text(" çıkıyor", "spoiler"), text(" diye biter."),
  ));

  it("art arda spoiler metinleri TEK parça; metin açılana kadar DOM'da yok", async () => {
    const el = render(<ReviewDoc doc={overlap} />);
    expect(buttons(el, "Spoiler'ı göster").length).toBe(1);
    expect(el.textContent).toBe("Sonu Spoiler'ı göster diye biter.");
    expect(el.innerHTML).not.toContain("uşak");
    const toggle = buttons(el, "Spoiler'ı göster")[0];
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(toggle.hasAttribute("aria-controls")).toBe(false);

    await click(toggle);
    const open = buttons(el, "Spoiler'ı gizle")[0];
    expect(open).toBe(toggle); // aynı düğme yerinde kalır: odak kaybolmaz
    expect(open.getAttribute("aria-expanded")).toBe("true");
    const piece = el.querySelector(`[id="${open.getAttribute("aria-controls")}"]`)!;
    expect(piece.textContent).toBe("katil uşak çıkıyor");
    expect(piece.querySelector("strong")!.textContent).toBe("uşak");

    await click(open);
    expect(el.innerHTML).not.toContain("uşak");
  });

  it("her parça ayrı açılır; öteki kapalı kalır", async () => {
    const el = render(<ReviewDoc doc={doc(
      p(text("a "), text("BİR", "spoiler"), text(" b "), text("İKİ", "spoiler")),
      p(text("c "), text("ÜÇ", "spoiler")),
    )} />);
    expect(buttons(el, "Spoiler'ı göster").length).toBe(3);
    await click(buttons(el, "Spoiler'ı göster")[1]);
    expect(el.textContent).toContain("İKİ");
    expect(el.textContent).not.toContain("BİR");
    expect(el.textContent).not.toContain("ÜÇ");
    expect(buttons(el, "Spoiler'ı göster").length).toBe(2);
  });

  it("tamamı spoiler olan inceleme tek düğmeyle açılır (eski 'spoiler içeriyor' kutusu)", async () => {
    const el = render(<ReviewDoc doc={doc(p(text("Katil", "spoiler")), { type: "paragraph" }, p(text("uşak", "spoiler")))} />);
    expect(el.textContent).toBe("Spoiler içeriyorSpoiler'ı göster");
    expect(buttons(el, "Spoiler'ı göster").length).toBe(1);
    await click(buttons(el, "Spoiler'ı göster")[0]);
    const body = el.querySelector(`[id="${buttons(el, "Spoiler'ı gizle")[0].getAttribute("aria-controls")}"]`)!;
    expect(Array.from(body.querySelectorAll("p")).map((x) => x.textContent)).toEqual(["Katil", "", "uşak"]);
    expect(body.querySelectorAll("button").length).toBe(0);
  });

  it("yazar kipi: her şey görünür, spoiler parçası işaretli, düğme yok", () => {
    const el = render(<ReviewDoc doc={overlap} mode="author" />);
    expect(el.textContent).toBe("Sonu katil uşak çıkıyor diye biter.");
    expect(el.querySelector("mark")!.textContent).toBe("katil uşak çıkıyor");
    expect(el.querySelectorAll("button").length).toBe(0);
  });
});
