-- demo_seed.sql — arayüzü dolu görmek için örnek katalog (türler, kitaplar, puanlar, yorumlar).
--
-- Kullanım (repo kökünden, migration'lar uygulanmışken):  make demo-seed
-- Tekrar çalıştırılabilir: slug/e-posta çakışan satırlar atlanır (ON CONFLICT DO NOTHING).
--
-- "Okur" hesaplarının password_hash'i '!' — bcrypt biçiminde olmadığı için bu hesaplarla
-- GİRİŞ YAPILAMAZ (karşılaştırma her zaman başarısız). Yalnızca puan/yorum sahibi olarak var.
-- Kendi admin'ini ayrıca oluştur: make admin EMAIL=sen@ornek.com

BEGIN;

INSERT INTO genres (name, slug) VALUES
    ('Roman', 'roman'), ('Bilim Kurgu', 'bilim-kurgu'), ('Felsefe', 'felsefe'),
    ('Tarih', 'tarih'), ('Yazılım', 'yazilim'), ('Distopya', 'distopya'), ('Klasik', 'klasik')
ON CONFLICT DO NOTHING;

INSERT INTO users (email, display_name, password_hash)
SELECT 'okur' || g || '@demo.bookhub', d.name, '!'
FROM generate_series(1, 12) g
JOIN (VALUES (1,'Ayşe'),(2,'Mehmet'),(3,'Zeynep'),(4,'Can'),(5,'Elif'),(6,'Burak'),
             (7,'Deniz'),(8,'Selin'),(9,'Emre'),(10,'Ece'),(11,'Kaan'),(12,'Naz')) AS d(i, name) ON d.i = g
ON CONFLICT DO NOTHING;

INSERT INTO books (title, slug, author, description, publication_year, page_count, language, status) VALUES
    ('Tutunamayanlar', 'tutunamayanlar', 'Oğuz Atay', 'Türk edebiyatının modernist başyapıtı: Selim Işık''ın ardından Turgut Özben''in arayışı.', 1972, 724, 'tr', 'APPROVED'),
    ('İnce Memed', 'ince-memed', 'Yaşar Kemal', 'Çukurova''da ağalara karşı dağa çıkan bir gencin destanı.', 1955, 436, 'tr', 'APPROVED'),
    ('Kürk Mantolu Madonna', 'kurk-mantolu-madonna', 'Sabahattin Ali', 'Raif Efendi''nin Berlin''de başlayan sessiz aşk hikâyesi.', 1943, 160, 'tr', 'APPROVED'),
    ('Saatleri Ayarlama Enstitüsü', 'saatleri-ayarlama-enstitusu', 'Ahmet Hamdi Tanpınar', 'Bürokrasi ve modernleşme üzerine ironik bir hiciv.', 1961, 382, 'tr', 'APPROVED'),
    ('1984', '1984', 'George Orwell', 'Büyük Birader''in gözetimindeki bir dünyada bireyin sonu.', 1949, 352, 'en', 'APPROVED'),
    ('Cesur Yeni Dünya', 'cesur-yeni-dunya', 'Aldous Huxley', 'Mutluluğun zorunlu olduğu steril bir gelecek.', 1932, 288, 'en', 'APPROVED'),
    ('Dune', 'dune', 'Frank Herbert', 'Arrakis çölünde imparatorluk, din ve ekoloji.', 1965, 704, 'en', 'APPROVED'),
    ('Vakıf', 'vakif', 'Isaac Asimov', 'Psikotarih ile bir galaktik imparatorluğun çöküşünü kısaltma planı.', 1951, 255, 'en', 'APPROVED'),
    ('Suç ve Ceza', 'suc-ve-ceza', 'Fyodor Dostoyevski', 'Raskolnikov''un vicdanla hesaplaşması.', 1866, 687, 'ru', 'APPROVED'),
    ('Sapiens', 'sapiens', 'Yuval Noah Harari', 'Hayvanlardan tanrılara: insan türünün kısa tarihi.', 2011, 443, 'en', 'APPROVED'),
    ('Clean Code', 'clean-code-demo', 'Robert C. Martin', 'Okunabilir, bakımı kolay kod yazmanın ilkeleri.', 2008, 464, 'en', 'APPROVED'),
    ('The Go Programming Language', 'gopl-demo', 'Alan Donovan, Brian Kernighan', 'Go''nun kutsal kitabı.', 2015, 380, 'en', 'APPROVED'),
    ('Devlet', 'devlet', 'Platon', 'Adalet nedir? İdeal devlet üzerine diyaloglar.', -375, 416, 'tr', 'APPROVED'),
    ('Tek Oyla Zirve', 'tek-oyla-zirve', 'Deney Yazarı', 'Tek bir 10 puanlık oyla basit ortalamada zirveye çıkan kitap — ağırlıklı skorda nereye düşer?', 2024, 100, 'tr', 'APPROVED'),
    ('Onay Bekleyen Öneri', 'onay-bekleyen-oneri', 'Bir Okur', 'Admin kuyruğunda görünmesi için PENDING bir öneri.', 2023, 200, 'tr', 'PENDING')
ON CONFLICT DO NOTHING;

-- Platon'un tarihi (MÖ) CHECK'e takılmaz ama uygulama doğrulaması 1450'den öncesini kabul
-- etmez; demo veride "bilinmiyor" (NULL) yapıyoruz.
UPDATE books SET publication_year = NULL WHERE slug = 'devlet';

INSERT INTO book_genres (book_id, genre_id)
SELECT b.id, g.id FROM (VALUES
    ('tutunamayanlar','roman'), ('tutunamayanlar','klasik'), ('ince-memed','roman'), ('ince-memed','klasik'),
    ('kurk-mantolu-madonna','roman'), ('saatleri-ayarlama-enstitusu','roman'), ('1984','distopya'),
    ('1984','roman'), ('cesur-yeni-dunya','distopya'), ('dune','bilim-kurgu'), ('vakif','bilim-kurgu'),
    ('suc-ve-ceza','roman'), ('suc-ve-ceza','klasik'), ('sapiens','tarih'), ('clean-code-demo','yazilim'),
    ('gopl-demo','yazilim'), ('devlet','felsefe'), ('devlet','klasik'), ('tek-oyla-zirve','roman')
) AS x(book, genre)
JOIN books b ON b.slug = x.book
JOIN genres g ON g.slug = x.genre
ON CONFLICT DO NOTHING;

-- Puanlar: her kitabın kendi "kalitesi" + okura göre küçük sapma (deterministik).
INSERT INTO ratings (user_id, book_id, score)
SELECT u.id, b.id, greatest(1, least(10, q.base + ((u.id + b.id) % 3) - 1))::smallint
FROM (VALUES ('tutunamayanlar', 9, 11), ('ince-memed', 8, 9), ('kurk-mantolu-madonna', 9, 12),
             ('saatleri-ayarlama-enstitusu', 8, 6), ('1984', 9, 12), ('cesur-yeni-dunya', 7, 8),
             ('dune', 9, 10), ('vakif', 8, 7), ('suc-ve-ceza', 9, 9), ('sapiens', 7, 11),
             ('clean-code-demo', 7, 5), ('gopl-demo', 8, 4), ('devlet', 7, 3)) AS q(slug, base, voters)
JOIN books b ON b.slug = q.slug
JOIN LATERAL (SELECT id FROM users WHERE email LIKE 'okur%@demo.bookhub' ORDER BY id LIMIT q.voters) u ON true
ON CONFLICT DO NOTHING;

INSERT INTO ratings (user_id, book_id, score)
SELECT (SELECT id FROM users WHERE email = 'okur1@demo.bookhub'), id, 10 FROM books WHERE slug = 'tek-oyla-zirve'
ON CONFLICT DO NOTHING;

-- Türetilmiş istatistikler: uygulamanın her yazımda yaptığı yeniden hesabın toptan hâli.
UPDATE books b
SET average_rating = s.avg, rating_count = s.cnt
FROM (SELECT book_id, round(avg(score), 2) AS avg, count(*)::int AS cnt FROM ratings GROUP BY book_id) s
WHERE b.id = s.book_id;

INSERT INTO comments (book_id, user_id, content)
SELECT b.id, u.id, c.content
FROM (VALUES ('tutunamayanlar', 'okur2@demo.bookhub', 'İlk yüz sayfayı geçince bırakamadım. Selim''in mektupları hâlâ aklımda.'),
             ('tutunamayanlar', 'okur3@demo.bookhub', 'Zor ama emeğe değer bir kitap.'),
             ('1984', 'okur4@demo.bookhub', 'Her okuyuşta daha güncel geliyor.'),
             ('dune', 'okur5@demo.bookhub', 'Dünya kurgusu inanılmaz; ilk bölüm yavaş başlıyor, sabır.'),
             ('tek-oyla-zirve', 'okur1@demo.bookhub', 'Tek oyla 10 aldım; ağırlıklı sıralama beni aşağı çekti :)')) AS c(slug, email, content)
JOIN books b ON b.slug = c.slug
JOIN users u ON u.email = c.email
WHERE NOT EXISTS (SELECT 1 FROM comments x WHERE x.book_id = b.id AND x.content = c.content);

COMMIT;

SELECT (SELECT count(*) FROM books WHERE status = 'APPROVED') AS yayinda,
       (SELECT count(*) FROM books WHERE status = 'PENDING') AS bekleyen,
       (SELECT count(*) FROM ratings) AS oy,
       (SELECT count(*) FROM comments) AS yorum;
