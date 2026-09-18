import { useCallback, useEffect, useRef, useState } from "react";
import type { Page } from "../api/types";

/**
 * usePaged: offset'li bir listeyi yükler. İki sayfada (admin kuyruğu, yorumlar) AYNI hatayı
 * iki kez yaptıktan sonra (review bulguları) mantık tek yerde:
 *  - Boş sayfa + offset > 0 → bir sayfa geri (son sayfadaki son öğe silinince/onaylanınca
 *    kullanıcı "liste boş" ekranında mahsur kalmasın).
 *  - Eski cevap koruması: hızlı sekme/sayfa değişiminde geç gelen ESKİ cevap yenisinin
 *    üzerine yazmasın (her istek bir sıra numarası alır; yalnız sonuncusu uygulanır).
 *  - showPager: offset > 0 iken de pager görünür (geri dönüş yolu hep açık).
 * `fetch` useCallback ile sabitlenmiş olmalı; `resetKey` değişince (sekme, kitap) ilk sayfaya dönülür.
 */
export function usePaged<T>(
  fetch: (offset: number) => Promise<Page<T>>,
  resetKey: string,
  onError: (e: unknown) => void,
) {
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<T> | null>(null);
  const seq = useRef(0);
  // onError REF'te: çağıran her render'da yeni bir fonksiyon verirse (ör. satır içi ok
  // fonksiyonu) reload yeniden oluşur, effect yeniden çalışır → SONSUZ istek döngüsü.
  // Testte tam olarak bu oldu; en güncel fonksiyonu ref'ten okuyup bağımlılıktan çıkarıyoruz.
  const onErrorRef = useRef(onError);
  useEffect(() => {
    onErrorRef.current = onError;
  });

  useEffect(() => setOffset(0), [resetKey]);

  const reload = useCallback(() => {
    const mine = ++seq.current;
    fetch(offset)
      .then((p) => {
        if (mine !== seq.current) return; // daha yeni bir istek var: bu cevap bayat
        if (p.items.length === 0 && offset > 0) {
          setOffset(Math.max(0, offset - p.limit));
          return;
        }
        setPage(p);
      })
      .catch((e) => {
        if (mine === seq.current) onErrorRef.current(e);
      });
  }, [fetch, offset]);

  useEffect(reload, [reload]);

  const showPager = page !== null && (offset > 0 || page.total > page.limit);
  return { page, offset, setOffset, reload, showPager };
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
  return (
    <div className="pager">
      <button disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - page.limit))}>{prev}</button>
      <span className="muted small">{page.total ? `${offset + 1}–${last} / ${page.total}` : ""}</span>
      <button disabled={offset + page.limit >= page.total} onClick={() => onChange(offset + page.limit)}>{next}</button>
    </div>
  );
}
