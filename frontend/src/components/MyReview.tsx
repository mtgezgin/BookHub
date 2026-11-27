import { lazy, Suspense, useId, useState } from "react";
import { ApiError } from "../api/client";
import { useDeleteReview, useMyReview, useSaveReview } from "../api/reviewQueries";
import type { Review } from "../api/types";
import { formatDate } from "../lib/format";
import { type DocNode, docLength, docOf, docProblem, emptyDoc, MAX_REVIEW } from "../lib/reviewDoc";
import { ErrorBox } from "./common";
import { ReviewDoc } from "./ReviewDoc";
import { useToast } from "./toast";
import { Alert } from "./ui/alert";
import { Button } from "./ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";

// Düzenleyici ayrı parça (Tiptap, gzip ~100 kB): yalnız yazmak isteyen okur indirir (spec karar 8).
const ReviewRichEditor = lazy(() => import("./review-editor/ReviewRichEditor"));

/**
 * Okurun bu kitaptaki incelemesi (Faz 17B): yoksa "İnceleme yaz", varsa metni, düzenle ve sil. Gizlenmişse
 * gerekçesi burada görünür; herkese açık listede yoktur. Düzenlenen inceleme gizli kalır (REV-01).
 */
export function MyReview({ userId, bookId }: Readonly<{ userId: number; bookId: number }>) {
  const mineQuery = useMyReview(userId, bookId);
  const save = useSaveReview(userId, bookId);
  const [editing, setEditing] = useState(false);

  if (mineQuery.isPending) return null;
  if (mineQuery.isError) return <ErrorBox error={mineQuery.error} className="mt-4" />;
  const mine = mineQuery.data;
  if (editing) return <ReviewEditor mine={mine} mutation={save} onDone={() => setEditing(false)} />;
  if (mine === null) {
    return <Button variant="outline" className="mt-4" onClick={() => setEditing(true)}>İnceleme yaz</Button>;
  }
  return <MyReviewCard review={mine} userId={userId} bookId={bookId} onEdit={() => setEditing(true)} />;
}

function MyReviewCard({ review, userId, bookId, onEdit }: Readonly<{
  review: Review; userId: number; bookId: number; onEdit: () => void;
}>) {
  const remove = useDeleteReview(userId, bookId);
  const toast = useToast();
  const [confirming, setConfirming] = useState(false);

  const confirmDelete = () => {
    if (remove.isPending) return;
    remove.mutate(undefined, {
      onSuccess: () => toast("İncelemen silindi; puanın duruyor"),
      // Başka bir sekmede zaten silinmiş: sonuç aynı, kanca ekranı güncelledi; hata göstermeye gerek yok.
      onError: (err) => {
        if (!(err instanceof ApiError && err.code === "no_review")) toast(err instanceof ApiError ? err.message : "Silinemedi", "error");
      },
      onSettled: () => setConfirming(false),
    });
  };

  return (
    <section aria-labelledby="my-review-heading" className="mt-4 flex flex-col gap-3 rounded-lg border p-4">
      <h3 id="my-review-heading" className="text-sm font-medium text-muted-foreground">Senin incelemen</h3>
      {review.hidden && (
        <Alert variant="destructive" role="status">
          Bu inceleme yönetici tarafından gizlendi: {review.hidden.reason}. Başkaları göremiyor.
        </Alert>
      )}
      <ReviewDoc doc={docOf(review)} mode="author" />
      <p className="text-xs text-muted-foreground">
        {formatDate(review.created_at)}
        {review.edited_at && " · düzenlendi"}
        {review.has_spoiler && " · spoiler içeriyor olarak işaretli"}
      </p>
      <div className="flex gap-2">
        <Button variant="secondary" size="sm" onClick={onEdit}>Düzenle</Button>
        <Button variant="outline" size="sm" onClick={() => setConfirming(true)}>Sil</Button>
      </div>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogTitle>İncelemen silinsin mi?</DialogTitle>
          <DialogDescription>Puanın silinmez.</DialogDescription>
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="button" variant="destructive" disabled={remove.isPending} onClick={confirmDelete}>Sil</Button>
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}

/**
 * Yazma ve düzenleme. Gönderilen sürüm o anki incelemeden (`mine.version`; ilk yazımda yok).
 *
 * 409 akışı (raf formlarıyla aynı): kanca incelemeyi yeniden okur ve hatayı ancak sonra yayar; bileşen yeni
 * hâlle yeniden çizilir. Form açık kalır, yazılan metin korunur, üstte ne olduğu söylenir: başka bir yerde
 * değişmişse şimdiki metin, silinmişse "silinmiş". "Benimkini kaydet" aynı metni GÜNCEL sürümle gönderir
 * (silinmişse sürümsüz, yeni inceleme olarak). Sessiz yeniden deneme yok.
 *
 * Faz 17C: metin kutusu yerine zengin metin düzenleyicisi; gönderilen şey belgedir (`doc`). "Spoiler
 * içeriyor" kutusu yok: okur spoiler olan kısmı seçip araç çubuğundan işaretler. Sayaç belgenin düz
 * metnini sunucunun saydığı gibi sayar (kod noktası; paragraf arası bir karakter).
 */
function ReviewEditor({ mine, mutation, onDone }: Readonly<{
  mine: Review | null; mutation: ReturnType<typeof useSaveReview>; onDone: () => void;
}>) {
  const toast = useToast();
  // Açılıştaki belge bir kez hesaplanır: 409'dan sonra `mine` değişse de yazılan metin yerinde kalır.
  const [initial] = useState<DocNode>(() => (mine ? docOf(mine) : emptyDoc()));
  const [doc, setDoc] = useState(initial);
  const labelId = useId();
  const length = docLength(doc);
  const [conflict, setConflict] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (mutation.isPending) return;
    const problem = docProblem(doc);
    if (problem !== null) {
      setError(problem);
      return;
    }
    setError(null);
    mutation.mutate({ doc, version: mine?.version }, {
      onSuccess: (saved) => {
        toast(saved.hidden ? "İncelemen kaydedildi; gizli kalmaya devam ediyor" : "İncelemen yayında");
        onDone();
      },
      onError: (err) => {
        if (err instanceof ApiError && err.code === "version_conflict") setConflict(true);
        else setError(err instanceof ApiError ? err.message : "Kaydedilemedi");
      },
    });
  }

  return (
    <form aria-label="İncelemeni yaz" noValidate onSubmit={submit} className="mt-4 flex flex-col gap-3 rounded-lg border p-4">
      {conflict && <ConflictNote mine={mine} />}
      <div className="flex flex-col gap-2">
        <span id={labelId} className="text-sm font-medium">İncelemen</span>
        <Suspense fallback={<p role="status" className="min-h-48 text-sm text-muted-foreground">Düzenleyici yükleniyor…</p>}>
          <ReviewRichEditor initial={initial} onChange={setDoc} labelledBy={labelId} invalid={error !== null} />
        </Suspense>
      </div>
      <p className="flex flex-wrap justify-between gap-x-4 text-xs text-muted-foreground">
        <span>Spoiler olan kısmı seçip "Spoiler"a bas; okuyan yalnız o kısmı açmadan görmez.</span>
        <span className={length > MAX_REVIEW ? "font-medium text-destructive tabular-nums" : "tabular-nums"}>{length}/{MAX_REVIEW}</span>
      </p>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onDone}>Vazgeç</Button>
        <Button type="submit" disabled={mutation.isPending}>
          {saveLabel(conflict, mutation.isPending)}
        </Button>
      </div>
    </form>
  );
}

// Sonar: iç içe üçlü yerine küçük bir fonksiyon.
function saveLabel(conflict: boolean, pending: boolean): string {
  if (pending) return "Kaydediliyor…";
  return conflict ? "Benimkini kaydet" : "Kaydet";
}

function ConflictNote({ mine }: Readonly<{ mine: Review | null }>) {
  if (mine === null) {
    return (
      <p role="alert" className="text-sm">
        İncelemen başka bir yerde silinmiş. Metnin aşağıda duruyor; kaydedersen yeniden yayımlanır.
      </p>
    );
  }
  return (
    <div role="alert" className="flex flex-col gap-1 text-sm">
      <p>İncelemen başka bir yerde değişmiş. Şimdiki hâli:</p>
      <div className="border-l-2 pl-3 text-muted-foreground"><ReviewDoc doc={docOf(mine)} mode="author" /></div>
      <p>Metnin aşağıda duruyor; kaydedersen onun yerine geçer.</p>
    </div>
  );
}
