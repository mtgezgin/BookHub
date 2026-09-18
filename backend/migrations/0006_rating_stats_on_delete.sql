-- 0006_rating_stats_on_delete.sql — CASCADE ile silinen puanlarda da istatistikleri yenile.
--
-- SORUN (review bulgusu F4): average_rating/rating_count türetilmiş veri ve uygulama onları
-- her Rate/Remove'da yeniden hesaplıyor. Ama bir KULLANICI silinince puanları
-- ON DELETE CASCADE ile uygulamanın haberi olmadan gider → kitaplar "hayalet oy" taşır
-- (sayım 2, satır 0). Smoke betiğinin temizliği tam olarak bunu yapıyordu.
--
-- ÇÖZÜM: puan satırı HANGİ yoldan silinirse silinsin (API, CASCADE, elle DELETE) kitabın
-- istatistiklerini kaynaktan yeniden hesaplayan bir trigger. Kural "türetilmiş veri her
-- değişimde kaynaktan hesaplanır"; artık bu kuralı atlayabilecek bir silme yolu yok.
-- (INSERT/UPDATE yolu uygulamada kalıyor: orada kitap satırı FOR UPDATE ile kilitleniyor.)

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION refresh_book_rating_stats() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE books b
    SET average_rating = s.avg,
        rating_count   = s.cnt
    FROM (
        SELECT round(avg(score), 2)::numeric(4, 2) AS avg, count(*)::integer AS cnt
        FROM ratings
        WHERE book_id = OLD.book_id
    ) s
    WHERE b.id = OLD.book_id;
    RETURN NULL; -- AFTER trigger: dönüş değeri kullanılmaz
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ratings_refresh_stats_on_delete
    AFTER DELETE ON ratings
    FOR EACH ROW EXECUTE FUNCTION refresh_book_rating_stats();

-- Mevcut bayat istatistikleri bir kez düzelt (bu trigger'dan önce oluşmuş hayalet oylar).
UPDATE books b
SET average_rating = s.avg,
    rating_count   = coalesce(s.cnt, 0)
FROM books b2
LEFT JOIN (
    SELECT book_id, round(avg(score), 2)::numeric(4, 2) AS avg, count(*)::integer AS cnt
    FROM ratings GROUP BY book_id
) s ON s.book_id = b2.id
WHERE b.id = b2.id
  AND (b.rating_count IS DISTINCT FROM coalesce(s.cnt, 0) OR b.average_rating IS DISTINCT FROM s.avg);

-- +goose Down
DROP TRIGGER ratings_refresh_stats_on_delete ON ratings;
DROP FUNCTION refresh_book_rating_stats();
