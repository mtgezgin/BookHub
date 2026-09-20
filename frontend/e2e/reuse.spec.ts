// reuse.spec.ts — çerez hijyeni + kullanılmış refresh token'ın TÜM aileyi iptal etmesi.
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts.
//
// Çerez hijyeni iddiaları buraya KATLANDI (ayrı bir senaryo + ayrı bir giriş olsaydı bütçe
// 11'i aşardı): zaten aynı girişin cevabına bakıyoruz.
//
// Reuse tespitinde ayırt edici iddia ikinci değil ÜÇÜNCÜ istek: harcanmış C1'i tekrar
// yollamak reuse tespiti OLMADAN da 401 alırdı (token zaten tüketilmiş). Asıl kanıt,
// C1'in tekrarından SONRA hâlâ taptaze olan HALEFİN (C2) de reddedilmesidir — bu ancak
// ailenin tamamı iptal edildiyse olur.
import { expect, test, type PlaywrightWorkerArgs } from "@playwright/test";

import { BASE_URL, READER_EMAIL, READER_NAME, REFRESH_COOKIE, readerPassword } from "./env";
import { expectLoggedIn, uiLogin } from "./helpers";

/** `playwright` fixture'ının tipi (modülün kendisi; request.newContext buradan gelir). */
type PlaywrightModule = PlaywrightWorkerArgs["playwright"];

/** Faz 13 öncesinden kalma, localStorage'daki refresh token anahtarı. */
const LEGACY_KEY = "bookhub.refresh";
/** Sır olmayan tek kalıcı iz: "bu tarayıcıda oturum açık olmalı". */
const HINT_KEY = "bookhub.session";

/**
 * Çerezi AÇIKÇA başlıkla yollayarak /api/auth/refresh'i dener.
 *
 * Her seferinde TAZE bir istek bağlamı: Playwright'in çerez kavanozu bir önceki cevabın
 * Set-Cookie'sini saklar ve isteğe İKİNCİ bir bookhub_refresh eklerdi. Backend aynı adlı
 * iki çerezi (gölgeleme girişimi) zaten reddediyor → 401 gelirdi ama YANLIŞ sebepten:
 * "aile iptal edildi" iddiası sessizce anlamsızlaşırdı.
 *
 * context.addCookies({url}) de BİLEREK kullanılmıyor: url'li ekleme Secure/HttpOnly
 * kurallarına takılabilir ve neyin gönderildiğini belirsizleştirir.
 */
async function refreshWithCookie(playwright: PlaywrightModule, token: string) {
  const api = await playwright.request.newContext({ baseURL: BASE_URL });
  try {
    const res = await api.post("/api/auth/refresh", {
      headers: { cookie: `${REFRESH_COOKIE}=${token}` },
    });
    const setCookies = res
      .headersArray()
      .filter((h) => h.name.toLowerCase() === "set-cookie")
      .map((h) => h.value);
    return { status: res.status(), token: parseRefreshCookie(setCookies) };
  } finally {
    await api.dispose();
  }
}

/** Set-Cookie başlıklarından refresh çerezinin DEĞERİNİ çıkarır (yoksa null). */
function parseRefreshCookie(setCookies: string[]): string | null {
  for (const raw of setCookies) {
    const [pair] = raw.split(";");
    const [name, ...rest] = pair.split("=");
    if (name.trim() === REFRESH_COOKIE) {
      const value = rest.join("=").trim();
      return value === "" ? null : value; // boş değer = silme çerezi
    }
  }
  return null;
}

test("refresh çerezi hijyenik ve harcanmış token tüm aileyi iptal eder", async ({
  page,
  context,
  playwright,
}) => {
  // Taşımanın kanıtı: eski sürümden kalmış bir refresh token'ı her açılışta yerleştir;
  // uygulamanın açılış yolu (api.restore) onu SİLMELİ — localStorage'da sır kalmasın.
  await context.addInitScript(
    ([key, value]) => {
      try {
        localStorage.setItem(key, value);
      } catch {
        /* storage yasaklıysa zaten sır da yok */
      }
    },
    [LEGACY_KEY, "faz-13-oncesinden-kalan-token"],
  );

  const loginResponse = await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);

  // (1) Cevap gövdesinde refresh_token YOK. Dönseydi httpOnly çerezin tüm anlamı giderdi:
  // XSS'li bir betik gövdeyi okuyup uzun ömürlü sırrı dışarı taşıyabilirdi.
  const body = await loginResponse.json();
  expect(body.access_token, "access token cevapta olmalı").toBeTruthy();
  expect(body.refresh_token, "refresh_token cevap gövdesinde OLMAMALI").toBeUndefined();
  expect(body.refresh_expires_at, "oturumun bitişi sır değil, kalmalı").toBeTruthy();

  // (2) Çerezin öznitelikleri. context.cookies() URL'SİZ çağrılıyor: URL verilseydi
  // tarayıcı yalnız o URL'e GÖNDERECEĞİ çerezleri döndürürdü ve Path=/api/auth olan
  // çerez "/" için listeye hiç girmezdi.
  const refresh = (await context.cookies()).find((c) => c.name === REFRESH_COOKIE);
  expect(refresh, `${REFRESH_COOKIE} çerezi yazılmalıydı`).toBeDefined();
  expect(refresh!.httpOnly, "HttpOnly: document.cookie görmesin").toBe(true);
  expect(refresh!.secure, "Secure: düz HTTP'de ağa çıkmasın").toBe(true);
  expect(refresh!.sameSite, "SameSite=Strict: başka siteden başlayan istekte gitmesin").toBe("Strict");
  expect(refresh!.path, "Path: yalnız /api/auth").toBe("/api/auth");

  // (3) localStorage'da SIR YOK: yalnız ipucu bayrağı. Legacy anahtar silinmiş olmalı.
  const stored = await page.evaluate(() => ({ ...localStorage }));
  expect(stored).toEqual({ [HINT_KEY]: "1" });

  // (4) Rotasyon: C1 bir kez geçer ve YENİ bir çerez (C2) döner.
  const c1 = refresh!.value;
  const rotated = await refreshWithCookie(playwright, c1);
  expect(rotated.status, "geçerli çerezle yenileme 200 olmalı").toBe(200);
  expect(rotated.token, "yenileme yeni bir çerez yazmalı").toBeTruthy();
  expect(rotated.token, "tek kullanımlık: halef eskisinden FARKLI olmalı").not.toBe(c1);

  // (5) Harcanmış C1 tekrar → 401. Bu tek başına AYIRT EDİCİ DEĞİL (reuse tespiti
  // olmadan da 401 gelirdi); asıl işlevi bir sonraki adımdaki tuzağı kurmak.
  const replayed = await refreshWithCookie(playwright, c1);
  expect(replayed.status, "harcanmış token reddedilmeli").toBe(401);

  // (6) AYIRT EDİCİ: halef C2 de artık ölü. Tek kullanımlık token'ın "tekrar kullanıldığı"
  // görüldüğü an sunucu AİLENİN TAMAMINI iptal ediyor — çalınmış bir çerezle yarışan
  // saldırgan, meşru kullanıcının oturumunu da kendiyle birlikte öldürür (ve kullanıcı
  // bir sonraki adımda yeniden giriş yapmak zorunda kalarak durumu fark eder).
  const successor = await refreshWithCookie(playwright, rotated.token!);
  expect(successor.status, "reuse tespiti ailenin HALEFİNİ de öldürmeliydi").toBe(401);
});
