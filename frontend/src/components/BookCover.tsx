import { useState } from "react";
import { cn } from "../lib/utils";

/** Üretilmiş kapak renkleri: koyu, doygun zeminler + krem yazı (yazı/zemin kontrastı ≥ 4.5:1). */
export const COVER_PALETTE = [
  { bg: "oklch(0.38 0.07 162)", fg: "oklch(0.96 0.02 85)" }, // yeşil
  { bg: "oklch(0.36 0.11 15)", fg: "oklch(0.96 0.02 85)" }, // bordo
  { bg: "oklch(0.33 0.07 255)", fg: "oklch(0.96 0.02 85)" }, // lacivert
  { bg: "oklch(0.42 0.08 60)", fg: "oklch(0.97 0.02 85)" }, // toprak
  { bg: "oklch(0.36 0.09 320)", fg: "oklch(0.96 0.02 85)" }, // mürdüm
  { bg: "oklch(0.4 0.07 200)", fg: "oklch(0.96 0.02 85)" }, // petrol
  { bg: "oklch(0.44 0.12 40)", fg: "oklch(0.97 0.02 85)" }, // kiremit
  { bg: "oklch(0.33 0.02 250)", fg: "oklch(0.96 0.02 85)" }, // arduvaz
] as const;

/**
 * Başlık + yazardan SABİT bir renk seçer (FNV-1a özeti): aynı kitap her yüklemede, her
 * ortamda aynı kapağı alır. id kullanılmadı: lokal ve canlıda aynı kitabın id'si farklı.
 */
export function coverPalette(title: string, author: string) {
  let h = 0x811c9dc5;
  for (const ch of `${title}|${author}`) {
    h ^= ch.codePointAt(0)!;
    h = Math.imul(h, 0x01000193);
  }
  return COVER_PALETTE[(h >>> 0) % COVER_PALETTE.length];
}

/**
 * Kitap kapağı (2:3). `src` varsa resim; yoksa ya da resim yüklenemezse üretilmiş kapak.
 * Dekoratiftir (aria-hidden): başlık kartta/sayfada zaten metin olarak var.
 */
export function BookCover({ title, author, src, className }: {
  title: string;
  author: string;
  src?: string;
  className?: string;
}) {
  // Yüklenemeyen ADRES tutulur: aynı kapak başka bir resme geçince eski hata taşınmaz.
  const [broken, setBroken] = useState<string | null>(null);
  const frame = "relative aspect-[2/3] w-full overflow-hidden rounded-md shadow-md";

  // src backend'de yalnız http(s) olarak kabul ediliyor (javascript: XSS'i yok).
  if (src && broken !== src) {
    return (
      <div aria-hidden="true" className={cn(frame, "bg-muted", className)}>
        {/* no-referrer: kapak başka bir sunucudan (Open Library) gelebilir; hangi sayfada olunduğu ona gitmez. */}
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          className="size-full object-cover"
          onError={() => setBroken(src)}
        />
      </div>
    );
  }

  const p = coverPalette(title, author);
  // Tarayıcılar Türkçeyi heceleyemiyor (hyphens:auto çalışmıyor): punto, en uzun kelime yazı
  // alanına (kapak genişliğinin ~%60'ı) sığacak kadar küçülür. Serif harf ≈ 0,58 em genişlikte.
  const longest = Math.max(...title.split(/\s+/).map((w) => w.length));
  const fontSize = `clamp(0.65rem, ${Math.min(10, 60 / (longest * 0.58)).toFixed(2)}cqw, 1.4rem)`;
  return (
    <div aria-hidden="true" className={cn(frame, "@container", className)} style={{ backgroundColor: p.bg, color: p.fg }}>
      {/* Sırt gölgesi: düz bir dikdörtgeni "kitap" gibi gösteren tek ayrıntı. */}
      <div className="absolute inset-y-0 left-0 w-[6%] bg-black/20" />
      <div className="absolute inset-[7%] left-[13%] flex flex-col justify-between border border-current/30 p-[8%]">
        {/* overflow-wrap:anywhere — "Otostopçunun" gibi uzun bir kelime çerçeveden taşıp kırpılmasın, satıra bölünsün. */}
        <p className="line-clamp-5 font-serif leading-tight font-semibold text-balance [overflow-wrap:anywhere]" style={{ fontSize }}>
          {title}
        </p>
        <p className="line-clamp-2 text-[clamp(0.55rem,6.5cqw,0.8rem)] tracking-widest uppercase opacity-85">{author}</p>
      </div>
    </div>
  );
}
