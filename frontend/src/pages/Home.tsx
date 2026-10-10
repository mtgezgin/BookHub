import { Search, SearchX } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import type { Book, Genre, Page, TopRated } from "../api/types";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager } from "../components/usePaged";
import { formatScore } from "../lib/format";

const PAGE = 12;
// 12'lik sayfa her sütun sayısında satırları tam doldurur (2, 3, 4, 6).
const GRID = "grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6";
const SEARCH_DELAY_MS = 300;

function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <Button size="sm" variant={active ? "default" : "outline"} aria-pressed={active} className="shrink-0 rounded-full" onClick={onClick}>
      {children}
    </Button>
  );
}

export function Home() {
  const [params, setParams] = useSearchParams();
  const q = params.get("q") ?? "";
  const genre = params.get("genre") ?? "";
  // URL elle değiştirilebilir (/?offset=abc): geçersiz/negatif değer 0 sayılır (NaN API'ye gitmesin).
  const parsed = parseInt(params.get("offset") ?? "0", 10);
  const offset = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;

  const [page, setPage] = useState<Page<Book> | null>(null);
  const [genres, setGenres] = useState<Genre[]>([]);
  const [top, setTop] = useState<TopRated | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [search, setSearch] = useState(q);

  useEffect(() => {
    api.genres().then(setGenres).catch(setError);
    api.topRated(5).then(setTop).catch(setError);
  }, []);

  // ESKİ CEVAP KORUMASI (usePaged'deki ile aynı dert, burada elle): filtre değişince yeni
  // bir istek gider ama ÖNCEKİ istek hâlâ uçuyordur. Filtresiz sorgu tüm katalogu taradığı
  // için tipik olarak daha YAVAŞTIR → "Dune" araması 47 ms'de dönerken 243 ms'lik filtresiz
  // cevap sonra gelip ızgaranın üstüne yazıyordu: URL ?q=Dune, liste ise tüm kitaplar.
  // (Playwright'in catalog senaryosu tam bu yüzden kırmızı yandı; katalog büyüdükçe fark
  // açıldığı için hata "ara sıra" görünüyordu.) Temizlik fonksiyonu effect yeniden koşmadan
  // ÖNCE çalışır: geç gelen cevabı sessizce düşürüyoruz.
  useEffect(() => {
    let current = true;
    api.books({ q, genre, limit: PAGE, offset })
      .then((p) => current && setPage(p))
      .catch((e) => current && setError(e));
    return () => {
      current = false;
    };
  }, [q, genre, offset]);

  // Fonksiyon biçimi (prev): gecikmeli arama 300 ms sonra çalışır; o arada bir tür seçildiyse
  // render anındaki `params`'tan kurmak o seçimi ESKİ parametrelerle ezerdi.
  const update = useCallback(
    (next: Record<string, string>, replace = false) =>
      setParams((prev) => {
        const p = new URLSearchParams(prev);
        for (const [k, v] of Object.entries(next)) (v ? p.set(k, v) : p.delete(k));
        if (!("offset" in next)) p.delete("offset");
        return p;
      }, { replace }),
    [setParams],
  );

  // Yazarken arama: yazmayı SEARCH_DELAY_MS bırakınca. replace: her duraklama tarayıcı
  // geçmişine ayrı bir kayıt eklemesin ("geri" tuşu harf harf geri gitmesin).
  useEffect(() => {
    const trimmed = search.trim();
    if (trimmed === q) return;
    const t = setTimeout(() => update({ q: trimmed }, true), SEARCH_DELAY_MS);
    return () => clearTimeout(t);
    // q ve update bilerek bağımlılık değil: yalnız KULLANICI yazınca zamanlayıcı kurulur.
  }, [search]);

  const clearFilters = () => {
    setSearch("");
    setParams(new URLSearchParams());
  };

  const filtered = q !== "" || genre !== "";
  const showTop = !filtered && offset === 0 && top !== null && top.items.length > 0;

  return (
    <div>
      <section className="mb-10 rounded-2xl border bg-card px-5 py-8 shadow-sm sm:px-10 sm:py-10">
        <h1 className="text-3xl font-semibold sm:text-4xl">Bir sonraki kitabını bul</h1>
        <p className="mt-2 max-w-xl text-muted-foreground">Klasiklerden bilim kurguya; okurların puanları ve yorumlarıyla.</p>
        <form role="search" className="mt-6 flex gap-2" onSubmit={(e) => { e.preventDefault(); update({ q: search.trim() }); }}>
          <div className="relative flex-1">
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label="Kitap ara"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Başlık ya da yazar ara…"
              maxLength={100}
              className="h-11 pl-11 md:text-base"
            />
          </div>
          <Button type="submit" size="lg" className="h-11">Ara</Button>
        </form>
        <div role="group" aria-label="Türler" className="-mx-1 mt-4 flex gap-2 overflow-x-auto px-1 pb-1">
          <Chip active={genre === ""} onClick={() => update({ genre: "" })}>Tümü</Chip>
          {genres.map((g) => (
            <Chip key={g.id} active={genre === g.slug} onClick={() => update({ genre: g.slug })}>{g.name}</Chip>
          ))}
        </div>
      </section>

      <ErrorBox error={error} className="mb-6" />

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
                  </div>
                  <span className="mt-2 line-clamp-2 block text-sm font-medium group-hover:underline">{b.title}</span>
                </Link>
                <span className="text-xs text-muted-foreground">{formatScore(b.weighted_score, 2)} · {b.rating_count} oy</span>
              </li>
            ))}
          </ol>
        </section>
      )}

      {!page && !error && (
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
          <div className="mb-4 flex items-baseline justify-between gap-4">
            {page.items.length > 0 && (
              <h2 id="results-heading" className="text-2xl font-semibold">{filtered ? "Sonuçlar" : "Tüm kitaplar"}</h2>
            )}
            <p aria-live="polite" className="text-sm text-muted-foreground">{page.total} kitap</p>
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
            <ul aria-label="Kitaplar" className={GRID}>
              {page.items.map((b) => <li key={b.id}><BookCard book={b} headingLevel="h3" /></li>)}
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
