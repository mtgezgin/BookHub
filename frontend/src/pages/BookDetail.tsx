import { Star } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Book, RatingSummary } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { ErrorBox, PageTitle, Rating } from "../components/common";
import { LibraryButton } from "../components/LibraryButton";
import { NotFound } from "../components/NotFound";
import { useToast } from "../components/toast";
import { useLibraryStatuses } from "../components/useLibraryStatuses";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Textarea } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager, usePaged } from "../components/usePaged";
import { formatDate, initialOf, languageName, timeAgo } from "../lib/format";
import { cn } from "../lib/utils";

const SCORES = Array.from({ length: 10 }, (_, i) => i + 1);
const MAX_COMMENT = 2000;

/**
 * 10 yıldızlık puan seçici. Üstüne gelince/odaklanınca önizleme; seçili olan aria-pressed.
 * Her düğmenin adı puanın kendisi (ekran okuyucuya özel metin): "7, düğme, basılı".
 */
function StarPicker({ value, onPick }: { value: number | null; onPick: (score: number) => void }) {
  const [preview, setPreview] = useState<number | null>(null);
  const shown = preview ?? value ?? 0;
  return (
    <div role="group" aria-labelledby="my-score-label" className="flex max-w-xs justify-between" onMouseLeave={() => setPreview(null)}>
      {SCORES.map((s) => (
        <button
          key={s}
          type="button"
          aria-pressed={value === s}
          title={`${s} puan`}
          className="rounded-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          onMouseEnter={() => setPreview(s)}
          onFocus={() => setPreview(s)}
          onBlur={() => setPreview(null)}
          onClick={() => onPick(s)}
        >
          <Star aria-hidden="true" className={cn("size-5 transition-colors", s <= shown ? "fill-star text-star" : "text-muted-foreground")} />
          <span className="sr-only">{s}</span>
        </button>
      ))}
    </div>
  );
}

function DetailSkeleton() {
  return (
    <>
      <PageTitle title="Yükleniyor" />
      <div role="status" aria-label="Kitap yükleniyor" className="mx-auto flex max-w-5xl gap-6 md:gap-10">
        <Skeleton className="aspect-[2/3] w-26 shrink-0 sm:w-36 md:w-64" />
        <div className="flex flex-1 flex-col gap-3">
          <Skeleton className="h-9 w-2/3" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-4 w-1/3" />
        </div>
      </div>
    </>
  );
}

// Benzer kitaplardan aynı rotada başka bir kitaba geçince bileşen yeniden kurulur: önceki kitabın
// başlığı, puanı ve yorum taslağı yenisi yüklenirken ekranda kalmaz (o arada verilen puan
// görünmeyen kitaba gitmez) ve sayfa en üstten açılır.
export function BookDetail() {
  const { id } = useParams();
  return <BookDetailView key={id} />;
}

function BookDetailView() {
  const id = Number(useParams().id);
  const { user } = useAuth();
  const location = useLocation();
  const toast = useToast();
  const [book, setBook] = useState<Book | null>(null);
  const [summary, setSummary] = useState<RatingSummary | null>(null);
  const [mine, setMine] = useState<number | null>(null);
  const [text, setText] = useState("");
  const [posting, setPosting] = useState(false);
  const [similar, setSimilar] = useState<Book[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => window.scrollTo(0, 0), []);

  // Kitap + puan özeti ve yorumlar AYRI yüklenir: yorum sayfası değişince kitabı yeniden
  // çekmeye gerek yok. Hatalar yutulmaz; yeni bir yükleme eski hata mesajını temizler.
  const loadBook = useCallback(() => {
    setError(null);
    api.book(id).then(setBook).catch(setError);
    api.ratingSummary(id).then(setSummary).catch(setError);
  }, [id]);
  useEffect(loadBook, [loadBook]);

  // Aynı türden kitaplar: yardımcı içerik. Başarısız olursa hata GÖSTERİLMEZ, bölüm çizilmez.
  // Bağımlılık id + tür (nesne değil): puan verince kitap yeniden çekilir ama bu liste değişmez.
  const bookId = book?.id;
  const firstGenre = book?.genres[0]?.slug;
  useEffect(() => {
    setSimilar(null);
    if (bookId === undefined || !firstGenre) return;
    let current = true;
    api.books({ genre: firstGenre, limit: 7 })
      .then((p) => current && setSimilar(p.items.filter((b) => b.id !== bookId).slice(0, 6)))
      .catch(() => current && setSimilar(null));
    return () => {
      current = false;
    };
  }, [bookId, firstGenre]);
  const similarStatuses = useLibraryStatuses((similar ?? []).map((b) => b.id));

  const fetchComments = useCallback((offset: number) => api.comments(id, offset), [id]);
  const comments = usePaged(fetchComments, String(id), setError);

  useEffect(() => {
    if (!user) return setMine(null);
    api.myRating(id)
      .then((r) => setMine(r.score))
      .catch((e) => (e instanceof ApiError && e.code === "not_rated" ? setMine(null) : setError(e)));
  }, [id, user]);

  const rate = async (score: number) => {
    setError(null);
    try {
      await api.rate(id, score);
      setMine(score);
      toast(`Puanın kaydedildi: ${score}/10`);
      loadBook();
    } catch (e) {
      setError(e);
    }
  };

  const unrate = async () => {
    setError(null);
    try {
      await api.removeRating(id);
      setMine(null);
      toast("Puanın kaldırıldı");
      loadBook();
    } catch (e) {
      setError(e);
    }
  };

  const post = async (e: React.FormEvent) => {
    e.preventDefault();
    if (posting) return; // çift tıklama iki yorum yazmasın
    setError(null);
    setPosting(true);
    try {
      await api.addComment(id, text);
      setText("");
      toast("Yorumun eklendi");
      if (comments.offset === 0) void comments.reload();
      else comments.setOffset(0); // en yeni yorum ilk sayfada
    } catch (err) {
      setError(err);
    } finally {
      setPosting(false);
    }
  };

  const remove = async (commentId: number) => {
    try {
      await api.deleteComment(commentId);
      toast("Yorum silindi");
    } catch (e) {
      setError(e);
    }
    void comments.reload();
  };

  if (!book) {
    if (error instanceof ApiError && error.status === 404) {
      return <NotFound title="Kitap bulunamadı">Bu kitap katalogda yok ya da yayından kaldırılmış.</NotFound>;
    }
    return error ? <><PageTitle title="Kitap yüklenemedi" /><ErrorBox error={error} /></> : <DetailSkeleton />;
  }
  const max = Math.max(1, ...(summary?.distribution.map((d) => d.votes) ?? [1]));
  const meta = [
    book.author,
    book.publication_year,
    book.page_count ? `${book.page_count} sayfa` : null,
    languageName(book.language),
  ].filter(Boolean);

  return (
    <>
      <PageTitle title={book.title} />
      {/* Izgara alanları: mobilde küçük kapak + başlık yan yana, altında puan kartı ve gövde;
          md+'da solda kapak ve onun altında puan kartı, sağda başlık ve gövde. */}
      <article
        aria-labelledby="book-title"
        className="mx-auto grid max-w-5xl grid-cols-[6.5rem_minmax(0,1fr)] gap-x-5 gap-y-8 [grid-template-areas:'cover_header'_'rating_rating'_'body_body'] sm:grid-cols-[9rem_minmax(0,1fr)] md:grid-cols-[16rem_minmax(0,1fr)] md:grid-rows-[auto_auto_1fr] md:gap-x-10 md:[grid-template-areas:'cover_header'_'cover_body'_'rating_body']"
      >
        <BookCover title={book.title} author={book.author} src={book.cover_url} className="self-start [grid-area:cover]" />
        <header className="flex flex-col gap-3 self-end [grid-area:header] md:self-start md:pt-2">
          <h1 id="book-title" className="text-2xl font-semibold sm:text-4xl">{book.title}</h1>
          <p className="text-sm text-muted-foreground sm:text-base">{meta.join(" · ")}</p>
          {book.genres.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {book.genres.map((g) => (
                <Badge key={g.id} asChild>
                  <Link to={`/?genre=${g.slug}`}>{g.name}</Link>
                </Badge>
              ))}
            </div>
          )}
          <div>
            <LibraryButton bookId={book.id} />
          </div>
        </header>

        <section aria-labelledby="rating-heading" className="self-start [grid-area:rating]">
          <Card className="p-5">
            <h2 id="rating-heading" className="text-lg font-semibold">Puan</h2>
            <Rating value={summary?.average_rating ?? null} count={summary?.rating_count ?? 0} className="mt-1 text-base" />
            {summary && summary.rating_count > 0 && (
              <ul aria-label="Puan dağılımı" className="mt-4 flex h-20 items-end gap-1">
                {summary.distribution.map((d) => (
                  <li key={d.score} title={`${d.score}: ${d.votes} oy`} className="flex h-full flex-1 flex-col items-center gap-1">
                    <span className="sr-only">{d.score} puan: {d.votes} oy</span>
                    <div aria-hidden="true" className="flex w-full flex-1 items-end">
                      <div className="min-h-0.5 w-full rounded-t-sm bg-primary/80" style={{ height: `${(d.votes / max) * 100}%` }} />
                    </div>
                    <span aria-hidden="true" className="text-[0.7rem] text-muted-foreground tabular-nums">{d.score}</span>
                  </li>
                ))}
              </ul>
            )}
            {user ? (
              <div className="mt-5 flex flex-col gap-2 border-t pt-4">
                <div className="flex items-baseline justify-between">
                  <span id="my-score-label" className="text-sm text-muted-foreground">Senin puanın</span>
                  {mine !== null && <span className="text-sm font-semibold tabular-nums">{mine}/10</span>}
                </div>
                <StarPicker value={mine} onPick={rate} />
                {mine !== null && <Button variant="link" size="sm" className="self-start" onClick={unrate}>kaldır</Button>}
              </div>
            ) : (
              <p className="mt-4 border-t pt-4 text-sm text-muted-foreground">
                Puan vermek için <Link to="/login" state={{ from: location }} className="font-medium text-primary underline-offset-4 hover:underline">giriş yap</Link>.
              </p>
            )}
          </Card>
        </section>

        <div className="min-w-0 [grid-area:body]">
          {/* İçerik React'te METİN olarak basılır → yorumdaki "<script>" çalışmaz, görünür. */}
          {book.description && <p className="text-lg leading-relaxed whitespace-pre-wrap">{book.description}</p>}
          <ErrorBox error={error} className="mt-6" />

          <section aria-labelledby="comments-heading" className="mt-10">
            <h2 id="comments-heading" className="text-2xl font-semibold">Yorumlar {comments.page ? `(${comments.page.total})` : ""}</h2>
            {user && (
              <form aria-label="Yorum yaz" onSubmit={post} className="mt-4 flex flex-col gap-2">
                <Textarea aria-label="Yorumun" value={text} onChange={(e) => setText(e.target.value)} maxLength={MAX_COMMENT} placeholder="Düşüncelerini yaz…" required />
                <div className="flex items-center justify-between gap-4">
                  <span className="text-xs text-muted-foreground tabular-nums">{text.length}/{MAX_COMMENT}</span>
                  <Button type="submit" disabled={posting}>{posting ? "Gönderiliyor…" : "Gönder"}</Button>
                </div>
              </form>
            )}
            {comments.page?.total === 0 && (
              <p className="mt-4 text-sm text-muted-foreground">{user ? "Henüz yorum yok. İlk yorumu sen yaz." : "Henüz yorum yok."}</p>
            )}
            <ul aria-label="Yorumlar" className="mt-6 flex flex-col gap-6">
              {comments.page?.items.map((c) => (
                <li key={c.id} className="flex gap-3">
                  <span
                    aria-hidden="true"
                    className={cn(
                      "grid size-9 shrink-0 place-items-center rounded-full font-serif font-semibold",
                      c.author ? "bg-accent text-accent-foreground" : "bg-muted text-muted-foreground",
                    )}
                  >
                    {c.author ? initialOf(c.author.display_name) : "?"}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-x-2 text-sm">
                      <strong className="font-medium">{c.author?.display_name ?? "silinmiş kullanıcı"}</strong>
                      <time dateTime={c.created_at} title={formatDate(c.created_at)} className="text-muted-foreground">{timeAgo(c.created_at)}</time>
                      {user?.role === "ADMIN" && (
                        <Button variant="link" size="sm" className="ml-auto h-auto text-destructive" onClick={() => remove(c.id)}>sil</Button>
                      )}
                    </div>
                    <p className="mt-1 leading-relaxed whitespace-pre-wrap">{c.content}</p>
                  </div>
                </li>
              ))}
            </ul>
            {comments.showPager && comments.page && (
              <Pager page={comments.page} offset={comments.offset} onChange={comments.setOffset} prev="← Yeniler" next="Eskiler →" />
            )}
          </section>
        </div>
      </article>

      {similar && similar.length > 0 && (
        <section aria-labelledby="similar-heading" className="mx-auto mt-16 max-w-5xl border-t pt-10">
          <h2 id="similar-heading" className="text-2xl font-semibold">{book.genres[0].name} türünden başka kitaplar</h2>
          <ul className="mt-6 grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 lg:grid-cols-6">
            {similar.map((b) => <li key={b.id}><BookCard book={b} headingLevel="h3" status={similarStatuses[b.id]} /></li>)}
          </ul>
        </section>
      )}
    </>
  );
}
