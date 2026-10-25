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
