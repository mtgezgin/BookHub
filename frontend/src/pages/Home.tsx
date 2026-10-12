import { Search, SearchX } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import type { Book, Genre, Page, TopRated } from "../api/types";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox, PageTitle } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager } from "../components/usePaged";
import { formatScore } from "../lib/format";
import { cn } from "../lib/utils";

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
  // Yalnız iki sıra var; tanınmayan değer varsayılana (en yeni) düşer.
  const sort = params.get("sort") === "rating" ? "rating" : "";
  // URL elle değiştirilebilir (/?offset=abc): geçersiz/negatif değer 0 sayılır (NaN API'ye gitmesin).
  const parsed = parseInt(params.get("offset") ?? "0", 10);
  const offset = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;

  const [page, setPage] = useState<Page<Book> | null>(null);
  const [genres, setGenres] = useState<Genre[]>([]);
  const [top, setTop] = useState<TopRated | null>(null);
  const [error, setError] = useState<unknown>(null);
  // Liste hatası ayrı: yeni bir liste isteği yalnız KENDİ eski hatasını temizler (tür/şerit
  // hatası kalır).
  const [listError, setListError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
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
    setListError(null);
    setLoading(true);
    // sort yalnız varsayılan değilken gider: istek adresi gereksiz yere uzamasın.
    api.books({ q, genre, limit: PAGE, offset, ...(sort && { sort }) })
      .then((p) => current && setPage(p))
      .catch((e) => current && setListError(e))
      .finally(() => current && setLoading(false));
    return () => {
      current = false;
    };
  }, [q, genre, offset, sort]);

  // Dikkat: react-router 7'de `prev`, setParams'ın OLUŞTUĞU render'daki URL'dir; en güncel URL
  // değil. Gecikmeli çağrılar bu yüzden aşağıdaki `latest` üzerinden son render'ın update'ini
  // kullanır.
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
  useEffect(() => {
    const trimmed = search.trim();
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

  const filtered = q !== "" || genre !== "";
  // Puan sırasında şerit, listenin başıyla aynı şeyi gösterirdi.
  const showTop = !filtered && sort === "" && offset === 0 && top !== null && top.items.length > 0;

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
          <Chip active={genre === ""} onClick={() => select({ genre: "" })}>Tümü</Chip>
          {genres.map((g) => (
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
                <Button size="sm" variant={sort === "" ? "default" : "ghost"} aria-pressed={sort === ""} onClick={() => select({ sort: "" })}>
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
