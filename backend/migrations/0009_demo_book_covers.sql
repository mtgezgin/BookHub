-- +goose Up
-- Demo kataloğun gerçek kapakları (#34): Open Library'nin kapak kimliğiyle bağlantı (kopya tutulmaz;
-- kimlikli adresler hız sınırına girmez). Kural: Türkçe baskı varsa onun kapağı, yoksa özgün baskı;
-- her kapak gözle doğrulandı. Open Library'de kapağı olmayan Tehlikeli Oyunlar listede yok, üretilmiş
-- kapak kalır; resim yüklenemezse arayüz de üretilmiş kapağa döner.
--
-- Yalnız BOŞ cover_url doldurulur: admin'in sonradan koyduğu kapak ezilmez. Kitap yoksa (test
-- veritabanı, taze kurulum) hiçbir satır etkilenmez; `make demo-seed` bu dosyayı seed'den sonra
-- psql ile yeniden çalıştırır (tek kaynak). Bu yüzden Down bölümü YOK: psql onu da çalıştırırdı.
UPDATE books SET cover_url = c.url, updated_at = now()
FROM (VALUES
    ('tutunamayanlar',                        'https://covers.openlibrary.org/b/id/7242417-L.jpg'),
    ('ince-memed',                            'https://covers.openlibrary.org/b/id/10217552-L.jpg'),
    ('kurk-mantolu-madonna',                  'https://covers.openlibrary.org/b/id/13315465-L.jpg'),
    ('saatleri-ayarlama-enstitusu',           'https://covers.openlibrary.org/b/id/12354821-L.jpg'),
    ('1984',                                  'https://covers.openlibrary.org/b/id/9267242-L.jpg'),
    ('cesur-yeni-dunya',                      'https://covers.openlibrary.org/b/id/11120906-L.jpg'),
    ('dune',                                  'https://covers.openlibrary.org/b/id/12227029-L.jpg'),
    ('vakif',                                 'https://covers.openlibrary.org/b/id/14612610-L.jpg'),
    ('suc-ve-ceza',                           'https://covers.openlibrary.org/b/id/12119770-L.jpg'),
    ('sapiens',                               'https://covers.openlibrary.org/b/id/12130527-L.jpg'),
    ('clean-code-demo',                       'https://covers.openlibrary.org/b/id/8065615-L.jpg'),
    ('gopl-demo',                             'https://covers.openlibrary.org/b/id/7427319-L.jpg'),
    ('devlet',                                'https://covers.openlibrary.org/b/id/12287137-L.jpg'),
    ('calikusu',                              'https://covers.openlibrary.org/b/id/10441700-L.jpg'),
    ('kuyucakli-yusuf',                       'https://covers.openlibrary.org/b/id/10850380-L.jpg'),
    ('yaban',                                 'https://covers.openlibrary.org/b/id/6954272-L.jpg'),
    ('bereketli-topraklar-uzerinde',          'https://covers.openlibrary.org/b/id/12306774-L.jpg'),
    ('anna-karenina',                         'https://covers.openlibrary.org/b/id/2560652-L.jpg'),
    ('donusum',                               'https://covers.openlibrary.org/b/id/12121659-L.jpg'),
    ('hayvan-ciftligi',                       'https://covers.openlibrary.org/b/id/12115318-L.jpg'),
    ('gulun-adi',                             'https://covers.openlibrary.org/b/id/10848813-L.jpg'),
    ('otomatik-portakal',                     'https://covers.openlibrary.org/b/id/15253217-L.jpg'),
    ('damizlik-kizin-oykusu',                 'https://covers.openlibrary.org/b/id/10848888-L.jpg'),
    ('ben-robot',                             'https://covers.openlibrary.org/b/id/12126245-L.jpg'),
    ('neuromancer',                           'https://covers.openlibrary.org/b/id/283860-L.jpg'),
    ('solaris',                               'https://covers.openlibrary.org/b/id/12313764-L.jpg'),
    ('karanligin-sol-eli',                    'https://covers.openlibrary.org/b/id/12366088-L.jpg'),
    ('yuzuk-kardesligi',                      'https://covers.openlibrary.org/b/id/15173637-L.jpg'),
    ('boyle-buyurdu-zerdust',                 'https://covers.openlibrary.org/b/id/12310947-L.jpg'),
    ('sofienin-dunyasi',                      'https://covers.openlibrary.org/b/id/223463-L.jpg'),
    ('sisifos-soyleni',                       'https://covers.openlibrary.org/b/id/12726570-L.jpg'),
    ('tufek-mikrop-ve-celik',                 'https://covers.openlibrary.org/b/id/7884018-L.jpg'),
    ('zamanin-kisa-tarihi',                   'https://covers.openlibrary.org/b/id/13211494-L.jpg'),
    ('dusunme-hizli-ve-yavas',                'https://covers.openlibrary.org/b/id/12311139-L.jpg'),
    ('dogu-ekspresinde-cinayet',              'https://covers.openlibrary.org/b/id/13908815-L.jpg'),
    ('the-pragmatic-programmer',              'https://covers.openlibrary.org/b/id/10143650-L.jpg'),
    ('design-patterns',                       'https://covers.openlibrary.org/b/id/6601119-L.jpg'),
    ('designing-data-intensive-applications', 'https://covers.openlibrary.org/b/id/8434671-L.jpg'),
    ('the-mythical-man-month',                'https://covers.openlibrary.org/b/id/6915361-L.jpg'),
    ('kucuk-prens',                           'https://covers.openlibrary.org/b/id/12307643-L.jpg'),
    ('yuzyillik-yalnizlik',                   'https://covers.openlibrary.org/b/id/12273258-L.jpg'),
    ('hobbit',                                'https://covers.openlibrary.org/b/id/12283424-L.jpg'),
    ('huzur',                                 'https://covers.openlibrary.org/b/id/6878782-L.jpg'),
    ('concurrency-in-go',                     'https://covers.openlibrary.org/b/id/8510272-L.jpg'),
    ('kozmos',                                'https://covers.openlibrary.org/b/id/8283901-L.jpg'),
    ('karamazov-kardesler',                   'https://covers.openlibrary.org/b/id/12236237-L.jpg'),
    ('otostopcunun-galaksi-rehberi',          'https://covers.openlibrary.org/b/id/13149878-L.jpg'),
    ('benim-adim-kirmizi',                    'https://covers.openlibrary.org/b/id/12630674-L.jpg'),
    ('yabanci',                               'https://covers.openlibrary.org/b/id/12316737-L.jpg'),
    ('fahrenheit-451',                        'https://covers.openlibrary.org/b/id/8151231-L.jpg'),
    ('aylak-adam',                            'https://covers.openlibrary.org/b/id/10516494-L.jpg'),
    ('martin-eden',                           'https://covers.openlibrary.org/b/id/9223765-L.jpg')
) AS c (slug, url)
WHERE books.slug = c.slug AND books.cover_url = '';
