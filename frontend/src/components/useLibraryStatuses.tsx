import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { keepPreviousOwnData, queryKeys } from "../api/queries";
import type { LibraryStatus } from "../api/types";
import { useAuth } from "../auth/AuthContext";

const NONE: Record<number, LibraryStatus> = {};

/**
 * Ekrandaki kitapların kitaplık durumları — kitap başına bir istek değil, TEK istek (N+1 yok).
 * Kişisel veri katalog cevabına gömülmez: katalog ve "en sevilenler" herkes için aynı kalır
 * (ortak önbellek), durum ayrı uçtan gelir. Giriş yoksa ya da liste boşsa istek atılmaz.
 * Anahtar sıralı ve tekrarsız kimliklerden kurulur: aynı kitaplar başka sırayla gelince yeniden sormaz.
 * Okuma başarısızsa kartlar rozetsiz kalır (süs bilgi; sayfayı bozmaz).
 * Kimlikler değişince yeni cevap gelene kadar öncekiler gösterilir (sayfa değişirken rozetler yanıp sönmesin).
 *
 * Faz 16d adım 7: artık bir sorgu. Eskiden burada bir etki, geç gelen cevap bayrağı ve "bu durumlar kime
 * ait" kaydı vardı:
 *  - geç gelen cevap: her kimlik listesi kendi anahtarına yazılır, ekran yalnız güncelini gösterir;
 *  - kime ait: kullanıcı kimliği anahtarın parçası; yer tutucu da yalnız aynı kullanıcınınsa gösterilir
 *    (keepPreviousOwnData);
 *  - okuma başarısızsa: hata olunca veri yok, kartlar rozetsiz. Fark: okuma artık 5xx'te bir kez yeniden
 *    deneniyor (lib/queryClient.ts), eskiden denenmiyordu.
 * Kazanç: kitap sayfasında rafa eklemek bu kayıtları geçersiz kılar (api/libraryQueries.ts); kataloğa
 * dönünce rozet güncel. Eskiden de günceldi ama her açılışta yeniden sorulduğu için; şimdi önbellekten
 * gelen kayıt yalnız bir değişiklik onu eskittiyse yeniden sorulur.
 */
export function useLibraryStatuses(bookIds: number[]): Record<number, LibraryStatus> {
  // Kimliğe bağlı: sessiz token yenilemesi yeni bir kullanıcı nesnesi getirir, aynı okur için yeniden sorulmaz.
  const userId = useAuth().user?.id;
  const ids = [...new Set(bookIds)].sort((a, b) => a - b).join(",");
  const query = useQuery({
    queryKey: queryKeys.libraryStatuses(userId ?? 0, ids),
    queryFn: () => api.libraryStatuses(ids.split(",").map(Number)),
    enabled: userId !== undefined && ids !== "",
    placeholderData: keepPreviousOwnData(userId),
  });
  // JSON nesne anahtarları zaten metin: statuses[3] ile statuses["3"] aynı alanı okur.
  return userId === undefined ? NONE : (query.data?.statuses ?? NONE);
}
