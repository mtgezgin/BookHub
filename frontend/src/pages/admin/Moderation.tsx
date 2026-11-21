import { Inbox } from "lucide-react";
import { useRef, useState } from "react";
import { type ModerationAction, useAdminBooks, useCreateGenre, useModerateBook } from "../../api/adminQueries";
import type { AdminBook, BookStatus } from "../../api/types";
import { ErrorBox, formatDate, PageTitle } from "../../components/common";
import { EmptyState } from "../../components/EmptyState";
import { Button } from "../../components/ui/button";
import { Card } from "../../components/ui/card";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "../../components/ui/dialog";
import { Input, Textarea } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { useToast } from "../../components/toast";
import { Pager, usePageView } from "../../components/usePaged";

const TABS: { status: BookStatus; label: string }[] = [
  { status: "PENDING", label: "Bekleyen" },
  { status: "APPROVED", label: "Yayında" },
  { status: "REJECTED", label: "Reddedilen" },
  { status: "ARCHIVED", label: "Arşiv" },
];

/** Admin moderasyon ekranı: statüye göre kuyruk + durum makinesindeki geçişler. */
export function Moderation() {
  const [status, setStatus] = useState<BookStatus>("PENDING");
  const [error, setError] = useState<unknown>(null);
  const [genre, setGenre] = useState("");
  // Reddedilecek kitap (diyalog açık) ve gerekçe. window.prompt yerine: erişilebilir, biçimlenebilir,
  // ve "Vazgeç" açıkça bir düğme (prompt'ta İptal = null'u ayırt etmek gerekiyordu).
  const [rejecting, setRejecting] = useState<AdminBook | null>(null);
  const [reason, setReason] = useState("");

  // Kuyruk bir sorgu (Faz 16d adım 6): sekme ve sayfa anahtarın parçası. Eskiden burada usePaged, tıklama
  // anındaki reload'u tutan bir ref ve onu güncelleyen bir etki vardı; işlemden sonra hangi listenin
  // yeniden çekileceğine artık useModerateBook karar veriyor (anahtara göre: ekrandaki sekme).
  const [offset, setOffset] = useState(0);
  const queue = useAdminBooks(status, offset);
  const { page, showPager } = usePageView(queue, offset, setOffset);
  const moderate = useModerateBook();
  const createGenre = useCreateGenre();

  // Başarıyla işlenmiş satırlar, işlemin BAŞLADIĞI sekmede pasif kalır: kitabın durumu değiştiği için
  // o sekmede bir daha işlem göremez. Anahtar "sekme:kitap": işlem sürerken sekme değişirse geç gelen
  // işaret yeni sekmeyi etkilemez. Sekme değişince temizlenir: arşivlenip geri yayınlanan kitap
  // Yayında'da yeniden işlem görebilir.
  const [settled, setSettled] = useState<ReadonlySet<string>>(new Set());
  const settledKey = (id: number) => `${status}:${id}`;
  // Sekme değişimi tek yerde: eskiden offset'i usePaged'in resetKey etkisi, işaretleri ayrı bir etki
  // sıfırlıyordu. Değişikliğe yol açan tıklamanın kendisinde yapmak iki etkiyi ve bir ara çizimi kaldırır.
  const switchTab = (next: BookStatus) => {
    setError(null);
    setStatus(next);
    setOffset(0);
    setSettled(new Set());
  };

  const toast = useToast();
  // Düğmeler işlem sürerken pasif (isPending; değişiklik ekrandaki liste yenilenene kadar sürer). Satır
  // düğmeleri ayrıca sekme değişiminde önceki sekmenin satırları yer tutucu olarak dururken de pasif.
  const busy = moderate.isPending || createGenre.isPending;
  const rowsLocked = busy || queue.isPlaceholderData;
  // isPending bir SONRAKİ çizimde görünür: aynı anda gelen iki tıklamanın ikincisi, düğme henüz pasif
  // değilken çalışabilir. Ref ikinci tıklamayı çizimden ÖNCE durdurur.
  const inFlight = useRef(false);
  const act = async (run: () => Promise<unknown>, done: string, key?: string) => {
    if (inFlight.current) return;
    inFlight.current = true;
    setError(null);
    try {
      await run();
      if (key !== undefined) setSettled((s) => new Set(s).add(key));
      toast(done);
    } catch (e) {
      setError(e); // ör. 409 invalid_transition: başka bir admin önce davrandı
    } finally {
      inFlight.current = false;
    }
  };
  const moderateBook = (a: ModerationAction, done: string) => act(() => moderate.mutateAsync(a), done, settledKey(a.id));

  return (
    <div className="flex flex-col gap-10">
      <PageTitle title="Moderasyon" />
      <section aria-labelledby="queue-heading">
        <header className="mb-4">
          <h1 id="queue-heading" className="text-3xl font-semibold">Moderasyon</h1>
          <p className="mt-1 text-sm text-muted-foreground">Önerileri onayla ya da reddet; yayındaki kitapları arşivle.</p>
        </header>
        <div role="group" aria-label="Durum" className="inline-flex flex-wrap gap-1 rounded-lg border bg-card p-1">
          {TABS.map((t) => (
            <Button
              key={t.status}
              size="sm"
              variant={t.status === status ? "default" : "ghost"}
              aria-pressed={t.status === status}
              onClick={() => switchTab(t.status)}
            >
              {t.label}
            </Button>
          ))}
        </div>
        <ErrorBox error={error ?? queue.error} className="mt-4" />
        {page && page.items.length === 0 && <EmptyState icon={Inbox} title="Bu listede kitap yok." className="mt-4" />}
        <ul aria-label="Moderasyon kuyruğu" className="mt-4 flex flex-col gap-3">
          {page?.items.map((b) => (
            <li key={b.id}>
              <Card className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="font-medium">{b.title}</div>
                  <div className="text-sm text-muted-foreground">{b.author} · {formatDate(b.created_at)}</div>
                  {b.rejection_reason && <p className="mt-1 text-sm text-muted-foreground">Red gerekçesi: {b.rejection_reason}</p>}
                </div>
                <div className="flex shrink-0 gap-2">
                  {b.status === "PENDING" && (
                    <>
                      <Button size="sm" disabled={rowsLocked || settled.has(settledKey(b.id))} onClick={() => void moderateBook({ kind: "approve", id: b.id }, `“${b.title}” yayına alındı`)}>Onayla</Button>
                      <Button size="sm" variant="destructive" disabled={rowsLocked || settled.has(settledKey(b.id))} onClick={() => { setReason(""); setRejecting(b); }}>Reddet</Button>
                    </>
                  )}
                  {b.status === "APPROVED" && <Button size="sm" variant="outline" disabled={rowsLocked || settled.has(settledKey(b.id))} onClick={() => void moderateBook({ kind: "archive", id: b.id }, `“${b.title}” arşivlendi`)}>Arşivle</Button>}
                  {b.status === "ARCHIVED" && <Button size="sm" variant="outline" disabled={rowsLocked || settled.has(settledKey(b.id))} onClick={() => void moderateBook({ kind: "restore", id: b.id }, `“${b.title}” yeniden yayında`)}>Geri yayınla</Button>}
                </div>
              </Card>
            </li>
          ))}
        </ul>
        {showPager && page && <Pager page={page} offset={offset} onChange={setOffset} />}
      </section>

      {/* REJECTED son durak (durum makinesinde çıkışı yok): karar yalnız "Reddet"e basınca verilir. */}
      <Dialog open={rejecting !== null} onOpenChange={(open) => !open && setRejecting(null)}>
        <DialogContent>
          <form
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (!rejecting) return;
              const target = rejecting;
              setRejecting(null);
              void moderateBook({ kind: "reject", id: target.id, reason }, `“${target.title}” reddedildi`);
            }}
          >
            <DialogTitle>“{rejecting?.title}” reddedilsin mi?</DialogTitle>
            <DialogDescription>Red son karardır, geri alınamaz: reddedilen kitap bir daha yayına alınamaz.</DialogDescription>
            <Label>Gerekçe (isteğe bağlı)<Textarea value={reason} onChange={(e) => setReason(e.target.value)} maxLength={500} /></Label>
            <div className="flex justify-end gap-2">
              <DialogClose asChild>
                <Button type="button" variant="outline">Vazgeç</Button>
              </DialogClose>
              <Button type="submit" variant="destructive" disabled={busy}>Reddet</Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <section aria-labelledby="genre-heading">
        <h2 id="genre-heading" className="text-xl font-semibold">Yeni tür</h2>
        <form aria-label="Yeni tür" className="mt-3 flex max-w-md gap-2" onSubmit={(e) => { e.preventDefault(); void act(async () => { await createGenre.mutateAsync(genre); setGenre(""); }, `“${genre.trim()}” türü eklendi`); }}>
          <Input aria-label="Tür adı" value={genre} onChange={(e) => setGenre(e.target.value)} placeholder="ör. Bilim Kurgu" required maxLength={100} />
          <Button type="submit" disabled={busy}>Ekle</Button>
        </form>
      </section>
    </div>
  );
}
