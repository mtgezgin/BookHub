// API sözleşmesinin TypeScript karşılığı. Kaynak: backend/cmd/api/*.go içindeki
// *Response struct'ları (json etiketleri). Backend DTO'su değişirse burası da değişmeli.

export interface Genre {
  id: number;
  name: string;
  slug: string;
}

export interface Book {
  id: number;
  title: string;
  slug: string;
  author: string;
  description: string;
  cover_url: string;
  publication_year: number | null;
  page_count: number | null;
  language: string;
  genres: Genre[];
  average_rating: number | null; // null = henüz puan yok
  rating_count: number;
  created_at: string;
  updated_at: string;
}

/**
 * Kitabın bir baskısı (Faz 16): kitap eserdir, baskı elde tutulan somut kitap. Kitabın kapağı ve
 * sayfa sayısı varsayılan baskıdan (is_primary) gelir. Boş metin ('') = bilinmiyor.
 */
export interface Edition {
  id: number;
  title: string;
  language: string;
  translator: string;
  publisher: string;
  publication_year: number | null;
  isbn: string;
  page_count: number | null;
  cover_url: string;
  is_primary: boolean;
}

/** GET /api/books/{id}: liste öğesinin alanları + baskılar (yalnız detayda; varsayılan önce). */
export type BookDetail = Book & { editions: Edition[] };

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface TopRated {
  items: (Book & { weighted_score: number })[];
  m: number;
  c: number;
}

export type BookStatus = "PENDING" | "APPROVED" | "REJECTED" | "ARCHIVED";

export interface AdminBook extends Book {
  status: BookStatus;
  submitted_by: number | null;
  rejection_reason: string | null;
  moderated_by: number | null;
  moderated_at: string | null;
}

export interface User {
  id: number;
  email: string;
  display_name: string;
  role: "USER" | "ADMIN";
  created_at: string;
}

// Session'da refresh_token YOK (Faz 13): uzun ömürlü sır yalnızca httpOnly çerezde taşınıyor
// ve JS onu hiç göremiyor. refresh_expires_at duruyor — o bir sır değil, sadece "oturum en
// geç ne zaman biter" bilgisi.
export interface Session {
  access_token: string;
  token_type: "Bearer";
  access_expires_at: string;
  refresh_expires_at: string;
  user: User;
}

export interface Comment {
  id: number;
  book_id: number;
  author: { id: number; display_name: string } | null;
  content: string;
  created_at: string;
}

export interface RatingSummary {
  average_rating: number | null;
  rating_count: number;
  distribution: { score: number; votes: number }[];
}

export interface MyRating {
  score: number;
  created_at: string;
  updated_at: string;
}

/** Kişisel kitaplık durumu (backend/internal/library); arayüzdeki adı: lib/format.ts. */
export type LibraryStatus = "WANT_TO_READ" | "READING" | "READ" | "ABANDONED";

/**
 * Kitaplığım öğesi: kitap + kişisel bilgiler. in_catalog=false → kitap yayından kaldırılmış.
 * unit/progress/total_pages yalnız READING'de dolu (Faz 17): başka durumda sunucu null döner, eski bir
 * okumadan kalan bayat değer kartta görünmesin.
 */
export interface LibraryItem {
  book: Book;
  status: LibraryStatus;
  added_at: string;
  my_rating: number | null;
  in_catalog: boolean;
  unit: ProgressUnit | null;
  progress: number | null;
  total_pages: number | null;
}

/*
 * RAF (Faz 17A-1): bir kitabın kullanıcının kitaplığındaki hâli. Kaynak: backend/cmd/api/shelf.go.
 * Günler (`started_on`, `ended_on`) saatsiz takvim günüdür, "YYYY-AA-GG": `new Date("2026-11-12")` UTC
 * gece yarısı olarak okunur ve batıdaki saat dilimlerinde bir önceki güne kayar; okumak ve yazmak için
 * lib/day.ts.
 */

/** İlerleme birimi; tek birim, dönüşüm yok (spec § Kararlar "İlerleme birimi"). */
export type ProgressUnit = "page" | "percent";

/**
 * Süren okuma. unit ve progress null = ilerleme henüz girilmedi (Faz 15'ten kalan "Şu an okuyorum");
 * total_pages null = sayfa toplamı bilinmiyor ya da birim yüzde.
 */
export interface Reading {
  edition_id: number | null;
  unit: ProgressUnit | null;
  progress: number | null;
  total_pages: number | null;
  started_on: string | null;
}

export interface ShelfEntry {
  status: LibraryStatus;
  /** Okurken düzeltmede (updateReading) geri gönderilir; eskiyse 409 version_conflict. */
  version: number;
  added_at: string;
  updated_at: string;
  /** Yalnız READING'de dolu. */
  reading: Reading | null;
}

/** Kapanmış bir okuma. ended_on null = "Zaten okudum", tarih bilinmiyor (sonradan eklenebilir). */
export interface ReadingLogEntry {
  id: number;
  edition_id: number | null;
  outcome: "FINISHED" | "ABANDONED";
  unit: ProgressUnit | null;
  progress: number | null;
  total_pages: number | null;
  started_on: string | null;
  ended_on: string | null;
  created_at: string;
}

/** Her raf ucunun cevabı. entry null = kitaplıkta değil; log en yeni önce ve o zaman da dolu olabilir. */
export interface Shelf {
  book_id: number;
  entry: ShelfEntry | null;
  log: ReadingLogEntry[];
}

/**
 * Komut ve GÖVDESİ birlikte: sunucu her komutta yalnız o komutun alanlarını kabul eder (fazlası 400),
 * tip de aynısını derleme anında söyler. Gövde her komutta isteğe bağlı: yoksa varsayılanlar (tek tık).
 */
export type ShelfCommand =
  | { command: "start"; body?: { mode?: "resume" | "restart"; edition_id?: number; unit?: ProgressUnit; started_on?: string } }
  | { command: "finish" | "abandon"; body?: { ended_on?: string } }
  | { command: "want" | "postpone" | "mark-read" | "remove" };

/** PATCH …/shelf/reading: version zorunlu; gelmeyen alan değişmez. Birim ya da baskı değişirse ilerleme sıfırlanır. */
export interface ReadingPatch {
  version: number;
  progress?: number;
  started_on?: string;
  unit?: ProgressUnit;
  edition_id?: number;
}

/** PATCH /api/me/reading-log/{id}: gelmeyen gün değişmez. */
export interface ReadingLogDates {
  started_on?: string;
  ended_on?: string;
}

export interface NewBook {
  title: string;
  author: string;
  slug?: string;
  description?: string;
  // cover_url YOK: API öneride kapak adresini reddeder (F37); kapağı onaydan sonra admin ekler.
  publication_year?: number;
  page_count?: number;
  language?: string;
}
