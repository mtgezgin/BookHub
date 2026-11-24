import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import { keepPreviousOwnData, type LibraryParams, queryKeys } from "./queries";
import type { LibraryStatus, ReadingLogDates, ReadingPatch, Shelf, ShelfCommand } from "./types";

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

/*
 * RAF (Faz 17A-1): kitap sayfasındaki tek raf kontrolünün verisi. Kitap sayfası artık rozet ucunu değil
 * bunu okur; eski iki kanca (useLibraryStatus, useSetLibraryStatus) raf kontrolü gelene kadar (adım 5)
 * LibraryMenu için duruyor.
 */

/** Bir kitabın rafı: durum, süren okuma, sürüm ve geçmiş. Girişsiz kullanıcıda sorgu hiç çalışmaz. */
export function useShelf(userId: number | undefined, bookId: number) {
  return useQuery({
    queryKey: queryKeys.shelf(userId ?? 0, bookId),
    queryFn: () => api.shelf(bookId),
    enabled: userId !== undefined,
  });
}

/** Raf değişikliğinden sonra eskiyen öteki kitaplık kayıtları; anahtarın dördüncü öğesi. */
type LibraryView = "list" | "statuses";

/**
 * Raf DEĞİŞİKLİKLERİNİN ortak kuralları; dört kanca yalnız neyi gönderdiklerinde ve neyi eskittiklerinde
 * ayrışır.
 *
 * BAŞARIDA cevap önbelleğe DOĞRUDAN yazılır (setQueryData): her raf ucu güncel rafın tamamını döner, yani
 * yeni değeri biliyoruz; Faz 15'teki kancalar gibi "eskit, yeniden sor" bir istek fazlası olurdu. Önce o
 * rafın SÜREN okuması iptal edilir (cancelQueries): değişiklikten önce başlamış bir GET (ör. pencereye
 * dönünce başlayan tazeleme) sonra gelip eski rafı yazmasın. Anahtar cevabın book_id'sinden: sunucu hangi
 * kitabın rafını döndüğünü kendisi söylüyor.
 *
 * Öteki görünümler (Kitaplığım listesi, katalog rozetleri) yalnız ESKİTİLİR, beklenmez: ekrandaki raf zaten
 * güncel, kontrol listenin yeniden gelmesini beklemeden açılır. Başka kitapların rafları eskitilmez; bir
 * kitaptaki komut ötekinin rafını değiştiremez.
 *
 * HATADA yalnız "ekrandaki raf eskimiş" diyen iki cevap rafı yeniden okutur:
 *  - 409 version_conflict: ilerleme başka bir sekmede güncellenmiş (spec § Kararlar "409 cevabı"). Gövdede
 *    güncel raf yok; istemci bir kez GET yapar.
 *  - 404: okuma ya da geçmiş kaydı başka bir yerde kapanmış / silinmiş.
 * onError bir Promise döndürür ve TanStack onu bekleyip hatayı SONRA yayar: arayüz hatayı gördüğünde raf
 * zaten yenidir, "İlerlemen başka bir yerde güncellenmiş: sayfa 57" mesajını yeni veriden kurar. Sessiz
 * yeniden deneme yok; ne yapılacağına kullanıcı karar verir (adım 5). 400 ve 500 hiçbir şeyi eskitmez:
 * değişiklik olmadıysa veri de değişmemiştir.
 */
function useShelfMutation<V>(
  userId: number | undefined,
  bookId: number,
  send: (vars: V) => Promise<Shelf>,
  stale: readonly LibraryView[],
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: send,
    onSuccess: async (shelf) => {
      if (userId === undefined) return;
      const key = queryKeys.shelf(userId, shelf.book_id);
      await queryClient.cancelQueries({ queryKey: key, exact: true });
      queryClient.setQueryData(key, shelf);
      if (stale.length > 0) {
        void queryClient.invalidateQueries({
          queryKey: ["me", userId, "library"],
          predicate: (q) => (stale as readonly unknown[]).includes(q.queryKey[3]),
        });
      }
    },
    onError: (err) => {
      if (userId === undefined || !showsStaleShelf(err)) return;
      return queryClient.invalidateQueries({ queryKey: queryKeys.shelf(userId, bookId), exact: true });
    },
  });
}

function showsStaleShelf(err: unknown): boolean {
  return err instanceof ApiError && (err.code === "version_conflict" || err.status === 404);
}

/**
 * Komutlar (istiyorum, başla, bitir, yarım bırak, ertele, zaten okudum, çıkar). Durum değişebilir: hem
 * Kitaplığım listesi hem rozetler eskir. `mutate({ command: "finish", body: { ended_on: today() } })`.
 */
export function useShelfCommand(userId: number | undefined, bookId: number) {
  return useShelfMutation(userId, bookId, (c: ShelfCommand) => api.shelfCommand(bookId, c), ["list", "statuses"]);
}

/**
 * Okurken düzeltme (ilerleme, başlangıç günü, birim, baskı). Durum değişmez, rozetler aynı kalır; yalnız
 * Kitaplığım kartlarındaki ilerleme eskir. version zorunlu: rafın son bilinen sürümü.
 */
export function useUpdateReading(userId: number | undefined, bookId: number) {
  return useShelfMutation(userId, bookId, (p: ReadingPatch) => api.updateReading(bookId, p), ["list"]);
}

/**
 * Geçmişteki bir okumanın günleri ("Okudun · tarih ekle" dahil) ve geçmişten silme. Kitaplık satırına
 * dokunmazlar: liste de rozetler de değişmez, yalnız raf yazılır. bookId yalnız 404'te hangi rafın
 * yeniden okunacağı için (kayıt kimliği kitabı söylemiyor).
 */
export function useUpdateReadingLog(userId: number | undefined, bookId: number) {
  return useShelfMutation(
    userId, bookId, ({ logId, dates }: { logId: number; dates: ReadingLogDates }) => api.updateReadingLog(logId, dates), [],
  );
}

export function useDeleteReadingLog(userId: number | undefined, bookId: number) {
  return useShelfMutation(userId, bookId, (logId: number) => api.deleteReadingLog(logId), []);
}
