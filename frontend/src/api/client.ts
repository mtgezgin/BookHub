// API istemcisi: fetch sarmalayıcısı + token yönetimi.
//
// TOKEN SAKLAMA KARARI (ödünleşim, bilerek):
//   - access token → YALNIZCA bellekte (değişken). Sayfa yenilenince kaybolur; sorun değil,
//     refresh token'la 15 dk'lık yenisi alınır.
//   - refresh token → localStorage. Sayfa yenilemede oturum sürsün diye. Bedeli: bir XSS
//     açığı olursa saldırgan script bu değeri okuyabilir. Savunmalar: React çıktıyı varsayılan
//     olarak kaçışlar ve kodda dangerouslySetInnerHTML YOK; NGINX CSP başlığı (Faz 11).
//     Daha güçlü seçenek httpOnly + SameSite=Strict çerez (JS hiç göremez) — backend'in
//     refresh'i gövde yerine çerezden okuması gerekir; bilinen borç olarak not edildi.

import type {
  AdminBook, Book, BookStatus, Comment, Genre, MyRating, NewBook, Page,
  RatingSummary, Session, TopRated, User,
} from "./types";

const REFRESH_KEY = "bookhub.refresh";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

let accessToken: string | null = null;
let onSessionChange: (user: User | null) => void = () => {};

/** AuthContext, oturum değiştiğinde (giriş, yenileme, düşme) haberdar olmak için kaydolur. */
export function subscribeSession(fn: (user: User | null) => void) {
  onSessionChange = fn;
}

function storeSession(s: Session) {
  accessToken = s.access_token;
  localStorage.setItem(REFRESH_KEY, s.refresh_token);
  onSessionChange(s.user);
}

function clearSession() {
  accessToken = null;
  localStorage.removeItem(REFRESH_KEY);
  onSessionChange(null);
}

// TEK UÇUŞLU YENİLEME (single-flight): backend'de refresh token TEK KULLANIMLIK (her
// kullanımda döndürülür, eskisi silinir — Faz 3). Access token süresi dolduğunda sayfadaki
// 3 istek aynı anda 401 alırsa ve her biri ayrı /refresh yaparsa, İLKİ token'ı tüketir,
// diğer ikisi "geçersiz token" alır ve kullanıcı sebepsiz yere oturumdan düşer. Bu yüzden
// uçuştaki tek bir yenileme Promise'ini herkes paylaşır.
let refreshing: Promise<boolean> | null = null;

export function refreshSession(): Promise<boolean> {
  if (!refreshing) {
    refreshing = doRefresh().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}

// SEKMELER ARASI: yukarıdaki Promise yalnız BU sekmeyi korur. Aynı localStorage'ı paylaşan
// üç sekme aynı anda açılırsa üçü de aynı tek kullanımlık token'la /refresh yapar; biri
// kazanır, diğer ikisi 401 alır. Web Locks API ile yenilemeyi TÜM sekmelerde sıraya sokuyoruz:
// kilidi alan sekme token'ı localStorage'dan KİLİT İÇİNDE okur — önceki sekme döndürdüyse
// yenisini görür. (Kilit desteklenmiyorsa — eski tarayıcı, test ortamı — doğrudan çalışır.)
async function doRefresh(): Promise<boolean> {
  const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
  return locks ? locks.request("bookhub-refresh", refreshOnce) : refreshOnce();
}

async function refreshOnce(): Promise<boolean> {
  const token = localStorage.getItem(REFRESH_KEY);
  if (!token) return false;
  let res: Response;
  try {
    res = await fetch("/api/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: token }),
    });
  } catch {
    return false; // ağ hatası: token hâlâ geçerli olabilir, SİLME
  }
  if (res.status === 401) {
    // Yalnız "token geçersiz" oturumu bitirir — ve yalnız hâlâ BİZİM gönderdiğimiz token
    // saklıysa: arada başka bir sekme döndürdüyse onun yeni token'ını silmeyelim.
    if (localStorage.getItem(REFRESH_KEY) === token) clearSession();
    return false;
  }
  // 429 (NGINX sınırı), 502/5xx (API yeniden başlıyor): geçici — token'ı koru, sonra yeniden dene.
  if (!res.ok) return false;
  storeSession((await res.json()) as Session);
  return true;
}

async function toError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json();
    return new ApiError(res.status, body?.error?.code ?? "unknown", body?.error?.message ?? res.statusText);
  } catch {
    return new ApiError(res.status, "unknown", res.statusText);
  }
}

/**
 * request: JSON istek atar. Access token varsa ekler; 401 gelirse BİR KEZ yenileyip
 * yeniden dener (yenileme de başarısızsa hatayı olduğu gibi döner).
 */
export async function request<T>(method: string, path: string, body?: unknown, retry = true): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;

  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (res.status === 401 && retry && localStorage.getItem(REFRESH_KEY)) {
    if (await refreshSession()) return request<T>(method, path, body, false);
  }
  if (!res.ok) throw await toError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

function qs(params: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") p.set(k, String(v));
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}

export const api = {
  // Oturum
  async login(email: string, password: string): Promise<User> {
    const s = await request<Session>("POST", "/api/auth/login", { email, password }, false);
    storeSession(s);
    return s.user;
  },
  register: (email: string, display_name: string, password: string) =>
    request<User>("POST", "/api/auth/register", { email, display_name, password }, false),
  async logout(): Promise<void> {
    const token = localStorage.getItem(REFRESH_KEY);
    clearSession();
    if (token) await request<void>("POST", "/api/auth/logout", { refresh_token: token }, false).catch(() => {});
  },
  /** Sayfa açılışında: kayıtlı refresh token varsa oturumu geri getir. */
  async restore(): Promise<User | null> {
    if (!(await refreshSession())) return null;
    return request<User>("GET", "/api/auth/me");
  },

  // Katalog
  books: (p: { q?: string; genre?: string; limit?: number; offset?: number }) =>
    request<Page<Book>>("GET", `/api/books${qs(p)}`),
  book: (id: number) => request<Book>("GET", `/api/books/${id}`),
  topRated: (limit = 10) => request<TopRated>("GET", `/api/books/top-rated${qs({ limit })}`),
  genres: () => request<Genre[]>("GET", "/api/genres"),
  suggest: (b: NewBook) => request<Book & { status: BookStatus }>("POST", "/api/books/suggestions", b),

  // Yorum + puan
  comments: (id: number, offset = 0) => request<Page<Comment>>("GET", `/api/books/${id}/comments${qs({ offset })}`),
  addComment: (id: number, content: string) => request<Comment>("POST", `/api/books/${id}/comments`, { content }),
  ratingSummary: (id: number) => request<RatingSummary>("GET", `/api/books/${id}/ratings/summary`),
  myRating: (id: number) => request<MyRating>("GET", `/api/books/${id}/rating`),
  rate: (id: number, score: number) => request<{ score: number }>("PUT", `/api/books/${id}/rating`, { score }),
  removeRating: (id: number) => request<unknown>("DELETE", `/api/books/${id}/rating`),

  // Admin
  adminBooks: (status: BookStatus, offset = 0) =>
    request<Page<AdminBook>>("GET", `/api/admin/books${qs({ status, offset })}`),
  approve: (id: number) => request<AdminBook>("POST", `/api/admin/books/${id}/approve`),
  reject: (id: number, reason: string) => request<AdminBook>("POST", `/api/admin/books/${id}/reject`, { reason }),
  archive: (id: number) => request<AdminBook>("DELETE", `/api/admin/books/${id}`),
  restoreBook: (id: number) => request<AdminBook>("POST", `/api/admin/books/${id}/restore`),
  createGenre: (name: string) => request<Genre>("POST", "/api/admin/genres", { name }),
  deleteComment: (id: number) => request<void>("DELETE", `/api/admin/comments/${id}`),
};

/** Test yardımcısı: modül durumunu sıfırla. */
export function __resetForTests() {
  accessToken = null;
  refreshing = null;
  localStorage.clear();
}
