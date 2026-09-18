import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import type { AdminBook, BookStatus, Page } from "../../api/types";
import { ErrorBox, formatDate } from "../../components/ui";

const TABS: { status: BookStatus; label: string }[] = [
  { status: "PENDING", label: "Bekleyen" },
  { status: "APPROVED", label: "Yayında" },
  { status: "REJECTED", label: "Reddedilen" },
  { status: "ARCHIVED", label: "Arşiv" },
];

/** Admin moderasyon ekranı: statüye göre kuyruk + durum makinesindeki geçişler. */
export function Moderation() {
  const [status, setStatus] = useState<BookStatus>("PENDING");
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<AdminBook> | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [genre, setGenre] = useState("");

  const load = useCallback(() => {
    api.adminBooks(status, offset).then(setPage).catch(setError);
  }, [status, offset]);
  useEffect(load, [load]);

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
    <div>
      <h1>Moderasyon</h1>
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t.status} className={t.status === status ? "active" : ""} onClick={() => { setStatus(t.status); setOffset(0); }}>{t.label}</button>
        ))}
      </div>
      <ErrorBox error={error} />
      {page && page.items.length === 0 && <p className="muted">Bu listede kitap yok.</p>}
      <ul className="queue">
        {page?.items.map((b) => (
          <li key={b.id} className="card">
            <strong>{b.title}</strong> <span className="muted">— {b.author} · {formatDate(b.created_at)}</span>
            {b.rejection_reason && <p className="muted small">Red gerekçesi: {b.rejection_reason}</p>}
            <div className="actions">
              {b.status === "PENDING" && (
                <>
                  <button onClick={() => act(() => api.approve(b.id))}>Onayla</button>
                  <button className="danger" onClick={() => {
                    // İptal = null → HİÇBİR ŞEY yapma. REJECTED son durak (durum makinesinde
                    // çıkışı yok); "vazgeç" tıklaması kalıcı bir karara dönüşmemeli.
                    const reason = window.prompt("Red gerekçesi (opsiyonel):");
                    if (reason === null) return;
                    act(() => api.reject(b.id, reason));
                  }}>Reddet</button>
                </>
              )}
              {b.status === "APPROVED" && <button onClick={() => act(() => api.archive(b.id))}>Arşivle</button>}
              {b.status === "ARCHIVED" && <button onClick={() => act(() => api.restoreBook(b.id))}>Geri yayınla</button>}
            </div>
          </li>
        ))}
      </ul>
      {page && page.total > page.limit && (
        <div className="pager">
          <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - page.limit))}>← Önceki</button>
          <span className="muted">{offset + 1}–{Math.min(offset + page.limit, page.total)} / {page.total}</span>
          <button disabled={offset + page.limit >= page.total} onClick={() => setOffset(offset + page.limit)}>Sonraki →</button>
        </div>
      )}

      <h2>Yeni tür</h2>
      <form className="search" onSubmit={(e) => { e.preventDefault(); act(async () => { await api.createGenre(genre); setGenre(""); }); }}>
        <input value={genre} onChange={(e) => setGenre(e.target.value)} placeholder="ör. Bilim Kurgu" required maxLength={100} />
        <button type="submit">Ekle</button>
      </form>
    </div>
  );
}
