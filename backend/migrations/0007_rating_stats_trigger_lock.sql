-- 0007_rating_stats_trigger_lock.sql — 0006'daki trigger'a kitap kilidi (review bulgusu R3).
--
-- 0006 uygulandı; uygulanmış bir migration DÜZENLENMEZ (her ortam ayrışır) → düzeltme yeni dosya.
--
-- SORUN: trigger agregatı kitap satırını KİLİTLEMEDEN hesaplıyordu. Aynı anda başka bir
-- kullanıcının Rate'i kitabı FOR UPDATE ile tutarken bir kullanıcı silinirse, trigger'ın
-- UPDATE'i kilidi bekler; Rate commit edince READ COMMITTED, bekleyen UPDATE'i ESKİ agregatla
-- uygular → az önceki oy istatistikten düşer (Faz 5'teki kayıp güncellemenin aynısı).
-- ÇÖZÜM: uygulamanın protokolüyle aynı — ÖNCE kitabı kilitle, SONRA agregatı oku. PL/pgSQL'de
-- her deyim yeni bir anlık görüntü alır; kilit beklendikten sonra hesaplanan agregat taze olur.
--
-- Bilinen kenar durum: bir kullanıcı HESABI SİLİNİRKEN aynı anda kendisi puan veriyorsa kilit
-- sırası ters döner (puan satırı → kitap vs kitap → puan satırı); Postgres bu kilitlenmeyi
-- (deadlock) tespit edip işlemlerden birini hatayla bitirir — veri bozulmaz.

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refresh_book_rating_stats() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1 FROM books WHERE id = OLD.book_id FOR UPDATE;
    UPDATE books b
    SET average_rating = s.avg,
        rating_count   = s.cnt
    FROM (
        SELECT round(avg(score), 2)::numeric(4, 2) AS avg, count(*)::integer AS cnt
        FROM ratings
        WHERE book_id = OLD.book_id
    ) s
    WHERE b.id = OLD.book_id;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refresh_book_rating_stats() RETURNS trigger
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
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
