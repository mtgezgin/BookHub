import { BookMarked, Search, SearchX } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Genre, LibraryItem, LibraryStatus, Page } from "../api/types";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox, PageTitle } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { useToast } from "../components/toast";
import { Button, buttonVariants } from "../components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager } from "../components/usePaged";
import { LIBRARY_STATUSES, libraryStatusLabel } from "../lib/format";
import { cn } from "../lib/utils";
import { Chip, GRID } from "./Home";

const PAGE = 12;
type Rated = "" | "yes" | "no";
type Sort = "" | "title" | "my_rating";

// Adres elle değiştirilebilir: tanınmayan değer filtre yokmuş gibi davranır (API'ye gitmez, 400 olmaz).
const asStatus = (v: string | null): LibraryStatus | "" => (LIBRARY_STATUSES.includes(v as LibraryStatus) ? (v as LibraryStatus) : "");
const asRated = (v: string | null): Rated => (v === "yes" || v === "no" ? v : "");
const asSort = (v: string | null): Sort => (v === "title" || v === "my_rating" ? v : "");

/**
 * Kitaplığım: kullanıcının kendi kitapları. Bütün filtre durumu ADRESTE (paylaşılabilir, yenilemede
 * kalır, geri/ileri ile korunur); filtre değişince ilk sayfaya dönülür. Arama Enter/"Ara" ile gider
 * (kişisel liste küçük; katalogdaki yazarken-arama yarışlarına gerek yok).
 */
export function Library() {
  const [params, setParams] = useSearchParams();
  const status = asStatus(params.get("status"));
  const q = params.get("q") ?? "";
  const genre = params.get("genre") ?? "";
  const rated = asRated(params.get("rated"));
  const sort = asSort(params.get("sort"));
  const parsed = parseInt(params.get("offset") ?? "0", 10);
  const offset = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;

  const toast = useToast();
  const [page, setPage] = useState<Page<LibraryItem> | null>(null);
  const [genres, setGenres] = useState<Genre[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState(q);
  const [removing, setRemoving] = useState<LibraryItem | null>(null);
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    api.genres().then(setGenres).catch(() => {});
  }, []);
  useEffect(() => setSearch(q), [q]);

  // Eski cevap koruması (Home ile aynı): filtre değişince geç gelen önceki cevap yok sayılır.
  useEffect(() => {
    let current = true;
    setError(null);
    setLoading(true);
    api.library({
      ...(status && { status }), ...(q && { q }), ...(genre && { genre }), ...(rated && { rated }), ...(sort && { sort }),
      limit: PAGE, offset,
    })
      .then((p) => current && setPage(p))
      .catch((e) => current && setError(e))
      .finally(() => current && setLoading(false));
    return () => {
      current = false;
    };
  }, [status, q, genre, rated, sort, offset, reload]);

  const update = useCallback(
    (next: Record<string, string>) =>
      setParams((prev) => {
        const p = new URLSearchParams(prev);
        for (const [k, v] of Object.entries(next)) (v ? p.set(k, v) : p.delete(k));
        if (!("offset" in next)) p.delete("offset");
        return p;
      }),
    [setParams],
  );
  // Zaten seçili değer yeniden seçilince bir şey olmaz (sayfa başa dönmez, geçmişe kayıt eklenmez).
  const select = (key: string, value: string) => {
    if ((params.get(key) ?? "") === value) return;
    update({ [key]: value });
  };

  const filtered = status !== "" || q !== "" || genre !== "" || rated !== "";

  async function remove(item: LibraryItem) {
    setBusy(true);
    try {
      await api.removeFromLibrary(item.book.id);
      setRemoving(null);
      toast(`“${item.book.title}” kitaplığından çıkarıldı`);
      // Son sayfadaki son kitap çıktıysa boş bir sayfada kalınmaz: bir önceki sayfaya dönülür.
      if (page && page.items.length === 1 && offset > 0) {
        update({ offset: offset - PAGE > 0 ? String(offset - PAGE) : "" });
      } else {
        setReload((n) => n + 1);
      }
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "Kitaplık güncellenemedi", "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <PageTitle title="Kitaplığım" />
      <section className="mb-8 flex flex-col gap-4">
        <h1 className="text-3xl font-semibold sm:text-4xl">Kitaplığım</h1>
        <div role="group" aria-label="Durum" className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
          <Chip active={status === ""} onClick={() => select("status", "")}>Tümü</Chip>
          {LIBRARY_STATUSES.map((s) => (
            <Chip key={s} active={status === s} onClick={() => select("status", s)}>{libraryStatusLabel(s)}</Chip>
          ))}
        </div>
        <form role="search" className="flex gap-2" onSubmit={(e) => { e.preventDefault(); if (search.trim() !== q) update({ q: search.trim() }); }}>
          <div className="relative flex-1">
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label="Kitaplığımda ara"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Başlık ya da yazar ara…"
              maxLength={100}
              className="h-11 pl-11 md:text-base"
            />
          </div>
          <Button type="submit" size="lg" className="h-11">Ara</Button>
        </form>
        {genres.length > 0 && (
          <div role="group" aria-label="Türler" className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
            <Chip active={genre === ""} onClick={() => select("genre", "")}>Tümü</Chip>
            {genres.map((g) => (
              <Chip key={g.id} active={genre === g.slug} onClick={() => select("genre", g.slug)}>{g.name}</Chip>
            ))}
          </div>
        )}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div role="group" aria-label="Puanım" className="inline-flex gap-1 rounded-lg border bg-card p-1">
            {([["", "Hepsi"], ["yes", "Puanladıklarım"], ["no", "Puanlamadıklarım"]] as const).map(([v, label]) => (
              <Button key={v} size="sm" variant={rated === v ? "default" : "ghost"} aria-pressed={rated === v} onClick={() => select("rated", v)}>
                {label}
              </Button>
            ))}
          </div>
          <div role="group" aria-label="Sıralama" className="inline-flex gap-1 rounded-lg border bg-card p-1">
            {([["", "Son eklenen"], ["title", "Başlık"], ["my_rating", "Puanıma göre"]] as const).map(([v, label]) => (
              <Button key={v} size="sm" variant={sort === v ? "default" : "ghost"} aria-pressed={sort === v} onClick={() => select("sort", v)}>
                {label}
              </Button>
            ))}
          </div>
          {page && <p aria-live="polite" className="text-sm text-muted-foreground">{page.total} kitap</p>}
        </div>
      </section>

      <ErrorBox error={error} className="mb-6" />

      {!page && !error && (
        <div role="status" aria-label="Kitaplık yükleniyor" className={GRID}>
          {Array.from({ length: PAGE }, (_, i) => (
            <div key={i} className="flex flex-col gap-3">
              <Skeleton className="aspect-[2/3] w-full" />
              <Skeleton className="h-4 w-3/4" />
            </div>
          ))}
        </div>
      )}

      {page && page.items.length === 0 && (
        filtered ? (
          <EmptyState
            icon={SearchX}
            title="Sonuç bulunamadı"
            action={<Button variant="outline" onClick={() => setParams(new URLSearchParams())}>Filtreleri temizle</Button>}
          >
            Bu filtrelerle kitaplığında kitap yok.
          </EmptyState>
        ) : (
          <EmptyState icon={BookMarked} title="Kitaplığın boş" action={<Link to="/" className={buttonVariants({ variant: "outline" })}>Kataloğa göz at</Link>}>
            Bir kitabın sayfasından onu kitaplığına ekleyebilirsin.
          </EmptyState>
        )
      )}

      {page && page.items.length > 0 && (
        <ul aria-label="Kitaplığım" aria-busy={loading} className={cn(GRID, "transition-opacity", loading && "opacity-60")}>
          {page.items.map((item) => (
            <li key={item.book.id}>
              {item.in_catalog ? (
                <BookCard
                  book={item.book}
                  headingLevel="h2"
                  status={item.status}
                  extra={item.my_rating !== null && <p className="text-sm font-medium">Puanım: {item.my_rating}</p>}
                />
              ) : (
                <article className="flex flex-col gap-3">
                  <BookCover title={item.book.title} author={item.book.author} src={item.book.cover_url} className="opacity-50 grayscale" />
                  <div className="flex flex-col gap-1">
                    <h2 className="line-clamp-2 text-base leading-snug font-semibold text-muted-foreground">{item.book.title}</h2>
                    <p className="text-sm text-muted-foreground">Katalogdan kaldırıldı</p>
                    <Button size="sm" variant="outline" className="mt-1 self-start" onClick={() => setRemoving(item)}>Çıkar</Button>
                  </div>
                </article>
              )}
            </li>
          ))}
        </ul>
      )}

      {page && (offset > 0 || page.total > PAGE) && (
        <Pager page={page} offset={offset} onChange={(o) => update({ offset: o > 0 ? String(o) : "" })} />
      )}

      <Dialog open={removing !== null} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent>
          <DialogTitle>“{removing?.book.title}” kitaplığından çıkarılsın mı?</DialogTitle>
          <DialogDescription>Puanın ve yorumların silinmez.</DialogDescription>
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="button" variant="destructive" disabled={busy} onClick={() => removing && void remove(removing)}>Çıkar</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
