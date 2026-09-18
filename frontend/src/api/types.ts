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

export interface Session {
  access_token: string;
  token_type: "Bearer";
  access_expires_at: string;
  refresh_token: string;
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

export interface NewBook {
  title: string;
  author: string;
  slug?: string;
  description?: string;
  cover_url?: string;
  publication_year?: number;
  page_count?: number;
  language?: string;
}
