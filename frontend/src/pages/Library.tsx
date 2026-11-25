import { BookMarked, Search, SearchX } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { ApiError } from "../api/client";
import { useLibrary, useRemoveFromLibrary } from "../api/libraryQueries";
import { useGenres } from "../api/queries";
import type { LibraryItem, LibraryStatus } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox, PageTitle } from "../components/common";
import { EmptyState } from "../components/EmptyState";
import { ReadingProgress } from "../components/ReadingProgress";
import { asGenre, asOffset, asQuery, Chip, GRID, useParamUpdater } from "../components/filters";
import { useToast } from "../components/toast";
import { Button, buttonVariants } from "../components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager, usePageView } from "../components/usePaged";
import { LIBRARY_STATUSES, libraryStatusLabel } from "../lib/format";
import { cn } from "../lib/utils";

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
  const q = asQuery(params.get("q"));
  const genre = asGenre(params.get("genre"));
  const rated = asRated(params.get("rated"));
  const sort = asSort(params.get("sort"));
  const offset = asOffset(params.get("offset"));

  const toast = useToast();
  const { user } = useAuth();
  const [search, setSearch] = useState(q);
  const [removing, setRemoving] = useState<LibraryItem | null>(null);

  const update = useParamUpdater(setParams);

  // Türler Home ile AYNI önbellek kaydından gelir (api/queries.ts useGenres): Home'dan buraya geçince istek
  // ikinci kez gitmez. Hata bilerek gösterilmez (eskiden de öyleydi): türler yalnız süzgeç çipleri, kitaplık
  // onlarsız da kullanılabilir.
  const genres = useGenres().data ?? [];
  useEffect(() => {
    setSearch(q);
  }, [q]);

  // Liste bir sorgu (Faz 16d adım 7): filtreler ve sayfa anahtarın parçası, eski cevap koruması oradan
  // gelir. Eskiden burada bir etki, geç gelen cevap bayrağı, page/error/loading durumları ve çıkarmadan
  // sonra listeyi yeniden yükletmek için bir sayaç (reload) vardı.
  const library = useLibrary(user?.id, {
    ...(status && { status }), ...(q && { q }), ...(genre && { genre }), ...(rated && { rated }), ...(sort && { sort }),
    limit: PAGE, offset,
  });
  // Sayfa boşaldıysa (ör. son kitap kendi sayfasından çıkarıldı, sonra "geri"; ya da burada çıkarıldı):
  // usePageView son GERÇEK sayfaya götürür. replace: geri tuşu yine bu boş adrese düşmesin.
  const toOffset = useCallback((o: number) => update({ offset: o > 0 ? String(o) : "" }, true), [update]);
  const { page, showPager } = usePageView(library, offset, toOffset);
  const loading = library.isPlaceholderData;
  const removal = useRemoveFromLibrary(user?.id);
  // Zaten seçili değer yeniden seçilince bir şey olmaz (sayfa başa dönmez, geçmişe kayıt eklenmez).
  const select = (key: string, value: string) => {
    if ((params.get(key) ?? "") === value) return;
    update({ [key]: value });
  };

  const filtered = status !== "" || q !== "" || genre !== "" || rated !== "";

  // Çıkarınca liste kendiliğinden yeniden çekilir (değişiklik kitaplık kayıtlarını geçersiz kılar). Son
  // sayfadaki son kitap çıktıysa yeni cevap boş gelir ve usePageView önceki sayfaya döner; eskiden bu
  // durum burada ayrıca ele alınıyordu.
  function remove(item: LibraryItem) {
    removal.mutate(item.book.id, {
      onSuccess: () => {
        setRemoving(null);
        toast(`“${item.book.title}” kitaplığından çıkarıldı`);
      },
      onError: (e) => toast(e instanceof ApiError ? e.message : "Kitaplık güncellenemedi", "error"),
    });
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
          <div role="group" aria-label="Puanım" className="inline-flex max-w-full gap-1 overflow-x-auto rounded-lg border bg-card p-1">
            {([["", "Hepsi"], ["yes", "Puanladıklarım"], ["no", "Puanlamadıklarım"]] as const).map(([v, label]) => (
              <Button key={v} size="sm" className="shrink-0" variant={rated === v ? "default" : "ghost"} aria-pressed={rated === v} onClick={() => select("rated", v)}>
                {label}
              </Button>
            ))}
          </div>
          <div role="group" aria-label="Sıralama" className="inline-flex max-w-full gap-1 overflow-x-auto rounded-lg border bg-card p-1">
            {([["", "Son eklenen"], ["title", "Başlık"], ["my_rating", "Puanıma göre"]] as const).map(([v, label]) => (
              <Button key={v} size="sm" className="shrink-0" variant={sort === v ? "default" : "ghost"} aria-pressed={sort === v} onClick={() => select("sort", v)}>
                {label}
              </Button>
            ))}
          </div>
          {page && <p aria-live="polite" className="text-sm text-muted-foreground">{page.total} kitap</p>}
        </div>
      </section>

      <ErrorBox error={library.error} className="mb-6" />

      {!page && !library.error && (
        <div role="status" aria-label="Kitaplık yükleniyor" className={GRID}>
          {Array.from({ length: PAGE }, (_, i) => (
            <div key={i} className="flex flex-col gap-3">
              <Skeleton className="aspect-[2/3] w-full" />
              <Skeleton className="h-4 w-3/4" />
            </div>
          ))}
        </div>
      )}

      {page && page.total === 0 && (
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
                  extra={
                    <>
                      {/* Faz 17A-1: okunan kitapta ilerleme (yalnız gösterim; komutlar kitap sayfasında). Sunucu
                          bu alanları yalnız READING'de doldurur; girilmemişse hiçbir şey çizilmez. */}
                      {item.status === "READING" && (
                        <ReadingProgress reading={item} label={`${item.book.title}: okuma ilerlemen`} className="mt-1" />
                      )}
                      {item.my_rating !== null && <p className="text-sm font-medium">Puanım: {item.my_rating}</p>}
                    </>
                  }
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

      {page && showPager && (
        <Pager page={page} offset={offset} onChange={(o) => update({ offset: o > 0 ? String(o) : "" })} />
      )}

      <Dialog open={removing !== null} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent>
          <DialogTitle>“{removing?.book.title}” kitaplığından çıkarılsın mı?</DialogTitle>
          <DialogDescription>Puanın, yorumların ve okuma geçmişin silinmez.</DialogDescription>
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="button" variant="destructive" disabled={removal.isPending} onClick={() => removing && remove(removing)}>Çıkar</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
