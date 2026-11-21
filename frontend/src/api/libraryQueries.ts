import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import { keepPreviousOwnData, type LibraryParams, queryKeys } from "./queries";
import type { LibraryStatus } from "./types";

/*
 * Kitaplık ekranlarının kancaları (Faz 16d adım 7). AYRI dosyada çünkü yalnız tembel yüklenen iki parça
 * kullanıyor: Kitaplığım sayfası ve kitap sayfasındaki kitaplık menüsü (LibraryMenu). Burada dursalar her
 * ziyaretçinin indirdiği ana pakete girmezler (adminQueries.ts ile aynı gerekçe). Katalogdaki rozetlerin
 * kancası (components/useLibraryStatuses.tsx) ana pakette, çünkü Home onu her açılışta kullanıyor.
 */

/**
 * Kitaplığım listesi: filtreler ve sayfa anahtarın parçası. Anahtar değişince yeni cevap gelene kadar
 * AYNI kullanıcının önceki sayfası ekranda kalır (keepPreviousOwnData); yoksa her filtre tıklamasında ızgara
 * boşalıp iskelete dönerdi.
 */
export function useLibrary(userId: number | undefined, params: LibraryParams) {
  return useQuery({
    queryKey: queryKeys.library(userId ?? 0, params),
    queryFn: () => api.library(params),
    placeholderData: keepPreviousOwnData(userId),
    enabled: userId !== undefined,
  });
}

/**
 * Kitap sayfasındaki menü için TEK kitabın durumu; kitaplıkta değilse null. Katalog rozetleriyle aynı uç ve
 * aynı anahtar biçimi (kimlik listesi yalnız "12"): önbellekte cevabın tamamı durur, `select` bileşene
 * yalnız bu kitabın durumunu verir.
 */
export function useLibraryStatus(userId: number | undefined, bookId: number) {
  return useQuery({
    queryKey: queryKeys.libraryStatuses(userId ?? 0, String(bookId)),
    queryFn: () => api.libraryStatuses([bookId]),
    select: (r) => r.statuses[String(bookId)] ?? null,
    enabled: userId !== undefined,
  });
}

/*
 * Kitaplık DEĞİŞİKLİKLERİ: rafa ekleme / durum değiştirme ve çıkarma. Başarıda bu kullanıcının BÜTÜN
 * kitaplık kayıtları geçersiz olur (["me", kullanıcı, "library"] öneki):
 *  - ekrandakiler hemen yeniden çekilir: kitap sayfasındaki düğme, Kitaplığım'daki liste;
 *  - ötekiler açılınca: katalog kartlarındaki rozetler, öteki filtrelerin ve sayfaların listeleri.
 * Adımın asıl dersi bu: değişiklik bir bileşende (menü), etkilediği veri başka bileşenlerde (rozet, liste).
 * Aralarında hiçbir bağlantı yok; ortak noktaları yalnız anahtar.
 *
 * NEDEN cevabı önbelleğe doğrudan yazmıyoruz (puandaki setQueryData gibi): bir kitabın durumu tek bir
 * kayıtta durmuyor, ekranda görünen her kimlik listesinin kaydında var. Hepsini bulup düzeltmek yerine
 * sunucuya yeniden sormak: bir küçük istek, karşılığında tek bir kural.
 *
 * Geçersiz kılma, o sırada süren bir okumayı (ör. pencereye dönünce başlayan tazeleme) iptal edip yeniden
 * başlatır (invalidateQueries → refetchQueries, cancelRefetch varsayılanı true). Değişiklikten ÖNCE başlamış
 * bir okumanın eski durumu ekrana yazması böylece önlenir.
 *
 * onSuccess Promise döndürür: ekrandaki kayıtlar yenilenene kadar değişiklik sürüyor sayılır (isPending),
 * menü pasif kalır. Hata olursa hiçbir şey geçersiz kılınmaz: değişiklik olmadıysa veri de değişmemiştir.
 * Çıkarma sunucuda tekrarlanabilir (kayıt yoksa da başarılı döner), o yüzden yorum silmedeki gibi hatada
 * tazeleme gerekmiyor.
 */
export function useSetLibraryStatus(userId: number | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ bookId, status }: { bookId: number; status: LibraryStatus }) => api.setLibraryStatus(bookId, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me", userId, "library"] }),
  });
}

export function useRemoveFromLibrary(userId: number | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (bookId: number) => api.removeFromLibrary(bookId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me", userId, "library"] }),
  });
}
