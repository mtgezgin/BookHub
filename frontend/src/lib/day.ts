/*
 * TAKVİM GÜNÜ yardımcıları (Faz 17A-1, spec kavram 6). Okumanın başlangıç ve bitişi bir an değil bir
 * gündür ("12 Kasım'da bitirdim"); API onu saatsiz "YYYY-AA-GG" olarak taşır, "bugün"ü sunucu değil
 * istemci söyler. İki tuzak var, ikisi de saat dilimi:
 *
 *  1. `new Date().toISOString().slice(0, 10)` UTC günüdür. Türkiye UTC+3: 00:00–03:00 arasında bitirilen
 *     kitap DÜN bitmiş kaydedilirdi. Bugün yerel alanlardan (getFullYear/getMonth/getDate) kurulur.
 *  2. `new Date("2026-11-12")` UTC gece yarısı olarak okunur (ECMAScript: saatsiz ISO biçimi UTC'dir).
 *     Batıdaki bir saat diliminde ekranda "11 Kasım" yazardı. Gün parçalarından YEREL bir tarih kurulur.
 *
 * Günle uğraşan her yer bu dosyadan geçer; tuzaklar tek yerde ve testli (day.test.ts).
 */

const pad = (n: number) => String(n).padStart(2, "0");

/** Kullanıcının yerel takvimine göre bugün, "YYYY-AA-GG". `now` testte sabitlensin diye parametre. */
export function today(now = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/** "2026-11-12" → "12 Kasım 2026". Gün parçalarından yerel tarih kurar, UTC'ye hiç uğramaz. */
export function formatDay(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString("tr-TR", { day: "numeric", month: "long", year: "numeric" });
}
