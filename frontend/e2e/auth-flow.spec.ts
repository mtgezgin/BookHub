// auth-flow.spec.ts — oturumun tam yaşam döngüsü, gerçek çerezle.
// RATE LIMIT BÜTÇESİ: 4 login/register (kayıt 1 + kayıt sonrası otomatik giriş 1 +
// cihaz B'nin girişi 1 + yeni parolayla giriş 1). Tablo: e2e/helpers.ts.
//
// Burada sınanan asıl şey: access token YALNIZCA bellekte, refresh token httpOnly çerezte.
// Yani "sayfayı yenile, hâlâ girişli ol" iddiası doğrudan doğruya çerezin ve /api/auth/refresh
// rotasyonunun kanıtıdır — localStorage'da taşınan bir sır olsaydı bu test onu da geçerdi,
// o yüzden çerez hijyeni ayrıca reuse.spec.ts'te doğrulanıyor.
import { expect, test } from "@playwright/test";

import { DIRECT_API_URL, e2eEmail, randomPassword } from "./env";
import { expectLoggedIn, expectLoggedOut, reloadAndExpectSessionRejected, topbar, uiLogin, uiRegister } from "./helpers";

test("kayıt → yenile → ikinci cihaz → parola değişimi → yalnız bu cihaz ayakta kalır", async ({
  page,
  browser,
  playwright,
}) => {
  const email = e2eEmail("authflow");
  const name = "E2E Auth";
  const oldPassword = randomPassword();
  const newPassword = randomPassword();

  // --- Cihaz A: kayıt (sayfa kaydın ardından kendiliğinden giriş de yapar) ---
  await uiRegister(page, { email, name, password: oldPassword });
  await expectLoggedIn(page, name);

  // Tam sayfa yüklemesi: bellekteki access token ÖLÜR. Hâlâ girişliysek oturumu geri
  // getiren tek şey httpOnly çerezdir.
  await page.reload();
  await expectLoggedIn(page, name);

  // --- Cihaz B: ayrı bir tarayıcı bağlamı = ayrı çerez kavanozu = AYRI oturum ailesi ---
  const deviceB = await browser.newContext();
  const pageB = await deviceB.newPage();
  await uiLogin(pageB, email, oldPassword);
  await expectLoggedIn(pageB, name);

  // --- Cihaz A'da parola değişimi ---
  // "Hesabım" linki SPA gezintisi: tam yükleme olmadığı için access token bellekte duruyor.
  await topbar(page).getByRole("link", { name: "Hesabım", exact: true }).click();
  await page.getByLabel("Mevcut parola").fill(oldPassword);
  await page.getByLabel("Yeni parola (en az 8 karakter)", { exact: true }).fill(newPassword);
  await page.getByLabel("Yeni parola (tekrar)", { exact: true }).fill(newPassword);

  const changed = page.waitForResponse((res) => new URL(res.url()).pathname === "/api/auth/password");
  await page.getByRole("button", { name: "Parolayı değiştir" }).click();
  expect((await changed).status(), "parola değişimi 200 dönmeliydi").toBe(200);
  await expect(page.getByRole("status")).toContainText("Diğer tüm cihazlardaki oturumlar kapatıldı");

  // --- Cihaz B düşmeli: parola değişimi kullanıcının TÜM ailelerini iptal etti ---
  await reloadAndExpectSessionRejected(pageB);

  // --- Cihaz A ayakta: değişim AYNI transaction'da bu cihaza yeni bir aile açtı ---
  await page.reload();
  await expectLoggedIn(page, name);

  // --- Çıkış: yerel oturum anında kapanır ---
  await topbar(page).getByRole("button", { name: "Çıkış" }).click();
  await expectLoggedOut(page);

  // Çıkıştan sonra HINT de silinmiş olmalı: yenilemede istemci /refresh'e hiç gitmez
  // (anonim ziyaretçi boşuna bir istek atıp kovadan kredi yemesin).
  await page.reload();
  await expectLoggedOut(page);
  expect(await page.evaluate(() => localStorage.getItem("bookhub.session"))).toBeNull();

  // --- ESKİ parola artık geçersiz ---
  // NGINX'i ATLAYIP doğrudan API'ye: bu kontrolün başarısız olması BEKLENİYOR ve sınırlı
  // login kovasından bir kredi yemesinin anlamı yok (bütçe tablosu: helpers.ts).
  const direct = await playwright.request.newContext({ baseURL: DIRECT_API_URL });
  try {
    const rejected = await direct.post("/api/auth/login", { data: { email, password: oldPassword } });
    expect(rejected.status(), "eski parola reddedilmeliydi").toBe(401);
  } finally {
    await direct.dispose();
  }

  // --- YENİ parolayla giriş çalışır ---
  await uiLogin(page, email, newPassword);
  await expectLoggedIn(page, name);

  await deviceB.close();
});
