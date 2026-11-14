import type { Edition } from "../api/types";
import { languageName } from "../lib/format";
import { BookCover } from "./BookCover";

/**
 * "Diğer baskılar" (Faz 16): kitabın varsayılan baskısı DIŞINDAKİ baskıları yatay bir şeritte
 * gösterir. Varsayılan baskı zaten sayfanın ana kapağı; kitabın tek baskısı varsa şerit hiç çizilmez.
 *
 * Kapak dekoratif (BookCover aria-hidden); ekran okuyucu her öğeyi metinden okur: baskının adı
 * (görsel olarak gizli; kapakta zaten yazıyor), dil, yıl ve varsa çevirmen.
 */
export function EditionStrip({ editions, author }: { editions: Edition[]; author: string }) {
  const others = editions.filter((e) => !e.is_primary);
  if (others.length === 0) return null;
  return (
    <section aria-labelledby="editions-heading" className="mt-10">
      <h2 id="editions-heading" className="text-2xl font-semibold">Diğer baskılar</h2>
      {/* Yatay kaydırma: dar ekranda sığmayan öğeler kaybolmaz. Liste klavyeyle odaklanabilir
          (tabIndex): içinde odaklanacak bağlantı yok, ok tuşlarıyla kaydırmanın tek yolu bu. */}
      <ul
        aria-labelledby="editions-heading"
        tabIndex={0}
        className="-mx-1 mt-4 flex gap-4 overflow-x-auto px-1 pb-3 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        {others.map((e) => {
          const line = [languageName(e.language), e.publication_year].filter(Boolean).join(" · ");
          return (
            <li key={e.id} className="w-24 shrink-0 sm:w-28">
              <BookCover title={e.title} author={author} src={e.cover_url} />
              <p className="mt-2 text-sm leading-snug">
                <span className="sr-only">{e.title}, </span>
                {line}
              </p>
              {e.translator && <p className="text-xs leading-snug text-muted-foreground">çev. {e.translator}</p>}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/**
 * Ana kapağın altındaki tek satır: sayfadaki kapak ve sayfa sayısının hangi baskıdan geldiğini söyler
 * ("Türkçe baskı · Yediveren Yayınları"). Yalnız kitabın birden çok baskısı varsa: tek baskıda bilgi
 * taşımaz, kalabalık yapar.
 */
export function PrimaryEditionNote({ editions }: { editions: Edition[] }) {
  const primary = editions.find((e) => e.is_primary);
  if (!primary || editions.length < 2) return null;
  const text = [`${languageName(primary.language)} baskı`, primary.publisher].filter(Boolean).join(" · ");
  return <p className="mt-2 text-xs leading-snug text-muted-foreground">{text}</p>;
}
