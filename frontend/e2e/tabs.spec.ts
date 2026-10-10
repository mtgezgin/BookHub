// tabs.spec.ts — aynı tarayıcıda üç sekme: Web Locks yenilemeyi sıraya sokar.
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts.
//
// Refresh token TEK KULLANIMLIK ve tekrar kullanımı TÜM aileyi iptal ediyor. Aynı çerezi
// paylaşan üç sekme aynı anda açılırsa üçü de /api/auth/refresh'e gider: biri kazanır,
// diğer ikisinin isteği "reuse" sayılır ve kullanıcı üç sekmeden birden düşerdi. İstemci
// bunu Web Locks ile (origin başına tek sıra) engelliyor — bu senaryo tam olarak onun
// regresyon testi. Kilit kalkarsa burada deterministik olarak kırmızı yanar.
import { expect, test } from "@playwright/test";

import { READER_EMAIL, READER_NAME, readerPassword } from "./env";
import { expectLoggedIn, expectLoggedOut, openAccountMenu, topbar, uiLogin } from "./helpers";

test("üç sekme aynı çerezle yenilenir, oturum ayakta kalır; bir sekmedeki çıkış hepsini düşürür", async ({
  context,
  page,
}) => {
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);

  // Aynı context = aynı çerez kavanozu + aynı localStorage = aynı "tarayıcı".
  const second = await context.newPage();
  const third = await context.newPage();

  // İki yeni sekme AYNI ANDA açılır: ikisi de aynı (tek kullanımlık) çerezle restore eder.
  await Promise.all([second.goto("/"), third.goto("/")]);
  await expectLoggedIn(second, READER_NAME);
  await expectLoggedIn(third, READER_NAME);

  // Ve bir tur daha, bu kez ÜÇÜ birden: kilit olmasaydı en az ikisi reuse'a düşer,
  // aile iptal olur ve aşağıdaki iddia kırmızı yanardı.
  await Promise.all([page.reload(), second.reload(), third.reload()]);
  for (const tab of [page, second, third]) {
    await expectLoggedIn(tab, READER_NAME);
  }

  // Bir sekmedeki çıkış diğerlerine "storage" olayıyla duyurulur: YENİLEME YOK.
  // (Duyurulmasaydı diğer sekmeler bellekteki access token'la 15 dk daha "girişli"
  // görünürdü — sunucudaki oturum çoktan ölmüşken.)
  await openAccountMenu(page);
  await topbar(page).getByRole("button", { name: "Çıkış" }).click();
  await expectLoggedOut(page);
  await expectLoggedOut(second);
  await expectLoggedOut(third);

  // Çıkış HINT'i de sildi: sekmelerin hiçbiri artık boşuna /refresh denemez.
  expect(await second.evaluate(() => localStorage.getItem("bookhub.session"))).toBeNull();
});
