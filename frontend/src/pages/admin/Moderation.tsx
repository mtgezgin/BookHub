import { Inbox } from "lucide-react";
import { useCallback, useState } from "react";
import { api } from "../../api/client";
import type { BookStatus } from "../../api/types";
import { ErrorBox, formatDate } from "../../components/common";
import { EmptyState } from "../../components/EmptyState";
import { Button } from "../../components/ui/button";
import { Card } from "../../components/ui/card";
import { Input } from "../../components/ui/input";
import { Pager, usePaged } from "../../components/usePaged";

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

  const fetchPage = useCallback((offset: number) => api.adminBooks(status, offset), [status]);
  const { page, offset, setOffset, reload: load, showPager } = usePaged(fetchPage, status, setError);

  const act = async (fn: () => Promise<unknown>) => {
    setError(null);
    try {
      await fn();
      load();
    } catch (e) {
      setError(e); // ör. 409 invalid_transition: başka bir admin önce davrandı
    }
  };

  return (
    <div className="flex flex-col gap-10">
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
              onClick={() => { setError(null); setStatus(t.status); }}
            >
              {t.label}
            </Button>
          ))}
        </div>
        <ErrorBox error={error} className="mt-4" />
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
                      <Button size="sm" onClick={() => act(() => api.approve(b.id))}>Onayla</Button>
                      <Button size="sm" variant="destructive" onClick={() => {
                        // İptal = null → HİÇBİR ŞEY yapma. REJECTED son durak (durum makinesinde
                        // çıkışı yok); "vazgeç" tıklaması kalıcı bir karara dönüşmemeli.
                        const reason = window.prompt("Red gerekçesi (opsiyonel):");
                        if (reason === null) return;
                        act(() => api.reject(b.id, reason));
                      }}>Reddet</Button>
                    </>
                  )}
                  {b.status === "APPROVED" && <Button size="sm" variant="outline" onClick={() => act(() => api.archive(b.id))}>Arşivle</Button>}
                  {b.status === "ARCHIVED" && <Button size="sm" variant="outline" onClick={() => act(() => api.restoreBook(b.id))}>Geri yayınla</Button>}
                </div>
              </Card>
            </li>
          ))}
        </ul>
        {showPager && page && <Pager page={page} offset={offset} onChange={setOffset} />}
      </section>

      <section aria-labelledby="genre-heading">
        <h2 id="genre-heading" className="text-xl font-semibold">Yeni tür</h2>
        <form aria-label="Yeni tür" className="mt-3 flex max-w-md gap-2" onSubmit={(e) => { e.preventDefault(); act(async () => { await api.createGenre(genre); setGenre(""); }); }}>
          <Input aria-label="Tür adı" value={genre} onChange={(e) => setGenre(e.target.value)} placeholder="ör. Bilim Kurgu" required maxLength={100} />
          <Button type="submit">Ekle</Button>
        </form>
      </section>
    </div>
  );
}
