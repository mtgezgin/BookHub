import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __resetForTests, api, ApiError, refreshSession, request } from "./client";

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

  it("204 cevabı gövdesiz döner", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(request("DELETE", "/api/admin/comments/1")).resolves.toBeUndefined();
  });
});
