-- demo_seed.sql — arayüzü dolu görmek için örnek katalog (türler, kitaplar, puanlar, yorumlar).
--
-- Kullanım (repo kökünden, migration'lar uygulanmışken):  make demo-seed
-- Tekrar çalıştırılabilir: slug/e-posta çakışan satırlar atlanır (ON CONFLICT DO NOTHING).
--
-- "Okur" hesaplarının password_hash'i '!' — bcrypt biçiminde olmadığı için bu hesaplarla
-- GİRİŞ YAPILAMAZ (karşılaştırma her zaman başarısız). Yalnızca puan/yorum sahibi olarak var.
-- Kendi admin'ini ayrıca oluştur: make admin EMAIL=sen@ornek.com
--
-- Kitaplar gerçek; yıl = ilk yayın, dil = özgün dil. Sayfa sayıları baskıya göre değişir
-- (yaklaşık).

BEGIN;

INSERT INTO genres (name, slug) VALUES
    ('Roman', 'roman'), ('Bilim Kurgu', 'bilim-kurgu'), ('Felsefe', 'felsefe'),
    ('Tarih', 'tarih'), ('Yazılım', 'yazilim'), ('Distopya', 'distopya'), ('Klasik', 'klasik'),
    ('Fantastik', 'fantastik'), ('Polisiye', 'polisiye'), ('Bilim', 'bilim')
ON CONFLICT DO NOTHING;

INSERT INTO users (email, display_name, password_hash)
SELECT 'okur' || g || '@demo.bookhub', d.name, '!'
FROM generate_series(1, 12) g
JOIN (VALUES (1,'Ayşe'),(2,'Mehmet'),(3,'Zeynep'),(4,'Can'),(5,'Elif'),(6,'Burak'),
             (7,'Deniz'),(8,'Selin'),(9,'Emre'),(10,'Ece'),(11,'Kaan'),(12,'Naz')) AS d(i, name) ON d.i = g
ON CONFLICT DO NOTHING;

-- SIRA ÖNEMLİ: katalog `created_at DESC, id DESC` sıralı ve bu INSERT'in satırları aynı
-- created_at'i paylaşır (tek transaction) → ilk sayfada (12 kitap) EN SONDAKİ satırlar,
-- sondan başa doğru görünür. Son satır (Aylak Adam) frontend/e2e/catalog.spec.ts'in
-- "olmaması gereken" kitabıdır: türü yalnız roman kalmalı ve hep en sonda durmalı.
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
    ('Çalıkuşu', 'calikusu', 'Reşat Nuri Güntekin', 'Feride''nin kırgın bir aşkın ardından Anadolu''da öğretmenlik yaparken tuttuğu günlükler.', 1922, 544, 'tr', 'APPROVED'),
    ('Kuyucaklı Yusuf', 'kuyucakli-yusuf', 'Sabahattin Ali', 'Kimsesiz kalan Yusuf''un Edremit''te Muazzez''e duyduğu aşk ve taşra eşrafının çürümüşlüğü.', 1937, 224, 'tr', 'APPROVED'),
    ('Yaban', 'yaban', 'Yakup Kadri Karaosmanoğlu', 'Kurtuluş Savaşı yıllarında bir Anadolu köyüne sığınan yedek subay Ahmet Celal''in yabancılığı.', 1932, 216, 'tr', 'APPROVED'),
    ('Bereketli Topraklar Üzerinde', 'bereketli-topraklar-uzerinde', 'Orhan Kemal', 'İş bulmak için Çukurova''ya inen üç köylü arkadaşın ırgatlık ve fabrika hayatı.', 1954, 336, 'tr', 'APPROVED'),
    ('Tehlikeli Oyunlar', 'tehlikeli-oyunlar', 'Oğuz Atay', 'Gecekondusunda hayatını oyunlarla kuran Hikmet Benol''un kendisiyle ve toplumla hesaplaşması.', 1973, 488, 'tr', 'APPROVED'),
    ('Anna Karenina', 'anna-karenina', 'Lev Tolstoy', 'Rus sosyetesinde yasak bir aşkın trajedisi; yanında Levin''in anlam arayışı.', 1878, 864, 'ru', 'APPROVED'),
    ('Dönüşüm', 'donusum', 'Franz Kafka', 'Bir sabah dev bir böceğe dönüşmüş olarak uyanan Gregor Samsa ve ailesi.', 1915, 96, 'de', 'APPROVED'),
    ('Hayvan Çiftliği', 'hayvan-ciftligi', 'George Orwell', 'Çiftlik sahibini kovan hayvanların devriminin yozlaşmasını anlatan siyasi alegori.', 1945, 152, 'en', 'APPROVED'),
    ('Gülün Adı', 'gulun-adi', 'Umberto Eco', '1327''de bir manastırda art arda gelen esrarengiz ölümler ve Baskervilleli William''ın soruşturması.', 1980, 536, 'it', 'APPROVED'),
    ('Otomatik Portakal', 'otomatik-portakal', 'Anthony Burgess', 'Şiddet düşkünü Alex''in devletin "tedavi" deneyine maruz kalması: özgür irade mi, zorunlu iyilik mi?', 1962, 192, 'en', 'APPROVED'),
    ('Damızlık Kızın Öyküsü', 'damizlik-kizin-oykusu', 'Margaret Atwood', 'Teokratik Gilead''da doğurganlıkları yüzünden köleleştirilen kadınların hikâyesi.', 1985, 312, 'en', 'APPROVED'),
    ('Ben, Robot', 'ben-robot', 'Isaac Asimov', 'Robotiğin Üç Yasası etrafında kurulan, birbirine bağlı öyküler.', 1950, 256, 'en', 'APPROVED'),
    ('Neuromancer', 'neuromancer', 'William Gibson', 'Siberpunk türünü başlatan roman: siber uzayda bir hacker''ın son büyük işi.', 1984, 272, 'en', 'APPROVED'),
    ('Solaris', 'solaris', 'Stanisław Lem', 'Düşünen bir okyanusla kaplı gezegende insanın "öteki"ni anlama çabası.', 1961, 208, 'pl', 'APPROVED'),
    ('Karanlığın Sol Eli', 'karanligin-sol-eli', 'Ursula K. Le Guin', 'Cinsiyetin sabit olmadığı buzul gezegeni Gethen''e gönderilen bir elçinin hikâyesi.', 1969, 304, 'en', 'APPROVED'),
    ('Yüzüklerin Efendisi: Yüzük Kardeşliği', 'yuzuk-kardesligi', 'J. R. R. Tolkien', 'Frodo''nun Tek Yüzük''ü yok etmek için çıktığı yolculuğun başlangıcı.', 1954, 528, 'en', 'APPROVED'),
    ('Böyle Buyurdu Zerdüşt', 'boyle-buyurdu-zerdust', 'Friedrich Nietzsche', 'Dağdan inen Zerdüşt''ün ağzından üstinsan, bengi dönüş ve değerlerin yeniden değerlendirilmesi.', 1883, 352, 'de', 'APPROVED'),
    ('Sofie''nin Dünyası', 'sofienin-dunyasi', 'Jostein Gaarder', 'On dört yaşındaki Sofie''ye gizemli mektuplarla anlatılan felsefe tarihi.', 1991, 520, 'no', 'APPROVED'),
    ('Sisifos Söyleni', 'sisifos-soyleni', 'Albert Camus', 'Absürt karşısında intihar mı, başkaldırı mı? Camus, Sisifos''u mutlu düşünmemizi önerir.', 1942, 160, 'fr', 'APPROVED'),
    ('Tüfek, Mikrop ve Çelik', 'tufek-mikrop-ve-celik', 'Jared Diamond', 'Toplumların kaderini coğrafyanın, tarımın ve salgınların nasıl belirlediğine dair geniş bir tarih.', 1997, 480, 'en', 'APPROVED'),
    ('Zamanın Kısa Tarihi', 'zamanin-kisa-tarihi', 'Stephen Hawking', 'Büyük Patlama''dan kara deliklere, evrenin yapısına herkes için bir giriş.', 1988, 212, 'en', 'APPROVED'),
    ('Düşünme, Hızlı ve Yavaş', 'dusunme-hizli-ve-yavas', 'Daniel Kahneman', 'Sezgisel "Sistem 1" ile yavaş ve hesaplı "Sistem 2": kararlarımızdaki yanılgılar.', 2011, 512, 'en', 'APPROVED'),
    ('Doğu Ekspresinde Cinayet', 'dogu-ekspresinde-cinayet', 'Agatha Christie', 'Karda mahsur kalan bir trende işlenen cinayet ve Hercule Poirot''nun en ünlü çözümlerinden biri.', 1934, 256, 'en', 'APPROVED'),
    ('The Pragmatic Programmer', 'the-pragmatic-programmer', 'Andrew Hunt, David Thomas', 'Kariyer boyu işe yarayan alışkanlıklar: DRY, ortogonallik, kırık pencereler.', 1999, 352, 'en', 'APPROVED'),
    ('Design Patterns', 'design-patterns', 'Erich Gamma, Richard Helm, Ralph Johnson, John Vlissides', '"Dörtlü Çete"nin 23 tasarım kalıbı: nesne yönelimli tasarımın ortak sözlüğü.', 1994, 395, 'en', 'APPROVED'),
    ('Designing Data-Intensive Applications', 'designing-data-intensive-applications', 'Martin Kleppmann', 'Replikasyon, bölümlendirme, tutarlılık: veri sistemlerinin arkasındaki ilkeler.', 2017, 616, 'en', 'APPROVED'),
    ('The Mythical Man-Month', 'the-mythical-man-month', 'Frederick P. Brooks Jr.', '"Geciken bir projeye adam eklemek onu daha da geciktirir": yazılım yönetiminin klasiği.', 1975, 336, 'en', 'APPROVED'),
    ('Küçük Prens', 'kucuk-prens', 'Antoine de Saint-Exupéry', 'Çölde mahsur kalan bir pilotla başka bir gezegenden gelen küçük prensin dostluk ve sorumluluk masalı.', 1943, 112, 'fr', 'APPROVED'),
    ('Yüzyıllık Yalnızlık', 'yuzyillik-yalnizlik', 'Gabriel García Márquez', 'Macondo kasabasında Buendía ailesinin yedi kuşaklık hikâyesi.', 1967, 464, 'es', 'APPROVED'),
    ('Hobbit', 'hobbit', 'J. R. R. Tolkien', 'Bilbo Baggins''in cüceler ve Gandalf''la birlikte ejderha Smaug''un hazinesine yolculuğu.', 1937, 320, 'en', 'APPROVED'),
    ('Huzur', 'huzur', 'Ahmet Hamdi Tanpınar', 'Mümtaz ile Nuran''ın aşkı üzerinden İstanbul, musiki ve Doğu ile Batı arasında kalmışlık.', 1949, 392, 'tr', 'APPROVED'),
    ('Concurrency in Go', 'concurrency-in-go', 'Katherine Cox-Buday', 'Goroutine, channel ve select ile eşzamanlılık kalıpları ve tuzakları.', 2017, 238, 'en', 'APPROVED'),
    ('Kozmos', 'kozmos', 'Carl Sagan', 'Evrenin ve bilimin tarihine merak uyandıran bir yolculuk.', 1980, 384, 'en', 'APPROVED'),
    ('Karamazov Kardeşler', 'karamazov-kardesler', 'Fyodor Dostoyevski', 'Bir baba cinayeti etrafında inanç, özgürlük ve vicdan üzerine büyük bir sorgulama.', 1880, 840, 'ru', 'APPROVED'),
    ('Otostopçunun Galaksi Rehberi', 'otostopcunun-galaksi-rehberi', 'Douglas Adams', 'Dünya yıkılmadan son anda kurtarılan Arthur Dent''in galaksiler arası absürt yolculuğu.', 1979, 216, 'en', 'APPROVED'),
    ('Benim Adım Kırmızı', 'benim-adim-kirmizi', 'Orhan Pamuk', '16. yüzyıl İstanbul''unda nakkaşlar arasında işlenen bir cinayet; Doğu ile Batı resim sanatının çatışması.', 1998, 472, 'tr', 'APPROVED'),
    ('Yabancı', 'yabanci', 'Albert Camus', 'Annesinin ölümüne bile kayıtsız kalan Meursault''nun absürt dünyası.', 1942, 112, 'fr', 'APPROVED'),
    ('Fahrenheit 451', 'fahrenheit-451', 'Ray Bradbury', 'Kitapların yasak olduğu ve itfaiyecilerin onları yaktığı bir gelecek.', 1953, 208, 'en', 'APPROVED'),
    ('Aylak Adam', 'aylak-adam', 'Yusuf Atılgan', 'Ailesinden kalan mirasla aylak bir hayat süren C.''nin, düşlediği kadını İstanbul sokaklarında arayışı.', 1959, 152, 'tr', 'APPROVED')
ON CONFLICT DO NOTHING;

-- Admin kuyruğu boş görünmesin: okurların önerdiği, onay bekleyen iki kitap. PENDING
-- kitaplar katalogda görünmez, yani yukarıdaki sıra kuralını etkilemezler.
INSERT INTO books (title, slug, author, description, publication_year, page_count, language, status, submitted_by)
SELECT x.title, x.slug, x.author, x.description, x.year, x.pages, x.lang, 'PENDING', u.id
FROM (VALUES
    ('Simyacı', 'simyaci', 'Paulo Coelho', 'Hazinesini bulmak için Endülüs''ten Mısır piramitlerine yola çıkan çoban Santiago''nun yolculuğu.', 1988, 188, 'pt', 'okur7@demo.bookhub'),
    ('Martin Eden', 'martin-eden', 'Jack London', 'Kendini yetiştirerek yazar olmaya çalışan genç bir denizcinin başarı ve yalnızlıkla sınavı.', 1909, 480, 'en', 'okur9@demo.bookhub')
) AS x(title, slug, author, description, year, pages, lang, email)
JOIN users u ON u.email = x.email
ON CONFLICT DO NOTHING;

-- Platon'un tarihi (MÖ) CHECK'e takılmaz ama uygulama doğrulaması 1450'den öncesini kabul
-- etmez; demo veride "bilinmiyor" (NULL) yapıyoruz.
UPDATE books SET publication_year = NULL WHERE slug = 'devlet';

-- Bilim Kurgu'da en fazla 12 kitap (bir sayfa): catalog.spec.ts Dune ve Vakıf'ı türün İLK
-- sayfasında arıyor, ikisi de listede eski kalıyor.
INSERT INTO book_genres (book_id, genre_id)
SELECT b.id, g.id FROM (VALUES
    ('tutunamayanlar','roman'), ('tutunamayanlar','klasik'), ('ince-memed','roman'), ('ince-memed','klasik'),
    ('kurk-mantolu-madonna','roman'), ('saatleri-ayarlama-enstitusu','roman'), ('1984','distopya'),
    ('1984','roman'), ('cesur-yeni-dunya','distopya'), ('dune','bilim-kurgu'), ('vakif','bilim-kurgu'),
    ('suc-ve-ceza','roman'), ('suc-ve-ceza','klasik'), ('sapiens','tarih'), ('clean-code-demo','yazilim'),
    ('gopl-demo','yazilim'), ('devlet','felsefe'), ('devlet','klasik'),
    ('calikusu','roman'), ('calikusu','klasik'), ('kuyucakli-yusuf','roman'), ('yaban','roman'),
    ('bereketli-topraklar-uzerinde','roman'), ('tehlikeli-oyunlar','roman'),
    ('anna-karenina','roman'), ('anna-karenina','klasik'), ('donusum','roman'), ('donusum','klasik'),
    ('hayvan-ciftligi','distopya'), ('hayvan-ciftligi','klasik'), ('gulun-adi','roman'), ('gulun-adi','polisiye'),
    ('otomatik-portakal','distopya'), ('damizlik-kizin-oykusu','distopya'), ('ben-robot','bilim-kurgu'),
    ('neuromancer','bilim-kurgu'), ('solaris','bilim-kurgu'), ('karanligin-sol-eli','bilim-kurgu'),
    ('yuzuk-kardesligi','fantastik'), ('boyle-buyurdu-zerdust','felsefe'), ('sofienin-dunyasi','felsefe'),
    ('sofienin-dunyasi','roman'), ('sisifos-soyleni','felsefe'), ('tufek-mikrop-ve-celik','tarih'),
    ('zamanin-kisa-tarihi','bilim'), ('dusunme-hizli-ve-yavas','bilim'), ('dogu-ekspresinde-cinayet','polisiye'),
    ('the-pragmatic-programmer','yazilim'), ('design-patterns','yazilim'),
    ('designing-data-intensive-applications','yazilim'), ('the-mythical-man-month','yazilim'),
    ('kucuk-prens','klasik'), ('yuzyillik-yalnizlik','roman'), ('yuzyillik-yalnizlik','klasik'),
    ('hobbit','fantastik'), ('huzur','roman'), ('huzur','klasik'), ('concurrency-in-go','yazilim'),
    ('kozmos','bilim'), ('karamazov-kardesler','roman'), ('karamazov-kardesler','klasik'),
    ('otostopcunun-galaksi-rehberi','bilim-kurgu'), ('benim-adim-kirmizi','roman'),
    ('benim-adim-kirmizi','polisiye'), ('yabanci','roman'), ('yabanci','felsefe'),
    ('fahrenheit-451','distopya'), ('fahrenheit-451','bilim-kurgu'), ('aylak-adam','roman'),
    ('simyaci','roman'), ('martin-eden','roman'), ('martin-eden','klasik')
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
             ('clean-code-demo', 7, 5), ('gopl-demo', 8, 4), ('devlet', 7, 3),
             ('calikusu', 8, 10), ('kuyucakli-yusuf', 8, 7), ('yaban', 7, 4),
             ('bereketli-topraklar-uzerinde', 8, 4), ('tehlikeli-oyunlar', 8, 5), ('anna-karenina', 8, 7),
             ('donusum', 8, 12), ('hayvan-ciftligi', 8, 12), ('gulun-adi', 8, 6), ('otomatik-portakal', 7, 7),
             ('damizlik-kizin-oykusu', 8, 6), ('ben-robot', 7, 6), ('neuromancer', 7, 4), ('solaris', 8, 3),
             ('karanligin-sol-eli', 8, 3), ('yuzuk-kardesligi', 9, 9), ('boyle-buyurdu-zerdust', 7, 5),
             ('sofienin-dunyasi', 8, 9), ('sisifos-soyleni', 7, 4), ('tufek-mikrop-ve-celik', 8, 5),
             ('zamanin-kisa-tarihi', 8, 8), ('dusunme-hizli-ve-yavas', 8, 6), ('dogu-ekspresinde-cinayet', 8, 9),
             ('the-pragmatic-programmer', 8, 5), ('design-patterns', 7, 4),
             ('designing-data-intensive-applications', 9, 4), ('the-mythical-man-month', 7, 3),
             ('kucuk-prens', 9, 12), ('yuzyillik-yalnizlik', 9, 9), ('hobbit', 8, 10), ('huzur', 9, 8),
             ('concurrency-in-go', 8, 3), ('kozmos', 9, 5), ('karamazov-kardesler', 9, 10),
             ('otostopcunun-galaksi-rehberi', 8, 8), ('benim-adim-kirmizi', 8, 8), ('yabanci', 8, 11),
             ('fahrenheit-451', 8, 9)) AS q(slug, base, voters)
JOIN books b ON b.slug = q.slug
JOIN LATERAL (SELECT id FROM users WHERE email LIKE 'okur%@demo.bookhub' ORDER BY id LIMIT q.voters) u ON true
ON CONFLICT DO NOTHING;

-- Faz 6'nın deneyi: TEK bir 10'luk oy. Basit ortalamada zirvede (10.00) olurdu; ağırlıklı
-- skor (m = RATING_MIN_VOTES) onu çok oylu kitapların gerisine çeker — "En iyiler"de yok.
INSERT INTO ratings (user_id, book_id, score)
SELECT (SELECT id FROM users WHERE email = 'okur1@demo.bookhub'), id, 10 FROM books WHERE slug = 'aylak-adam'
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
             ('huzur', 'okur6@demo.bookhub', 'Tanpınar''ın cümleleri müzik gibi; İstanbul''a bir daha aynı gözle bakamıyorsunuz.'),
             ('yuzyillik-yalnizlik', 'okur7@demo.bookhub', 'İsimler karışmasın diye soy ağacı çizerek okudum, kesinlikle değdi.'),
             ('kucuk-prens', 'okur8@demo.bookhub', 'Çocukken bir masal, büyüyünce bambaşka bir kitap.'),
             ('karamazov-kardesler', 'okur9@demo.bookhub', 'Büyük Engizisyoncu bölümü tek başına okunmaya değer.'),
             ('yabanci', 'okur10@demo.bookhub', 'İlk cümlesiyle akılda kalan, kısa ama sarsıcı bir roman.'),
             ('concurrency-in-go', 'okur11@demo.bookhub', 'Goroutine sızıntıları ve context ile iptal bölümleri çok işime yaradı.'),
             ('designing-data-intensive-applications', 'okur12@demo.bookhub', 'Backend''e geçen herkesin okuması gereken kitap; replikasyon bölümü harika.'),
             ('hobbit', 'okur3@demo.bookhub', 'Yüzüklerin Efendisi''ne başlamadan önce mükemmel bir giriş.'),
             ('dogu-ekspresinde-cinayet', 'okur2@demo.bookhub', 'Sonunu hiç tahmin edemedim; Poirot yine şaşırttı.'),
             ('aylak-adam', 'okur1@demo.bookhub', 'Kısa ama çok yoğun; C.''nin kalabalık içindeki yalnızlığı yıllardır aklımdan çıkmıyor.')) AS c(slug, email, content)
JOIN books b ON b.slug = c.slug
JOIN users u ON u.email = c.email
WHERE NOT EXISTS (SELECT 1 FROM comments x WHERE x.book_id = b.id AND x.content = c.content);

COMMIT;

SELECT (SELECT count(*) FROM books WHERE status = 'APPROVED') AS yayinda,
       (SELECT count(*) FROM books WHERE status = 'PENDING') AS bekleyen,
       (SELECT count(*) FROM ratings) AS oy,
       (SELECT count(*) FROM comments) AS yorum;
