import { describe, expect, it } from "vitest";
import type { Edition, LibraryStatus, ReadingLogEntry, Shelf } from "../api/types";
import {
  abandonedText, closedReadingOf, commandToast, dayRange, editionLabel, finishedText, hasProgress, progressPercent,
  progressText,
} from "./shelf";

const p = (unit: "page" | "percent" | null, progress: number | null, total_pages: number | null) => ({ unit, progress, total_pages });

describe("progressText", () => {
  it.each([
    [p("page", 42, 310), "42 / 310 sayfa"],
    [p("page", 0, 310), "0 / 310 sayfa"],
    [p("percent", 35, null), "%35"],
    [p("percent", 0, null), "%0"],
    [p("page", 42, null), "42. sayfa"],
    // Spec Doğrulama 8: toplamı bilinmeyen okumada "0 sayfa" görünmez; girilmemiş sayılır.
    [p("page", 0, null), null],
    [p(null, null, null), null],
  ])("%o → %s", (input, want) => {
    expect(progressText(input)).toBe(want);
  });
});

describe("progressPercent", () => {
  it.each([
    [p("page", 42, 310), 14],
    [p("page", 310, 310), 100],
    [p("page", 0, 310), 0],
    [p("percent", 35, null), 35],
    [p("page", 42, null), null], // neyin yüzdesi bilinmiyor
    [p("page", 42, 0), null], // sıfıra bölme yok
    [p(null, null, null), null],
  ])("%o → %s", (input, want) => {
    expect(progressPercent(input)).toBe(want);
  });
});

it("hasProgress sunucunun kuralıyla aynı: yalnız > 0", () => {
  expect(hasProgress({ ...p("page", 1, 310), edition_id: 1, started_on: null })).toBe(true);
  expect(hasProgress({ ...p("page", 0, 310), edition_id: 1, started_on: null })).toBe(false);
  expect(hasProgress({ ...p(null, null, null), edition_id: null, started_on: null })).toBe(false);
  expect(hasProgress(null)).toBe(false);
});

const log = (outcome: ReadingLogEntry["outcome"], over: Partial<ReadingLogEntry> = {}): ReadingLogEntry => ({
  id: 1, edition_id: 1, outcome, unit: "page", progress: 42, total_pages: 310, started_on: "2026-11-01",
  ended_on: "2026-11-12", created_at: "", ...over,
});
const shelf = (status: LibraryStatus, entries: ReadingLogEntry[]): Shelf => ({
  book_id: 1, entry: { status, version: 1, added_at: "", updated_at: "", reading: null }, log: entries,
});

describe("closedReadingOf", () => {
  it("Okudum'da son kayıt bitirilmişse o; Yarım bıraktım'da son kayıt yarım kalmışsa o", () => {
    expect(closedReadingOf(shelf("READ", [log("FINISHED", { id: 2 }), log("ABANDONED")]))?.id).toBe(2);
    expect(closedReadingOf(shelf("ABANDONED", [log("ABANDONED", { id: 3 })]))?.id).toBe(3);
  });

  it("son kayıt duruma uymuyorsa (Faz 15'ten kalan durum, silinmiş kayıt) yok", () => {
    expect(closedReadingOf(shelf("READ", [log("ABANDONED")]))).toBeUndefined();
    expect(closedReadingOf(shelf("ABANDONED", [log("FINISHED")]))).toBeUndefined();
    expect(closedReadingOf(shelf("READ", []))).toBeUndefined();
    expect(closedReadingOf(shelf("WANT_TO_READ", [log("FINISHED")]))).toBeUndefined();
  });
});

describe("durum satırları", () => {
  const now = new Date(2026, 10, 24, 10); // yerel 24 Kasım 2026
  it.each([
    [log("FINISHED", { ended_on: "2026-11-24" }), "Bugün bitirdin"],
    [log("FINISHED", { ended_on: "2025-03-02" }), "Bitirdin · 2 Mart 2025"],
    [log("FINISHED", { ended_on: null }), "Okudun"],
    [undefined, "Okudun"],
  ])("finishedText %#", (entry, want) => {
    expect(finishedText(entry, now)).toBe(want);
  });

  it.each([
    [log("ABANDONED"), "Yarım bıraktın · 42 / 310 sayfa"],
    [log("ABANDONED", { unit: "percent", progress: 35, total_pages: null }), "Yarım bıraktın · %35"],
    [log("ABANDONED", { unit: null, progress: null, total_pages: null }), "Yarım bıraktın"],
    [undefined, "Yarım bıraktın"],
  ])("abandonedText %#", (entry, want) => {
    expect(abandonedText(entry)).toBe(want);
  });

  it.each([
    ["2026-11-01", "2026-11-12", "1 Kasım 2026 – 12 Kasım 2026"],
    [null, "2026-11-12", "12 Kasım 2026"],
    ["2026-11-01", null, "1 Kasım 2026 başlangıç"],
    [null, null, "Tarih yok"],
  ])("dayRange(%s, %s)", (start, end, want) => {
    expect(dayRange(start, end)).toBe(want);
  });
});

it("editionLabel boş alanları atlar", () => {
  const e: Edition = {
    id: 1, title: "Dune", language: "tr", translator: "", publisher: "", publication_year: null, isbn: "",
    page_count: 412, cover_url: "", is_primary: true,
  };
  expect(editionLabel(e)).toBe("Türkçe · 412 sayfa");
  expect(editionLabel({ ...e, publisher: "Ithaki", publication_year: 2019, page_count: null })).toBe("Türkçe · Ithaki · 2019");
});

it("commandToast her komut için bir cümle; Devam et ayrı", () => {
  expect(commandToast({ command: "start" })).toBe("Okumaya başladın");
  expect(commandToast({ command: "start", body: { mode: "restart" } })).toBe("Okumaya başladın");
  expect(commandToast({ command: "start", body: { mode: "resume" } })).toBe("Kaldığın yerden devam ediyorsun");
  for (const command of ["want", "finish", "abandon", "postpone", "mark-read", "remove"] as const) {
    expect(commandToast({ command })).toMatch(/\S/);
  }
});
