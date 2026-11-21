import { useEffect } from "react";
import type { Page } from "../api/types";
import { MAX_OFFSET } from "./filters";
import { Button } from "./ui/button";

/**
 * usePageView: sayfalı bir SORGUNUN sonucunu ekrana hazırlar (Faz 16d adım 6). Veriyi kendisi çekmez;
 * offset sayfanın durumu, veri sorgunun (useComments, useAdminBooks). Eski usePaged'in üç kuralı:
 *  1. Boş sayfa + offset > 0 → son GERÇEK sayfaya dön (son sayfadaki son öğe silinince/onaylanınca
 *     kullanıcı "liste boş" ekranında mahsur kalmasın). HÂLÂ gerekli: sunucu boş sayfa döndürür, sorgu bunu
 *     bilemez. Hedef toplamdan hesaplanır, "bir sayfa geri" değil (adım 7, Kitaplığım'ın eski kuralı):
 *     adresteki offset elle büyütülmüşse (?offset=600, 20 kitap) tek adımda dönülür, sayfa sayfa değil.
 *     Geri adım atılırken boş sayfa (ve onun yerine geçici olarak gösterilen boş sayfa) ÇİZİLMEZ: eski
 *     satırlar işlem yapılabilir görünmesin, "liste boş" ekranı yanıp sönmesin.
 *  2. Eski cevap koruması: artık SORGUDAN geliyor. Her offset kendi anahtarına yazılır, ekran yalnız
 *     güncel anahtarı gösterir; eskiden bunun için bir sıra numarası (ref) tutuluyordu.
 *  3. showPager: offset > 0 iken de pager görünür (geri dönüş yolu hep açık). Aynı hesap.
 * Eskiden bir de onError ref'i ve resetKey etkisi vardı: hata artık sorgunun `error`'ında duruyor, sekme
 * değişince offset'i sıfırlamak da sekmeyi değiştiren tıklamanın işi.
 */
export function usePageView<T>(
  query: { data?: Page<T>; isPlaceholderData: boolean },
  offset: number,
  setOffset: (offset: number) => void,
) {
  const data = query.data;
  const empty = data !== undefined && data.items.length === 0;
  // Yer tutucu (önceki anahtarın verisi) boşsa geri adım atılmaz: gerçek cevabı beklemek gerek.
  const stepBackTo = empty && offset > 0 && !query.isPlaceholderData ? lastPageOffset(data.total, data.limit) : null;
  useEffect(() => {
    if (stepBackTo !== null) setOffset(stepBackTo);
  }, [stepBackTo, setOffset]);
  const page = empty && (offset > 0 || query.isPlaceholderData) ? undefined : data;
  const showPager = page !== undefined && (offset > 0 || page.total > page.limit);
  return { page, showPager };
}

/** Son dolu sayfanın offset'i; liste tamamen boşsa 0. Boş bir sayfanın offset'i hep bundan büyüktür. */
function lastPageOffset(total: number, limit: number): number {
  return total > 0 ? Math.floor((total - 1) / limit) * limit : 0;
}

/** Pager: önceki/sonraki düğmeleri ve "a–b / toplam" etiketi. */
export function Pager<T>({ page, offset, onChange, prev = "← Önceki", next = "Sonraki →" }: {
  page: Page<T>;
  offset: number;
  onChange: (offset: number) => void;
  prev?: string;
  next?: string;
}) {
  const last = Math.min(offset + page.limit, page.total);
  const nextOffset = offset + page.limit;
  // Sunucu en çok MAX_OFFSET'e kadar sayfa verir (derin sayfa pahalıdır; Faz 16c). Toplam daha büyükse
  // "Sonraki" sınırda kapanır ve kullanıcıya nedeni söylenir. Eskiden düğme açık kalıyor, tıklanınca
  // geçersiz offset 0 sayılıp ilk sayfaya dönülüyordu (REVIEW_FEEDBACK F25).
  const atCap = nextOffset < page.total && nextOffset > MAX_OFFSET;
  return (
    <div className="mt-8 space-y-2">
      <nav aria-label="Sayfalar" className="flex items-center justify-between gap-4">
        <Button variant="outline" disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - page.limit))}>{prev}</Button>
        <span className="text-sm text-muted-foreground tabular-nums">{page.total ? `${offset + 1}–${last} / ${page.total}` : ""}</span>
        <Button variant="outline" disabled={nextOffset >= page.total || atCap} onClick={() => onChange(nextOffset)}>{next}</Button>
      </nav>
      {atCap && (
        <p className="text-center text-sm text-muted-foreground">
          En fazla ilk {MAX_OFFSET.toLocaleString("tr-TR")} sonuç sayfalanabiliyor; daha fazlası için aramanı ya da türü daralt.
        </p>
      )}
    </div>
  );
}
