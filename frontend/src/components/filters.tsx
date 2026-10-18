import { useCallback, type ReactNode } from "react";
import type { SetURLSearchParams } from "react-router-dom";
import { Button } from "./ui/button";

// Katalog (Home) ile Kitaplığım'ın ortak süzgeç parçaları: kart ızgarası, çip düğmesi ve adres güncelleyici.

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
