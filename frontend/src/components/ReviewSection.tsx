import { Star } from "lucide-react";
import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { ApiError } from "../api/client";
import { useModerateReview, useReviews } from "../api/reviewQueries";
import type { PublicReview } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { formatDate, initialOf, timeAgo } from "../lib/format";
import { docOf } from "../lib/reviewDoc";
import { ErrorBox } from "./common";
import { MyReview } from "./MyReview";
import { ReviewDoc } from "./ReviewDoc";
import { useToast } from "./toast";
import { Alert } from "./ui/alert";
import { Button } from "./ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Textarea } from "./ui/input";
import { Label } from "./ui/label";
import { Pager, usePageView } from "./usePaged";

/** Sunucudaki sınır (reviews.MaxReasonLen). */
const MAX_REASON = 500;

type Moderate = ReturnType<typeof useModerateReview>;

/**
 * Kitap sayfasındaki İncelemeler bölümü (Faz 17B), yorumların üstünde. Herkese açık liste en yeni önce;
 * giriş yapmış okurun kendi incelemesi listenin başında ayrı durur (MyReview) ve listede ikinci kez
 * görünmez. Yönetici her incelemeyi gerekçeyle gizleyebilir; gizlediği inceleme listeden kalkar, aynı
 * ziyarette "Geri aç" ile geri alınabilir (gizlenenler ekranı kapsam dışı, spec § Kapsam dışı).
 */
export function ReviewSection({ bookId }: Readonly<{ bookId: number }>) {
  const { user } = useAuth();
  const location = useLocation();
  const toast = useToast();
  const [offset, setOffset] = useState(0);
  const query = useReviews(bookId, offset);
  const view = usePageView(query, offset, setOffset);
  const moderate = useModerateReview(user?.id, bookId);
  const [hiding, setHiding] = useState<PublicReview | null>(null);
  const [justHidden, setJustHidden] = useState<PublicReview | null>(null);
  const isAdmin = user?.role === "ADMIN";
  const items = view.page?.items.filter((r) => r.author.id !== user?.id) ?? [];

  const unhide = (r: PublicReview) => {
    moderate.mutate({ id: r.id, action: "unhide" }, {
      onSuccess: () => {
        setJustHidden(null);
        toast("İnceleme yeniden yayında");
      },
      onError: (err) => toast(err instanceof ApiError ? err.message : "Geri açılamadı", "error"),
    });
  };

  return (
    <section aria-labelledby="reviews-heading" className="mt-10">
      <h2 id="reviews-heading" className="text-2xl font-semibold">İncelemeler {view.page ? `(${view.page.total})` : ""}</h2>
      {user ? (
        <MyReview userId={user.id} bookId={bookId} />
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">
          İnceleme yazmak için <Link to="/login" state={{ from: location }} className="font-medium text-primary underline-offset-4 hover:underline">giriş yap</Link>.
        </p>
      )}
      {justHidden && (
        <Alert role="status" className="mt-4 flex flex-wrap items-center gap-x-3">
          {justHidden.author.display_name} tarafından yazılan inceleme gizlendi.
          <Button variant="link" size="sm" className="h-auto p-0" disabled={moderate.isPending} onClick={() => unhide(justHidden)}>Geri aç</Button>
        </Alert>
      )}
      <ErrorBox error={query.error} className="mt-4" />
      {view.page?.total === 0 && <p className="mt-4 text-sm text-muted-foreground">Henüz inceleme yok.</p>}
      <ul aria-label="İncelemeler" className="mt-6 flex flex-col gap-6">
        {items.map((r) => (
          <li key={r.id}>
            <ReviewCard review={r} onHide={isAdmin ? () => setHiding(r) : undefined} />
          </li>
        ))}
      </ul>
      {view.showPager && view.page && (
        <Pager page={view.page} offset={offset} onChange={setOffset} prev="← Yeniler" next="Eskiler →" />
      )}
      <HideDialog key={hiding?.id} review={hiding} mutation={moderate} onClose={() => setHiding(null)} onHidden={setJustHidden} />
    </section>
  );
}

/**
 * Bir inceleme. Metni ve spoiler parçalarını ReviewDoc çizer: spoiler metni kullanıcı açana kadar DOM'da
 * HİÇ yok (CSS ile gizlemek ekran okuyucuya ve sayfa içi aramaya metni sızdırırdı).
 */
function ReviewCard({ review, onHide }: Readonly<{ review: PublicReview; onHide?: () => void }>) {
  return (
    <div className="flex gap-3">
      <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-accent font-serif font-semibold text-accent-foreground">
        {initialOf(review.author.display_name)}
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2 text-sm">
          <strong className="font-medium">{review.author.display_name}</strong>
          {review.author_score !== null && (
            <span className="inline-flex items-center gap-0.5 tabular-nums">
              <Star aria-hidden="true" className="size-3.5 fill-star text-star" />
              <span className="sr-only">puanı</span> {review.author_score}/10
            </span>
          )}
          <time dateTime={review.created_at} title={formatDate(review.created_at)} className="text-muted-foreground">{timeAgo(review.created_at)}</time>
          {review.edited_at && <span className="text-muted-foreground">· düzenlendi</span>}
          {onHide && (
            <Button variant="link" size="sm" className="ml-auto h-auto text-destructive" onClick={onHide}>gizle</Button>
          )}
        </div>
        <ReviewDoc doc={docOf(review)} />
      </div>
    </div>
  );
}

/** Yöneticinin gizleme diyaloğu. Gerekçe zorunlu: incelemenin sahibine gösterilir. */
function HideDialog({ review, mutation, onClose, onHidden }: Readonly<{
  review: PublicReview | null; mutation: Moderate; onClose: () => void; onHidden: (r: PublicReview) => void;
}>) {
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (review === null || mutation.isPending) return;
    const text = reason.trim();
    if (text === "") {
      setError("Gerekçe yaz; incelemenin sahibine gösterilir.");
      return;
    }
    mutation.mutate({ id: review.id, action: "hide", reason: text }, {
      onSuccess: () => {
        toast("İnceleme gizlendi");
        onHidden(review);
        onClose();
      },
      onError: (err) => setError(err instanceof ApiError ? err.message : "Gizlenemedi"),
    });
  }

  return (
    <Dialog open={review !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <form noValidate onSubmit={submit} className="flex flex-col gap-4">
          <DialogTitle>İnceleme gizlensin mi?</DialogTitle>
          <DialogDescription>Herkese açık listeden kalkar. Gerekçe incelemenin sahibine gösterilir.</DialogDescription>
          <Label>
            Gerekçe
            <Textarea value={reason} onChange={(e) => setReason(e.target.value)} maxLength={MAX_REASON} rows={3} aria-invalid={error !== null} />
          </Label>
          <span className="self-end text-xs text-muted-foreground tabular-nums">{reason.length}/{MAX_REASON}</span>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="submit" variant="destructive" disabled={mutation.isPending}>Gizle</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
