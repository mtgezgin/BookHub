-- +goose Up
-- Demo kataloğun gerçek kapakları (#34): Open Library'nin kapak kimliğiyle bağlantı (kopya tutulmaz;
-- kimlikli adresler hız sınırına girmez). Kural: Türkçe baskı varsa onun kapağı, yoksa özgün baskı;
-- her kapak gözle doğrulandı. Open Library'de kapağı olmayan Tehlikeli Oyunlar listede yok, üretilmiş
-- kapak kalır. `?default=false`: kapak kimliği bir gün kalkarsa Open Library boş bir yer tutucu değil
-- 404 döner, arayüz de üretilmiş kapağa düşer.
--
-- Eşleşme slug + yazar: aynı slug'la eklenmiş başka bir kitaba kapak yazılmaz. Yalnız BOŞ cover_url
-- doldurulur: admin'in sonradan koyduğu kapak ezilmez. Kitap yoksa (test veritabanı, taze kurulum)
-- hiçbir satır etkilenmez; `make demo-seed` ve DEPLOYMENT.md'nin ilk kurulumu bu dosyayı seed'den sonra
-- psql ile yeniden çalıştırır (tek kaynak). Bu yüzden Down bölümü YOK: psql onu da çalıştırırdı.
-- Bir kapak sonradan düzeltilecekse yeni bir migration gerekir ve o da `make demo-seed`'e eklenir.
UPDATE books SET cover_url = c.url, updated_at = now()
FROM (VALUES
    ('tutunamayanlar',                        'Oğuz Atay',                                                'https://covers.openlibrary.org/b/id/7242417-L.jpg?default=false'),
    ('ince-memed',                            'Yaşar Kemal',                                              'https://covers.openlibrary.org/b/id/10217552-L.jpg?default=false'),
    ('kurk-mantolu-madonna',                  'Sabahattin Ali',                                           'https://covers.openlibrary.org/b/id/13315465-L.jpg?default=false'),
    ('saatleri-ayarlama-enstitusu',           'Ahmet Hamdi Tanpınar',                                     'https://covers.openlibrary.org/b/id/12354821-L.jpg?default=false'),
    ('1984',                                  'George Orwell',                                            'https://covers.openlibrary.org/b/id/9267242-L.jpg?default=false'),
    ('cesur-yeni-dunya',                      'Aldous Huxley',                                            'https://covers.openlibrary.org/b/id/11120906-L.jpg?default=false'),
    ('dune',                                  'Frank Herbert',                                            'https://covers.openlibrary.org/b/id/12227029-L.jpg?default=false'),
    ('vakif',                                 'Isaac Asimov',                                             'https://covers.openlibrary.org/b/id/14612610-L.jpg?default=false'),
    ('suc-ve-ceza',                           'Fyodor Dostoyevski',                                       'https://covers.openlibrary.org/b/id/12119770-L.jpg?default=false'),
    ('sapiens',                               'Yuval Noah Harari',                                        'https://covers.openlibrary.org/b/id/12130527-L.jpg?default=false'),
    ('clean-code-demo',                       'Robert C. Martin',                                         'https://covers.openlibrary.org/b/id/8065615-L.jpg?default=false'),
    ('gopl-demo',                             'Alan Donovan, Brian Kernighan',                            'https://covers.openlibrary.org/b/id/7427319-L.jpg?default=false'),
    ('devlet',                                'Platon',                                                   'https://covers.openlibrary.org/b/id/12287137-L.jpg?default=false'),
    ('calikusu',                              'Reşat Nuri Güntekin',                                      'https://covers.openlibrary.org/b/id/10441700-L.jpg?default=false'),
    ('kuyucakli-yusuf',                       'Sabahattin Ali',                                           'https://covers.openlibrary.org/b/id/10850380-L.jpg?default=false'),
    ('yaban',                                 'Yakup Kadri Karaosmanoğlu',                                'https://covers.openlibrary.org/b/id/6954272-L.jpg?default=false'),
    ('bereketli-topraklar-uzerinde',          'Orhan Kemal',                                              'https://covers.openlibrary.org/b/id/12306774-L.jpg?default=false'),
    ('anna-karenina',                         'Lev Tolstoy',                                              'https://covers.openlibrary.org/b/id/2560652-L.jpg?default=false'),
    ('donusum',                               'Franz Kafka',                                              'https://covers.openlibrary.org/b/id/12121659-L.jpg?default=false'),
    ('hayvan-ciftligi',                       'George Orwell',                                            'https://covers.openlibrary.org/b/id/12115318-L.jpg?default=false'),
    ('gulun-adi',                             'Umberto Eco',                                              'https://covers.openlibrary.org/b/id/10848813-L.jpg?default=false'),
    ('otomatik-portakal',                     'Anthony Burgess',                                          'https://covers.openlibrary.org/b/id/15253217-L.jpg?default=false'),
    ('damizlik-kizin-oykusu',                 'Margaret Atwood',                                          'https://covers.openlibrary.org/b/id/10848888-L.jpg?default=false'),
    ('ben-robot',                             'Isaac Asimov',                                             'https://covers.openlibrary.org/b/id/12126245-L.jpg?default=false'),
    ('neuromancer',                           'William Gibson',                                           'https://covers.openlibrary.org/b/id/283860-L.jpg?default=false'),
    ('solaris',                               'Stanisław Lem',                                            'https://covers.openlibrary.org/b/id/12313764-L.jpg?default=false'),
    ('karanligin-sol-eli',                    'Ursula K. Le Guin',                                        'https://covers.openlibrary.org/b/id/12366088-L.jpg?default=false'),
    ('yuzuk-kardesligi',                      'J. R. R. Tolkien',                                         'https://covers.openlibrary.org/b/id/15173637-L.jpg?default=false'),
    ('boyle-buyurdu-zerdust',                 'Friedrich Nietzsche',                                      'https://covers.openlibrary.org/b/id/12310947-L.jpg?default=false'),
    ('sofienin-dunyasi',                      'Jostein Gaarder',                                          'https://covers.openlibrary.org/b/id/223463-L.jpg?default=false'),
    ('sisifos-soyleni',                       'Albert Camus',                                             'https://covers.openlibrary.org/b/id/12726570-L.jpg?default=false'),
    ('tufek-mikrop-ve-celik',                 'Jared Diamond',                                            'https://covers.openlibrary.org/b/id/7884018-L.jpg?default=false'),
    ('zamanin-kisa-tarihi',                   'Stephen Hawking',                                          'https://covers.openlibrary.org/b/id/13211494-L.jpg?default=false'),
    ('dusunme-hizli-ve-yavas',                'Daniel Kahneman',                                          'https://covers.openlibrary.org/b/id/12311139-L.jpg?default=false'),
    ('dogu-ekspresinde-cinayet',              'Agatha Christie',                                          'https://covers.openlibrary.org/b/id/13908815-L.jpg?default=false'),
    ('the-pragmatic-programmer',              'Andrew Hunt, David Thomas',                                'https://covers.openlibrary.org/b/id/10143650-L.jpg?default=false'),
    ('design-patterns',                       'Erich Gamma, Richard Helm, Ralph Johnson, John Vlissides', 'https://covers.openlibrary.org/b/id/6601119-L.jpg?default=false'),
    ('designing-data-intensive-applications', 'Martin Kleppmann',                                         'https://covers.openlibrary.org/b/id/8434671-L.jpg?default=false'),
    ('the-mythical-man-month',                'Frederick P. Brooks Jr.',                                  'https://covers.openlibrary.org/b/id/6915361-L.jpg?default=false'),
    ('kucuk-prens',                           'Antoine de Saint-Exupéry',                                 'https://covers.openlibrary.org/b/id/12307643-L.jpg?default=false'),
    ('yuzyillik-yalnizlik',                   'Gabriel García Márquez',                                   'https://covers.openlibrary.org/b/id/12273258-L.jpg?default=false'),
    ('hobbit',                                'J. R. R. Tolkien',                                         'https://covers.openlibrary.org/b/id/12283424-L.jpg?default=false'),
    ('huzur',                                 'Ahmet Hamdi Tanpınar',                                     'https://covers.openlibrary.org/b/id/6878782-L.jpg?default=false'),
    ('concurrency-in-go',                     'Katherine Cox-Buday',                                      'https://covers.openlibrary.org/b/id/8510272-L.jpg?default=false'),
    ('kozmos',                                'Carl Sagan',                                               'https://covers.openlibrary.org/b/id/8283901-L.jpg?default=false'),
    ('karamazov-kardesler',                   'Fyodor Dostoyevski',                                       'https://covers.openlibrary.org/b/id/12236237-L.jpg?default=false'),
    ('otostopcunun-galaksi-rehberi',          'Douglas Adams',                                            'https://covers.openlibrary.org/b/id/13149878-L.jpg?default=false'),
    ('benim-adim-kirmizi',                    'Orhan Pamuk',                                              'https://covers.openlibrary.org/b/id/12630674-L.jpg?default=false'),
    ('yabanci',                               'Albert Camus',                                             'https://covers.openlibrary.org/b/id/12316737-L.jpg?default=false'),
    ('fahrenheit-451',                        'Ray Bradbury',                                             'https://covers.openlibrary.org/b/id/8151231-L.jpg?default=false'),
    ('aylak-adam',                            'Yusuf Atılgan',                                            'https://covers.openlibrary.org/b/id/10516494-L.jpg?default=false'),
    ('simyaci',                               'Paulo Coelho',                                             'https://covers.openlibrary.org/b/id/12296155-L.jpg?default=false'),
    ('martin-eden',                           'Jack London',                                              'https://covers.openlibrary.org/b/id/9223765-L.jpg?default=false')
) AS c (slug, author, url)
WHERE books.slug = c.slug AND books.author = c.author AND books.cover_url = '';
