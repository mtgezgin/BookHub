// moderation.spec.ts — öneri → admin kuyruğu → onay / red.
// RATE LIMIT BÜTÇESİ: 2 login (okur + admin). Tablo: e2e/helpers.ts.
//
// Admin AYRI bir tarayıcı bağlamında: storageState paylaşmıyoruz. Paylaşsaydık iki bağlam
// AYNI refresh çerezini taşır, ikisi de yenilemeye kalkar ve sunucu bunu "tekrar kullanım"
// sayıp ailenin tamamını iptal ederdi — testler sebepsiz yere oturumdan düşerdi.
import { expect, test, type Page } from "@playwright/test";

import {
  ADMIN_NAME,
  READER_EMAIL,
  READER_NAME,
  RUN_ID,
  adminEmail,
  adminPassword,
  e2eTitlePrefix,
  readerPassword,
} from "./env";
import { expectLoggedIn, topbar, uiLogin } from "./helpers";

test("admin öneriyi onaylar, diğerini gerekçeyle reddeder", async ({ page, browser }) => {
  // Başlıklarda E2E_RUN_ID: arka arkaya koşular birbirinin kuyruğundaki satırı seçmesin.
  // DENEMEYE ÖZEL rastgelelik de şart (e2eTitlePrefix): CI'da `retries: 1` var ve yeniden
  // deneme AYNI E2E_RUN_ID ile koşar — başlık aynı kalsaydı slug da aynı olurdu, ilk
  // denemenin bıraktığı kitap yüzünden API 409 döner ve test ".success yok" diye gerçek
  // sebebi söylemeyen bir yerde patlardı (ayrıntı: e2e/env.ts).
  // ORTAK ÖNEK bilerek başta ve bitişik: arama ILIKE '%q%' ile çalışıyor, bu denemenin iki
  // kitabını tek sorguda bulabilmek için ortak parçanın bitişik olması gerekiyor.
  const prefix = e2eTitlePrefix();
  const approvedTitle = `${prefix} Onaylanan Kitap`;
  const rejectedTitle = `${prefix} Reddedilen Kitap`;
  const reason = `E2E gerekçesi ${RUN_ID}: demo veri`;

  // --- Okur iki kitap önerir (ikisi de PENDING doğar, katalogda görünmez) ---
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);
  await topbar(page).getByRole("link", { name: "Kitap öner" }).click();
  await suggest(page, approvedTitle);
  await suggest(page, rejectedTitle);

  // --- Admin ayrı bir bağlamda ---
  const adminContext = await browser.newContext();
  const admin = await adminContext.newPage();
  try {
    await uiLogin(admin, adminEmail(), adminPassword());
    await expectLoggedIn(admin, ADMIN_NAME);
    await topbar(admin).getByRole("link", { name: "Moderasyon" }).click();

    // Onayla: satır bekleyen kuyruğundan çıkar (liste yeniden yüklenir).
    await queueRow(admin, approvedTitle).getByRole("button", { name: "Onayla" }).click();
    await expect(queueRow(admin, approvedTitle)).toHaveCount(0);

    // Reddet, window.prompt ile gerekçe sorar. Dinleyici TIKLAMADAN ÖNCE kurulmalı:
    // Playwright, kimse dinlemiyorsa diyaloğu kendiliğinden REDDEDER (prompt null döner)
    // ve arayüz "vazgeçildi" sayıp hiçbir şey yapmazdı.
    admin.once("dialog", (dialog) => dialog.accept(reason));
    await queueRow(admin, rejectedTitle).getByRole("button", { name: "Reddet" }).click();
    await expect(queueRow(admin, rejectedTitle)).toHaveCount(0);

    // Gerekçe kayıtlı: red son duraktır, sebebi kuyruğun "Reddedilen" sekmesinde durur.
    await admin.getByRole("button", { name: "Reddedilen" }).click();
    await expect(queueRow(admin, rejectedTitle)).toContainText(`Red gerekçesi: ${reason}`);
  } finally {
    await adminContext.close();
  }

  // --- Onaylanan kitap artık ANONİM katalogda; reddedilen değil ---
  const visitorContext = await browser.newContext();
  const visitor = await visitorContext.newPage();
  try {
    await visitor.goto("/");
    await visitor.getByPlaceholder("Başlık ya da yazar ara…").fill(prefix);
    await visitor.getByRole("button", { name: "Ara" }).click();

    const grid = visitor.locator(".grid");
    await expect(grid).toContainText(approvedTitle);
    await expect(grid).not.toContainText(rejectedTitle);
  } finally {
    await visitorContext.close();
  }
});

/** Moderasyon kuyruğunda başlığı geçen satır. */
function queueRow(page: Page, title: string) {
  return page.locator("ul.queue li.card").filter({ hasText: title });
}

/** Öneri formunu doldurup gönderir (zaten /suggest sayfasındayız). */
async function suggest(page: Page, title: string): Promise<void> {
  await page.getByLabel("Başlık").fill(title);
  await page.getByLabel("Yazar").fill("E2E Yazar");
  await page.getByRole("button", { name: "Öner" }).click();
  await expect(page.locator(".success")).toContainText(title);
}
