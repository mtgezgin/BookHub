// Türkçe biçimler. toFixed() ondalık NOKTA yazar ("8.1"); Türkçede virgül ("8,1").

const scoreFormats = new Map<number, Intl.NumberFormat>();

export function formatScore(value: number, digits = 1): string {
  let f = scoreFormats.get(digits);
  if (!f) {
    f = new Intl.NumberFormat("tr-TR", { minimumFractionDigits: digits, maximumFractionDigits: digits });
    scoreFormats.set(digits, f);
  }
  return f.format(value);
}

const languages = new Intl.DisplayNames(["tr"], { type: "language", fallback: "code" });

/** "en" → "İngilizce". Bilinmeyen ya da bozuk kod olduğu gibi döner (veriden geldiği için patlamamalı). */
export function languageName(code: string): string {
  if (!code) return "";
  try {
    return languages.of(code) ?? code;
  } catch {
    return code;
  }
}

const relative = new Intl.RelativeTimeFormat("tr", { numeric: "auto" });
const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * "3 gün önce", "dün", "2 saat önce". Bir aydan eskiyse tam tarih ("1 Eylül 2026"): "47 gün
 * önce" okuması zor. `now` testte sabitlensin diye parametre.
 */
export function timeAgo(iso: string, now = Date.now()): string {
  const diff = now - Date.parse(iso);
  if (diff < MINUTE) return "az önce";
  if (diff < HOUR) return relative.format(-Math.floor(diff / MINUTE), "minute");
  if (diff < DAY) return relative.format(-Math.floor(diff / HOUR), "hour");
  if (diff < 30 * DAY) return relative.format(-Math.floor(diff / DAY), "day");
  return formatDate(iso);
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("tr-TR", { day: "numeric", month: "long", year: "numeric" });
}

/** Avatar için adın ilk harfi; Türkçe büyük harf ("ilkay" → "İ"). */
export function initialOf(name: string): string {
  const first = Array.from(name.trim())[0];
  return first ? first.toLocaleUpperCase("tr") : "?";
}
