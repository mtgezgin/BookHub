import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { api } from "./client";

/**
 * Sunucu verisinin OKUMA kancaları (Faz 16d). Sayfalar api.* fonksiyonlarını useEffect içinde çağırmak
 * yerine bunları kullanır; önbellek, eski cevap koruması ve yeniden deneme TanStack Query'den gelir.
 *
 * Sorgu ANAHTARLARI tek yerde (queryKeys): bir veri hem burada okunur hem başka bir yerde (ör. moderasyon
 * işleminden sonra) "geçersiz" ilan edilir; iki yer anahtarı ayrı ayrı yazarsa biri değişince öteki
 * sessizce boşa düşer. Anahtar bir DİZİ ve hiyerarşik: ["books"] ile başlayan her şey kitap listesidir,
 * invalidateQueries({ queryKey: ["books"] }) liste ve şerit dahil hepsini tazeler.
 */
export type BookListParams = Parameters<typeof api.books>[0];

export const queryKeys = {
  genres: ["genres"] as const,
  // Parametreler nesne olarak anahtarın içinde: TanStack nesneyi alan sırasından bağımsız karşılaştırır,
  // { q, genre } ile { genre, q } aynı kayıttır.
  books: (params: BookListParams) => ["books", "list", params] as const,
  topRated: (limit: number) => ["books", "top-rated", limit] as const,
};

/**
 * Türler: Home'da ve Kitaplığım'da aynı liste. Anahtar aynı olduğu için iki sayfa TEK kaydı paylaşır;
 * sayfalar arası geçişte istek ikinci kez gitmez.
 *
 * NEDEN 5 dk: türleri yalnız admin değiştirir ve nadiren. Genel 30 sn'lik süre (lib/queryClient.ts) bu
 * veri için gereksiz istek demek. Admin bir tür eklediğinde moderasyon ekranı bu anahtarı geçersiz
 * kılacak (adım 6); o zamana kadar admin'in kendisi en çok 5 dk eski listeyi görür.
 */
export function useGenres() {
  return useQuery({
    queryKey: queryKeys.genres,
    queryFn: () => api.genres(),
    staleTime: 5 * 60_000,
  });
}

/**
 * Katalog sayfası (arama, tür, sıra, sayfa).
 *
 * ESKİ CEVAP KORUMASI burada kendiliğinden gelir: her cevap KENDİ anahtarının kaydına yazılır, ekran
 * yalnız güncel anahtarın kaydını gösterir. "Dune" araması dönmüşken geç gelen filtresiz cevap filtresiz
 * kaydı doldurur ama ekrana çıkmaz (eskiden bunu Home.tsx'te elle bir bayrakla yapıyorduk; F10 sınıfı).
 *
 * placeholderData: keepPreviousData: anahtar değişince (yeni sayfa, yeni tür) yeni cevap gelene kadar
 * ÖNCEKİ sayfa ekranda kalır ve `isPlaceholderData` true olur; arayüz bunu "meşgul" diye soluk gösterir.
 * Bu olmasaydı her sayfa geçişinde ızgara boşalıp iskelete dönerdi.
 */
export function useBooks(params: BookListParams) {
  return useQuery({
    queryKey: queryKeys.books(params),
    queryFn: () => api.books(params),
    placeholderData: keepPreviousData,
  });
}

/** "Okurların en sevdikleri" şeridi. Sunucu bu listeyi zaten 60 sn önbellekte tutuyor. */
export function useTopRated(limit: number) {
  return useQuery({
    queryKey: queryKeys.topRated(limit),
    queryFn: () => api.topRated(limit),
  });
}
