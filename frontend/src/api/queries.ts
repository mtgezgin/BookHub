import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import type { BookStatus } from "./types";

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
export type LibraryParams = Parameters<typeof api.library>[0];

export const queryKeys = {
  genres: ["genres"] as const,
  // Parametreler nesne olarak anahtarın içinde: TanStack nesneyi alan sırasından bağımsız karşılaştırır,
  // { q, genre } ile { genre, q } aynı kayıttır.
  books: (params: BookListParams) => ["books", "list", params] as const,
  topRated: (limit: number) => ["books", "top-rated", limit] as const,
  book: (id: number) => ["books", "detail", id] as const,
  ratingSummary: (bookId: number) => ["ratings", "summary", bookId] as const,
  // Kullanıcıya özel veri "me" altında ve KULLANICI KİMLİĞİYLE: aynı tarayıcıda başka biri giriş yaparsa
  // anahtar farklıdır, öncekinin puanı önbellekten görünmez. Çıkışta "me" kayıtları ayrıca silinecek (adım 8).
  myRating: (userId: number, bookId: number) => ["me", userId, "rating", bookId] as const,
  // Yorumlar kitabın ALTINDA değil ayrı bir kökte: ["books"] her puandan sonra bayat işaretleniyor;
  // yorumlar onun altında olsaydı puan vermek yorumları da gereksiz yere yeniden çekerdi.
  comments: (bookId: number, offset: number) => ["comments", bookId, offset] as const,
  adminBooks: (status: BookStatus, offset: number) => ["admin", "books", status, offset] as const,
  // Kitaplık (adım 7): liste ve durum rozetleri AYNI önekte, ["me", kullanıcı, "library"]. Rafa ekleme ya
  // da çıkarma bu öneki geçersiz kılar; Kitaplığım listesi, katalogdaki rozetler ve kitap sayfasındaki
  // düğme tek satırla tazelenir (api/libraryQueries.ts).
  library: (userId: number, params: LibraryParams) => ["me", userId, "library", "list", params] as const,
  // ids: sıralı ve tekrarsız kimlikler ("3,4,9"); aynı kitaplar başka sırayla gelince aynı kayıt kullanılır.
  libraryStatuses: (userId: number, ids: string) => ["me", userId, "library", "statuses", ids] as const,
  // Raf (Faz 17A-1): kitap sayfasının tek raf kontrolü. Kitaplık önekinin ALTINDA: çıkışta temizlik ve
  // eski kancaların "kitaplığı eskit"i onu kendiliğinden kapsar (spec § Kararlar "Veri katmanı").
  shelf: (userId: number, bookId: number) => ["me", userId, "library", "shelf", bookId] as const,
};

/**
 * Kullanıcıya özel veride `keepPreviousData`'nın güvenli hâli: anahtar değişirken önceki kaydı yer tutucu
 * olarak gösterir, ama yalnız o kayıt AYNI kullanıcınınsa (anahtarın ikinci öğesi, ["me", kullanıcı, …]).
 * Düz keepPreviousData kullanıcı değişince öncekinin rozetlerini ya da kitaplığını bir an gösterirdi.
 *
 * NEDEN her çizimde yeni bir fonksiyon: TanStack, yer tutucu fonksiyonu öncekiyle AYNI nesneyse önceki yer
 * tutucuyu yeniden kullanır, fonksiyonu çağırmaz (query-core queryObserver.js). userId'yi kapsayan yeni
 * bir fonksiyon, kullanıcı değişince kontrolün yeniden yapılmasını garanti eder.
 */
export function keepPreviousOwnData(userId: number | undefined) {
  return <T>(previous: T | undefined, previousQuery: { queryKey: readonly unknown[] } | undefined) =>
    previousQuery?.queryKey[1] === userId ? previous : undefined;
}

/**
 * Türler: Home'da ve Kitaplığım'da aynı liste. Anahtar aynı olduğu için iki sayfa TEK kaydı paylaşır;
 * sayfalar arası geçişte istek ikinci kez gitmez.
 *
 * NEDEN 5 dk: türleri yalnız admin değiştirir ve nadiren. Genel 30 sn'lik süre (lib/queryClient.ts) bu
 * veri için gereksiz istek demek. Admin bir tür eklediğinde moderasyon ekranı bu anahtarı geçersiz
 * kılar (useCreateGenre); yeni tür, admin'in kendi ekranında beklemeden görünür.
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
export function useBooks(params: BookListParams, enabled = true) {
  return useQuery({
    queryKey: queryKeys.books(params),
    queryFn: () => api.books(params),
    placeholderData: keepPreviousData,
    // enabled false: sorgu hiç çalışmaz (ör. kitap sayfasında tür henüz bilinmiyorken "benzer kitaplar").
    enabled,
  });
}

/** "Okurların en sevdikleri" şeridi. Sunucu bu listeyi zaten 60 sn önbellekte tutuyor. */
export function useTopRated(limit: number) {
  return useQuery({
    queryKey: queryKeys.topRated(limit),
    queryFn: () => api.topRated(limit),
  });
}

/** Kitap sayfasının kendisi (baskılarıyla). */
export function useBook(id: number) {
  return useQuery({ queryKey: queryKeys.book(id), queryFn: () => api.book(id) });
}

/** Puan özeti: ortalama, oy sayısı ve 1–10 dağılımı. */
export function useRatingSummary(bookId: number) {
  return useQuery({ queryKey: queryKeys.ratingSummary(bookId), queryFn: () => api.ratingSummary(bookId) });
}

/**
 * Giriş yapmış okurun bu kitaba verdiği puan; puan yoksa null.
 *
 * Sunucu "puan yok" durumunu 404 `not_rated` ile söyler. Bu bir HATA değil, bir cevap: queryFn onu null'a
 * çevirir, böylece ekranda hata kutusu çıkmaz ve önbellekte "puanı yok" bilgisi de saklanır.
 * Girişsiz kullanıcıda sorgu hiç çalışmaz (enabled).
 */
export function useMyRating(userId: number | undefined, bookId: number) {
  return useQuery({
    queryKey: queryKeys.myRating(userId ?? 0, bookId),
    queryFn: async () => {
      try {
        return (await api.myRating(bookId)).score;
      } catch (e) {
        if (e instanceof ApiError && e.code === "not_rated") return null;
        throw e;
      }
    },
    enabled: userId !== undefined,
  });
}

/**
 * Puan verme ve kaldırma: ilk DEĞİŞİKLİK (mutation). mutate(7) puan verir, mutate(null) kaldırır.
 *
 * Değişiklik başarılı olunca üç şey olur (onSuccess):
 *  1. "Benim puanım" doğrudan yazılır (setQueryData): yeni değeri zaten biliyoruz, sunucuya sormaya gerek yok.
 *  2. Puan özeti GEÇERSİZ kılınır: ekranda olduğu için hemen yeniden çekilir (ortalama ve dağılım değişti).
 *  3. Bütün kitap listeleri ("books" ile başlayan her anahtar) bayat İŞARETLENİR ama şimdi yeniden
 *     çekilmez (refetchType: "none"): kartlardaki ortalama değişti, fakat o listeler şu an ekranda değil
 *     ya da (benzer kitaplar) bir oyla anlamlı değişmez. Kullanıcı kataloğa dönünce tazelenir.
 *  4. Kitaplığım listeleri de aynı şekilde bayat işaretlenir (adım 7): her kartta "Puanım" yazıyor, liste
 *     puana göre süzülüp sıralanabiliyor. Bu satır olmasaydı puan verip 30 sn içinde Kitaplığım'a geçen
 *     okur eski puanını görürdü (eskiden sayfa her açılışta yeniden çektiği için sorun yoktu).
 *
 * NEDEN bu iş kancada, sayfada değil: eskiden sayfa her başarılı puandan sonra `loadBook()` çağırıp kitabı
 * ve özeti elle yeniden yüklüyordu. "Puan değişince neyin tazeleneceği" bilgisi verinin yanında durunca,
 * puanın verildiği her yer (Faz 17'de kitaplık satırı da olacak) aynı kuralı kendiliğinden uygular.
 *
 * onSuccess bir Promise döndürür: özet yeniden gelene kadar değişiklik "sürüyor" (isPending) sayılır;
 * sayfa o arada yıldızları kilitler, ekranda yarım güncellenmiş bir durum görünmez.
 */
export function useRateBook(bookId: number, userId: number | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (score: number | null) => {
      if (score === null) await api.removeRating(bookId);
      else await api.rate(bookId, score);
      return score;
    },
    onSuccess: (score) => {
      if (userId !== undefined) {
        queryClient.setQueryData(queryKeys.myRating(userId, bookId), score);
        void queryClient.invalidateQueries({ queryKey: ["me", userId, "library", "list"], refetchType: "none" });
      }
      void queryClient.invalidateQueries({ queryKey: ["books"], refetchType: "none" });
      return queryClient.invalidateQueries({ queryKey: queryKeys.ratingSummary(bookId) });
    },
  });
}

/**
 * Bir kitabın yorumları, sayfa sayfa (offset). Her sayfa kendi anahtarında: "Eskiler →" sonra "← Yeniler"
 * deyince ilk sayfa önbellekten gelir. keepPreviousData: yeni sayfa gelene kadar eskisi ekranda kalır.
 */
export function useComments(bookId: number, offset: number) {
  return useQuery({
    queryKey: queryKeys.comments(bookId, offset),
    queryFn: () => api.comments(bookId, offset),
    placeholderData: keepPreviousData,
  });
}

/**
 * Yorum yazma. Başarıda bu kitabın BÜTÜN yorum sayfaları geçersiz olur (["comments", kitap] öneki):
 * ekrandaki sayfa hemen yeniden çekilir, ötekiler açılınca. Hata olursa hiçbir şey geçersiz kılınmaz;
 * yorum yazılmadıysa liste değişmemiştir.
 */
export function useAddComment(bookId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (content: string) => api.addComment(bookId, content),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["comments", bookId] }),
  });
}

/**
 * Yorum silme (admin). Geçersiz kılma onSettled'da, yani HATADA DA: silme "bulunamadı" ile dönerse yorum
 * büyük olasılıkla zaten silinmiştir (başka bir admin); liste yine de tazelenir ki ekrandaki hayalet satır
 * gitsin. Eski kod da hatadan sonra listeyi yeniden yüklüyordu.
 */
export function useDeleteComment(bookId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (commentId: number) => api.deleteComment(commentId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["comments", bookId] }),
  });
}
