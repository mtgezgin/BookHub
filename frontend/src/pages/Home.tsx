import { Search, SearchX } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import type { Book, Genre, Page, TopRated } from "../api/types";
import { BookCard } from "../components/BookCard";
import { ErrorBox } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Input, NativeSelect } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager } from "../components/usePaged";
import { formatScore } from "../lib/format";

const PAGE = 12;
const GRID = "grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 lg:grid-cols-4";

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

  const update = (next: Record<string, string>) => {
    const p = new URLSearchParams(params);
    for (const [k, v] of Object.entries(next)) (v ? p.set(k, v) : p.delete(k));
    if (!("offset" in next)) p.delete("offset");
    setParams(p);
  };

  const clearFilters = () => {
    setSearch("");
    setParams(new URLSearchParams());
  };

  const filtered = q !== "" || genre !== "";

  return (
    <div className="grid gap-10 lg:grid-cols-[1fr_17rem]">
      <section className="min-w-0">
        <h1 className="sr-only">Kitap kataloğu</h1>
        <form role="search" className="mb-6 flex flex-col gap-2 sm:flex-row" onSubmit={(e) => { e.preventDefault(); update({ q: search.trim() }); }}>
          <div className="relative flex-1">
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label="Kitap ara"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Başlık ya da yazar ara…"
              maxLength={100}
              className="pl-9"
            />
          </div>
          <div className="flex gap-2">
            <NativeSelect aria-label="Tür" value={genre} onChange={(e) => update({ genre: e.target.value })} className="flex-1 sm:flex-none">
              <option value="">Tüm türler</option>
              {genres.map((g) => <option key={g.id} value={g.slug}>{g.name}</option>)}
            </NativeSelect>
            <Button type="submit">Ara</Button>
          </div>
        </form>
        <ErrorBox error={error} className="mb-6" />
        {!page && !error && (
          <div role="status" aria-label="Kitaplar yükleniyor" className={GRID}>
            {Array.from({ length: 8 }, (_, i) => (
              <div key={i} className="flex flex-col gap-3">
                <Skeleton className="aspect-[2/3] w-full" />
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            ))}
          </div>
        )}
        {page && (
          <>
            <p aria-live="polite" className="mb-4 text-sm text-muted-foreground">{page.total} kitap</p>
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
                {page.items.map((b) => <li key={b.id}><BookCard book={b} /></li>)}
              </ul>
            )}
            {(offset > 0 || page.total > PAGE) && (
              <Pager page={page} offset={offset} onChange={(o) => update({ offset: String(o) })} />
            )}
          </>
        )}
      </section>
      <aside aria-labelledby="top-heading" className="lg:sticky lg:top-24 lg:self-start">
        <Card className="p-5">
          <h2 id="top-heading" className="text-lg font-semibold">En iyiler</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            Basit ortalama değil, ağırlıklı (Bayes) puan: az oylu kitaplar genel ortalamaya
            {top ? ` (C = ${formatScore(top.c, 2)}, m = ${top.m})` : ""} doğru çekilir.
          </p>
          <ol className="mt-4 flex list-inside list-decimal flex-col gap-2 text-sm marker:font-serif marker:text-muted-foreground">
            {top?.items.map((b) => (
              <li key={b.id}>
                <Link to={`/books/${b.id}`} className="font-medium hover:underline">{b.title}</Link>
                <span className="text-xs text-muted-foreground">
                  {" "}— {formatScore(b.weighted_score, 2)} ({b.rating_count} oy, ort. {b.average_rating === null ? "—" : formatScore(b.average_rating)})
                </span>
              </li>
            ))}
          </ol>
          {top && top.items.length === 0 && <p className="mt-2 text-sm text-muted-foreground">Henüz puanlanmış kitap yok.</p>}
        </Card>
      </aside>
    </div>
  );
}
