import { describe, expect, it } from "vitest";
import { formatScore, languageName } from "./format";

describe("formatScore", () => {
  it("Türkçe ondalık virgülle, varsayılan tek basamak yazar", () => {
    expect(formatScore(8.09)).toBe("8,1");
  });

  it("tam sayıda da basamağı korur: liste hizalı kalsın", () => {
    expect(formatScore(10)).toBe("10,0");
  });

  it("istenen basamak sayısına yuvarlar", () => {
    expect(formatScore(7.456, 2)).toBe("7,46");
  });
});

describe("languageName", () => {
  it("ISO kodunu Türkçe dil adına çevirir", () => {
    expect(languageName("en")).toBe("İngilizce");
    expect(languageName("tr")).toBe("Türkçe");
  });

  it("boş kodda boş döner", () => {
    expect(languageName("")).toBe("");
  });

  it("geçersiz etikette patlamaz, kodu olduğu gibi döner", () => {
    expect(languageName("!!")).toBe("!!");
  });
});
