-- 0008_refresh_token_families.sql — refresh token AİLELERİ + yeniden kullanım (reuse) tespiti (Faz 13).
--
-- SORUN: rotasyon (her /refresh'te yeni token, eskisi ölür) çalınan bir token'ın ömrünü kısaltır
-- ama HIRSIZLIĞI FARK ETMEZ. Saldırgan çaldığı token'ı gerçek sahibinden ÖNCE kullanırsa, yeni
-- zinciri o taşır; sahibinin elindeki eski token "bilinmiyor" der, kullanıcı yeniden girer ve
-- saldırganın zinciri yaşamaya devam eder.
-- ÇÖZÜM (OAuth 2.0 Security BCP'deki "refresh token rotation + reuse detection"): bir girişten
-- doğan tüm token'lar tek bir AİLE'dir. Kullanılmış token SİLİNMEZ, used_at ile işaretlenir.
-- Kullanılmış bir token İKİNCİ kez gelirse iki kişiden biri (sahip ya da hırsız) eski bir kopyayı
-- kullanıyor demektir — hangisi olduğunu bilemeyiz, bu yüzden AİLENİN TAMAMI ölür.
--
-- Tüm dosya goose'un açtığı TEK transaction'da koşar (NO TRANSACTION işareti yok): yarım kalmış
-- bir şema (tablo var, kolon yok) olamaz.

-- +goose Up

-- (1) Var olan token'ları sil: aileleri olmadığı için NOT NULL family_id'ye taşınamazlar.
-- Bedeli: herkes bir kez yeniden giriş yapar (taşıma zaten değişiyor — Faz 13'te refresh token
-- localStorage'dan httpOnly çereze geçiyor ve eski token'lar o yüzden de geçersiz kalacaktı).
-- TRUNCATE ACCESS EXCLUSIVE kilit alır: bu transaction bitene dek tabloya kimse yazamaz, yani
-- eski sürümdeki bir pod TRUNCATE ile ALTER arasına aile'siz bir satır sıkıştıramaz.
-- BİLİNEN KENAR DURUM (demo için kabul): çok kopyalı bir rollout'ta migration'dan sonra hâlâ
-- ESKİ kodla çalışan pod'lar family_id'yi bilmediği için INSERT'te NOT NULL hatası alır → yeni
-- sürüm devralana kadar kısa süre giriş hatası. Sıfır kesinti isteseydik iki aşamalı (önce
-- nullable kolon + yeni kod, sonra NOT NULL) bir geçiş gerekirdi.
TRUNCATE refresh_tokens;

-- (2) Aile = bir giriş. absolute_expires_at MUTLAK tavandır: rotasyon token'ı her seferinde
-- yeniden JWT_REFRESH_TTL kadar uzatır (kayan pencere); tavan olmasa, yarışı önce kazanan
-- saldırgan aileyi sonsuza dek taşıyabilirdi. revoked_at: aile öldürüldüyse (reuse ya da çıkış).
CREATE TABLE refresh_token_families (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id             bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    absolute_expires_at timestamptz NOT NULL,
    revoked_at          timestamptz
);

-- FK'ye index (Postgres otomatik yapmaz): girişteki temizlik kullanıcının ailelerini bununla
-- arar; kullanıcı silinince CASCADE de bu kolonla arar.
CREATE INDEX refresh_token_families_user_id_idx ON refresh_token_families (user_id);

-- (3) Her token bir aileye bağlı. Aile silinince token'ları da gider (CASCADE) — temizlik tek
-- DELETE ile olur. used_at: NULL = henüz kullanılmadı; dolu = rotasyonda harcandı (silinmez,
-- çünkü İKİNCİ kullanımını tanıyabilmemiz için satırın durması gerek).
ALTER TABLE refresh_tokens
    ADD COLUMN family_id bigint NOT NULL REFERENCES refresh_token_families (id) ON DELETE CASCADE,
    ADD COLUMN used_at   timestamptz;

-- Aileyi öldürürken "bu ailenin tüm token'larını sil" bu kolonla arar.
CREATE INDEX refresh_tokens_family_id_idx ON refresh_tokens (family_id);

-- +goose Down

-- Önce token'ları sil: kolonlar düşünce used_at bilgisi kaybolur ve HARCANMIŞ token'lar eski
-- şemada yeniden geçerli (tek kullanımlık, henüz kullanılmamış) görünürdü — dirilmesinler.
-- DELETE değil TRUNCATE: ACCESS EXCLUSIVE kilidi, geri alma sürerken hâlâ yeni kodla çalışan
-- bir pod'un araya harcanmış bir satır yazıp eski şemaya "canlı" olarak taşımasını engeller.
TRUNCATE refresh_tokens;
ALTER TABLE refresh_tokens
    DROP COLUMN used_at,
    DROP COLUMN family_id;
DROP TABLE refresh_token_families;
