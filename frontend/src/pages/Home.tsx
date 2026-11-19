import { Search, SearchX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useBooks, useGenres, useTopRated } from "../api/queries";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox, PageTitle } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { asGenre, asOffset, asQuery, autoSearchable, Chip, GRID, MIN_AUTO_SEARCH, useParamUpdater } from "../components/filters";
import { LibraryBadge } from "../components/LibraryBadge";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { useLibraryStatuses } from "../components/useLibraryStatuses";
import { Pager } from "../components/usePaged";
import { formatScore } from "../lib/format";
import { cn } from "../lib/utils";

const PAGE = 12;
const SEARCH_DELAY_MS = 300;

export function Home() {
  const [params, setParams] = useSearchParams();
  // Elle değiştirilmiş adres (/?offset=abc, /?genre=Bilim Kurgu) API'ye gitmez: bkz. filters.tsx.
  const q = asQuery(params.get("q"));
  const genre = asGenre(params.get("genre"));
  // Sıra: "" varsayılan (aramasız en yeni, aramada en ilgili), "rating" ağırlıklı puan, "newest" yalnız
  // aramada anlamlıdır (aramasız listede zaten varsayılan). Tanınmayan değer varsayılana düşer.
  const sortParam = params.get("sort");
  const sort: "" | "rating" | "newest" = sortParam === "rating" ? "rating" : sortParam === "newest" && q !== "" ? "newest" : "";
  const offset = asOffset(params.get("offset"));

  const [search, setSearch] = useState(q);

  // Sunucu verisi üç sorgudan gelir (Faz 16d adım 2–4; kancalar api/queries.ts'te). Eskiden burada üç
  // useState, iki useEffect ve elle bir "current" bayrağı vardı.
  //
  // ESKİ CEVAP KORUMASI artık useBooks'un içinde: "Dune" araması dönmüşken geç gelen filtresiz cevap
  // kendi önbellek kaydına yazılır, ekrana çıkmaz (gerçek hata: Playwright catalog senaryosu; testler
  // "geç gelen ESKİ liste cevabı…" ve "…HATASI da yutulur").
  const genres = useGenres();
  const topRated = useTopRated(5);
  // sort yalnız varsayılan değilken gider: istek adresi (ve önbellek anahtarı) gereksiz yere uzamasın.
  const books = useBooks({ q, genre, limit: PAGE, offset, ...(sort && { sort }) });
  const page = books.data;
  const top = topRated.data;
  // Liste hatası anahtara bağlı: filtre değişince yeni anahtarın hatası yoktur, eski hata kendiliğinden
  // kalkar. Tür ya da şerit hatası ayrı sorgularda olduğu için listeyle birlikte silinmez.
  const listError = books.error;
  const error = genres.error ?? topRated.error;
  // Yeni anahtarın cevabı beklenirken önceki sayfa "yer tutucu" olarak ekranda: soluk ve meşgul göster.
  // Aynı anahtarın arka planda tazelenmesi (pencereye dönünce) listeyi soluklaştırmaz.
  const loading = books.isPlaceholderData;

  // Gecikmeli çağrılar aşağıdaki `latest` üzerinden son render'ın update'ini kullanır (bkz. useParamUpdater).
  const update = useParamUpdater(setParams);

  // Zamanlayıcı kurulduktan sonraki 300 ms'de tür, sıra ya da sayfa seçilmiş, Enter'a basılmış
  // olabilir. Kurulduğu anın q/update'ini kullanırsa o seçimi eski URL ile ezer.
  const latest = useRef({ q, update });
  // Kutunun KENDİSİNİN URL'ye yazdığı q (zamanlayıcı ya da Enter). URL bu değere geçince kutu
  // geri senkronlanmaz: o arada yazılmış bir harf silinmesin.
  const ownQ = useRef<string | null>(null);
  useEffect(() => {
    latest.current = { q, update };
  });

  // Yazarken arama: yazmayı SEARCH_DELAY_MS bırakınca. replace: her duraklama tarayıcı
  // geçmişine ayrı bir kayıt eklemesin ("geri" tuşu harf harf geri gitmesin).
  // Eşitlik kontrolü zamanlayıcının İÇİNDE, son render'ın q'suyla yapılır: render anındaki q, URL
  // değişikliği henüz uygulanmamışsa eskidir.
  // Çok kısa metinde zamanlayıcı hiç kurulmaz (autoSearchable; gerekçesi filters.tsx'te): bekleme
  // süresi tek başına yetmez, yavaş yazan biri her harfte duraklar ve her harf bir istek olurdu.
  useEffect(() => {
    const trimmed = search.trim();
    if (!autoSearchable(trimmed)) return;
    const t = setTimeout(() => {
      const now = latest.current;
      if (trimmed === now.q) return;
      ownQ.current = trimmed;
      now.update({ q: trimmed }, true);
    }, SEARCH_DELAY_MS);
    return () => clearTimeout(t);
    // q bilerek bağımlılık değil: karar zamanlayıcının içinde son render'ın q'suyla verilir.
  }, [search]);

  // URL dışarıdan değişince (geri/ileri, logo) kutu da onu göstersin. Kutudaki değer zaten q'ya
  // denkse dokunulmaz: kullanıcının yazdığı baştaki/sondaki boşluklar silinmesin.
  // İşaret her q değişiminde tüketilir: önce başka bir gezinme geldiyse eski işaret sonraki bir
  // geri/ileri'yi yanlışlıkla "kendi yazdığım" sanmasın.
  useEffect(() => {
    const own = ownQ.current;
    ownQ.current = null;
    if (q === own) return;
    setSearch((s) => (s.trim() === q ? s : q));
  }, [q]);

  // Zaten seçili tür ya da sıra yeniden seçilince bir şey olmaz: update offset'i silip sayfayı başa
  // alır ve geçmişe kayıt eklerdi.
  // Karşılaştırma ekranda görünen (normalleştirilmiş) değerle: ?sort=foo da "En yeni" sayılır.
  const select = (next: Record<string, string>) => {
    const shown: Record<string, string> = { genre, sort };
    if (Object.entries(next).every(([k, v]) => shown[k] === v)) return;
    update(next);
  };

  const submitSearch = () => {
    const next = search.trim();
    if (next !== q) ownQ.current = next;
    update({ q: next });
  };

  const clearFilters = () => {
    setSearch("");
    setParams(new URLSearchParams());
  };

  // Kutudaki metin otomatik aranmayacak kadar kısaysa (ve henüz aranmadıysa) kullanıcıya ne olduğunu
  // söyle: ızgara önceki sonuçları göstermeye devam ediyor.
  const typed = search.trim();
  const tooShort = typed !== q && !autoSearchable(typed) && !typed.includes(" ");

  const filtered = q !== "" || genre !== "";
  // Puan sırasında şerit, listenin başıyla aynı şeyi gösterirdi.
  const showTop = !filtered && sort === "" && offset === 0 && top !== undefined && top.items.length > 0;
  // Izgara ve şerit için TEK istek: kimlikler birleşir, tekrarları kanca ayıklar.
  const statuses = useLibraryStatuses([...(page?.items ?? []), ...(showTop && top ? top.items : [])].map((b) => b.id));

  return (
    <div>
      <PageTitle />
      <section className="mb-10 rounded-2xl border bg-card px-5 py-8 shadow-sm sm:px-10 sm:py-10">
        <h1 className="text-3xl font-semibold sm:text-4xl">Bir sonraki kitabını bul</h1>
        <p className="mt-2 max-w-xl text-muted-foreground">Klasiklerden bilim kurguya; okurların puanları ve yorumlarıyla.</p>
        <form role="search" className="mt-6 flex gap-2" onSubmit={(e) => { e.preventDefault(); submitSearch(); }}>
          <div className="relative flex-1">
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label="Kitap ara"
              aria-describedby="search-hint"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Başlık ya da yazar ara…"
              maxLength={100}
              className="h-11 pl-11 md:text-base"
            />
          </div>
          <Button type="submit" size="lg" className="h-11">Ara</Button>
        </form>
        {/* Satır her zaman yerinde (sabit yükseklik): ipucu belirip kaybolurken türler zıplamaz. */}
        <p id="search-hint" role="status" className="mt-1 h-4 text-xs text-muted-foreground">
          {tooShort && `Yazarken arama ${MIN_AUTO_SEARCH} harfte başlar; daha kısası için Enter'a bas.`}
        </p>
        <div role="group" aria-label="Türler" className="-mx-1 mt-1 flex gap-2 overflow-x-auto px-1 pb-1">
          <Chip active={genre === ""} onClick={() => select({ genre: "" })}>Tümü</Chip>
          {(genres.data ?? []).map((g) => (
            <Chip key={g.id} active={genre === g.slug} onClick={() => select({ genre: g.slug })}>{g.name}</Chip>
          ))}
        </div>
      </section>

      <ErrorBox error={listError ?? error} className="mb-6" />

      {showTop && (
        <section aria-labelledby="top-heading" className="mb-12">
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <h2 id="top-heading" className="text-2xl font-semibold">Okurların en sevdikleri</h2>
            <details className="text-sm">
              <summary className="cursor-pointer text-muted-foreground hover:text-foreground">Nasıl sıralanıyor?</summary>
              <p className="mt-2 max-w-md text-muted-foreground">
                Basit ortalama değil, ağırlıklı (Bayes) puan: az oylu kitaplar genel ortalamaya
                (C = {formatScore(top.c, 2)}) doğru çekilir; m = {top.m} oyu geçen kitabın kendi ortalaması ağır basar.
              </p>
            </details>
          </div>
          <ol className="-mx-1 mt-4 flex gap-4 overflow-x-auto px-1 pt-2 pb-2 md:grid md:grid-cols-5 md:gap-6 md:overflow-visible">
            {top.items.map((b, i) => (
              <li key={b.id} className="w-32 shrink-0 md:w-auto">
                <Link to={`/books/${b.id}`} className="group block rounded-md">
                  <div className="relative">
                    <BookCover title={b.title} author={b.author} src={b.cover_url} className="transition duration-200 group-hover:-translate-y-1 group-hover:shadow-lg" />
                    <span aria-hidden="true" className="absolute -top-2 -left-2 grid size-8 place-items-center rounded-full bg-primary font-serif text-sm font-semibold text-primary-foreground shadow">
                      {i + 1}
                    </span>
                    {statuses[b.id] && <LibraryBadge status={statuses[b.id]} className="absolute bottom-2 left-2" />}
                  </div>
                  <span className="mt-2 line-clamp-2 block text-sm font-medium group-hover:underline">{b.title}</span>
                </Link>
                <span className="text-xs text-muted-foreground">{formatScore(b.weighted_score, 2)} · {b.rating_count} oy</span>
              </li>
            ))}
          </ol>
        </section>
      )}

      {!page && !listError && (
        <div role="status" aria-label="Kitaplar yükleniyor" className={GRID}>
          {Array.from({ length: PAGE }, (_, i) => (
            <div key={i} className="flex flex-col gap-3">
              <Skeleton className="aspect-[2/3] w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-3 w-1/2" />
            </div>
          ))}
        </div>
      )}
      {page && (
        <section aria-labelledby={page.items.length > 0 ? "results-heading" : undefined}>
          <div className="mb-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
            {page.items.length > 0 && (
              <h2 id="results-heading" className="text-2xl font-semibold">{filtered ? "Sonuçlar" : "Tüm kitaplar"}</h2>
            )}
            <div className="flex items-center gap-3">
              <p aria-live="polite" className="text-sm text-muted-foreground">{page.total} kitap</p>
              <div role="group" aria-label="Sıralama" className="inline-flex gap-1 rounded-lg border bg-card p-1">
                {/* Aramada varsayılan sıra ilgi sırasıdır (en iyi eşleşme önce); "En yeni" ayrı bir seçenek olur. */}
                {q !== "" && (
                  <Button size="sm" variant={sort === "" ? "default" : "ghost"} aria-pressed={sort === ""} onClick={() => select({ sort: "" })}>
                    En ilgili
                  </Button>
                )}
                <Button
                  size="sm"
                  variant={sort === (q !== "" ? "newest" : "") ? "default" : "ghost"}
                  aria-pressed={sort === (q !== "" ? "newest" : "")}
                  onClick={() => select({ sort: q !== "" ? "newest" : "" })}
                >
                  En yeni
                </Button>
                <Button size="sm" variant={sort === "rating" ? "default" : "ghost"} aria-pressed={sort === "rating"} onClick={() => select({ sort: "rating" })}>
                  En yüksek puan
                </Button>
              </div>
            </div>
          </div>
          {page.items.length === 0 ? (
            <EmptyState
              icon={SearchX}
              title="Sonuç bulunamadı"
              action={filtered && <Button variant="outline" onClick={clearFilters}>Filtreleri temizle</Button>}
            >
              {filtered ? "Aramanı ya da türü değiştirmeyi dene." : "Katalogda henüz kitap yok."}
            </EmptyState>
          ) : (
            <ul aria-label="Kitaplar" aria-busy={loading} className={cn(GRID, "transition-opacity", loading && "opacity-60")}>
              {page.items.map((b) => <li key={b.id}><BookCard book={b} headingLevel="h3" status={statuses[b.id]} /></li>)}
            </ul>
          )}
          {(offset > 0 || page.total > PAGE) && (
            <Pager page={page} offset={offset} onChange={(o) => update({ offset: String(o) })} />
          )}
        </section>
      )}
    </div>
  );
}
