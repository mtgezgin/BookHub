import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { type DocNode, docHasSpoiler, docLength, docOf, docProblem, docText, emptyDoc, isAllSpoiler, MAX_REVIEW, plainToDoc } from "./reviewDoc";

/*
 * ORTAK ÖRNEKLER: sunucunun doğrulayıcısı (backend/internal/richtext) aynı dosyayı kendi testinde okur.
 * Düz metin ve spoiler hesabı iki tarafta ayrışırsa bu test ya da Go'daki TestVectors kırmızıya döner.
 * Dosya import edilmiyor, okunuyor: frontend/ dışındaki bir yol derlemeye (Docker bağlamı) girmesin.
 */
interface Vector { name: string; doc: DocNode; text: string; has_spoiler: boolean }
const vectors: Vector[] = JSON.parse(
  readFileSync(resolve(import.meta.dirname, "../../../backend/internal/richtext/testdata/vectors.json"), "utf8"),
);

const para = (...content: DocNode[]): DocNode => ({ type: "doc", content: [{ type: "paragraph", content }] });
const text = (t: string, ...marks: string[]): DocNode =>
  marks.length ? { type: "text", text: t, marks: marks.map((type) => ({ type })) } : { type: "text", text: t };

describe("sunucuyla ortak örnekler", () => {
  it("dosya okunuyor", () => expect(vectors.length).toBeGreaterThanOrEqual(6));
  it.each(vectors.map((v) => [v.name, v] as const))("%s", (_name, v) => {
    expect(docText(v.doc)).toBe(v.text);
    expect(docHasSpoiler(v.doc)).toBe(v.has_spoiler);
    expect(docLength(v.doc)).toBe([...v.text].length);
  });
});

describe("uzunluk", () => {
  it("kod noktası sayar: emoji 1, satır sonu 1", () => {
    const doc: DocNode = { type: "doc", content: [{ type: "paragraph", content: [text("📚ş")] }, { type: "paragraph", content: [text("a")] }] };
    expect(docText(doc)).toBe("📚ş\na");
    expect(docLength(doc)).toBe(4);
    expect(docText(doc)).toHaveLength(5); // string.length emojiyi 2 sayar; sayaç bunu KULLANMAZ
  });
  it("sınır sunucununkiyle aynı", () => expect(MAX_REVIEW).toBe(5000));
});

describe("tamamı spoiler mi", () => {
  it("her metin işaretliyse evet", () => {
    expect(isAllSpoiler(plainToDoc("bir\n\niki", true))).toBe(true);
    expect(isAllSpoiler(para(text("a", "spoiler"), text("b", "bold", "spoiler")))).toBe(true);
  });
  it("bir parça açıksa hayır; metin yoksa hayır", () => {
    expect(isAllSpoiler(para(text("a", "spoiler"), text("b")))).toBe(false);
    expect(isAllSpoiler({ type: "doc", content: [{ type: "paragraph" }] })).toBe(false);
  });
});

// Go'daki TestFromPlain ile aynı satırlar.
describe("düz metinden belge", () => {
  it("tek satır", () => expect(plainToDoc("Güzel.", false)).toEqual(para(text("Güzel."))));
  it("spoiler bütün metne", () => expect(plainToDoc("Katil uşak.", true)).toEqual(para(text("Katil uşak.", "spoiler"))));
  it("satırlar paragraf olur, boş satır boş paragraf", () => {
    expect(plainToDoc("bir\n\niki", false)).toEqual({
      type: "doc",
      content: [{ type: "paragraph", content: [text("bir")] }, { type: "paragraph" }, { type: "paragraph", content: [text("iki")] }],
    });
  });
  it("Windows satır sonu", () => expect(docText(plainToDoc("bir\r\niki\rüç", false))).toBe("bir\niki\nüç"));
  it("denetim karakteri atılır, sekme kalır", () => expect(docText(plainToDoc("a\u0000b\tc\u007f\u0085", false))).toBe("ab\tc"));
});

// Sunucudaki ErrEmpty ve ErrLimit'in ön kontrolü (richtext.Parse ile aynı sınırlar).
describe("kaydedilebilir mi", () => {
  const nest = (depth: number): DocNode => (depth === 0 ? { type: "paragraph", content: [text("a")] } : { type: "blockquote", content: [nest(depth - 1)] });
  it("geçerli belge", () => expect(docProblem(para(text("Güzel")))).toBeNull());
  it("boş ve yalnız boşluk", () => {
    expect(docProblem(emptyDoc())).toBe("İncelemen boş olamaz.");
    expect(docProblem(para(text(" \t ")))).toBe("İncelemen boş olamaz.");
  });
  it("5000 karakter geçer, 5001 geçmez (çok baytlı)", () => {
    expect(docProblem(para(text("ş".repeat(5000))))).toBeNull();
    expect(docProblem(para(text("ş".repeat(5001))))).toBe("İncelemen en fazla 5000 karakter olabilir.");
  });
  it("12 kat geçer, 13 geçmez", () => {
    // doc (1) + alıntılar + paragraf + metin
    expect(docProblem({ type: "doc", content: [nest(9)] })).toBeNull();
    expect(docProblem({ type: "doc", content: [nest(10)] })).toMatch(/iç içe/);
  });
  it("2000 düğüm geçer, 2001 geçmez", () => {
    const many = (n: number): DocNode => ({ type: "doc", content: [{ type: "paragraph", content: [text("a")] }, ...Array.from({ length: n - 3 }, () => ({ type: "paragraph" }))] });
    expect(docProblem(many(2000))).toBeNull();
    expect(docProblem(many(2001))).toMatch(/iç içe/);
  });
});

describe("docOf", () => {
  it("sunucunun belgesi varsa o", () => {
    const doc = para(text("belge"));
    expect(docOf({ doc, body: "eski", has_spoiler: false })).toBe(doc);
  });
  it("alan yoksa düz metinden çevrilir (eski API ile konuşan yeni arayüz)", () => {
    expect(docOf({ body: "eski", has_spoiler: true })).toEqual(para(text("eski", "spoiler")));
  });
});
