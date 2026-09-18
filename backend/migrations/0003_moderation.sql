-- 0003_moderation.sql — öneri + moderasyon + denetim kaydı (Faz 4).

-- +goose Up

-- Yeni statü: ARCHIVED (yayından kaldırılmış; silinmemiş). CHECK kısıtını değiştirmek =
-- düşür + yeniden ekle. Native ENUM seçseydik "ALTER TYPE ... ADD VALUE" gerekirdi ve bazı
-- sürümlerde transaction içinde çalışmazdı — TEXT+CHECK kararının (Faz 2) karşılığı bu.
ALTER TABLE books DROP CONSTRAINT books_status_check;
ALTER TABLE books ADD CONSTRAINT books_status_check
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'ARCHIVED'));

ALTER TABLE books
    -- Kim önerdi / kim ekledi. Kullanıcı silinirse kitap kalır, bağ kopar (SET NULL).
    ADD COLUMN submitted_by     bigint REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN rejection_reason text,
    ADD COLUMN moderated_by     bigint REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN moderated_at     timestamptz,
    -- Red gerekçesi yalnızca REJECTED kitapta anlamlı: tutarsız durumu DB reddetsin.
    ADD CONSTRAINT books_rejection_reason_check
        CHECK (rejection_reason IS NULL OR status = 'REJECTED');

-- FK index'leri (Postgres otomatik yapmaz) + admin kuyruğu: "bekleyen öneriler".
CREATE INDEX books_submitted_by_idx ON books (submitted_by);
CREATE INDEX books_moderated_by_idx ON books (moderated_by);
CREATE INDEX books_pending_idx ON books (created_at DESC) WHERE status = 'PENDING';

-- DENETİM KAYDI (audit log): kim, ne zaman, neyi, neden yaptı.
-- Moderasyon kararıyla AYNI transaction'da yazılır: karar var ama kaydı yok (ya da tersi)
-- durumu oluşamaz.
CREATE TABLE audit_logs (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- actor_id'de FK YOK, bilerek: kayıt, aktörün hesabı silindikten sonra da yaşamalı.
    -- FK + ON DELETE SET NULL, aşağıdaki "değiştirilemez" kuralıyla çakışırdı (SET NULL
    -- bir UPDATE'tir) ve kullanıcı silmeyi imkânsız kılardı.
    actor_id    bigint,
    action      text NOT NULL,        -- ör. 'book.approve'
    entity_type text NOT NULL,        -- ör. 'book'
    entity_id   bigint NOT NULL,
    details     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- "Bu kitabın geçmişi" sorgusu (en yeniden eskiye).
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, created_at DESC);

-- APPEND-ONLY: denetim kaydı değiştirilemez ve silinemez. Uygulamadaki bir bug (ya da
-- izlerini silmeye çalışan biri) UPDATE/DELETE denerse DB reddeder.
-- +goose StatementBegin
CREATE FUNCTION audit_logs_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs append-only: % yasak', TG_OP;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_append_only
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

-- Satır (FOR EACH ROW) trigger'ı TRUNCATE'te ÇALIŞMAZ: TRUNCATE satırları tek tek silmez,
-- tabloyu toptan boşaltır. Bu yüzden deyim (FOR EACH STATEMENT) seviyesinde ayrı trigger.
CREATE TRIGGER audit_logs_no_truncate
    BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_append_only();

-- +goose Down

DROP TABLE audit_logs;
DROP FUNCTION audit_logs_append_only();

DROP INDEX books_pending_idx;
ALTER TABLE books
    DROP CONSTRAINT books_rejection_reason_check,
    DROP COLUMN moderated_at,
    DROP COLUMN moderated_by,
    DROP COLUMN rejection_reason,
    DROP COLUMN submitted_by;

-- Kayıplı geri alma (yalnızca dev): ARCHIVED eski şemada yok; o kitaplar REJECTED olur.
UPDATE books SET status = 'REJECTED' WHERE status = 'ARCHIVED';
ALTER TABLE books DROP CONSTRAINT books_status_check;
ALTER TABLE books ADD CONSTRAINT books_status_check
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'));
