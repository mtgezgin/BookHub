-- 0012_demo_editions.sql — demo kataloğun baskıları (Faz 16, #36).
--
-- 0009'un ilkeleriyle: eşleşme slug + yazar (aynı slug'la eklenmiş başka bir kitaba dokunulmaz), kitap
-- yoksa (test veritabanı, taze kurulum) hiçbir satır etkilenmez, tekrar çalıştırılabilir. Down bölümü
-- YOK: `make demo-seed` ve DEPLOYMENT.md'nin ilk kurulumu bu dosyayı seed'den sonra psql ile yeniden
-- çalıştırır; psql Down'u da koşardı. Bir değer sonradan düzeltilecekse yeni bir migration gerekir.
--
-- Kaynak: 0009'daki kapak kimliği → Open Library kapak kaydı (olid) → baskı kaydı (2026-11-12). Kayıt
-- verisi temizlendi: yayınevi adlarındaki düşmüş Türkçe harfler düzeltildi, dil boşsa yayınevinden
-- çıkarıldı (Türkçe yayınevi → tr). Çevirmen Open Library'de çoğunlukla yok; Solaris ve Gülün Adı'nın
-- çevirmeni kapaktan okundu. Clean Code, Yaban ve Kozmos'un kapağı bir baskı kaydına bağlı değil: dil
-- ve (görünüyorsa) yayınevi kapaktan. Anna Karenina'nın kapağı bir sesli kitap kaydına bağlı (Books on
-- Tape); basılı kitap gibi görünüp yanıltmasın diye yayınevi ve yıl boş. Bilinmeyen alan '' / NULL
-- kalır.

-- +goose Up

-- 1) Varsayılan baskıların meta verisi. 0011 her kitaba kitabın dili (eserin özgün dili) ve kapağıyla
--    bir varsayılan baskı açtı; burada baskının gerçek dili, yayınevi, çevirmeni ve yılı yazılır.
--    Kapak 0009'daki adresin aynısı: canlıda 0011 onu zaten kopyaladı (değer değişmez); taze kurulumda
--    seed'in açtığı boş kapaklı baskı dolar. Sayfa sayısına dokunulmaz.
--    Koruma: yalnız kapağı hâlâ '' ya da 0009'daki adres olan varsayılan baskı güncellenir; admin'in
--    sonradan değiştirdiği baskıya dokunulmaz. Baskı adı yalnız Türkçe olmayan baskılarda verilir.
UPDATE editions e
SET language         = v.lang,
    publisher        = CASE WHEN v.publisher <> '' THEN v.publisher ELSE e.publisher END,
    translator       = CASE WHEN v.translator <> '' THEN v.translator ELSE e.translator END,
    publication_year = COALESCE(v.year, e.publication_year),
    title            = COALESCE(v.title, e.title),
    cover_url        = v.url,
    updated_at       = now()
FROM books b
JOIN (VALUES
    ('tutunamayanlar', 'Oğuz Atay', 'tr', 'İletişim Yayınları', '', 1984, NULL::text, 'https://covers.openlibrary.org/b/id/7242417-L.jpg?default=false'),
    ('ince-memed', 'Yaşar Kemal', 'tr', 'Yapı Kredi Yayınları', '', 2004, NULL::text, 'https://covers.openlibrary.org/b/id/10217552-L.jpg?default=false'),
    ('kurk-mantolu-madonna', 'Sabahattin Ali', 'tr', 'Yapı Kredi Yayınları', '', 2022, NULL::text, 'https://covers.openlibrary.org/b/id/13315465-L.jpg?default=false'),
    ('saatleri-ayarlama-enstitusu', 'Ahmet Hamdi Tanpınar', 'tr', 'Dergâh Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/12354821-L.jpg?default=false'),
    ('1984', 'George Orwell', 'en', 'Harcourt, Brace and Company', '', 1949, 'Nineteen Eighty-Four', 'https://covers.openlibrary.org/b/id/9267242-L.jpg?default=false'),
    ('cesur-yeni-dunya', 'Aldous Huxley', 'tr', 'İthaki Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/11120906-L.jpg?default=false'),
    ('dune', 'Frank Herbert', 'tr', 'İthaki Yayınları', '', 2015, NULL::text, 'https://covers.openlibrary.org/b/id/12227029-L.jpg?default=false'),
    ('vakif', 'Isaac Asimov', 'en', 'Panther', '', 1962, 'Foundation', 'https://covers.openlibrary.org/b/id/14612610-L.jpg?default=false'),
    ('suc-ve-ceza', 'Fyodor Dostoyevski', 'tr', 'Yediveren Yayınları', '', 2021, NULL::text, 'https://covers.openlibrary.org/b/id/12119770-L.jpg?default=false'),
    ('sapiens', 'Yuval Noah Harari', 'tr', 'Kolektif Kitap', '', 2015, NULL::text, 'https://covers.openlibrary.org/b/id/12130527-L.jpg?default=false'),
    ('gopl-demo', 'Alan Donovan, Brian Kernighan', 'en', 'Addison-Wesley', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/7427319-L.jpg?default=false'),
    ('devlet', 'Platon', 'tr', 'Profil Kitap', '', 2011, NULL::text, 'https://covers.openlibrary.org/b/id/12287137-L.jpg?default=false'),
    ('calikusu', 'Reşat Nuri Güntekin', 'tr', 'İnkılap Kitabevi', '', 2010, NULL::text, 'https://covers.openlibrary.org/b/id/10441700-L.jpg?default=false'),
    ('kuyucakli-yusuf', 'Sabahattin Ali', 'tr', 'Parga Yayıncılık', '', 2019, NULL::text, 'https://covers.openlibrary.org/b/id/10850380-L.jpg?default=false'),
    ('bereketli-topraklar-uzerinde', 'Orhan Kemal', 'tr', 'Everest Yayınları', '', 2010, NULL::text, 'https://covers.openlibrary.org/b/id/12306774-L.jpg?default=false'),
    ('anna-karenina', 'Lev Tolstoy', 'en', '', '', NULL::integer, 'Anna Karenina', 'https://covers.openlibrary.org/b/id/2560652-L.jpg?default=false'),
    ('donusum', 'Franz Kafka', 'tr', 'Destek Yayınları', '', 2018, NULL::text, 'https://covers.openlibrary.org/b/id/12121659-L.jpg?default=false'),
    ('hayvan-ciftligi', 'George Orwell', 'tr', 'İskele Yayıncılık', '', 2021, NULL::text, 'https://covers.openlibrary.org/b/id/12115318-L.jpg?default=false'),
    ('gulun-adi', 'Umberto Eco', 'tr', 'Can Yayınları', 'Şadan Karadeniz', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/10848813-L.jpg?default=false'),
    ('otomatik-portakal', 'Anthony Burgess', 'tr', 'Türkiye İş Bankası Kültür Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/15253217-L.jpg?default=false'),
    ('damizlik-kizin-oykusu', 'Margaret Atwood', 'tr', 'Doğan Kitap', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/10848888-L.jpg?default=false'),
    ('ben-robot', 'Isaac Asimov', 'tr', 'İthaki Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/12126245-L.jpg?default=false'),
    ('neuromancer', 'William Gibson', 'en', 'Ace Books', '', 2000, NULL::text, 'https://covers.openlibrary.org/b/id/283860-L.jpg?default=false'),
    ('solaris', 'Stanisław Lem', 'tr', 'İletişim Yayınları', 'Mehmet Aközer', 1997, NULL::text, 'https://covers.openlibrary.org/b/id/12313764-L.jpg?default=false'),
    ('karanligin-sol-eli', 'Ursula K. Le Guin', 'tr', 'Ayrıntı Yayınları', '', 1993, NULL::text, 'https://covers.openlibrary.org/b/id/12366088-L.jpg?default=false'),
    ('yuzuk-kardesligi', 'J. R. R. Tolkien', 'en', 'HarperCollins', '', 2005, 'The Fellowship of the Ring', 'https://covers.openlibrary.org/b/id/15173637-L.jpg?default=false'),
    ('boyle-buyurdu-zerdust', 'Friedrich Nietzsche', 'tr', 'Cem Yayınevi', '', 1990, NULL::text, 'https://covers.openlibrary.org/b/id/12310947-L.jpg?default=false'),
    ('sofienin-dunyasi', 'Jostein Gaarder', 'en', 'Farrar, Straus and Giroux', 'Paulette Møller', 1994, 'Sophie''s World', 'https://covers.openlibrary.org/b/id/223463-L.jpg?default=false'),
    ('sisifos-soyleni', 'Albert Camus', 'en', 'Vintage Books', 'Justin O''Brien', 1991, 'The Myth of Sisyphus and Other Essays', 'https://covers.openlibrary.org/b/id/12726570-L.jpg?default=false'),
    ('tufek-mikrop-ve-celik', 'Jared Diamond', 'en', 'Jonathan Cape', '', 1997, 'Guns, Germs and Steel', 'https://covers.openlibrary.org/b/id/7884018-L.jpg?default=false'),
    ('zamanin-kisa-tarihi', 'Stephen Hawking', 'tr', 'Alfa Yayınları', '', 2021, NULL::text, 'https://covers.openlibrary.org/b/id/13211494-L.jpg?default=false'),
    ('dusunme-hizli-ve-yavas', 'Daniel Kahneman', 'tr', 'Varlık Yayınları', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/12311139-L.jpg?default=false'),
    ('dogu-ekspresinde-cinayet', 'Agatha Christie', 'tr', 'Altın Kitaplar', '', 1994, NULL::text, 'https://covers.openlibrary.org/b/id/13908815-L.jpg?default=false'),
    ('the-pragmatic-programmer', 'Andrew Hunt, David Thomas', 'en', '', '', 2019, NULL::text, 'https://covers.openlibrary.org/b/id/10143650-L.jpg?default=false'),
    ('design-patterns', 'Erich Gamma, Richard Helm, Ralph Johnson, John Vlissides', 'en', 'Addison-Wesley', '', 1995, NULL::text, 'https://covers.openlibrary.org/b/id/6601119-L.jpg?default=false'),
    ('designing-data-intensive-applications', 'Martin Kleppmann', 'en', 'O''Reilly Media', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/8434671-L.jpg?default=false'),
    ('the-mythical-man-month', 'Frederick P. Brooks Jr.', 'en', 'Addison-Wesley', '', 1975, NULL::text, 'https://covers.openlibrary.org/b/id/6915361-L.jpg?default=false'),
    ('kucuk-prens', 'Antoine de Saint-Exupéry', 'tr', 'Mavibulut Yayıncılık', '', 2015, NULL::text, 'https://covers.openlibrary.org/b/id/12307643-L.jpg?default=false'),
    ('yuzyillik-yalnizlik', 'Gabriel García Márquez', 'tr', 'Can Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/12273258-L.jpg?default=false'),
    ('hobbit', 'J. R. R. Tolkien', 'tr', 'İthaki Yayınları', '', 2000, NULL::text, 'https://covers.openlibrary.org/b/id/12283424-L.jpg?default=false'),
    ('huzur', 'Ahmet Hamdi Tanpınar', 'tr', 'Yapı Kredi Yayınları', '', 2001, NULL::text, 'https://covers.openlibrary.org/b/id/6878782-L.jpg?default=false'),
    ('concurrency-in-go', 'Katherine Cox-Buday', 'en', 'O''Reilly Media', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/8510272-L.jpg?default=false'),
    ('karamazov-kardesler', 'Fyodor Dostoyevski', 'tr', 'Alter Yayınları', '', 2011, NULL::text, 'https://covers.openlibrary.org/b/id/12236237-L.jpg?default=false'),
    ('otostopcunun-galaksi-rehberi', 'Douglas Adams', 'tr', 'Alfa Yayınları', '', 2018, NULL::text, 'https://covers.openlibrary.org/b/id/13149878-L.jpg?default=false'),
    ('benim-adim-kirmizi', 'Orhan Pamuk', 'tr', 'İletişim Yayınları', '', 2010, NULL::text, 'https://covers.openlibrary.org/b/id/12630674-L.jpg?default=false'),
    ('yabanci', 'Albert Camus', 'tr', 'Kurukafa Yayınevi', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/12316737-L.jpg?default=false'),
    ('fahrenheit-451', 'Ray Bradbury', 'tr', 'İthaki Yayınları', '', 2007, NULL::text, 'https://covers.openlibrary.org/b/id/8151231-L.jpg?default=false'),
    ('aylak-adam', 'Yusuf Atılgan', 'tr', 'Can Yayınları', '', 2017, NULL::text, 'https://covers.openlibrary.org/b/id/10516494-L.jpg?default=false'),
    ('simyaci', 'Paulo Coelho', 'tr', 'Can Yayınları', '', 2016, NULL::text, 'https://covers.openlibrary.org/b/id/12296155-L.jpg?default=false'),
    ('martin-eden', 'Jack London', 'en', 'Macmillan', '', 1909, NULL::text, 'https://covers.openlibrary.org/b/id/9223765-L.jpg?default=false'),
    ('clean-code-demo', 'Robert C. Martin', 'en', 'Prentice Hall', '', NULL::integer, NULL::text, 'https://covers.openlibrary.org/b/id/8065615-L.jpg?default=false'),
    ('yaban', 'Yakup Kadri Karaosmanoğlu', 'tr', 'İletişim Yayınları', '', NULL::integer, NULL::text, 'https://covers.openlibrary.org/b/id/6954272-L.jpg?default=false'),
    ('kozmos', 'Carl Sagan', 'en', '', '', NULL::integer, 'Cosmos', 'https://covers.openlibrary.org/b/id/8283901-L.jpg?default=false')

) AS v(slug, author, lang, publisher, translator, year, title, url) ON b.slug = v.slug AND b.author = v.author
WHERE e.book_id = b.id
  AND e.is_primary
  AND (e.cover_url = '' OR e.cover_url = v.url);

-- 2) Özgün dil baskıları: sekiz çeviri kitaba eserin özgün dilinde birer baskı (varsayılan değil).
--    1984'ün varsayılan baskısı zaten İngilizce olduğu için yerine Cesur Yeni Dünya; Simyacı demo
--    katalogda onay bekleyen bir öneri olduğu için yerine Solaris (Lehçe) seçildi. Kapaklar gözle
--    doğrulandı (#36). Aynı kitapta aynı dil ve adla bir baskı varsa eklenmez.
INSERT INTO editions (book_id, title, language, publisher, publication_year, isbn, page_count, cover_url, is_primary)
SELECT b.id, v.title, v.lang, v.publisher, v.year, v.isbn, v.pages, v.url, false
FROM (VALUES
    ('suc-ve-ceza', 'Fyodor Dostoyevski', 'Преступление и наказание', 'ru', 'Художественная литература', 2012, '5280033022', NULL::integer, 'https://covers.openlibrary.org/b/id/14352226-L.jpg?default=false'),
    ('donusum', 'Franz Kafka', 'Die Verwandlung', 'de', 'Philipp Reclam jun.', 2002, '3150099005', 79, 'https://covers.openlibrary.org/b/id/10583545-L.jpg?default=false'),
    ('kucuk-prens', 'Antoine de Saint-Exupéry', 'Le Petit Prince', 'fr', 'Wolters-Noordhoff', 1976, '9789001708528', 100, 'https://covers.openlibrary.org/b/id/10778405-L.jpg?default=false'),
    ('yuzyillik-yalnizlik', 'Gabriel García Márquez', 'Cien años de soledad', 'es', 'Mondadori', 1988, '8439711085', 456, 'https://covers.openlibrary.org/b/id/14359127-L.jpg?default=false'),
    ('solaris', 'Stanisław Lem', 'Solaris', 'pl', 'Wydawnictwo Ministerstwa Obrony Narodowej', 1961, '', 196, 'https://covers.openlibrary.org/b/id/15235930-L.jpg?default=false'),
    ('gulun-adi', 'Umberto Eco', 'Il nome della rosa', 'it', 'Bompiani', 1989, '8845207056', 533, 'https://covers.openlibrary.org/b/id/10144473-L.jpg?default=false'),
    ('cesur-yeni-dunya', 'Aldous Huxley', 'Brave New World', 'en', 'Penguin Books', 1964, '', NULL::integer, 'https://covers.openlibrary.org/b/id/15170943-L.jpg?default=false'),
    ('dune', 'Frank Herbert', 'Dune', 'en', 'Chilton Books', 1965, '', NULL::integer, 'https://covers.openlibrary.org/b/id/12375564-L.jpg?default=false')

) AS v(slug, author, title, lang, publisher, year, isbn, pages, url)
JOIN books b ON b.slug = v.slug AND b.author = v.author
WHERE NOT EXISTS (
    SELECT 1 FROM editions e WHERE e.book_id = b.id AND e.language = v.lang AND e.title = v.title
);
