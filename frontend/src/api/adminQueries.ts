import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import { queryKeys } from "./queries";
import type { BookStatus } from "./types";

/*
 * Yalnız admin ekranının (Moderasyon) kancaları. AYRI dosyada çünkü Moderasyon tembel yüklenen bir parça
 * (App.tsx'te lazy): bu dosyayı yalnız o içe aktardığı için kod her ziyaretçinin indirdiği ana pakete
 * girmez. queries.ts'te dursaydı, o dosyayı katalog da içe aktardığı için ana pakete girerdi. Anahtarlar
 * yine tek yerde (queryKeys), çünkü buradaki değişiklikler katalog kayıtlarını da geçersiz kılıyor.
 */

/** Admin moderasyon kuyruğu: duruma (sekme) ve sayfaya göre. */
export function useAdminBooks(status: BookStatus, offset: number) {
  return useQuery({
    queryKey: queryKeys.adminBooks(status, offset),
    queryFn: () => api.adminBooks(status, offset),
    placeholderData: keepPreviousData,
  });
}

/** Moderasyon geçişleri; red gerekçesi yalnız red'de var (tip bunu zorunlu kılar). */
export type ModerationAction =
  | { kind: "approve" | "archive" | "restore"; id: number }
  | { kind: "reject"; id: number; reason: string };

/**
 * Moderasyon işlemi. Başarıda iki şey geçersiz olur:
 *  1. Bütün moderasyon sekmeleri (["admin", "books"]): kitap bir sekmeden ötekine geçti. Ekrandaki sekme
 *     hemen yeniden çekilir; öteki sekmeler açılınca.
 *  2. Bütün katalog kayıtları (["books"]): onaylanan kitap kataloğa girer, arşivlenen çıkar, kitap sayfası
 *     da değişir. Moderasyon ekranında bu kayıtlar ekranda olmadığı için şimdi istek gitmez.
 *
 * onSuccess yalnız (1)'in Promise'ini döndürür: işlem, ekrandaki liste yenilenene kadar "sürüyor" sayılır.
 * Bu, eski koddaki "koruma liste yeniden yüklenene kadar sürer" kuralının karşılığı: istek dönse bile
 * satır eski düğmeleriyle ekrandayken ikinci bir onay 409 alırdı.
 *
 * Eskiden bu iş "tıklandığı andaki reload" ile yapılıyordu ve işlem sürerken sekme değişirse ESKİ sekmeyi
 * çekip yenisinin üstüne yazıyordu (bir ref'le önlenmişti). Geçersiz kılma anahtara göre çalıştığı için
 * bu sorun yok: hangi sekme o an ekrandaysa o yeniden çekilir.
 */
export function useModerateBook() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (a: ModerationAction) => {
      switch (a.kind) {
        case "approve": return api.approve(a.id);
        case "archive": return api.archive(a.id);
        case "restore": return api.restoreBook(a.id);
        case "reject": return api.reject(a.id, a.reason);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["books"] });
      return queryClient.invalidateQueries({ queryKey: ["admin", "books"] });
    },
  });
}

/**
 * Yeni tür (admin). Başarıda tür listesi geçersiz olur: 5 dakikalık staleTime'ı (useGenres) beklemeden,
 * katalogdaki ve kitaplıktaki filtre yeni türü bir sonraki açılışta gösterir.
 */
export function useCreateGenre() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.createGenre(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.genres }),
  });
}
