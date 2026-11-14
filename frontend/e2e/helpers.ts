// e2e/helpers.ts — senaryoların paylaştığı arayüz adımları.
//
// ============================ RATE LIMIT BÜTÇESİ (SERT KURAL) ============================
// NGINX'te login ve register TEK bir kovayı paylaşır (frontend/nginx/default.conf,
// `zone=auth rate=20r/m burst=10 nodelay`): host'tan gelen tüm istekler konteyner
// açısından aynı adrestir (172.28.0.1). Kova DÜZ BİR TAVAN DEĞİL, sızdıran kova:
// ANINDA 11 istek (burst 10 + o anda dolu olan 1 jeton), sonra 3 saniyede 1 jeton daha.
// Dolum bu koşu için ihmal edilir (aşağıya bak): çalışılan tavan DÜZ 11'dir.
//
// (API'nin KENDİ sınırı ikinci bir katman; onu scripts/e2e.sh AUTH_RATE_LIMIT_PER_MIN=1000
//  ile yükseltiyor. NGINX'inki yükseltilemez — imajın içindeki yapılandırmadır — bu yüzden
//  asıl bütçe budur.)
//
// HER KOŞU BOŞ KOVAYLA BAŞLAR: scripts/e2e.sh, Playwright'i başlatmadan önce bookhub-web'i
// yeniden başlatır. limit_req'in sayacı nginx sürecinin paylaşılan belleğinde durur, yani
// restart onu siler ve önceki koşunun ne zaman bittiğinden bağımsız olarak tam 11 jetonla
// başlarız. (Bu olmadan arka arkaya iki "make e2e" ikincisinde deterministik 429 verirdi.)
//
// YENİDEN DENEME DE BÜTÇEDEN YER: playwright.config.ts `retries: process.env.CI ? 1 : 0`.
// Bir yeniden deneme TESTİ baştan koşar, yani o senaryonun tüm girişlerini tekrarlar —
// ve `retries` düşen HER testi ayrı ayrı tekrarlar: sistemik bir arızada giriş yapan tüm
// senaryolar tekrar eder (10 → 20). "Kararlı hâl + 4" bu yüzden en kötü durum DEĞİLDİR.
//
//   senaryo                              | :3000'e giden login/register
//   -------------------------------------|-----------------------------
//   global-setup (paylaşılan okur kaydı)  | 0  (NGINX'i atlar: DIRECT_API_URL)
//   catalog.spec                          | 0  (anonim)
//   auth-flow.spec                        | 4  (register + otomatik giriş + cihaz B + yeni parolayla giriş)
//   reuse.spec (çerez hijyeni dahil)      | 1
//   tabs.spec                             | 1
//   cross-origin.spec                     | 0  (yalnız /api/auth/refresh)
//   rating-comment.spec                   | 1
//   moderation.spec                       | 2  (okur + admin)
//   library.spec                          | 1
//   -------------------------------------|-----------------------------
//   KARARLI HÂL                           | 10
//   + CI'da bir yeniden deneme (auth-flow) | 14
//
// SERT KURAL — YALNIZ SAYI ÜZERİNDEN, DOLUM SIFIR SAYILIR:  kararlı_hâl ≤ 11  (bugün 10).
// Yeni bir senaryo eklerken önce bu tabloya bir satır ekle ve toplamı 11'in altında tut.
//
// Yeniden deneme marjını GARANTİ diye yazma: ölçülen koşu süresi `11 passed (~5-7 sn)` ve
// dahası 9 sınırlı isteğin TAMAMI ~3 sn'lik bir pencereye iniyor (paralel worker'lar işe
// girişte hemen giriş yapıyor) → kova dolumu pratikte +1 jeton, +4 değil. Yani 13 istek
// CI'nın yavaş koşusuna sığıyor olabilir, LOKAL hızda SIĞMAZ. (Ölçümü NGINX access log'undan
// al: `docker logs bookhub-web | grep 'POST /api/auth/\(login\|register\)'` — ilk ve son
// satırın saat farkı.) Bütçeyi tutan şey marj değil, her koşuda sıfırlanan kovadır.
//
// Sığmıyorsa yeni giriş yapma — var olan bir senaryoya iliştir ya da
// NGINX'i atlayıp doğrudan API'ye git (DIRECT_API_URL; yalnız tarayıcı/çerez semantiği
// gerektirmeyen saf API kontrolleri için, global-setup'taki gibi).
// =========================================================================================

import { expect, type Page, type Response } from "@playwright/test";

/** Kovayı tüketen İKİ yol. (refresh/logout/me ve /api/auth/password başka kovalardadır.) */
const LIMITED_PATHS = /^\/api\/auth\/(login|register)$/;

// Bu mesaj bir TRİPWIRE'dır: koşu başında kova sıfırlandığı için 429 artık "çok çabuk
// tekrar koştum" demek değil, "bu koşu 11 sınırlı isteği aştı" demektir. Sessizce yeniden
// deneme EKLEME — o, bütçe aşımını gizler (bkz. yukarıdaki SERT KURAL).
const RATE_LIMIT_HINT =
  "NGINX auth rate limit: bu koşu 11 login/register bütçesini aştı — kovayı scripts/e2e.sh " +
  "her koşuda sıfırlıyor, yani sorun zamanlama değil SAYI (bkz. helpers.ts'teki bütçe tablosu)";

/** Sunucunun cevabını "bu bir kota hatası mı" diye süzer: 429'un mesajı açık olsun. */
function assertNotRateLimited(res: Response, what: string): void {
  if (res.status() === 429) throw new Error(`${what}: 429 — ${RATE_LIMIT_HINT}`);
}

function isLimitedAuthResponse(res: Response, endpoint: "login" | "register"): boolean {
  const { pathname } = new URL(res.url());
  return LIMITED_PATHS.test(pathname) && pathname.endsWith(endpoint) && res.request().method() === "POST";
}

/**
 * Durum kodunu doğrular; YALNIZ beklenmedik durumda gövdeyi okur. Gövdeyi koşulsuz
 * okumak (expect'in mesajında `await res.text()`) her çağrıda gereksiz bir ağ/IPC turu
 * demekti — üstelik hata mesajı asıl işe yaradığı yerde zaten okunuyor.
 */
async function expectStatus(res: Response, expected: number, what: string): Promise<void> {
  if (res.status() === expected) return;
  expect(res.status(), `${what} — gövde: ${await res.text()}`).toBe(expected);
}

/** Üst çubuk: hem giriş durumunun hem de gezinti linklerinin tek kaynağı. */
export function topbar(page: Page) {
  // Rol + ad: görünüm (sınıflar) değişince kırılmaz; ekran okuyucunun gördüğü adla aynı (Faz 14).
  return page.getByRole("navigation", { name: "Ana menü" });
}

/** Kullanıcı menüsünün düğmesi (adı "Hesap menüsü: <ad>"); Hesabım ve Çıkış onun içinde. */
function accountMenuButton(page: Page) {
  return topbar(page).getByRole("button", { name: /^Hesap menüsü/ });
}

/** Kullanıcı menüsünü açar: Hesabım ve Çıkış menü kapalıyken sayfada yok. */
export async function openAccountMenu(page: Page): Promise<void> {
  await accountMenuButton(page).click();
}

/**
 * Girişli olmayı BEKLER (yalnız "kontrol etmez"): tam sayfa yüklemesinden sonra oturum
 * çerezle geri geliyor ve o sırada üst çubuk bir an "Giriş" gösteriyor. Adın görünmesini
 * beklemek, tasarımın "restore bitmeden ilerleme" kuralının pratikteki hâli.
 */
export async function expectLoggedIn(page: Page, displayName: string): Promise<void> {
  await expect(topbar(page).getByText(displayName, { exact: true })).toBeVisible();
}

/**
 * Girişsiz olmayı bekler. "Giriş linki var" TEK BAŞINA yetmez: restore henüz bitmemişken
 * de o link görünür (user hâlâ null) — yani iddia, daha oturum geri gelmeden YANLIŞLIKLA
 * yeşile dönebilirdi. Bu yüzden girişliyken görünen hesap menüsü düğmesinin YOK olduğunu da
 * şart koşuyoruz; çağıranlar ayrıca restore'un bittiğini (401 cevabı ya da HINT'in
 * silinmiş olması) kendi bağlamlarında doğruluyor.
 */
export async function expectLoggedOut(page: Page): Promise<void> {
  await expect(topbar(page).getByRole("link", { name: "Giriş", exact: true })).toBeVisible();
  await expect(accountMenuButton(page)).toBeHidden();
}

/**
 * Arayüzden giriş. Cevabı waitForResponse ile bekliyoruz: 429 ya da 401, "buton tıklandı
 * ama hiçbir şey olmadı" gibi görünmesin, ilgili satırda patlasın.
 * BÜTÇE: her çağrı 1 sınırlı istek.
 */
export async function uiLogin(page: Page, email: string, password: string): Promise<Response> {
  await page.goto("/login");
  await page.getByLabel("E-posta").fill(email);
  // exact: "Parolayı göster" düğmesinin adı da "Parola" ile başlıyor (Faz 14 parola alanı).
  await page.getByLabel("Parola", { exact: true }).fill(password);

  // waitForResponse TIKLAMADAN ÖNCE kurulur: cevap, dinleyici takılmadan dönebilir.
  const login = page.waitForResponse((res) => isLimitedAuthResponse(res, "login"));
  await page.getByRole("button", { name: "Giriş", exact: true }).click();
  const res = await login;

  assertNotRateLimited(res, `giriş (${email})`);
  await expectStatus(res, 200, `giriş 200 dönmeliydi (${email})`);
  return res;
}

/**
 * Arayüzden kayıt. Kayıt sayfası kaydın ardından KENDİLİĞİNDEN giriş de yapar (Auth.tsx)
 * → BÜTÇE: her çağrı 2 sınırlı istek.
 */
export async function uiRegister(
  page: Page,
  user: { email: string; name: string; password: string },
): Promise<void> {
  await page.goto("/register");
  await page.getByLabel("E-posta").fill(user.email);
  await page.getByLabel("Görünen ad").fill(user.name);
  await page.getByLabel("Parola (en az 8 karakter)", { exact: true }).fill(user.password);

  const register = page.waitForResponse((res) => isLimitedAuthResponse(res, "register"));
  const login = page.waitForResponse((res) => isLimitedAuthResponse(res, "login"));
  await page.getByRole("button", { name: "Kayıt ol" }).click();

  const registerRes = await register;
  assertNotRateLimited(registerRes, `kayıt (${user.email})`);
  await expectStatus(registerRes, 201, `kayıt 201 dönmeliydi (${user.email})`);

  const loginRes = await login;
  assertNotRateLimited(loginRes, `kayıt sonrası giriş (${user.email})`);
  await expectStatus(loginRes, 200, `kayıt sonrası giriş 200 dönmeliydi (${user.email})`);
}

/**
 * Sayfayı yeniler ve oturumun SUNUCUDA öldüğünü doğrular. Yenilemeden ÖNCE dinleyiciyi
 * kuruyoruz ve 401'i bekliyoruz: "Giriş linki göründü" iddiası tek başına, restore daha
 * bitmemişken de doğru olurdu (bkz. expectLoggedOut).
 */
export async function reloadAndExpectSessionRejected(page: Page): Promise<void> {
  const refresh = page.waitForResponse((res) => new URL(res.url()).pathname === "/api/auth/refresh");
  await page.reload();
  const res = await refresh;
  expect(res.status(), "sunucu bu oturumu artık tanımamalıydı").toBe(401);
  await expectLoggedOut(page);
}
