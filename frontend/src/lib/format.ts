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
