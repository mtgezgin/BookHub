import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __resetForTests, api, ApiError, refreshSession, request, subscribeSession } from "./client";
import type { NewBook } from "./types";

const session = (n: number) => ({
  access_token: `access-${n}`,
  token_type: "Bearer",
  access_expires_at: "2026-09-01T00:15:00Z",
  refresh_token: `refresh-${n}`,
  refresh_expires_at: "2026-09-08T00:00:00Z",
  user: { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" },
});

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

describe("api client", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("hata zarfını ApiError'a çevirir (code + message)", async () => {
    fetchMock.mockResolvedValueOnce(json(404, { error: { code: "not_found", message: "kitap bulunamadı" } }));
    const err = await request("GET", "/api/books/9").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: "not_found", message: "kitap bulunamadı" });
  });

  it("login access token'ı sonraki isteklere Bearer olarak ekler, refresh token'ı saklar", async () => {
    fetchMock.mockResolvedValueOnce(json(200, session(1))).mockResolvedValueOnce(json(200, { id: 7 }));
    await api.login("ali@example.com", "parola");
    await request("GET", "/api/auth/me");
    const headers = fetchMock.mock.calls[1][1].headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer access-1");
    expect(localStorage.getItem("bookhub.refresh")).toBe("refresh-1");
  });

  it("401 → bir kez yenile → aynı isteği yeni token'la tekrar dene", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }))
      .mockResolvedValueOnce(json(200, session(1))) // /refresh
      .mockResolvedValueOnce(json(200, { ok: true })); // tekrar
    const res = await request<{ ok: boolean }>("GET", "/api/auth/me");
    expect(res.ok).toBe(true);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/refresh");
    expect((fetchMock.mock.calls[2][1].headers as Record<string, string>).Authorization).toBe("Bearer access-1");
  });

  // Backend'de refresh token TEK KULLANIMLIK: eşzamanlı yenilemeler aynı token'ı iki kez
  // harcasaydı ikincisi 401 alır ve kullanıcı oturumdan düşerdi.
  it("eşzamanlı yenileme isteklerinde /refresh YALNIZCA BİR KEZ çağrılır", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    let resolve!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolve = r)));
    const results = [refreshSession(), refreshSession(), refreshSession()];
    resolve(json(200, session(1)));
    expect(await Promise.all(results)).toEqual([true, true, true]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // Review bulgusu F3: 429/5xx/ağ hatası GEÇİCİDİR — sunucuda hâlâ geçerli olan token'ı
  // silmek kullanıcıyı sebepsiz yere oturumdan atardı.
  it.each([429, 502, 500])("yenileme %i dönerse token KORUNUR", async (status) => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockResolvedValueOnce(json(status, { error: { code: "x", message: "" } }));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem("bookhub.refresh")).toBe("refresh-0");
  });

  it("ağ hatasında token korunur", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem("bookhub.refresh")).toBe("refresh-0");
  });

  // Review bulgusu F5: başka bir sekme arada token'ı döndürdüyse (yeni token saklı), bizim
  // eski token'la aldığımız 401 onun oturumunu SİLMEMELİ.
  it("401 geldiğinde saklı token değişmişse (başka sekme döndürdü) silinmez", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockImplementationOnce(async () => {
      localStorage.setItem("bookhub.refresh", "refresh-from-other-tab");
      return json(401, { error: { code: "invalid_token", message: "" } });
    });
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem("bookhub.refresh")).toBe("refresh-from-other-tab");
  });

  it("yenileme 401 ile reddedilirse oturum temizlenir", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem("bookhub.refresh")).toBeNull();
  });

  // Review bulgusu R4: yenileme uçuştayken çıkış yapılırsa, yenilemenin cevabı oturumu
  // geri diriltmemeli ve çıkış sunucuya YENİ (döndürülmüş) token'ı iptal ettirmeli.
  it("yenileme sırasında çıkış: oturum dirilmez, yeni token iptal edilir", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    let resolveRefresh!: (r: Response) => void;
    fetchMock
      .mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r))) // /refresh
      .mockResolvedValueOnce(new Response(null, { status: 204 })); // /logout
    const inFlight = refreshSession();
    const loggingOut = api.logout();
    resolveRefresh(json(200, session(1)));
    await inFlight;
    await loggingOut;
    expect(localStorage.getItem("bookhub.refresh")).toBeNull();
    const logoutCall = fetchMock.mock.calls[1];
    expect(logoutCall[0]).toBe("/api/auth/logout");
    expect(JSON.parse(logoutCall[1].body)).toEqual({ refresh_token: "refresh-1" });
  });

  // Review bulgusu R3: başka bir sekme çıkış yapınca (localStorage'dan token silindi) bu
  // sekme de oturumu bırakmalı — yoksa bellekteki access token'la "girişli" kalırdı.
  it("başka sekmede çıkış bu sekmenin oturumunu da düşürür", async () => {
    const seen: (unknown | null)[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: "bookhub.refresh", oldValue: "r", newValue: null }));
    expect(seen).toEqual([null]);
  });

  it("204 cevabı gövdesiz döner", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(request("DELETE", "/api/admin/comments/1")).resolves.toBeUndefined();
  });
});

describe("api client — oturum kenar durumları", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    subscribeSession(() => {});
  });

  // Web Locks sahtesi: callback'in kilit İÇİNDE çalışıp çalışmadığını kaydeder.
  function stubLocks() {
    const state = { held: false, names: [] as string[] };
    const locks = {
      request: vi.fn(async (name: string, fn: () => Promise<unknown>) => {
        state.names.push(name);
        state.held = true;
        try {
          return await fn();
        } finally {
          state.held = false;
        }
      }),
    };
    vi.stubGlobal("navigator", { ...navigator, locks });
    return state;
  }

  it("refreshSession /refresh isteğini \"bookhub-refresh\" kilidinin içinde atar", async () => {
    const lock = stubLocks();
    localStorage.setItem("bookhub.refresh", "refresh-0");
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return json(200, session(1));
    });
    expect(await refreshSession()).toBe(true);
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(heldDuringFetch).toBe(true);
  });

  it("api.logout token'ı \"bookhub-refresh\" kilidinin içinde okuyup iptal eder", async () => {
    const lock = stubLocks();
    localStorage.setItem("bookhub.refresh", "refresh-0");
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return new Response(null, { status: 204 });
    });
    await api.logout();
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(heldDuringFetch).toBe(true);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/logout");
  });

  it("başka sekme token'ı DÖNDÜRDÜYSE (newValue dolu) oturum düşmez", () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: "bookhub.refresh", oldValue: "r0", newValue: "r1" }));
    expect(seen).toEqual([]);
  });

  it("başka bir anahtarın silinmesi oturumu düşürmez", () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: "baska.anahtar", oldValue: "x", newValue: null }));
    expect(seen).toEqual([]);
  });

  it("JSON olmayan hata gövdesi (NGINX 502 HTML) ApiError(status, unknown, statusText) olur", async () => {
    fetchMock.mockResolvedValueOnce(new Response("<html>502 Bad Gateway</html>", { status: 502, statusText: "Bad Gateway" }));
    const err = await request("GET", "/api/books").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 502, code: "unknown", message: "Bad Gateway" });
  });

  it("401 sonrası yenileme başarısızsa asıl 401 fırlatılır, tekrar denenmez", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "token_expired", message: "süre doldu" } }))
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } })); // /refresh
    const err = await request("GET", "/api/auth/me").catch((e) => e);
    expect(err).toMatchObject({ status: 401, code: "token_expired" });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/refresh");
  });

  it("saklı refresh token yokken 401 /refresh çağırmaz", async () => {
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "missing_token", message: "" } }));
    await expect(request("GET", "/api/auth/me")).rejects.toMatchObject({ status: 401 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("yanlış parola (login 401) /refresh çağırmadan fırlatır", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_credentials", message: "e-posta ya da parola hatalı" } }));
    await expect(api.login("ali@example.com", "yanlis")).rejects.toMatchObject({ status: 401, code: "invalid_credentials" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/login");
  });

  it("saklı token yokken logout oturumu temizler, /logout isteği atmaz", async () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    await api.logout();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(seen).toEqual([null]);
  });

  it("reddedilen /logout isteği yutulur, yerel oturum yine temizlenir", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await expect(api.logout()).resolves.toBeUndefined();
    expect(localStorage.getItem("bookhub.refresh")).toBeNull();
  });

  it("restore: yenileme başarısızsa null döner, /me çağrılmaz", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }));
    expect(await api.restore()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/refresh");
  });

  it("restore: yenileme başarılıysa kullanıcıyı GET /api/auth/me ile getirir", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    const me = session(1).user;
    fetchMock.mockResolvedValueOnce(json(200, session(1))).mockResolvedValueOnce(json(200, me));
    expect(await api.restore()).toEqual(me);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/me");
    expect(fetchMock.mock.calls[1][1].method).toBe("GET");
    expect((fetchMock.mock.calls[1][1].headers as Record<string, string>).Authorization).toBe("Bearer access-1");
  });

  it("qs: undefined ve boş parametreler düşer, hiç kalmazsa ? eklenmez", async () => {
    fetchMock.mockImplementation(async () => json(200, { items: [], total: 0, limit: 20, offset: 0 }));
    await api.books({ q: "", genre: undefined });
    await api.books({ q: "", genre: "roman", offset: undefined });
    expect(fetchMock.mock.calls[0][0]).toBe("/api/books");
    expect(fetchMock.mock.calls[1][0]).toBe("/api/books?genre=roman");
  });
});

describe("api client — uç tablosu", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn(async () => json(200, {}));
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  const book: NewBook = { title: "T", author: "A", language: "tr" };
  // Her api.* çağrısının backend rotasıyla (method + yol + gövde) eşleştiğini sabitler:
  // ör. puan PUT, arşiv DELETE — POST'a dönse backend 405 verirdi.
  it.each<[string, () => Promise<unknown>, string, string, unknown]>([
    ["register", () => api.register("a@b.co", "Ali", "parola123"), "POST", "/api/auth/register", { email: "a@b.co", display_name: "Ali", password: "parola123" }],
    ["book", () => api.book(5), "GET", "/api/books/5", undefined],
    ["topRated (varsayılan)", () => api.topRated(), "GET", "/api/books/top-rated?limit=10", undefined],
    ["topRated", () => api.topRated(5), "GET", "/api/books/top-rated?limit=5", undefined],
    ["genres", () => api.genres(), "GET", "/api/genres", undefined],
    ["suggest", () => api.suggest(book), "POST", "/api/books/suggestions", book],
    ["comments", () => api.comments(5), "GET", "/api/books/5/comments?offset=0", undefined],
    ["comments (offset)", () => api.comments(5, 10), "GET", "/api/books/5/comments?offset=10", undefined],
    ["addComment", () => api.addComment(5, "güzel"), "POST", "/api/books/5/comments", { content: "güzel" }],
    ["ratingSummary", () => api.ratingSummary(5), "GET", "/api/books/5/ratings/summary", undefined],
    ["myRating", () => api.myRating(5), "GET", "/api/books/5/rating", undefined],
    ["rate", () => api.rate(5, 8), "PUT", "/api/books/5/rating", { score: 8 }],
    ["removeRating", () => api.removeRating(5), "DELETE", "/api/books/5/rating", undefined],
    ["adminBooks", () => api.adminBooks("PENDING", 20), "GET", "/api/admin/books?status=PENDING&offset=20", undefined],
    ["approve", () => api.approve(5), "POST", "/api/admin/books/5/approve", undefined],
    ["reject", () => api.reject(5, "spam"), "POST", "/api/admin/books/5/reject", { reason: "spam" }],
    ["archive", () => api.archive(5), "DELETE", "/api/admin/books/5", undefined],
    ["restoreBook", () => api.restoreBook(5), "POST", "/api/admin/books/5/restore", undefined],
    ["createGenre", () => api.createGenre("Roman"), "POST", "/api/admin/genres", { name: "Roman" }],
    ["deleteComment", () => api.deleteComment(9), "DELETE", "/api/admin/comments/9", undefined],
  ])("%s", async (_name, call, method, path, body) => {
    await call();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(path);
    expect(init.method).toBe(method);
    expect(init.body === undefined ? undefined : JSON.parse(init.body as string)).toEqual(body);
  });

  it("deleteComment 204'te undefined döner", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    expect(await api.deleteComment(9)).toBeUndefined();
  });

  it("register 401 alırsa (saklı token olsa bile) /refresh denemez", async () => {
    localStorage.setItem("bookhub.refresh", "refresh-0");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "unauthorized", message: "yetkisiz" } }));
    const err = await api.register("a@b.co", "Ali", "parola123").catch((e) => e);
    expect(err).toMatchObject({ status: 401, code: "unauthorized" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("zarfsız JSON hata gövdesi ({}) ApiError(status, unknown, statusText) olur", async () => {
    fetchMock.mockResolvedValueOnce(new Response("{}", { status: 503, statusText: "Service Unavailable" }));
    const err = await api.book(5).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 503, code: "unknown", message: "Service Unavailable" });
  });
});
