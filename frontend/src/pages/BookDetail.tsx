import { Star } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { ApiError } from "../api/client";
import {
  useAddComment, useBook, useBooks, useComments, useDeleteComment, useMyRating, useRateBook, useRatingSummary,
} from "../api/queries";
import { useAuth } from "../auth/AuthContext";
import { BookCard } from "../components/BookCard";
import { BookCover } from "../components/BookCover";
import { EditionStrip, PrimaryEditionNote } from "../components/EditionStrip";
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
import { Pager, usePageView } from "../components/usePaged";
import { formatDate, initialOf, languageName, timeAgo } from "../lib/format";
import { cn } from "../lib/utils";

const SCORES = Array.from({ length: 10 }, (_, i) => i + 1);
const MAX_COMMENT = 2000;

/**
 * 10 yıldızlık puan seçici. Üstüne gelince/odaklanınca önizleme; seçili olan aria-pressed.
 * Her düğmenin adı puanın kendisi (ekran okuyucuya özel metin): "7, düğme, basılı".
 */
function StarPicker({ value, onPick, disabled }: Readonly<{ value: number | null; onPick: (score: number) => void; disabled: boolean }>) {
  const [preview, setPreview] = useState<number | null>(null);
  const shown = preview ?? value ?? 0;
  return (
    <div role="group" aria-labelledby="my-score-label" className="flex max-w-xs justify-between" onMouseLeave={() => setPreview(null)}>
      {SCORES.map((s) => (
        <button
          key={s}
          type="button"
          aria-pressed={value === s}
          disabled={disabled}
          title={`${s} puan`}
          className="rounded-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-60"
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
  const [text, setText] = useState("");
  // Yorum sayfası (offset) sayfanın durumu; o sayfanın VERİSİ sorgunun. Kitap değişince bileşen yeniden
  // kurulduğu için (BookDetail'deki key) offset kendiliğinden 0'dan başlar.
  const [offset, setOffset] = useState(0);

  // Süslü parantez şart: yeni Chromium'da scrollTo bir Promise döndürüyor; etki onu döndürseydi
  // React ayrılırken Promise'i temizlik fonksiyonu sanıp çağırır, uygulama boş ekranla çökerdi.
  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  // Sunucu verisi (Faz 16d adım 5; kancalar api/queries.ts'te). Eskiden burada dört useState, iki useEffect,
  // elle bir "current" bayrağı ve her puandan sonra kitabı yeniden yükleyen loadBook vardı. Kitap, özet ve
  // yorumlar yine AYRI sorgular: yorum sayfası değişince kitap yeniden çekilmez.
  const bookQuery = useBook(id);
  const summaryQuery = useRatingSummary(id);
  const myRating = useMyRating(user?.id, id);
  const rating = useRateBook(id, user?.id);
  const book = bookQuery.data;
  const summary = summaryQuery.data;
  const mine = user ? (myRating.data ?? null) : null;

  // Aynı türden kitaplar: yardımcı içerik. Başarısız olursa hata GÖSTERİLMEZ, bölüm çizilmez. Tür
  // bilinmeden sorgu çalışmaz (enabled); anahtar tür + limit olduğu için aynı türdeki başka bir kitaba
  // geçince liste önbellekten gelir.
  const firstGenre = book?.genres[0]?.slug;
  const similarQuery = useBooks({ genre: firstGenre, limit: 7 }, firstGenre !== undefined);
  const similar = similarQuery.data?.items.filter((b) => b.id !== id).slice(0, 6);
  const similarStatuses = useLibraryStatuses((similar ?? []).map((b) => b.id));

  const commentsQuery = useComments(id, offset);
  const comments = usePageView(commentsQuery, offset, setOffset);
  const addComment = useAddComment(id);
  const deleteComment = useDeleteComment(id);

  // Her değişikliğin hatası kendi kancasında durur (rating.error, addComment.error…). Eskiden tek bir
  // `error` durumu vardı ve yeni bir işlem onu temizliyordu; aynı davranış: yeni bir işleme başlarken
  // ÖTEKİ işlemlerin eski hatası silinir (reset). Yalnız hatalı olanlar: süren bir işlemi sıfırlamak onun
  // kilidini (isPending) de kaldırırdı.
  const clearActionErrors = () => {
    for (const m of [rating, addComment, deleteComment]) if (m.isError) m.reset();
  };

  // Puan ver (score) ya da kaldır (null). Bildirim sayfaya ait (kanca bildirimin metnini bilmemeli).
  const rate = (score: number | null) => {
    clearActionErrors();
    rating.mutate(score, {
      onSuccess: () => toast(score === null ? "Puanın kaldırıldı" : `Puanın kaydedildi: ${score}/10`),
    });
  };

  const post = (e: React.FormEvent) => {
    e.preventDefault();
    if (addComment.isPending) return; // çift gönderim iki yorum yazmasın (düğme de kilitli)
    clearActionErrors();
    addComment.mutate(text, {
      onSuccess: () => {
        setText("");
        toast("Yorumun eklendi");
        setOffset(0); // en yeni yorum ilk sayfada; o sayfa geçersiz kılındığı için yeniden çekilir
      },
    });
  };

  const remove = (commentId: number) => {
    clearActionErrors();
    deleteComment.mutate(commentId, { onSuccess: () => toast("Yorum silindi") });
  };

  // "Puan ver" daveti (raf kontrolü, bitirince): puan seçiciye kaydırır ve ilk yıldıza odaklanır. Puan
  // seçici bu sayfanın, o yüzden davet ne yapacağını bilmez; yalnız bunu çağırır.
  const focusRating = () => {
    const first = document.querySelector<HTMLButtonElement>('[aria-labelledby="my-score-label"] button');
    first?.scrollIntoView({ block: "center", behavior: "smooth" });
    first?.focus({ preventScroll: true });
  };

  if (!book) {
    const loadError = bookQuery.error;
    if (loadError instanceof ApiError && loadError.status === 404) {
      return <NotFound title="Kitap bulunamadı">Bu kitap katalogda yok ya da yayından kaldırılmış.</NotFound>;
    }
    return loadError ? <><PageTitle title="Kitap yüklenemedi" /><ErrorBox error={loadError} /></> : <DetailSkeleton />;
  }
  // Sayfadaki tek hata kutusu: önce işlemlerin hatası, sonra okumalarınki.
  const shownError = addComment.error ?? deleteComment.error ?? rating.error
    ?? commentsQuery.error ?? summaryQuery.error ?? myRating.error;
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
        className="mx-auto grid max-w-5xl grid-cols-[6.5rem_minmax(0,1fr)] gap-x-5 gap-y-8 [grid-template-areas:'cover_header'_'shelf_shelf'_'rating_rating'_'body_body'] sm:grid-cols-[9rem_minmax(0,1fr)] md:grid-cols-[16rem_minmax(0,1fr)] md:grid-rows-[auto_auto_auto_1fr] md:gap-x-10 md:[grid-template-areas:'cover_header'_'cover_shelf'_'cover_body'_'rating_body']"
      >
        <div className="self-start [grid-area:cover]">
          <BookCover title={book.title} author={book.author} src={book.cover_url} />
          {/* Kapak ve sayfa sayısı varsayılan baskıdan; meta satırındaki dil ise eserin özgün dili. */}
          <PrimaryEditionNote editions={book.editions} />
        </div>
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
        </header>

        {/* Raf kontrolü kendi alanında (Faz 17A-1): ilerleme, formlar ve geçmişle birlikte başlığa sığmıyor;
            dar ekranda kapak ve başlığın altında tam genişlik. */}
        <section aria-label="Kitaplığın" className="min-w-0 [grid-area:shelf]">
          <LibraryButton bookId={book.id} editions={book.editions} onRate={focusRating} />
        </section>

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
                {/* Kayıt sürerken kilitli: hızlı iki tık iki istek göndermez, sıraları karışmaz (F11). */}
                <StarPicker value={mine} onPick={rate} disabled={rating.isPending} />
                {mine !== null && <Button variant="link" size="sm" className="self-start" disabled={rating.isPending} onClick={() => rate(null)}>kaldır</Button>}
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
          <EditionStrip editions={book.editions} author={book.author} />
          <ErrorBox error={shownError} className="mt-6" />

          <section aria-labelledby="comments-heading" className="mt-10">
            <h2 id="comments-heading" className="text-2xl font-semibold">Yorumlar {comments.page ? `(${comments.page.total})` : ""}</h2>
            {user && (
              <form aria-label="Yorum yaz" onSubmit={post} className="mt-4 flex flex-col gap-2">
                <Textarea aria-label="Yorumun" value={text} onChange={(e) => setText(e.target.value)} maxLength={MAX_COMMENT} placeholder="Düşüncelerini yaz…" required />
                <div className="flex items-center justify-between gap-4">
                  <span className="text-xs text-muted-foreground tabular-nums">{text.length}/{MAX_COMMENT}</span>
                  <Button type="submit" disabled={addComment.isPending}>{addComment.isPending ? "Gönderiliyor…" : "Gönder"}</Button>
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
              <Pager page={comments.page} offset={offset} onChange={setOffset} prev="← Yeniler" next="Eskiler →" />
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
