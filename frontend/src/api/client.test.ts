import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __resetForTests, api, ApiError, refreshSession, request, subscribeSession } from "./client";
import type { NewBook } from "./types";

// Faz 13: refresh token artık httpOnly ÇEREZTE. Testler onu göremez (jsdom + sahte fetch
// zaten çerez taşımaz) — bu yüzden iddialar "hangi isteği attık, localStorage'a ne yazdık"
// üzerinden kurulur. localStorage'da SIR YOK; yalnız bayraklar:
const HINT = "bookhub.session"; // "bu tarayıcıda oturum açık olmalı"
const LOGOUT_PENDING = "bookhub.logout_pending"; // sunucuya ulaşmamış çıkış
const LEGACY = "bookhub.refresh"; // Faz 13 öncesinden kalan token (silinmeli)

const session = (n: number) => ({
  access_token: `access-${n}`,
  token_type: "Bearer",
  access_expires_at: "2026-09-01T00:15:00Z",
  refresh_expires_at: "2026-09-08T00:00:00Z",
  user: { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" },
});

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const noContent = () => new Response(null, { status: 204 });

/** Backend'in refresh çerezinin adı (COOKIE_NAME varsayılanı). */
const REFRESH_COOKIE = "bookhub_refresh";
/** Sunucunun SİLME çerezi — backend clearRefreshCookie'nin ürettiğiyle aynı biçim. */
const CLEARING_SET_COOKIE = `${REFRESH_COOKIE}=; Path=/api/auth; Max-Age=0; HttpOnly; Secure; SameSite=Strict`;
/** 204 + silme çerezi: başarılı çıkışın (ve çerezsiz çıkışın) gerçek cevabı. */
const noContentClearingCookie = () =>
  new Response(null, { status: 204, headers: { "Set-Cookie": CLEARING_SET_COOKIE } });

/** tick: bekleyen mikro/makro görevlerin (kilit zinciri) koşmasına izin verir. */
const tick = () => new Promise((r) => setTimeout(r, 0));

/**
 * SIRAYA SOKAN Web Locks sahtesi: aynı isimli istekler birbirini BEKLER (gerçek Web Locks
 * gibi, isim başına bir promise zinciri). "Hemen çalıştır" biçimindeki basit bir sahte,
 * "kilidi tutan çıkış bitmeden sıradaki yenileme koşmasın" gibi SIRALAMA iddialarını hiç
 * sınayamaz — kilidin asıl varlık sebebi tam olarak o sıralamadır.
 */
function stubLocks() {
  const state = { held: false, names: [] as string[] };
  const chains = new Map<string, Promise<unknown>>();
  const locks = {
    request: vi.fn((name: string, fn: () => Promise<unknown>) => {
      state.names.push(name);
      const prev = chains.get(name) ?? Promise.resolve();
      const run = prev.then(async () => {
        state.held = true;
        try {
          return await fn();
        } finally {
          state.held = false;
        }
      });
      // Zincir, hata dönen bir adımda kopmasın (gerçek kilit de bırakılır).
      chains.set(name, run.catch(() => undefined));
      return run;
    }),
  };
  vi.stubGlobal("navigator", { ...navigator, locks });
  return state;
}

/**
 * ÇEREZ MODELİ + sahte /api/auth/logout sunucusu. Sahte fetch çerez taşımaz, httpOnly çerez
 * zaten JS'e görünmez — "yeniden deneme çerezle mi gitti, sunucuda GERÇEKTEN bir iptal oldu
 * mu?" sorusunu ancak elle modelleyerek sınayabiliriz. Kurallar gerçeğin aynısı:
 *   - tarayıcı: Max-Age=0 taşıyan bir Set-Cookie çerezi siler (bir daha hiçbir istekte gitmez);
 *   - sunucu (auth.go handleLogout): çerez yoksa DB'ye hiç gitmeden 204 + silme çerezi;
 *     çerez varsa iptali dener — başarıda 204 + silme çerezi, HATADA 500 ve çerez KORUNUR.
 * Model olmasaydı "500'de çerezi de sil" hatası testten geçerdi: yeniden deneme çerezsiz gider,
 * 204 alır, bayrak silinir ve hiçbir iddia bozulmaz — oysa aile sunucuda YAŞAMAYA devam eder.
 */
function fakeLogoutServer(failures: number) {
  const state = { hasCookie: true, revokes: 0 };
  const send = (res: Response) => {
    const setCookie = res.headers.get("set-cookie") ?? "";
    if (setCookie.startsWith(`${REFRESH_COOKIE}=`) && /max-age=0/i.test(setCookie)) state.hasCookie = false;
    return res;
  };
  return {
    state,
    handle(): Response {
      if (!state.hasCookie) return send(noContentClearingCookie()); // iptal edilecek bir şey yok
      if (failures-- > 0) return json(500, { error: { code: "internal", message: "" } });
      state.revokes++;
      return send(noContentClearingCookie());
    },
  };
}
type FakeLogoutServer = ReturnType<typeof fakeLogoutServer>;

describe("api client", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    subscribeSession(() => {});
  });

  it("hata zarfını ApiError'a çevirir (code + message)", async () => {
    fetchMock.mockResolvedValueOnce(json(404, { error: { code: "not_found", message: "kitap bulunamadı" } }));
    const err = await request("GET", "/api/books/9").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: "not_found", message: "kitap bulunamadı" });
  });

  it("giriş access token'ı Bearer olarak ekler ve localStorage'a SIR DEĞİL, yalnız ipucu yazar", async () => {
    fetchMock.mockResolvedValueOnce(json(200, session(1))).mockResolvedValueOnce(json(200, { id: 7 }));
    await api.login("ali@example.com", "parola");
    await request("GET", "/api/auth/me");
    const headers = fetchMock.mock.calls[1][1].headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer access-1");
    // Tüm localStorage'da yalnızca ipucu bayrağı var; hiçbir token değeri yok.
    expect(localStorage.getItem(HINT)).toBe("1");
    expect(Object.keys(localStorage)).toEqual([HINT]);
  });

  it("401 → bir kez yenile → aynı isteği yeni token'la tekrar dene", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }))
      .mockResolvedValueOnce(json(200, session(1))) // /refresh
      .mockResolvedValueOnce(json(200, { ok: true })); // tekrar
    const res = await request<{ ok: boolean }>("GET", "/api/auth/me");
    expect(res.ok).toBe(true);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/refresh");
    expect((fetchMock.mock.calls[2][1].headers as Record<string, string>).Authorization).toBe("Bearer access-1");
  });

  // Yenileme isteğinin BİÇİMİ sözleşmedir: backend gövdeyi hiç okumuyor, token'ı yalnız
  // çerezden alıyor. credentials olmasaydı çerez gitmez, keepalive olmasaydı sayfa
  // kapanırken cevap kaybolur ve harcanmış çerez tarayıcıda kalırdı (sahte reuse).
  it("yenileme isteği GÖVDESİZ, same-origin ve keepalive", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(200, session(1)));
    expect(await refreshSession()).toBe(true);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/auth/refresh");
    expect(init.method).toBe("POST");
    expect(init.body).toBeUndefined();
    expect(init.headers).toBeUndefined();
    expect(init.credentials).toBe("same-origin");
    expect(init.keepalive).toBe(true);
  });

  // Backend'de refresh token TEK KULLANIMLIK ve kullanılmışının tekrar gelmesi TÜM AİLEYİ
  // öldürüyor: eşzamanlı iki yenileme, kullanıcıyı her cihazdan düşürürdü.
  it("eşzamanlı yenileme isteklerinde /refresh YALNIZCA BİR KEZ çağrılır", async () => {
    localStorage.setItem(HINT, "1");
    let resolve!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolve = r)));
    const results = [refreshSession(), refreshSession(), refreshSession()];
    resolve(json(200, session(1)));
    expect(await Promise.all(results)).toEqual([true, true, true]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // Oturum ipucu yoksa ortada yenilenecek bir şey yok: anonim ziyaretçi her açılışta
  // boşuna bir /refresh atıp NGINX kovasından kredi harcamasın.
  it("HINT yokken yenileme hiç fetch ATMAZ", async () => {
    expect(await refreshSession()).toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Review bulgusu F3: 429/5xx/ağ hatası GEÇİCİDİR — sunucuda hâlâ geçerli olan oturumu
  // düşürmek kullanıcıyı sebepsiz yere dışarı atardı.
  it.each([429, 502, 500])("yenileme %i dönerse oturum KORUNUR", async (status) => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(status, { error: { code: "x", message: "" } }));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem(HINT)).toBe("1");
  });

  it("ağ hatasında oturum korunur", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem(HINT)).toBe("1");
  });

  it("yenileme 401 ile reddedilirse oturum temizlenir ve aboneye null gider", async () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }));
    expect(await refreshSession()).toBe(false);
    expect(localStorage.getItem(HINT)).toBeNull();
    expect(seen).toEqual([null]);
  });

  // HINT'i YALNIZCA giriş yazar. Yenileme de yazsaydı, kilidi tutan bir çıkışın hemen
  // ardından gelen cevap oturumu geri diriltir ve "çıkış yaptım" diyen kullanıcı girişli kalırdı.
  it("yenileme HINT'i KENDİSİ yazmaz (localStorage'a hiç yazmaz)", async () => {
    localStorage.setItem(HINT, "1");
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    fetchMock.mockResolvedValueOnce(json(200, session(1)));
    expect(await refreshSession()).toBe(true);
    expect(setItem).not.toHaveBeenCalled();
  });

  it("cevap beklenirken HINT silinirse oturum DİRİLMEZ (ve HINT geri yazılmaz)", async () => {
    localStorage.setItem(HINT, "1");
    const seen: unknown[] = [];
    let resolve!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolve = r)));
    const inFlight = refreshSession();
    await tick();
    subscribeSession((u) => seen.push(u));
    localStorage.removeItem(HINT); // başka sekme çıkış yaptı
    resolve(json(200, session(1)));
    expect(await inFlight).toBe(false);
    expect(localStorage.getItem(HINT)).toBeNull();
    expect(seen).toEqual([]); // kimseye "giriş yapıldı" denmedi
  });

  // Review bulgusu R4'ün İKİNCİ yarısı: HINT kontrolü tek başına YETMEZ. Web Locks yoksa
  // (Safari <15.4, güvenli olmayan bağlam — client.ts'deki `if (!locks) return fn()` yolu;
  // jsdom da tam olarak bu yolda koşar) başka bir sekme uçuştaki yenilememiz sürerken önce
  // ÇIKIŞ, sonra YENİ GİRİŞ yapabilir: HINT silinip geri yazılır, yani cevap geldiğinde HINT
  // yine "1"dir. Oturumun DEĞİŞTİĞİNİ yalnız epoch söyler; o kontrol olmasaydı geciken cevap
  // ESKİ kullanıcının access token'ını diriltirdi.
  it("uçuşta çıkış + yeni giriş olursa (HINT geri gelse bile) eski oturum DİRİLMEZ", async () => {
    localStorage.setItem(HINT, "1");
    const seen: unknown[] = [];
    let resolveRefresh!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r)));

    const inFlight = refreshSession();
    await tick();
    subscribeSession((u) => seen.push(u));

    // Başka sekme çıkış yaptı: HINT silindi → storage olayı → epoch artar.
    localStorage.removeItem(HINT);
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: "1", newValue: null }));
    await tick();
    // Başka sekme yeniden giriş yaptı: paylaşılan localStorage'da HINT geri geldi.
    localStorage.setItem(HINT, "1");
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: null, newValue: "1" }));
    await tick();

    resolveRefresh(json(200, session(1))); // ESKİ oturumun geciken cevabı
    expect(await inFlight).toBe(false);
    // Çıkışın kendi null bildirimi dışında kimseye "girişli" denmedi.
    expect(seen.filter(Boolean)).toEqual([]);
    // Bellekteki access token da yazılmadı: sonraki istek Authorization taşımaz.
    fetchMock.mockResolvedValueOnce(json(200, { ok: true }));
    await request("GET", "/api/books/1");
    const headers = fetchMock.mock.calls[1][1].headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  // Aynı yarışın AYNA yüzü: geciken cevap 401 ise oturumu DİRİLTMEK değil, yanlışlıkla
  // ÖLDÜRMEK tehlikesi var. 401 eski aileye aittir; koşulsuz clearSession() paylaşılan
  // localStorage'dan HINT'i siler, "storage" olayıyla tüm sekmelere yayılır ve sunucuda hâlâ
  // CANLI olan yeni oturum her yerde düşerdi.
  it("uçuşta çıkış + yeni giriş olursa geciken 401 YENİ oturumu düşürmez", async () => {
    localStorage.setItem(HINT, "1");
    const seen: unknown[] = [];
    let resolveRefresh!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r)));

    const inFlight = refreshSession();
    await tick();
    subscribeSession((u) => seen.push(u));

    // Başka sekme çıkış yaptı: HINT silindi → storage olayı → epoch artar.
    localStorage.removeItem(HINT);
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: "1", newValue: null }));
    await tick();
    // Başka sekme yeniden giriş yaptı: HINT geri geldi, sunucuda YENİ bir aile var.
    localStorage.setItem(HINT, "1");
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: null, newValue: "1" }));
    await tick();

    resolveRefresh(json(401, { error: { code: "invalid_token", message: "" } })); // ESKİ ailenin 401'i
    expect(await inFlight).toBe(false);
    // HINT DURUYOR: yeni oturumun "bu tarayıcıda oturum var" ipucu silinmedi.
    expect(localStorage.getItem(HINT)).toBe("1");
    // Çıkışın kendi null bildirimi dışında kimseye ikinci bir "düştün" denmedi.
    expect(seen).toEqual([null]);
  });

  it("204 cevabı gövdesiz döner", async () => {
    fetchMock.mockResolvedValueOnce(noContent());
    await expect(request("DELETE", "/api/admin/comments/1")).resolves.toBeUndefined();
  });
});

describe("api client — kilit (Web Locks) sıralaması", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    subscribeSession(() => {});
  });

  it("yenileme \"bookhub-refresh\" kilidinin İÇİNDE fetch atar", async () => {
    const lock = stubLocks();
    localStorage.setItem(HINT, "1");
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return json(200, session(1));
    });
    expect(await refreshSession()).toBe(true);
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(heldDuringFetch).toBe(true);
  });

  // Giriş de çerez YAZAR: yenileme/çıkışla aynı kilidin içinde koşmazsa, uçuştaki bir
  // rotasyonun cevabı girişin taze çerezini ezebilir (ya da tersi) ve yeni oturum doğar doğmaz ölür.
  it("giriş kilidin İÇİNDE koşar ve HINT'i yazar", async () => {
    const lock = stubLocks();
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return json(200, session(1));
    });
    await api.login("ali@example.com", "parola");
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(heldDuringFetch).toBe(true);
    expect(localStorage.getItem(HINT)).toBe("1");
  });

  it("çıkış kilidin İÇİNDE koşar", async () => {
    const lock = stubLocks();
    localStorage.setItem(HINT, "1");
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return noContent();
    });
    await api.logout();
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(heldDuringFetch).toBe(true);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/logout");
  });

  it("parola değiştirme kilidin İÇİNDE koşar ve yeni oturumu yazar", async () => {
    const lock = stubLocks();
    localStorage.setItem(HINT, "1");
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    let heldDuringFetch = false;
    fetchMock.mockImplementationOnce(async () => {
      heldDuringFetch = lock.held;
      return json(200, session(2));
    });

    await api.changePassword("eski-parola", "yeni-parola");

    // NEDEN kilit: cevap ÇEREZ YAZIYOR (sunucu tüm aileleri öldürüp bu cihaza yenisini açtı).
    // Aynı anda koşan bir yenileme, ölmüş ailenin token'ını tarayıcıya geri yazabilirdi.
    expect(heldDuringFetch).toBe(true);
    expect(lock.names).toEqual(["bookhub-refresh"]);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/password");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body as string))
      .toEqual({ current_password: "eski-parola", new_password: "yeni-parola" });
    expect(localStorage.getItem(HINT)).toBe("1");
    expect(seen).toEqual([session(2).user]); // yeni oturum arayüze bildirildi
  });

  // 401 = access token'ın süresi dolmuş (yanlış parola 403'tür, bu yola düşmez). Yenilemeyi
  // kilidin DIŞINDA yapmak şart: Web Locks yeniden girilemez, kilidin içinden istemek
  // kilitlenme olurdu (ve bu test onu yakalar — zincir asla ilerlemez).
  it("401'de kilit DIŞINDA yenileyip bir kez daha dener", async () => {
    stubLocks();
    localStorage.setItem(HINT, "1");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } })) // /password
      .mockResolvedValueOnce(json(200, session(3))) // /refresh
      .mockResolvedValueOnce(json(200, session(4))); // /password (tekrar)

    const user = await api.changePassword("eski-parola", "yeni-parola");

    expect(fetchMock.mock.calls.map((c) => c[0]))
      .toEqual(["/api/auth/password", "/api/auth/refresh", "/api/auth/password"]);
    expect(user).toEqual(session(4).user);
  });

  it("401 + yenileme de başarısızsa asıl hata fırlatılır (üçüncü istek YOK)", async () => {
    stubLocks();
    localStorage.setItem(HINT, "1");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "oturum bitti" } }))
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } })); // /refresh de 401

    const err = await api.changePassword("eski-parola", "yeni-parola").catch((e) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 401, message: "oturum bitti" });
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/password", "/api/auth/refresh"]);
  });

  // 403 "mevcut parola hatalı": yenilenecek bir şey yok. Yeniden denemek kullanıcının parola
  // kotasını (NGINX password kovası) boşuna tüketir ve hatayı gizlerdi.
  it("403 yenilemeye kalkışmadan fırlatılır", async () => {
    stubLocks();
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(403, { error: { code: "invalid_current_password", message: "mevcut parola hatalı" } }));

    const err = await api.changePassword("yanlis", "yeni-parola").catch((e) => e);

    expect(err).toMatchObject({ status: 403, code: "invalid_current_password" });
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/password"]);
    expect(localStorage.getItem(HINT)).toBe("1"); // oturum düşmedi
  });

  // ASIL SENARYO: kilidi tutan çıkış HINT'i siler; sırada bekleyen yenileme, kilidi
  // aldığında bunu GÖRMELİ. HINT'e kilitten ÖNCE bakılsaydı, bu yenileme ölü çerezle
  // /refresh atar ve sunucuda sahte bir "reuse" alarmı üretirdi.
  //
  // SIRALAMA BURADA İDDİANIN KENDİSİ: yenileme, çıkış HINT'i SİLMEDEN sıraya giriyor. Önce
  // tick atıp (çıkış HINT'i siler) sonra sıraya sokan bir kurgu, kontrol kilidin DIŞINA
  // taşınsa bile yeşil kalırdı — iki yerde de HINT çoktan yok olurdu.
  it("kilidi tutan çıkış HINT'i silerse, sıradaki yenileme fetch ATMAZ", async () => {
    stubLocks();
    localStorage.setItem(HINT, "1");
    let resolveLogout!: (r: Response) => void;
    fetchMock
      .mockImplementationOnce(() => new Promise<Response>((r) => (resolveLogout = r))) // /logout
      .mockResolvedValue(json(200, session(2))); // yenileme kaçarsa yakalayalım

    const loggingOut = api.logout();
    // Yenilemeyi çıkış HINT'i SİLMEDEN sıraya sok: kilit içi kontrolü pinleyen tek sıralama bu.
    const refreshed = refreshSession(); // kilidin ARKASINA girer — HINT hâlâ yerinde
    expect(localStorage.getItem(HINT)).toBe("1");
    await tick(); // çıkış kilidi alsın: clearSession() + /logout isteği
    resolveLogout(noContent());
    await loggingOut;

    expect(await refreshed).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/logout");
  });
});

// Web Locks YOKKEN (Safari <15.4, güvenli olmayan bağlam — ve jsdom'da HER test) withRefreshLock
// `return fn()`e kısa devre yapar: çerez yazan istekleri SIRAYA SOKAN tek şey api.login/api.logout
// başındaki `if (refreshing) await refreshing` satırıdır. Bu blok stubLocks'u BİLEREK çağırmaz.
// NEDEN önemli: giriş, uçuştaki yenilemeyle aynı anda giderse ikisi de AYNI çerezi (C) taşır
// (çerez Path=/api/auth). handleLogin (auth.go) eski çerezin ailesini İPTAL edip yeni bir aile
// (G) kurar; yenilemenin geciken cevabı tarayıcıya C2'yi (iptal edilmiş ailenin token'ı) yazar —
// taze giriş bir sonraki yenilemede sessizce ölür. Ters sırada yenileme 401 alır ve epoch
// değişmediği için clearSession() oturumu daha doğmadan düşürür.
describe("api client — Web Locks YOKKEN sıralama", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    subscribeSession(() => {});
  });

  // Blokun dayandığı varsayım: jsdom Web Locks'ı uygulamıyor, yani gerçekten `!locks` dalındayız.
  it("jsdom'da navigator.locks TANIMSIZ (bu blok gerçekten kilitsiz yolu sınıyor)", () => {
    expect((navigator as { locks?: unknown }).locks).toBeUndefined();
  });

  it("giriş, uçuştaki yenileme bitmeden POST /login ATMAZ", async () => {
    localStorage.setItem(HINT, "1");
    let resolveRefresh!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r)));
    const inFlight = refreshSession();
    await tick();

    const login = api.login("ali@example.com", "parola");
    await tick();
    await tick();
    // Yenileme hâlâ uçuşta: giriş isteği HENÜZ atılmadı.
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh"]);

    fetchMock.mockResolvedValueOnce(json(200, session(2)));
    resolveRefresh(json(200, session(1)));
    await inFlight;
    await login;
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh", "/api/auth/login"]);
  });

  it("parola değiştirme, uçuştaki yenileme bitmeden POST /password ATMAZ", async () => {
    localStorage.setItem(HINT, "1");
    let resolveRefresh!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r)));
    const inFlight = refreshSession();
    await tick();

    const change = api.changePassword("eski-parola", "yeni-parola");
    await tick();
    await tick();
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh"]);

    fetchMock.mockResolvedValueOnce(json(200, session(2)));
    resolveRefresh(json(200, session(1)));
    await inFlight;
    await change;
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh", "/api/auth/password"]);
  });

  it("çıkış, uçuştaki yenileme bitmeden POST /logout ATMAZ", async () => {
    localStorage.setItem(HINT, "1");
    let resolveRefresh!: (r: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((r) => (resolveRefresh = r)));
    const inFlight = refreshSession();
    await tick();

    const logout = api.logout();
    await tick();
    await tick();
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh"]);

    fetchMock.mockResolvedValueOnce(noContentClearingCookie());
    resolveRefresh(json(200, session(1)));
    await inFlight;
    await logout;
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/auth/refresh", "/api/auth/logout"]);
  });
});

describe("api client — çıkış ve açılış", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    __resetForTests();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    subscribeSession(() => {});
  });

  // Faz 13'te DEĞİŞTİ: eskiden saklı token yoksa istek atılmazdı. Artık token bu sekmenin
  // bilmediği bir ÇEREZTE olabilir (başka sekmede giriş yapıldı, bu sekme yeni açıldı) —
  // istek atılmazsa sunucudaki aile canlı kalır.
  it("HINT yokken bile çıkış isteği atılır (gövdesiz, keepalive)", async () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    fetchMock.mockResolvedValueOnce(noContent());
    await api.logout();
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/auth/logout");
    expect(init.method).toBe("POST");
    expect(init.body).toBeUndefined();
    expect(init.credentials).toBe("same-origin");
    expect(init.keepalive).toBe(true);
    expect(seen).toEqual([null]);
    expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
  });

  it("başarılı çıkış HINT'i siler ve bekleyen çıkış bırakmaz", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(noContent());
    await api.logout();
    expect(localStorage.getItem(HINT)).toBeNull();
    expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
  });

  // Sunucuya ulaşamayan ya da iptali beceremeyen çıkış: yerel oturum kapanır ama sunucudaki
  // aile CANLI kalır — çalınmış bir çerez hâlâ işe yarardı. Bayrak + sınırlı geri çekilme ile
  // ısrar ediyoruz. ASIL İDDİA (sahte sunucunun çerez modeli sayesinde): yeniden deneme
  // ÇEREZLE gider ve aileyi GERÇEKTEN iptal eder. Sunucu 500'de silme çerezi yazsaydı yeniden
  // deneme çerezsiz gider, boş bir 204 alır ve aile sunucuda yaşamaya devam ederdi.
  it.each<[string, number, (srv: FakeLogoutServer) => Promise<Response>]>([
    // İstek sunucuya hiç ULAŞMADI: çerez de sunucudaki aile de yerinde (sunucu hatasız).
    ["ağ hatası", 0, () => Promise.reject(new TypeError("Failed to fetch"))],
    // Sunucu cevap verdi ama iptal edemedi (ör. DB kapalı): 500 + çerez KORUNUR.
    ["500", 1, (srv) => Promise.resolve(srv.handle())],
  ])("çıkış isteği başarısızsa (%s) LOGOUT_PENDING kalır ve yeniden deneme aileyi GERÇEKTEN iptal eder", async (_n, failures, firstAttempt) => {
    vi.useFakeTimers();
    try {
      const srv = fakeLogoutServer(failures);
      localStorage.setItem(HINT, "1");
      fetchMock.mockImplementationOnce(() => firstAttempt(srv));
      await api.logout();

      expect(localStorage.getItem(HINT)).toBeNull(); // yerel oturum yine de kapanır
      expect(localStorage.getItem(LOGOUT_PENDING)).toBe("1");
      expect(srv.state.revokes).toBe(0); // sunucuda henüz hiçbir şey iptal edilmedi

      fetchMock.mockImplementationOnce(() => Promise.resolve(srv.handle()));
      await vi.advanceTimersByTimeAsync(2_000);
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
      expect(srv.state.revokes).toBe(1); // aile öldü — bayrağı silmek artık dürüst
      expect(srv.state.hasCookie).toBe(false); // ve çerez ancak ŞİMDİ tarayıcıdan silindi
    } finally {
      vi.useRealTimers();
    }
  });

  // Yeniden deneme SONSUZ değil: üç denemeden sonra susar (bayrak kalır, açılışta denenir).
  it("yeniden denemeler sınırlıdır (3 deneme) ve sonrasında bayrak korunur", async () => {
    vi.useFakeTimers();
    try {
      fetchMock.mockResolvedValue(json(500, { error: { code: "internal", message: "" } }));
      await api.logout();
      await vi.advanceTimersByTimeAsync(120_000);
      expect(fetchMock).toHaveBeenCalledTimes(4); // ilk istek + 3 yeniden deneme
      expect(localStorage.getItem(LOGOUT_PENDING)).toBe("1");
    } finally {
      vi.useRealTimers();
    }
  });

  // Bekleyen çıkış, araya giren YENİ bir girişin çerezini öldürmemeli: giriş bayrağı siler,
  // zamanlayıcı da kilidin içinde bayrağa yeniden bakar.
  it("yeni giriş bekleyen çıkışı iptal eder", async () => {
    vi.useFakeTimers();
    try {
      fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
      await api.logout();
      expect(localStorage.getItem(LOGOUT_PENDING)).toBe("1");

      fetchMock.mockResolvedValueOnce(json(200, session(1)));
      await api.login("ali@example.com", "parola");
      expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();

      await vi.advanceTimersByTimeAsync(60_000);
      expect(fetchMock).toHaveBeenCalledTimes(2); // zamanlayıcı bir şey ATMADI
      expect(localStorage.getItem(HINT)).toBe("1");
    } finally {
      vi.useRealTimers();
    }
  });

  it("restore: Faz 13 öncesinden kalan localStorage token'ını siler", async () => {
    localStorage.setItem(LEGACY, "eski-refresh-token");
    expect(await api.restore()).toBeNull();
    expect(localStorage.getItem(LEGACY)).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled(); // HINT yok → istek de yok
  });

  it("restore: bekleyen çıkış varsa ÖNCE onu gönderir, oturum açmaz", async () => {
    localStorage.setItem(LOGOUT_PENDING, "1");
    localStorage.setItem(HINT, "1"); // tutarsız durum: çıkış yarıda kalmış
    fetchMock.mockResolvedValueOnce(noContent());

    expect(await api.restore()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/logout");
    expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
  });

  // Açılıştaki bekleyen-çıkış yolu, scheduleLogoutRetry'ın kilit İÇİ bayrak kontrolünü de
  // yapmalı: kilidi beklerken başka bir sekmede giriş yapıldıysa bayrak silinmiştir ve bu
  // istek artık YENİ oturumun çerezini öldürürdü. Kontrol kilitten önce yapılsaydı, üstelik
  // bu yol her sayfa yüklenişinde koştuğu için yarışı en sık burada kaybederdik.
  it("restore: kilidi beklerken giriş yapılmışsa bekleyen çıkışı GÖNDERMEZ", async () => {
    stubLocks();
    localStorage.setItem(LOGOUT_PENDING, "1");
    let resolveLogin!: (r: Response) => void;
    fetchMock
      .mockImplementationOnce(() => new Promise<Response>((r) => (resolveLogin = r))) // /login
      .mockResolvedValue(noContentClearingCookie()); // kaçak bir /logout olursa yakalansın

    const loggingIn = api.login("ali@example.com", "parola");
    await tick(); // giriş kilidi alsın ve isteğini atsın
    const restoring = api.restore(); // kilidin ARKASINA girer
    resolveLogin(json(200, session(1)));
    await loggingIn;

    expect(await restoring).toBeNull(); // bu yükleme oturumu görmez (bir sonrakinde görür)
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/login");
    expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
    expect(localStorage.getItem(HINT)).toBe("1"); // yeni oturum ayakta kaldı
  });

  // React StrictMode geliştirmede efektleri İKİ KEZ çalıştırır: restore() de iki kez çağrılır.
  // Kilit sıraya soksa bile ikincisi bayrağa yeniden bakmazsa, ilkinin iptal ettiği ailenin
  // ardından bir çıkış daha gider (en iyi ihtimalle boşuna bir istek, kötüsünde yeni oturum).
  it("restore: aynı anda iki kez çağrılırsa çıkış YALNIZCA BİR KEZ gönderilir", async () => {
    stubLocks();
    localStorage.setItem(LOGOUT_PENDING, "1");
    fetchMock.mockResolvedValue(noContentClearingCookie());

    expect(await Promise.all([api.restore(), api.restore()])).toEqual([null, null]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/logout");
    expect(localStorage.getItem(LOGOUT_PENDING)).toBeNull();
  });

  it("restore: bekleyen çıkış yine ulaşmazsa bayrak korunur", async () => {
    localStorage.setItem(LOGOUT_PENDING, "1");
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    expect(await api.restore()).toBeNull();
    expect(localStorage.getItem(LOGOUT_PENDING)).toBe("1");
  });

  it("restore: HINT yoksa hiç istek atmaz", async () => {
    expect(await api.restore()).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("restore: yenileme başarısızsa null döner, /me çağrılmaz", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } }));
    expect(await api.restore()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/refresh");
  });

  it("restore: yenileme başarılıysa kullanıcıyı GET /api/auth/me ile getirir", async () => {
    localStorage.setItem(HINT, "1");
    const me = session(1).user;
    fetchMock.mockResolvedValueOnce(json(200, session(1))).mockResolvedValueOnce(json(200, me));
    expect(await api.restore()).toEqual(me);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/me");
    expect(fetchMock.mock.calls[1][1].method).toBe("GET");
    expect((fetchMock.mock.calls[1][1].headers as Record<string, string>).Authorization).toBe("Bearer access-1");
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
    vi.restoreAllMocks();
    subscribeSession(() => {});
  });

  // Review bulgusu R3 (Faz 13'te anahtar değişti): başka sekme çıkış yapınca HINT silinir;
  // bu sekme de bellekteki access token'ı bırakmalı — yoksa 15 dk daha "girişli" görünürdü.
  it("başka sekmede çıkış bu sekmenin oturumunu da düşürür", () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: "1", newValue: null }));
    expect(seen).toEqual([null]);
  });

  it("başka sekmede giriş (newValue dolu) oturumu düşürmez", () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: HINT, oldValue: null, newValue: "1" }));
    expect(seen).toEqual([]);
  });

  it("başka bir anahtarın silinmesi oturumu düşürmez", () => {
    const seen: unknown[] = [];
    subscribeSession((u) => seen.push(u));
    window.dispatchEvent(new StorageEvent("storage", { key: LOGOUT_PENDING, oldValue: "1", newValue: null }));
    expect(seen).toEqual([]);
  });

  // localStorage YASAKLI olabilir (Safari özel pencere, "veri saklama" kapalı): erişimin
  // KENDİSİ fırlatır. Sarmalanmasaydı uygulama boş ekran verirdi.
  it("localStorage erişimi fırlatsa bile uygulama çalışır", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage yasak");
    });
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
      throw new Error("storage yasak");
    });

    fetchMock.mockResolvedValueOnce(json(200, { id: 5, title: "Dune" }));
    await expect(api.book(5)).resolves.toMatchObject({ id: 5 });

    // 401 yolu HINT'i OKUR: sarmalanmamış olsaydı burada ApiError yerine depolama hatası patlardı.
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "missing_token", message: "" } }));
    await expect(api.book(5)).rejects.toBeInstanceOf(ApiError);

    // Açılış: bayraklar okunamıyor → "oturum yok" varsayılır, istek atılmaz.
    expect(await api.restore()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("JSON olmayan hata gövdesi (NGINX 502 HTML) ApiError(status, unknown, statusText) olur", async () => {
    fetchMock.mockResolvedValueOnce(new Response("<html>502 Bad Gateway</html>", { status: 502, statusText: "Bad Gateway" }));
    const err = await request("GET", "/api/books").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 502, code: "unknown", message: "Bad Gateway" });
  });

  it("401 sonrası yenileme başarısızsa asıl 401 fırlatılır, tekrar denenmez", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock
      .mockResolvedValueOnce(json(401, { error: { code: "token_expired", message: "süre doldu" } }))
      .mockResolvedValueOnce(json(401, { error: { code: "invalid_token", message: "" } })); // /refresh
    const err = await request("GET", "/api/auth/me").catch((e) => e);
    expect(err).toMatchObject({ status: 401, code: "token_expired" });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/auth/refresh");
  });

  it("HINT yokken 401 alan istek /refresh çağırmaz", async () => {
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "missing_token", message: "" } }));
    await expect(request("GET", "/api/auth/me")).rejects.toMatchObject({ status: 401 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("yanlış parola (login 401) /refresh çağırmadan fırlatır", async () => {
    localStorage.setItem(HINT, "1");
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: "invalid_credentials", message: "e-posta ya da parola hatalı" } }));
    await expect(api.login("ali@example.com", "yanlis")).rejects.toMatchObject({ status: 401, code: "invalid_credentials" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/login");
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
    expect(init.credentials).toBe("same-origin");
    expect(init.body === undefined ? undefined : JSON.parse(init.body as string)).toEqual(body);
  });

  it("deleteComment 204'te undefined döner", async () => {
    fetchMock.mockResolvedValueOnce(noContent());
    expect(await api.deleteComment(9)).toBeUndefined();
  });

  it("register 401 alırsa (oturum ipucu olsa bile) /refresh denemez", async () => {
    localStorage.setItem(HINT, "1");
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
