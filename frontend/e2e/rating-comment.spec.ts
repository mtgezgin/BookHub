// rating-comment.spec.ts — girişli kullanıcının kitaba puan vermesi ve yorum yazması.
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts.
//
// Kitaba aramayla gidiyoruz (goto ile doğrudan /books/<id> yerine): id demo veriye göre
// değişir, ayrıca SPA gezintisi olduğu için bellekteki access token korunur — tam sayfa
// yüklemesi olsaydı önce restore'un bitmesini beklemek gerekirdi.
import { expect, test } from "@playwright/test";

import { READER_EMAIL, READER_NAME, RUN_ID, readerPassword } from "./env";
import { expectLoggedIn, uiLogin } from "./helpers";

/** Demo katalogdaki (backend/scripts/demo_seed.sql) puanlanmış bir kitap. */
const BOOK = "Vakıf";

test("puan özeti anında güncellenir, yorum listenin başında görünür", async ({ page }) => {
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);

  await page.getByPlaceholder("Başlık ya da yazar ara…").fill(BOOK);
  await page.getByRole("button", { name: "Ara" }).click();

  // "Benim puanım" cevabını kitaba GİRMEDEN ÖNCE dinlemeye başla (istek, detay sayfası
  // açılır açılmaz gidiyor; dinleyici sonra kurulsaydı cevabı kaçırabilirdik).
  // NEDEN gerekli: paylaşılan okur (e2e-reader-<RUN_ID>) koşu boyunca TEK hesap ve CI'da
  // `retries: 1` var (playwright.config.ts). Puan (user_id, book_id) üzerinde UPSERT edilir
  // (backend/sql/queries/ratings.sql) → aynı okur ikinci kez puan verince oy SAYISI ARTMAZ.
  // Beklenti sabit "before + 1" olsaydı yeniden deneme aritmetik olarak İMKÂNSIZ bir iddiada
  // kesin olarak düşer, ilk denemedeki gerçek hatanın yerini alakasız bir mesaj alırdı.
  //
  // Bunu ".score-picker button.active" sayarak anlamak YANLIŞ olurdu: `mine` sayfaya
  // asenkron düşüyor (BookDetail.tsx useEffect → GET /api/books/{id}/rating), yani cevap
  // gelmeden sayım 0 okunur ve puanı OLAN okur "puanı yok" sanılırdı. Cevabın kendisi
  // kesin: 200 = puan var, 404 not_rated = yok (backend/cmd/api/community.go).
  // status() !== 401 şart: api/client.ts 401'i bir kez refreshSession ile yeniliyor ve
  // dinleyici asıl cevap yerine o ara 401'e takılabilirdi.
  const myRating = page.waitForResponse(
    (res) =>
      res.request().method() === "GET" &&
      /^\/api\/books\/\d+\/rating$/.test(new URL(res.url()).pathname) &&
      res.status() !== 401,
  );
  await page.locator(".grid").getByRole("link", { name: BOOK, exact: true }).click();
  await expect(page.getByRole("heading", { name: BOOK, level: 1 })).toBeVisible();
  const ratingRes = await myRating;
  const alreadyRated = ratingRes.ok();

  // Puanı ÖNCEKİNDEN FARKLI seç: yeniden denemede okur bu kitabı zaten puanlamıştır ve
  // sayfa, tıklamadan ÖNCE o puanı "active" basar — aynı puanı göndermek iddiayı boşa
  // çıkarırdı (tıklama/POST hiçbir şey yapmasa da geçerdi). Mevcut puanı DOM'dan değil
  // cevabın kendisinden okuyoruz: `mine` sayfaya asenkron düşer (yukarıdaki nota bak).
  const mine: number | null = alreadyRated ? (await ratingRes.json()).score : null;
  const score = mine === 8 ? 9 : 8;

  // Oy sayısını ÖNCEDEN oku: demo veri (ve önceki koşular) değiştikçe sabit bir sayı
  // tutmazdı; iddia bu denemenin KENDİ başlangıç durumuna göre kurulmalı.
  const summary = page.locator("span.rating");
  const before = voteCount(await summary.innerText());
  const expected = before + (alreadyRated ? 0 : 1); // UPSERT: zaten puanlıysa sayı artmaz

  const picker = page.locator(".score-picker");
  await picker.getByRole("button", { name: String(score), exact: true }).click();
  // Puan yazıldıktan sonra kitap + özet kaynaktan yeniden çekilir (ortalama türetilmiş veri).
  // Bu iddia artık yeniden denemede de AYIRT EDİCİ: tıklama/POST hiçbir şey yapmazsa
  // "active" eski puanda (8) kalır, beklenen ise 9'dur.
  await expect(picker.locator("button.active")).toHaveText(String(score));
  await expect(summary).toContainText(`(${expected} oy)`);

  // Yorum: içerik REACT'te metin olarak basılır — köşeli parantezli bir şey yazsak bile
  // çalışmaz, görünür (XSS'e karşı asıl duvar).
  const comment = `E2E yorumu ${RUN_ID}: puan verdim, yorumu da bırakıyorum.`;
  await page.getByPlaceholder("Düşüncelerini yaz…").fill(comment);
  await page.locator("form.comment-form").getByRole("button", { name: "Gönder" }).click();

  // Yorumlar en yeniden eskiye sıralı → yeni yorum ilk sayfanın başında.
  await expect(page.locator("ul.comments li").first()).toContainText(comment);
  await expect(page.getByPlaceholder("Düşüncelerini yaz…")).toHaveValue("");
});

/** "★ 8.12 (7 oy)" → 7 */
function voteCount(text: string): number {
  const match = /\((\d+) oy\)/.exec(text);
  expect(match, `puan özetinden oy sayısı okunamadı: ${text}`).not.toBeNull();
  return Number(match![1]);
}
