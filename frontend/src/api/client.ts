// API istemcisi: fetch sarmalayıcısı + oturum durum makinesi.
//
// TOKEN SAKLAMA KARARI (Faz 13'te DEĞİŞTİ — ödünleşim, bilerek):
//   - refresh token → httpOnly ÇEREZ. Bu dosya onu hiç görmez: ne okur, ne yazar, ne yollar.
//     Tarayıcı çerezi kendiliğinden /api/auth isteklerine ekler (Path=/api/auth,
//     SameSite=Strict, Secure). Eskiden localStorage'daydı; bir XSS açığında saldırgan
//     script'i 7 günlük sırrı okuyup dışarı sızdırabilirdi. Artık okuyamaz.
//   - access token → YALNIZCA bellekte (değişken). Sayfa yenilenince kaybolur; sorun değil,
//     çerezle 15 dk'lık yenisi alınır.
//   - localStorage'da SIR YOK. Yalnızca bir İPUCU (hint) bayrağı: "bu tarayıcıda bir oturum
//     açık olmalı". Sır değil — bilinmesi kimseye bir şey kazandırmaz — ama üç işi yapar:
//     (1) anonim ziyaretçi her açılışta boşuna bir /refresh isteği atmasın,
//     (2) bir sekmedeki çıkış diğer sekmelere "storage" olayıyla duyurulsun,
//     (3) uçuştaki bir yenileme, arada çıkış yapıldığını anlayıp oturumu DİRİLTMESİN.
//
// KALAN RİSK (dürüst olmak gerekirse): httpOnly çerez XSS'i çözmez. Enjekte edilmiş bir
// script hâlâ /api/auth/refresh'e POST atıp 15 dk'lık bir access token alabilir. Çerez
// yalnızca UZUN ÖMÜRLÜ sırrın saldırganın sunucusuna KOPYALANMASINI engeller.

import type {
  AdminBook, Book, BookStatus, Comment, Genre, MyRating, NewBook, Page,
  RatingSummary, Session, TopRated, User,
} from "./types";

/** HINT: "bu tarayıcıda oturum açık olmalı" ipucu. Sır DEĞİL (bkz. dosya başı). */
const HINT_KEY = "bookhub.session";
/** LOGOUT_PENDING: sunucuya ulaşamamış bir çıkış var; ilk fırsatta yeniden denenecek. */
const LOGOUT_PENDING_KEY = "bookhub.logout_pending";
/** LEGACY: Faz 13 öncesi refresh token'ının anahtarı. Yalnızca SİLİNMEK için duruyor. */
const LEGACY_REFRESH_KEY = "bookhub.refresh";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

// localStorage YASAKLI olabilir: Safari'nin özel penceresi, "siteler veri saklayamasın"
// ayarı ya da üçüncü taraf bağlamı SADECE ERİŞMEKTE bile fırlatır. Sarmalamasaydık tek bir
// getItem çağrısı modül yüklenirken patlayıp uygulamayı BOŞ EKRANA çevirirdi. Bayrak
// okunamazsa "oturum yok" varsayıyoruz: kullanıcı yalnızca tekrar giriş yapar.
function hasFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) !== null;
  } catch {
    return false;
  }
}

function setFlag(key: string) {
  try {
    localStorage.setItem(key, "1");
  } catch {
    /* saklanamıyorsa oturum yalnız bu sekmede, bellekte yaşar */
  }
}

function clearFlag(key: string) {
  try {
    localStorage.removeItem(key);
  } catch {
    /* yok sayılır */
  }
}

let accessToken: string | null = null;
let onSessionChange: (user: User | null) => void = () => {};

/** AuthContext, oturum değiştiğinde (giriş, yenileme, düşme) haberdar olmak için kaydolur. */
export function subscribeSession(fn: (user: User | null) => void) {
  onSessionChange = fn;
}

// setSession, HINT'i YAZMAZ. Yalnızca giriş (api.login) "bu tarayıcıda oturum açıldı" diyebilir.
// NEDEN: bir yenileme, çıkış yapılmış bir oturumu yeniden canlandırabilirdi — çıkış kilidi
// bırakırken HINT'i siler, arkasında bekleyen yenileme cevabı gelir ve HINT'i geri yazarsa
// kullanıcı "çıkış yaptım" dediği hâlde girişli kalırdı.
function setSession(s: Session) {
  accessToken = s.access_token;
  onSessionChange(s.user);
}

// Oturum "dönemi": her çıkış/düşme onu artırır. Uçuştaki bir yenileme, başladığından beri
// dönem değiştiyse (kullanıcı bu arada çıkış yaptı) sonucunu YAZMAZ — yoksa çıkıştan sonra
// gelen yenileme cevabı oturumu geri diriltirdi (review bulgusu R4).
//
// BİLİNEN BOŞLUK (bilerek, şimdilik): dönem yalnız HINT SİLİNDİĞİNDE artar. Başka bir sekme
// ÇIKIŞ YAPMADAN yeni bir giriş yaparsa (HINT zaten "1" iken tekrar "1" yazılır) tarayıcı
// "storage" olayını hiç üretmez (HTML spec: eski değer yenisine eşitse olay yok) — yani bu
// sekmedeki uçuşta bir yenileme, artık BAŞKA bir oturuma ait olduğunu anlayamaz. Kapatmanın
// yolu HINT'te "1" yerine rastgele bir OTURUM KİMLİĞİ tutup fetch öncesi/sonrası
// karşılaştırmak; bu dosyadaki her HINT okumasını etkilediği için ayrı bir değişiklik.
let sessionEpoch = 0;

function dropLocalSession() {
  sessionEpoch++;
  accessToken = null;
  onSessionChange(null);
}

function clearSession() {
  clearFlag(HINT_KEY);
  dropLocalSession();
}

// Başka bir SEKME çıkış yaptı (HINT silindi): bu sekme de oturumu bıraksın. "storage" olayı
// yalnız DİĞER sekmelerin yaptığı değişiklikte tetiklenir. (Sunucudaki çerez zaten o çıkışla
// öldü; bu sekme bilmezse bellekteki access token'la 15 dk daha "girişli" görünürdü.)
if (typeof window !== "undefined") {
  window.addEventListener("storage", (e) => {
    if (e.key === HINT_KEY && e.newValue === null) dropLocalSession();
  });
}

async function withRefreshLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
  if (!locks) return fn();
  // locks.request'in tipi callback'in dönüşünü Promise içinde Promise olarak görüyor; await ile düzleştir.
  return (await locks.request("bookhub-refresh", fn)) as T;
}

// TEK UÇUŞLU YENİLEME (single-flight): backend'de refresh token TEK KULLANIMLIK (her
// kullanımda döndürülür, eskisi ölür) ve kullanılmış bir token'ın tekrar gelmesi artık TÜM
// AİLEYİ iptal ettiriyor (Faz 13 reuse tespiti). Access token süresi dolduğunda sayfadaki 3
// istek aynı anda 401 alıp ayrı ayrı /refresh yapsaydı ilki token'ı tüketir, diğerleri
// "reuse" sayılır ve kullanıcı sebepsiz yere TÜM oturumundan düşerdi.
let refreshing: Promise<boolean> | null = null;

export function refreshSession(): Promise<boolean> {
  if (!refreshing) {
    refreshing = doRefresh().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}

// SEKMELER ARASI: yukarıdaki Promise yalnız BU sekmeyi korur. Aynı çerezi paylaşan üç sekme
// aynı anda açılırsa üçü de aynı tek kullanımlık çerezle /refresh yapar; biri kazanır,
// diğerlerinin isteği REUSE sayılır ve üç sekme birden düşer. Web Locks API ile çerez yazan
// TÜM istekleri (yenileme, giriş, çıkış) origin başına SIRAYA sokuyoruz: kilidi alan sekme,
// bir öncekinin döndürdüğü taze çerezle çalışır. (Kilit desteklenmiyorsa doğrudan çalışır.)
async function doRefresh(): Promise<boolean> {
  return withRefreshLock(refreshOnce);
}

async function refreshOnce(): Promise<boolean> {
  // HINT kontrolü KİLİDİN İÇİNDE: kilidi bizden önce tutan bir çıkış, tam biz beklerken
  // oturumu kapatmış olabilir. Dışarıda bakıp içeride kullansaydık, çıkıştan sonra çalışan
  // bu yenileme ölü çerezle /refresh atar; sunucu onu REUSE sayıp (çıkış zaten aileyi
  // iptal etti) boşuna bir güvenlik alarmı üretirdi.
  if (!hasFlag(HINT_KEY)) return false;
  const epoch = sessionEpoch;
  let res: Response;
  try {
    // GÖVDE YOK: token istekte çerez olarak gider (Content-Type bile göndermiyoruz).
    // credentials:"same-origin" — çerezi ekle (aynı origin; NGINX sayesinde API de burada).
    // keepalive: sayfa kapanırken/yenilenirken atılan istek iptal EDİLMESİN. Edilseydi
    // sunucu token'ı çoktan harcamış olur, cevabı (yeni çerez) kaybolur ve bir sonraki
    // açılışta eski çerez reuse sayılıp tüm oturum ailesi iptal olurdu.
    res = await fetch("/api/auth/refresh", { method: "POST", credentials: "same-origin", keepalive: true });
  } catch {
    return false; // ağ hatası: çerez hâlâ geçerli olabilir, oturumu DÜŞÜRME
  }
  if (res.status === 401) {
    // 401 "bu oturum yok" demek — ama HANGİ oturum? Cevap yolda gelirken oturum DEĞİŞMİŞ
    // olabilir (başka sekmede çıkış + yeni giriş): o zaman bu 401 ESKİ ailenin cevabıdır.
    // Koşulsuz clearSession() çağırsaydık paylaşılan localStorage'dan HINT'i siler, bu da
    // diğer sekmelerde "storage" olayını tetikleyip sunucuda HÂLÂ CANLI olan yeni oturumu
    // düşürürdü. Yalnız KENDİ dönemimizin 401'i kesin sonuçtur. (Başarı yolundaki
    // epoch/HINT kontrolünün aynısı: diriltme kadar yanlış-öldürme de yasak.)
    if (epoch === sessionEpoch && hasFlag(HINT_KEY)) clearSession();
    return false;
  }
  // 429 (NGINX sınırı), 502/5xx (API yeniden başlıyor): GEÇİCİ — HINT korunur, sonra denenir.
  if (!res.ok) return false;
  const session = (await res.json()) as Session;
  // Cevap beklenirken çıkış yapılmış olabilir (bu sekmede: epoch arttı; başka sekmede:
  // HINT silindi). İkisinde de sonucu ATIYORUZ — diriltme yok.
  if (epoch !== sessionEpoch || !hasFlag(HINT_KEY)) return false;
  setSession(session);
  return true;
}

// postLogout, /api/auth/logout'a GÖVDESİZ istek atar: çerez gider, cevabın gövdesi
// umursanmaz, hata fırlatmaz. Dönüş: sunucu isteği kabul etti mi.
async function postLogout(): Promise<boolean> {
  try {
    const res = await fetch("/api/auth/logout", { method: "POST", credentials: "same-origin", keepalive: true });
    return res.ok;
  } catch {
    return false;
  }
}

// ÇIKIŞ SUNUCUYA ULAŞMAZSA: yerel oturum zaten kapandı ama sunucudaki aile CANLI kaldı —
// çalınmış bir çerez hâlâ işe yarardı. Bayrağı bırakıp sınırlı sayıda geri çekiliyoruz
// (2s, 10s, 30s) ve bir de açılışta (api.restore) deniyoruz. Sonsuz döngü YOK: kapalı bir
// sekmede zaten çalışamaz, açık bir sekmede de sonsuza kadar ağ denemesi yapmak doğru değil.
const LOGOUT_RETRY_DELAYS_MS = [2_000, 10_000, 30_000];
let logoutRetryTimer: ReturnType<typeof setTimeout> | null = null;

function scheduleLogoutRetry(attempt: number) {
  if (attempt >= LOGOUT_RETRY_DELAYS_MS.length) return;
  logoutRetryTimer = setTimeout(() => {
    logoutRetryTimer = null;
    void withRefreshLock(async () => {
      // Kilidin içinde yeniden bak: arada yeni bir giriş yapıldıysa bayrak silinmiştir ve
      // bu istek YENİ oturumun çerezini öldürürdü.
      if (!hasFlag(LOGOUT_PENDING_KEY)) return;
      if (await postLogout()) clearFlag(LOGOUT_PENDING_KEY);
      else scheduleLogoutRetry(attempt + 1);
    });
  }, LOGOUT_RETRY_DELAYS_MS[attempt]);
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
    credentials: "same-origin",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  // HINT yoksa yenilemeye hiç kalkışma: anonim ziyaretçinin aldığı her 401, boşuna bir
  // /refresh isteği (ve NGINX kovasından bir kredi) demek olurdu.
  if (res.status === 401 && retry && hasFlag(HINT_KEY)) {
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
  // Oturum. ÇEREZ YAZAN/SİLEN her istek (login, refresh, logout) aynı kilidin içinde koşar:
  // ikisi aynı anda koşarsa sunucudaki rotasyon sırası ile tarayıcıdaki çerez sırası
  // birbirini tutmaz ve oturum kendiliğinden ölürdü.
  async login(email: string, password: string): Promise<User> {
    // Uçuştaki bir yenileme varsa önce bitsin: kilidi O tutuyor olabilir ve kilidin İÇİNDE
    // beklesek kilitlenirdik (Web Locks yeniden girilemez).
    if (refreshing) await refreshing.catch(() => false);
    return withRefreshLock(async () => {
      // retry=false: 401 "parola yanlış" demektir, yenilenecek bir oturum yok — üstelik
      // kilidin içinden refreshSession çağırmak aynı kilidi tekrar isteyip kilitlenirdi.
      const s = await request<Session>("POST", "/api/auth/login", { email, password }, false);
      setFlag(HINT_KEY);
      // Bekleyen bir çıkış varsa artık geçersiz: sunucu, BU girişin çerezini öldürürdü.
      clearFlag(LOGOUT_PENDING_KEY);
      setSession(s);
      return s.user;
    });
  },
  register: (email: string, display_name: string, password: string) =>
    request<User>("POST", "/api/auth/register", { email, display_name, password }, false),
  /**
   * Parola değiştirme (Faz 13). Sunucu kullanıcının TÜM oturum ailelerini öldürür ve bu
   * cihaz için yenisini açar — yani cevap ÇEREZ YAZAR: login ile aynı kilit kuralı.
   */
  async changePassword(current: string, next: string): Promise<User> {
    if (refreshing) await refreshing.catch(() => false);
    const attempt = () =>
      withRefreshLock(async () => {
        // retry=false: request'in 401→refresh yolu kilidin İÇİNDEN aynı adlı kilidi tekrar
        // isterdi; Web Locks yeniden girilemez → kilitlenme. 401'i aşağıda, kilit DIŞINDA ele alıyoruz.
        const s = await request<Session>(
          "POST", "/api/auth/password", { current_password: current, new_password: next }, false,
        );
        setFlag(HINT_KEY);
        setSession(s);
        return s.user;
      });
    try {
      return await attempt();
    } catch (err) {
      // 401 = access token bitti (yanlış parola 403'tür, buraya düşmez). Bir kez yenile (kilit
      // dışında) ve bir kez daha dene; yenileme de olmadıysa asıl hatayı olduğu gibi ver.
      if (!(err instanceof ApiError && err.status === 401) || !(await refreshSession())) throw err;
      return attempt();
    }
  },
  async logout(): Promise<void> {
    if (refreshing) await refreshing.catch(() => false);
    await withRefreshLock(async () => {
      // Önce YEREL oturum: kullanıcı "çıkış" dedikten sonra arayüz bir an bile girişli
      // görünmemeli, sunucu cevabı gecikse ya da hiç gelmese bile.
      clearSession();
      // İstek HER ZAMAN atılır — HINT olmasa bile. Bu sekme oturumu bilmiyor olabilir
      // (ör. başka sekmede açıldı, bu sekme yeni yüklendi) ama TARAYICIDA çerez durabilir;
      // atmasaydık sunucudaki aile canlı kalırdı. Çerez yoksa sunucu zaten 204 diyor.
      if (!(await postLogout())) {
        setFlag(LOGOUT_PENDING_KEY);
        scheduleLogoutRetry(0);
      }
    });
  },
  /** Sayfa açılışında: çerez hâlâ geçerliyse oturumu geri getir. */
  async restore(): Promise<User | null> {
    // Faz 13 öncesinden kalan refresh token'ı sil: o değer artık hiçbir işe yaramıyor ama
    // localStorage'da bir SIR olarak durmaya devam ederdi (taşınan kullanıcıların tarayıcısı).
    clearFlag(LEGACY_REFRESH_KEY);

    // Ulaşamamış bir çıkış varsa ÖNCE onu bitir: sunucudaki aile hâlâ canlı. Aynı açılışta
    // yenileme de yapsaydık, çıkış isteği o taze çerezi öldürür ve kullanıcı girdiği anda
    // düşerdi. Bu açılışta oturum yok (null); başarısız olursa bayrak kalır, bir dahakine.
    // (Kilidi beklerken başka sekmede giriş yapılmışsa çıkışı ATLARIZ ama yine null döneriz:
    // bu sekme yeni oturumu bir sonraki yüklemede görür — ucuz ve güvenli taraf.)
    if (hasFlag(LOGOUT_PENDING_KEY)) {
      await withRefreshLock(async () => {
        // Kilidin İÇİNDE yeniden bak (scheduleLogoutRetry ile aynı gerekçe): biz sırada
        // beklerken başka bir sekmede giriş yapıldıysa bayrak silinmiştir ve bu istek
        // YENİ oturumun çerezini öldürürdü. Bayrağa kilitten önce bakmak yetmez; bu yol
        // (açılış) her sayfa yüklenişinde koştuğu için yarışın en olası yeri burası.
        if (!hasFlag(LOGOUT_PENDING_KEY)) return;
        if (await postLogout()) clearFlag(LOGOUT_PENDING_KEY);
      });
      return null;
    }
    if (!hasFlag(HINT_KEY)) return null;
    if (!(await refreshSession())) return null;
    return request<User>("GET", "/api/auth/me");
  },

  // Katalog
  books: (p: { q?: string; genre?: string; limit?: number; offset?: number; sort?: "rating" }) =>
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
  if (logoutRetryTimer !== null) {
    clearTimeout(logoutRetryTimer);
    logoutRetryTimer = null;
  }
  localStorage.clear();
}
