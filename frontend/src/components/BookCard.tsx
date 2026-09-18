import { Link } from "react-router-dom";
import type { Book } from "../api/types";
import { Rating } from "./ui";

export function BookCard({ book, extra }: { book: Book; extra?: React.ReactNode }) {
  return (
    <article className="book-card">
      {/* cover_url backend'de yalnız http(s) olarak kabul ediliyor (javascript: XSS'i yok). */}
      {book.cover_url ? <img src={book.cover_url} alt="" className="cover" /> : <div className="cover placeholder">📖</div>}
      <div>
        <h3>
          <Link to={`/books/${book.id}`}>{book.title}</Link>
        </h3>
        <p className="muted">{book.author}{book.publication_year ? ` · ${book.publication_year}` : ""}</p>
        <div className="tags">
          {book.genres.map((g) => (
            <Link key={g.id} className="tag" to={`/?genre=${g.slug}`}>{g.name}</Link>
          ))}
        </div>
        <Rating value={book.average_rating} count={book.rating_count} />
        {extra}
      </div>
    </article>
  );
}
