import { Link } from "react-router-dom";
import type { Book } from "../api/types";
import { BookCover } from "./BookCover";
import { Rating } from "./common";
import { Badge } from "./ui/badge";

/**
 * Katalog kartı. Bütün kart tıklanır: başlık bağlantısının ::after'ı kartı kaplıyor ("stretched
 * link"). Kartı <a> ile sarmak yerine böyle: içerideki tür bağlantıları iç içe <a> olmadan
 * çalışır — onlar `relative z-10` ile o katmanın ÜSTÜNDE durur.
 */
export function BookCard({ book, extra }: { book: Book; extra?: React.ReactNode }) {
  return (
    <article className="group relative flex flex-col gap-3">
      <BookCover
        title={book.title}
        author={book.author}
        src={book.cover_url}
        className="transition duration-200 group-hover:-translate-y-1 group-hover:shadow-lg"
      />
      <div className="flex flex-col gap-1">
        <h2 className="line-clamp-2 text-base leading-snug font-semibold">
          <Link
            to={`/books/${book.id}`}
            className="outline-none group-hover:underline after:absolute after:inset-0 after:rounded-lg focus-visible:after:ring-[3px] focus-visible:after:ring-ring/50"
          >
            {book.title}
          </Link>
        </h2>
        <p className="text-sm text-muted-foreground">
          {book.author}
          {book.publication_year ? ` · ${book.publication_year}` : ""}
        </p>
        <Rating value={book.average_rating} count={book.rating_count} />
        {book.genres.length > 0 && (
          <div className="relative z-10 mt-1 flex flex-wrap gap-1">
            {book.genres.map((g) => (
              <Badge key={g.id} asChild>
                <Link to={`/?genre=${g.slug}`}>{g.name}</Link>
              </Badge>
            ))}
          </div>
        )}
        {extra}
      </div>
    </article>
  );
}
