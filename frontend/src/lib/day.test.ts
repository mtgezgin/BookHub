import { afterEach, describe, expect, it, vi } from "vitest";
import { formatDay, today } from "./day";

/*
 * Saat dilimine bağlı davranış, saat dilimi DEĞİŞTİRİLEREK sınanır: Node `process.env.TZ` atamasını
 * çalışırken uygular. Her testin ilk satırı bunun gerçekten olduğunu doğrular (getHours); olmasaydı test
 * makinenin kendi diliminde koşar ve tuzağı görmeden yeşil kalırdı.
 */
const originalTZ = process.env.TZ;
afterEach(() => {
  if (originalTZ === undefined) delete process.env.TZ;
  else process.env.TZ = originalTZ;
});

describe("today", () => {
  it("İstanbul'da gece 01:00'de (UTC'de önceki gün) yerel günü verir", () => {
    process.env.TZ = "Europe/Istanbul";
    const now = new Date("2026-11-23T22:00:00Z"); // İstanbul: 24 Kasım 01:00
    expect(now.getHours()).toBe(1);
    expect(today(now)).toBe("2026-11-24");
    // Tuzak: aynı an UTC günüyle bir önceki gün.
    expect(now.toISOString().slice(0, 10)).toBe("2026-11-23");
  });

  it("ayı ve günü iki haneyle yazar", () => {
    process.env.TZ = "Europe/Istanbul";
    const now = new Date("2026-01-05T09:00:00Z");
    expect(now.getHours()).toBe(12);
    expect(today(now)).toBe("2026-01-05");
  });

  it("varsayılanı şu an", () => {
    process.env.TZ = "Europe/Istanbul";
    vi.useFakeTimers({ now: new Date("2026-11-23T22:00:00Z") });
    try {
      expect(today()).toBe("2026-11-24");
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("formatDay", () => {
  it("batıdaki bir saat diliminde günü kaydırmaz", () => {
    process.env.TZ = "America/New_York";
    expect(new Date("2026-11-12T12:00:00Z").getHours()).toBe(7);
    expect(formatDay("2026-11-12")).toBe("12 Kasım 2026");
    // Tuzak: saatsiz ISO metni UTC gece yarısı, New York'ta önceki gün.
    expect(new Date("2026-11-12").getDate()).toBe(11);
  });

  it("İstanbul'da da aynı günü yazar", () => {
    process.env.TZ = "Europe/Istanbul";
    expect(formatDay("2026-01-05")).toBe("5 Ocak 2026");
  });
});
