import type { Edition, Reading, ReadingLogEntry, Shelf, ShelfCommand } from "../api/types";
import { formatDay, today } from "./day";
import { languageName } from "./format";

/*
 * Raf kontrolünün METİNLERİ (Faz 17A-1 adım 5). Saf fonksiyonlar: bileşen yalnız çizer, ne yazılacağına
 * burada karar verilir ve tabloyla test edilir (shelf.test.ts). Kitaplığım kartları (adım 6) da aynı
 * ilerleme metnini kullanır; iki yerde iki ayrı biçim olmaz.
 *
 * Türkçe ek sorunu: "Sayfa 42'de", "2025'te" gibi sayıya gelen ekler sayının OKUNUŞUNA göre değişir.
 * Metinler eki sayıya değil sabit bir kelimeye bağlayacak biçimde kuruldu ("Bitirdin · 12 Kasım 2025",
 * "Yarım bıraktın · 42 / 310 sayfa"); yanlış ek riski yok.
 */

type Progress = Pick<Reading, "unit" | "progress" | "total_pages">;

/**
 * "42 / 310 sayfa", "%35", toplam bilinmiyorsa "42. sayfa". İlerleme yoksa null: arayüz "İlerlemeni gir"
 * der. Toplamı bilinmeyen okumada 0 da "yok" sayılır: "0. sayfa" anlamsız (spec Doğrulama 8).
 */
export function progressText(p: Progress): string | null {
  if (p.unit === null || p.progress === null) return null;
  if (p.unit === "percent") return `%${p.progress}`;
  if (p.total_pages !== null) return `${p.progress} / ${p.total_pages} sayfa`;
  return p.progress > 0 ? `${p.progress}. sayfa` : null;
}

/** Çubuğun doluluğu (0–100, tam sayı); oran bilinmiyorsa null ve çubuk çizilmez. */
export function progressPercent(p: Progress): number | null {
  if (p.progress === null) return null;
  if (p.unit === "percent") return p.progress;
  if (p.unit === "page" && p.total_pages) return Math.min(100, Math.round((p.progress / p.total_pages) * 100));
  return null;
}

/**
 * İlerleme var mı: sunucunun "ertele ve çıkar geçmişe yazar" kuralının aynısı (progress > 0). Onay
 * diyaloğu yalnız bu durumda sorar; yoksa geçiş yan etkisizdir.
 */
export function hasProgress(r: Reading | null | undefined): boolean {
  return r?.progress != null && r.progress > 0;
}

/**
 * Bulunulan durumu açıklayan, son kapanmış okuma: Okudum'da son bitirilen, Yarım bıraktım'da son yarım
 * kalan. Geçmiş en yeni önce geldiği için ilk kayıt; sonucu duruma uymuyorsa (Faz 15'ten kalan durum ya
 * da kayıt silinmiş) yok sayılır.
 */
export function closedReadingOf(shelf: Shelf): ReadingLogEntry | undefined {
  const last = shelf.log[0];
  const status = shelf.entry?.status;
  if (status === "READ" && last?.outcome === "FINISHED") return last;
  if (status === "ABANDONED" && last?.outcome === "ABANDONED") return last;
  return undefined;
}

/** Okudum satırı: "Bugün bitirdin", "Bitirdin · 12 Kasım 2026", tarih yoksa "Okudun". */
export function finishedText(entry: ReadingLogEntry | undefined, now = new Date()): string {
  if (!entry?.ended_on) return "Okudun";
  if (entry.ended_on === today(now)) return "Bugün bitirdin";
  return `Bitirdin · ${formatDay(entry.ended_on)}`;
}

/** Yarım bıraktım satırı: nerede bırakıldığı biliniyorsa onunla. */
export function abandonedText(entry: ReadingLogEntry | undefined): string {
  const where = entry ? progressText(entry) : null;
  return where ? `Yarım bıraktın · ${where}` : "Yarım bıraktın";
}

/** Geçmiş satırının günleri: "1 Kasım 2026 – 12 Kasım 2026", tek gün ya da "Tarih yok". */
export function dayRange(startedOn: string | null, endedOn: string | null): string {
  if (startedOn && endedOn) return `${formatDay(startedOn)} – ${formatDay(endedOn)}`;
  if (endedOn) return formatDay(endedOn);
  if (startedOn) return `${formatDay(startedOn)} başlangıç`;
  return "Tarih yok";
}

const outcomeLabels: Record<ReadingLogEntry["outcome"], string> = { FINISHED: "Bitirdin", ABANDONED: "Yarım bıraktın" };

export function outcomeLabel(o: ReadingLogEntry["outcome"]): string {
  return outcomeLabels[o];
}

/** Baskı seçicisinde ve geçmişte baskının adı: "Türkçe · Yediveren · 2019 · 412 sayfa". */
export function editionLabel(e: Edition): string {
  return [languageName(e.language), e.publisher, e.publication_year, e.page_count ? `${e.page_count} sayfa` : null]
    .filter(Boolean)
    .join(" · ");
}

/** Komut başarıyla bitince gösterilen bildirim. */
export function commandToast(c: ShelfCommand): string {
  if (c.command === "start") return c.body?.mode === "resume" ? "Kaldığın yerden devam ediyorsun" : "Okumaya başladın";
  return toastText[c.command];
}

const toastText: Record<Exclude<ShelfCommand["command"], "start">, string> = {
  want: "Okumak istediklerine eklendi",
  finish: "Bitirdin",
  abandon: "Yarım bırakıldı",
  postpone: "Ertelendi",
  "mark-read": "Okuduklarına eklendi",
  remove: "Kitaplığından çıkarıldı",
};
