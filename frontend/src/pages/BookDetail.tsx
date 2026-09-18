import { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Book, Comment, Page, RatingSummary } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox, formatDate, Rating } from "../components/ui";

export function BookDetail() {
  const id = Number(useParams().id);
  const { user } = useAuth();
  const [book, setBook] = useState<Book | null>(null);
  const [summary, setSummary] = useState<RatingSummary | null>(null);
  const [comments, setComments] = useState<Page<Comment> | null>(null);
  const [commentOffset, setCommentOffset] = useState(0);
  const [mine, setMine] = useState<number | null>(null);
  const [text, setText] = useState("");
  const [error, setError] = useState<unknown>(null);

  // Hatalar YUTULMAZ: özet ya da yorumlar gelmezse boş ("henüz puan yok") gibi görünüp
  // yanıltmasın, hata kutusunda görünsün (review bulgusu R2).
  const reload = useCallback(() => {
    api.book(id).then(setBook).catch(setError);
    api.ratingSummary(id).then(setSummary).catch(setError);
    api.comments(id, commentOffset).then(setComments).catch(setError);
  }, [id, commentOffset]);

  useEffect(reload, [reload]);
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
      reload();
    } catch (e) {
      setError(e);
    }
  };

  const unrate = async () => {
    setError(null);
    try {
      await api.removeRating(id);
      setMine(null);
      reload();
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
      reload();
    } catch (err) {
      setError(err);
    }
  };

  const remove = async (commentId: number) => {
    await api.deleteComment(commentId).catch(setError);
    reload();
  };

  if (!book) return <ErrorBox error={error} />;
  const max = Math.max(1, ...(summary?.distribution.map((d) => d.votes) ?? [1]));

  return (
    <article className="detail">
      <header>
        <h1>{book.title}</h1>
        <p className="muted">
          {book.author}
          {book.publication_year ? ` · ${book.publication_year}` : ""}
          {book.page_count ? ` · ${book.page_count} sayfa` : ""} · {book.language}
        </p>
        <div className="tags">{book.genres.map((g) => <span key={g.id} className="tag">{g.name}</span>)}</div>
      </header>
      {/* İçerik React'te METİN olarak basılır → yorumdaki "<script>" çalışmaz, görünür. */}
      {book.description && <p className="description">{book.description}</p>}
      <ErrorBox error={error} />

      <section>
        <h2>Puan</h2>
        <Rating value={summary?.average_rating ?? null} count={summary?.rating_count ?? 0} />
        {summary && summary.rating_count > 0 && (
          <div className="histogram">
            {summary.distribution.map((d) => (
              <div key={d.score} className="bar" title={`${d.score}: ${d.votes} oy`}>
                <div style={{ height: `${(d.votes / max) * 100}%` }} />
                <span>{d.score}</span>
              </div>
            ))}
          </div>
        )}
        {user ? (
          <div className="score-picker">
            <span className="muted">Senin puanın:</span>
            {Array.from({ length: 10 }, (_, i) => i + 1).map((s) => (
              <button key={s} className={mine === s ? "active" : ""} onClick={() => rate(s)}>{s}</button>
            ))}
            {mine !== null && <button className="link" onClick={unrate}>kaldır</button>}
          </div>
        ) : (
          <p className="muted">Puan vermek için giriş yap.</p>
        )}
      </section>

      <section>
        <h2>Yorumlar {comments ? `(${comments.total})` : ""}</h2>
        {user && (
          <form onSubmit={post} className="comment-form">
            <textarea value={text} onChange={(e) => setText(e.target.value)} maxLength={2000} placeholder="Düşüncelerini yaz…" required />
            <button type="submit">Gönder</button>
          </form>
        )}
        <ul className="comments">
          {comments?.items.map((c) => (
            <li key={c.id}>
              <div className="muted small">
                <strong>{c.author?.display_name ?? "silinmiş kullanıcı"}</strong> · {formatDate(c.created_at)}
                {user?.role === "ADMIN" && <button className="link danger" onClick={() => remove(c.id)}>sil</button>}
              </div>
              <p>{c.content}</p>
            </li>
          ))}
        </ul>
        {comments && comments.total > comments.limit && (
          <div className="pager">
            <button disabled={commentOffset === 0} onClick={() => setCommentOffset(Math.max(0, commentOffset - comments.limit))}>← Yeniler</button>
            <span className="muted small">{commentOffset + 1}–{Math.min(commentOffset + comments.limit, comments.total)} / {comments.total}</span>
            <button disabled={commentOffset + comments.limit >= comments.total} onClick={() => setCommentOffset(commentOffset + comments.limit)}>Eskiler →</button>
          </div>
        )}
      </section>
    </article>
  );
}
