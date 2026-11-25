import { keepPreviousData, type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import { queryKeys } from "./queries";
import type { Review, ReviewDraft } from "./types";

/*
 * İncelemelerin kancaları (Faz 17B). Ayrı dosyada: yalnız kitap sayfasının İncelemeler bölümü kullanıyor;
 * bölüm tembel yüklenirse kancalar da ana pakete girmez (libraryQueries.ts ile aynı gerekçe).
 *
 * İki kayıt var:
 *  - herkese açık liste, sayfa sayfa: ["reviews", kitap, offset];
 *  - benim incelemem: ["me", kullanıcı, "review", kitap]; yoksa null.
 * Yazım ikisini de etkiler: benim incelemem doğrudan yazılır (cevap onu taşıyor), liste eskitilir (sırası
 * ve sayısı sunucuda belli olur).
 */

/**
 * Bir kitabın herkese açık incelemeleri. Her sayfa kendi anahtarında; yeni sayfa gelene kadar eskisi
 * ekranda kalır (yorumlardaki gibi).
 */
export function useReviews(bookId: number, offset: number) {
  return useQuery({
    queryKey: queryKeys.reviews(bookId, offset),
    queryFn: () => api.reviews(bookId, offset),
    placeholderData: keepPreviousData,
  });
}

/**
 * Giriş yapmış okurun bu kitaptaki incelemesi; yoksa null. Sunucu "incelemen yok"u 404 `no_review` ile
 * söyler: bir HATA değil, bir cevap (puandaki not_rated gibi). Kitap yoksa 404 `not_found` hata kalır.
 * Girişsiz kullanıcıda sorgu hiç çalışmaz.
 */
export function useMyReview(userId: number | undefined, bookId: number) {
  return useQuery({
    queryKey: queryKeys.myReview(userId ?? 0, bookId),
    queryFn: async (): Promise<Review | null> => {
      try {
        return await api.myReview(bookId);
      } catch (e) {
        if (e instanceof ApiError && e.code === "no_review") return null;
        throw e;
      }
    },
    enabled: userId !== undefined,
  });
}

/**
 * Yaz ya da düzenle (tek uç). `mutate({ body, has_spoiler })` ilk yazım; düzenlemede `version` incelemenin
 * son bilinen sürümü.
 *
 * BAŞARIDA cevap "benim incelemem"e doğrudan yazılır; önce süren bir okuması iptal edilir ki değişiklikten
 * önce başlamış bir GET sonra gelip eski hâli yazmasın (raf kancalarıyla aynı kural). Bu kitabın bütün
 * liste sayfaları eskitilir: yeni inceleme listenin başına girer, düzenlenen "düzenlendi" olur.
 *
 * 409 version_conflict'te benim incelemem bir kez yeniden okunur ve hata ANCAK SONRA yayılır (onError
 * Promise döndürür, TanStack onu bekler): başka bir sekme yazmış ya da silmiş; arayüz mesajını güncel
 * hâlden kurar ("İncelemen başka bir yerde değişmiş"). Sessiz yeniden deneme yok: kullanıcının metni
 * ötekini ezmesin. 400 ve 404 hiçbir şeyi eskitmez.
 */
export function useSaveReview(userId: number | undefined, bookId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (d: ReviewDraft) => api.saveReview(bookId, d),
    onSuccess: async (review) => {
      if (userId !== undefined) await writeMine(queryClient, userId, bookId, review);
      void queryClient.invalidateQueries({ queryKey: ["reviews", bookId] });
    },
    onError: (err) => {
      if (userId === undefined || !(err instanceof ApiError && err.code === "version_conflict")) return;
      return queryClient.invalidateQueries({ queryKey: queryKeys.myReview(userId, bookId), exact: true });
    },
  });
}

/**
 * İncelemeyi sil (puan kalır). Başarıda benim incelemem null olur, liste eskir. 404 no_review: inceleme
 * başka bir sekmede zaten silinmiş; sonuç aynı olduğu için ekran da aynı şekilde güncellenir, hata yine
 * yayılır (arayüz isterse "zaten silinmiş" diyebilir).
 */
export function useDeleteReview(userId: number | undefined, bookId: number) {
  const queryClient = useQueryClient();
  const deleted = async () => {
    if (userId !== undefined) await writeMine(queryClient, userId, bookId, null);
    void queryClient.invalidateQueries({ queryKey: ["reviews", bookId] });
  };
  return useMutation({
    mutationFn: () => api.deleteReview(bookId),
    onSuccess: deleted,
    onError: (err) => (err instanceof ApiError && err.code === "no_review" ? deleted() : undefined),
  });
}

/** Yönetici işlemi: gerekçeyle gizle ya da geri aç. */
export type ModerateReview = { id: number; action: "hide"; reason: string } | { id: number; action: "unhide" };

/**
 * Yöneticinin gizlemesi ve geri açması. Liste HATADA DA eskitilir (onSettled): "bulunamadı" dönerse inceleme
 * büyük olasılıkla silinmiştir, ekrandaki hayalet satır gitsin (yorum silmedeki gibi). Yönetici kendi
 * incelemesini gizlemiş olabilir: onun "benim incelemem"i de eskitilir. Başkalarının önbelleği bu
 * tarayıcıda yok; onlar kendi incelemelerini bir sonraki okumada gizli görür.
 */
export function useModerateReview(adminId: number | undefined, bookId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (m: ModerateReview) => (m.action === "hide" ? api.hideReview(m.id, m.reason) : api.unhideReview(m.id)),
    onSettled: () => {
      if (adminId !== undefined) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.myReview(adminId, bookId), exact: true });
      }
      return queryClient.invalidateQueries({ queryKey: ["reviews", bookId] });
    },
  });
}

/** Benim incelememi önbelleğe yazar; önce süren okumasını iptal eder (eski cevap yenisini ezmesin). */
async function writeMine(queryClient: QueryClient, userId: number, bookId: number, review: Review | null) {
  const key = queryKeys.myReview(userId, bookId);
  await queryClient.cancelQueries({ queryKey: key, exact: true });
  queryClient.setQueryData(key, review);
}
