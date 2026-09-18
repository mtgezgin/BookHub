import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import type { Book, Genre, Page, TopRated } from "../api/types";
import { BookCard } from "../components/BookCard";
import { ErrorBox } from "../components/ui";

const PAGE = 12;

export function Home() {
  const [params, setParams] = useSearchParams();
  const q = params.get("q") ?? "";
  const genre = params.get("genre") ?? "";
  // URL elle değiştirilebilir (/?offset=abc): geçersiz/negatif değer 0 sayılır (NaN API'ye gitmesin).
  const parsed = parseInt(params.get("offset") ?? "0", 10);
  const offset = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;

  const [page, setPage] = useState<Page<Book> | null>(null);
  const [genres, setGenres] = useState<Genre[]>([]);
  const [top, setTop] = useState<TopRated | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [search, setSearch] = useState(q);

  useEffect(() => {
    api.genres().then(setGenres).catch(setError);
    api.topRated(5).then(setTop).catch(setError);
  }, []);

  useEffect(() => {
    api.books({ q, genre, limit: PAGE, offset }).then(setPage).catch(setError);
  }, [q, genre, offset]);

  const update = (next: Record<string, string>) => {
    const p = new URLSearchParams(params);
    for (const [k, v] of Object.entries(next)) (v ? p.set(k, v) : p.delete(k));
    if (!("offset" in next)) p.delete("offset");
    setParams(p);
  };

  return (
    <div className="layout">
      <section>
        <form className="search" onSubmit={(e) => { e.preventDefault(); update({ q: search.trim() }); }}>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Başlık ya da yazar ara…" maxLength={100} />
          <select value={genre} onChange={(e) => update({ genre: e.target.value })}>
            <option value="">Tüm türler</option>
            {genres.map((g) => <option key={g.id} value={g.slug}>{g.name}</option>)}
          </select>
          <button type="submit">Ara</button>
        </form>
        <ErrorBox error={error} />
        {page && (
          <>
            <p className="muted">{page.total} kitap</p>
            <div className="grid">{page.items.map((b) => <BookCard key={b.id} book={b} />)}</div>
            {page.items.length === 0 && <p>Sonuç yok.</p>}
            <div className="pager">
              <button disabled={offset === 0} onClick={() => update({ offset: String(Math.max(0, offset - PAGE)) })}>← Önceki</button>
              <span className="muted">{page.total ? `${offset + 1}–${Math.min(offset + PAGE, page.total)}` : ""}</span>
              <button disabled={offset + PAGE >= page.total} onClick={() => update({ offset: String(offset + PAGE) })}>Sonraki →</button>
            </div>
          </>
        )}
      </section>
      <aside>
        <h2>En iyiler</h2>
        <p className="muted small">
          Basit ortalama değil, ağırlıklı (Bayes) puan: az oylu kitaplar genel ortalamaya
          {top ? ` (C = ${top.c.toFixed(2)}, m = ${top.m})` : ""} doğru çekilir.
        </p>
        <ol className="top">
          {top?.items.map((b) => (
            <li key={b.id}>
              <Link to={`/books/${b.id}`}>{b.title}</Link>
              <span className="muted small"> — {b.weighted_score.toFixed(2)} ({b.rating_count} oy, ort. {b.average_rating?.toFixed(2)})</span>
            </li>
          ))}
        </ol>
        {top && top.items.length === 0 && <p className="muted">Henüz puanlanmış kitap yok.</p>}
      </aside>
    </div>
  );
}
