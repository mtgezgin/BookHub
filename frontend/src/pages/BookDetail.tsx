import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Book, RatingSummary } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { BookCover } from "../components/BookCover";
import { ErrorBox, formatDate, Rating } from "../components/common";
import { NotFound } from "../components/NotFound";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Textarea } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Pager, usePaged } from "../components/usePaged";
import { languageName } from "../lib/format";

function DetailSkeleton() {
  return (
    <div role="status" aria-label="Kitap yükleniyor" className="mx-auto flex max-w-3xl flex-col gap-6 sm:flex-row">
      <Skeleton className="aspect-[2/3] w-32 shrink-0 sm:w-40" />
      <div className="flex flex-1 flex-col gap-3">
        <Skeleton className="h-9 w-2/3" />
        <Skeleton className="h-4 w-1/2" />
        <Skeleton className="h-4 w-1/3" />
      </div>
    </div>
  );
}

export function BookDetail() {
  const id = Number(useParams().id);
  const { user } = useAuth();
  const [book, setBook] = useState<Book | null>(null);
  const [summary, setSummary] = useState<RatingSummary | null>(null);
  const [mine, setMine] = useState<number | null>(null);
  const [text, setText] = useState("");
  const [error, setError] = useState<unknown>(null);

  // Kitap + puan özeti ve yorumlar AYRI yüklenir: yorum sayfası değişince kitabı yeniden
  // çekmeye gerek yok. Hatalar yutulmaz; yeni bir yükleme eski hata mesajını temizler.
  const loadBook = useCallback(() => {
    setError(null);
    api.book(id).then(setBook).catch(setError);
    api.ratingSummary(id).then(setSummary).catch(setError);
  }, [id]);
  useEffect(loadBook, [loadBook]);

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
      loadBook();
    } catch (e) {
      setError(e);
    }
  };

  const post = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await api.addComment(id, text);
      setText("");
      if (comments.offset === 0) comments.reload();
      else comments.setOffset(0); // en yeni yorum ilk sayfada
    } catch (err) {
      setError(err);
    }
  };

  const remove = async (commentId: number) => {
    await api.deleteComment(commentId).catch(setError);
    comments.reload();
  };

  if (!book) {
    if (error instanceof ApiError && error.status === 404) {
      return <NotFound title="Kitap bulunamadı">Bu kitap katalogda yok ya da yayından kaldırılmış.</NotFound>;
    }
    return error ? <ErrorBox error={error} /> : <DetailSkeleton />;
  }
  const max = Math.max(1, ...(summary?.distribution.map((d) => d.votes) ?? [1]));
  const meta = [
    book.author,
    book.publication_year,
    book.page_count ? `${book.page_count} sayfa` : null,
    languageName(book.language),
  ].filter(Boolean);

  return (
    <article aria-labelledby="book-title" className="mx-auto max-w-3xl">
      <header className="flex flex-col gap-6 sm:flex-row sm:items-end">
        <BookCover title={book.title} author={book.author} src={book.cover_url} className="w-32 shrink-0 sm:w-40" />
        <div className="flex flex-col gap-3">
          <h1 id="book-title" className="text-3xl font-semibold sm:text-4xl">{book.title}</h1>
          <p className="text-muted-foreground">{meta.join(" · ")}</p>
          {book.genres.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {book.genres.map((g) => (
                <Badge key={g.id} asChild>
                  <Link to={`/?genre=${g.slug}`}>{g.name}</Link>
                </Badge>
              ))}
            </div>
          )}
        </div>
      </header>
      {/* İçerik React'te METİN olarak basılır → yorumdaki "<script>" çalışmaz, görünür. */}
      {book.description && <p className="mt-8 leading-relaxed whitespace-pre-wrap">{book.description}</p>}
      <ErrorBox error={error} className="mt-6" />

      <section aria-labelledby="rating-heading" className="mt-10">
        <h2 id="rating-heading" className="text-xl font-semibold">Puan</h2>
        <Card className="mt-3 p-5">
          <Rating value={summary?.average_rating ?? null} count={summary?.rating_count ?? 0} className="text-base" />
          {summary && summary.rating_count > 0 && (
            <ul aria-label="Puan dağılımı" className="mt-4 flex h-24 max-w-sm items-end gap-1">
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
            <div className="mt-5 flex flex-wrap items-center gap-2">
              <span id="my-score-label" className="text-sm text-muted-foreground">Senin puanın:</span>
              <div role="group" aria-labelledby="my-score-label" className="flex flex-wrap gap-1">
                {Array.from({ length: 10 }, (_, i) => i + 1).map((s) => (
                  <Button
                    key={s}
                    size="sm"
                    variant={mine === s ? "default" : "outline"}
                    aria-pressed={mine === s}
                    className="w-9 px-0 tabular-nums"
                    onClick={() => rate(s)}
                  >
                    {s}
                  </Button>
                ))}
              </div>
              {mine !== null && <Button variant="link" size="sm" onClick={unrate}>kaldır</Button>}
            </div>
          ) : (
            <p className="mt-4 text-sm text-muted-foreground">
              Puan vermek için <Link to="/login" className="font-medium text-primary underline-offset-4 hover:underline">giriş yap</Link>.
            </p>
          )}
        </Card>
      </section>

      <section aria-labelledby="comments-heading" className="mt-10">
        <h2 id="comments-heading" className="text-xl font-semibold">Yorumlar {comments.page ? `(${comments.page.total})` : ""}</h2>
        {user && (
          <form aria-label="Yorum yaz" onSubmit={post} className="mt-4 flex flex-col gap-2">
            <Textarea aria-label="Yorumun" value={text} onChange={(e) => setText(e.target.value)} maxLength={2000} placeholder="Düşüncelerini yaz…" required />
            <div className="flex justify-end">
              <Button type="submit">Gönder</Button>
            </div>
          </form>
        )}
        {comments.page?.total === 0 && (
          <p className="mt-4 text-sm text-muted-foreground">{user ? "Henüz yorum yok. İlk yorumu sen yaz." : "Henüz yorum yok."}</p>
        )}
        <ul aria-label="Yorumlar" className="mt-4 flex flex-col divide-y">
          {comments.page?.items.map((c) => (
            <li key={c.id} className="py-4">
              <div className="flex items-center gap-1.5 text-sm">
                <strong className="font-medium">{c.author?.display_name ?? "silinmiş kullanıcı"}</strong>
                <span className="text-muted-foreground">· {formatDate(c.created_at)}</span>
                {user?.role === "ADMIN" && (
                  <Button variant="link" size="sm" className="ml-auto text-destructive" onClick={() => remove(c.id)}>sil</Button>
                )}
              </div>
              <p className="mt-1 leading-relaxed whitespace-pre-wrap">{c.content}</p>
            </li>
          ))}
        </ul>
        {comments.showPager && comments.page && (
          <Pager page={comments.page} offset={comments.offset} onChange={comments.setOffset} prev="← Yeniler" next="Eskiler →" />
        )}
      </section>
    </article>
  );
}
