import { useCallback, type ReactNode } from "react";
import type { SetURLSearchParams } from "react-router-dom";
import { Button } from "./ui/button";

// Katalog (Home) ile Kitaplığım'ın ortak süzgeç parçaları: kart ızgarası, çip düğmesi ve adres güncelleyici.

// Adres elle değiştirilebilir: sunucunun 400 vereceği değer API'ye gitmez, o süzgeç yokmuş gibi davranılır.
// Sınırlar sunucuyla aynı: backend/cmd/api/books.go (maxQueryLen, maxPageOffset) ve validate.go (slugPattern).
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const MAX_QUERY = 100;
const MAX_OFFSET = 10_000;
export const asGenre = (v: string | null) => {
  const s = (v ?? "").trim();
  return SLUG.test(s) ? s : "";
};
// Sunucu rune sayar; [...s] de kod noktası sayar (ç, ğ tek karakter).
export const asQuery = (v: string | null) => {
  const s = (v ?? "").trim();
  return [...s].length <= MAX_QUERY ? s : "";
};
/**
 * Yazarken arama (Enter'a basmadan) bu metin için yapılır mı? Metin kırpılmış hâliyle verilir.
 *
 * - Tek kelime: en az MIN_AUTO_SEARCH harf. Bir iki harfin öneki kataloğun büyük bölümüyle eşleşir:
 *   sonuç işe yaramaz, sunucudaki en pahalı sorgu da budur (500 bin kitapta 2 harf ortanca 29 ms,
 *   en kötü 180 ms; 3 harf 7,6 ms). Yazarken en çok istek de bu aşamada gider.
 * - Birden çok kelime: son kelime en az 2 harf. "ince m" yazılırken kullanıcı yeni kelimenin ortasındadır;
 *   tek harflik önek yine geniş eşleşir ve bir harf sonra zaten yeniden aranacaktır.
 * - Boş metin her zaman geçerlidir: kutu boşalınca arama kalkar.
 *
 * Kural yalnız OTOMATİK aramayı sınırlar. Enter her uzunlukta arar ("Us", "O" gibi kısa adlar için).
 */
export const MIN_AUTO_SEARCH = 3;
export const autoSearchable = (text: string) => {
  if (text === "") return true;
  const words = text.split(/\s+/);
  const last = [...words[words.length - 1]].length; // kod noktası: ç, ğ tek harf
  return words.length === 1 ? last >= MIN_AUTO_SEARCH : last >= 2;
};

export const asOffset = (v: string | null) => {
  const n = Number.parseInt(v ?? "0", 10);
  return Number.isFinite(n) && n > 0 && n <= MAX_OFFSET ? n : 0;
};

// 12'lik sayfa her sütun sayısında satırları tam doldurur (2, 3, 4, 6).
export const GRID = "grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6";

export function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <Button size="sm" variant={active ? "default" : "outline"} aria-pressed={active} className="shrink-0 rounded-full" onClick={onClick}>
      {children}
    </Button>
  );
}

/**
 * Süzgeç durumunu adreste günceller: boş değer anahtarı siler; `offset` verilmediyse sayfa başa döner
 * (filtre değişince ilk sayfa). Dikkat: react-router 7'de `prev`, setParams'ın OLUŞTUĞU render'daki
 * URL'dir; gecikmeli çağıranlar son render'ın güncelleyicisini kullanmalı (Home'daki `latest`).
 */
export function useParamUpdater(setParams: SetURLSearchParams) {
  return useCallback(
    (next: Record<string, string>, replace = false) =>
      setParams((prev) => {
        const p = new URLSearchParams(prev);
        for (const [k, v] of Object.entries(next)) (v ? p.set(k, v) : p.delete(k));
        if (!("offset" in next)) p.delete("offset");
        return p;
      }, { replace }),
    [setParams],
  );
}
