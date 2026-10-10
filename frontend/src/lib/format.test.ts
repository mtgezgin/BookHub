import { describe, expect, it } from "vitest";
import { formatScore, initialOf, languageName, timeAgo } from "./format";

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

describe("timeAgo", () => {
  const now = Date.parse("2026-10-10T12:00:00Z");
  const ago = (ms: number) => new Date(now - ms).toISOString();
  const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;

  it("bir dakikadan yeniyse \"az önce\" der", () => {
    expect(timeAgo(ago(30_000), now)).toBe("az önce");
  });

  it("dakika, saat ve gün olarak yazar", () => {
    expect(timeAgo(ago(5 * MIN), now)).toBe("5 dakika önce");
    expect(timeAgo(ago(2 * HOUR), now)).toBe("2 saat önce");
    expect(timeAgo(ago(3 * DAY), now)).toBe("3 gün önce");
  });

  it("bir gün önceye \"dün\" der", () => {
    expect(timeAgo(ago(DAY + HOUR), now)).toBe("dün");
  });

  it("bir aydan eskiyse tam tarihe döner", () => {
    expect(timeAgo("2026-09-01T12:00:00Z", now)).toBe("1 Eylül 2026");
  });
});

describe("initialOf", () => {
  it("adın ilk harfini Türkçe büyük harfle verir", () => {
    expect(initialOf("ilkay")).toBe("İ");
    expect(initialOf("  ali")).toBe("A");
  });

  it("boş adda soru işareti döner", () => {
    expect(initialOf("")).toBe("?");
  });
});
