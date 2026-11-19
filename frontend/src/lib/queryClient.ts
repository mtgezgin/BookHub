import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "../api/client";

/**
 * Sunucu verisinin önbelleği (TanStack Query; Faz 16d, F33). Uygulamada TEK bir QueryClient var:
 * main.tsx onu kurar ve QueryClientProvider ile bütün bileşenlere verir. Bir bileşen useQuery ile
 * veri istediğinde önce burada, verinin anahtarıyla (queryKey) bakılır.
 *
 * Varsayılanlar paketin kaynağında doğrulandı (5.106.0, query-core): staleTime 0, gcTime 5 dk,
 * tarayıcıda hatalı sorgu 3 kez yeniden denenir, pencereye dönünce bayat sorgular tazelenir.
 * Aşağıda yalnız bize uymayan ikisini değiştiriyoruz; gcTime ve pencere davranışı varsayılanda kalır.
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // NEDEN 30 sn: staleTime 0 olsaydı veri gelir gelmez "bayat" sayılır, sayfalar arası her
        // geçişte (ve pencereye her dönüşte) yeniden çekilirdi; önbellek yalnız "bir an önceki veriyi
        // göster" işine yarardı. Sunucudaki önbellekler zaten 60 sn (katalog sayımı, en sevilenler):
        // istemcide 30 sn, kullanıcının görebileceği bayatlığı neredeyse artırmaz. Seyrek değişen veri
        // (türler) kendi sorgusunda daha uzun süre alır.
        staleTime: 30_000,
        retry: shouldRetry,
      },
      // Değişiklikler (puan ver, rafa ekle…) HİÇ yeniden denenmez: aynı yorumu iki kez yazmak gibi
      // yan etkiler doğurabilir. TanStack'in varsayılanı da bu; açıkça yazıyoruz ki değiştirilmesin.
      mutations: { retry: false },
    },
  });
}

/**
 * Hatalı bir okuma yeniden denenmeli mi?
 *
 * - 4xx (ApiError.status 400–499): HAYIR. İstek yanlış ya da yetkisiz; aynısını göndermek aynı cevabı
 *   alır. 401'i client.ts zaten kendisi ele alıyor (oturumu yeniler ve isteği bir kez tekrarlar):
 *   burada da denersek oturum yenileme iki kat çalışırdı.
 * - 5xx ve ağ hatası: BİR kez. Geçici olabilir (yayın sırasında kısa kesinti, soğuk bağlantı).
 *   Varsayılan 3 deneme ve artan bekleme, hata ekranını ~7 sn geciktirirdi.
 */
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 1;
}
