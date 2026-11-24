// library.spec.ts — kişisel kitaplık (Faz 15; Faz 17A-1'de raf kontrolüyle): ekle → "Zaten okudum" →
// katalog kartında rozet → Kitaplığım'da süz → çıkar; ayrıca 360 px genişlikte yatay taşma yok (UX-02, F13).
// Okuma akışı (başla → ilerle → 409 → bitir) ayrı bir senaryo olacak (17A-1 adım 7).
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts.
//
// Paylaşılan okur (e2e-reader-<RUN_ID>) her koşuda yeni açılır, yani kitaplığı boş başlar. CI'daki
// yeniden deneme ise AYNI hesabı kullanır: ilk deneme kitabı kitaplıkta bırakmış olabilir. Senaryo
// bu yüzden önce kitabı (varsa) çıkarır ve sonunda kitaplığı yine boş bırakır.
import { expect, test, type Page } from "@playwright/test";

import { READER_EMAIL, READER_NAME, readerPassword } from "./env";
import { expectLoggedIn, openAccountMenu, topbar, uiLogin } from "./helpers";

/** Demo katalogdaki (backend/scripts/demo_seed.sql) bir kitap. */
const BOOK = "Dune";

// Raf kontrolü (ShelfControl): durum rozetinde ekran okuyucu için "Kitaplığında: …" yazar; komutlar
// düğmelerde ve "Diğer seçenekler" menüsünde.
const shelfGroup = (page: Page) => page.getByRole("group", { name: "Kitaplık" });
const moreButton = (page: Page) => shelfGroup(page).getByRole("button", { name: "Diğer seçenekler" });
const addButton = (page: Page) => page.getByRole("button", { name: "Okumak istiyorum", exact: true });
const expectShelfStatus = (page: Page, label: string) => expect(shelfGroup(page)).toContainText(`Kitaplığında: ${label}`);

async function openBookFromCatalog(page: Page) {
  await page.getByPlaceholder("Başlık ya da yazar ara…").fill(BOOK);
  await page.getByRole("button", { name: "Ara" }).click();
  await page.getByRole("list", { name: "Kitaplar" }).getByRole("link", { name: BOOK, exact: true }).click();
  await expect(page.getByRole("heading", { name: BOOK, level: 1 })).toBeVisible();
}

async function removeFromLibrary(page: Page) {
  await moreButton(page).click();
  await page.getByRole("menuitem", { name: "Kitaplığımdan çıkar" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("silinmez");
  await dialog.getByRole("button", { name: "Çıkar", exact: true }).click();
  await expect(addButton(page)).toBeEnabled();
}

async function expectNoHorizontalOverflow(page: Page) {
  const { width, offenders } = await page.evaluate(() => {
    // Hata mesajı taşan öğeleri adıyla söylesin: "361" tek başına nereye bakılacağını söylemez.
    // Kendi içinde kaydırılan bir kabın (overflow-x: auto) çocukları taşma sayılmaz.
    const scrolls = (el: Element | null): boolean => {
      for (let p = el?.parentElement; p; p = p.parentElement) {
        if (getComputedStyle(p).overflowX !== "visible") return true;
      }
      return false;
    };
    const offenders = Array.from(document.body.querySelectorAll("*"))
      .filter((el) => el.getBoundingClientRect().right > window.innerWidth + 0.5 && !scrolls(el))
      .slice(0, 5)
      .map((el) => `${el.tagName.toLowerCase()}[${el.getAttribute("aria-label") ?? el.textContent?.trim().slice(0, 30)}]`);
    return { width: document.documentElement.scrollWidth, offenders };
  });
  expect(width, `sayfa 360 px'ten geniş, taşanlar: ${offenders.join(", ")}`).toBeLessThanOrEqual(360);
}

test("kitabı ekle, durumunu değiştir, rozeti gör, Kitaplığım'da süz ve çıkar", async ({ page }) => {
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);
  await openBookFromCatalog(page);

  // Durum yüklenene kadar düğmeler pasif; ikisinden biri etkinleşince kitabın yeri belli olur.
  await expect(moreButton(page)).toBeEnabled();
  if (!(await addButton(page).isVisible())) await removeFromLibrary(page);

  await addButton(page).click();
  await expectShelfStatus(page, "Okumak istiyorum");
  await moreButton(page).click();
  await page.getByRole("menuitem", { name: "Zaten okudum" }).click();
  await expectShelfStatus(page, "Okudum");

  // Katalog kartında rozet (durum ayrı, toplu bir istekle geliyor).
  await topbar(page).getByRole("link", { name: "BookHub" }).click();
  await page.getByPlaceholder("Başlık ya da yazar ara…").fill(BOOK);
  await page.getByRole("button", { name: "Ara" }).click();
  await expect(page.getByRole("list", { name: "Kitaplar" }).getByRole("listitem").filter({ hasText: BOOK })).toContainText("Okudum");

  // Kitaplığım: sekmeler adrese yazılır ve listeyi süzer.
  await topbar(page).getByRole("link", { name: "Kitaplığım" }).click();
  const shelf = page.getByRole("list", { name: "Kitaplığım" });
  await page.getByRole("group", { name: "Durum" }).getByRole("button", { name: "Okudum" }).click();
  await expect(page).toHaveURL(/status=READ\b/);
  await expect(shelf.getByRole("link", { name: BOOK, exact: true })).toBeVisible();
  await page.getByRole("group", { name: "Durum" }).getByRole("button", { name: "Şu an okuyorum" }).click();
  await expect(page.getByRole("heading", { name: "Sonuç bulunamadı" })).toBeVisible();
  await page.getByRole("group", { name: "Durum" }).getByRole("button", { name: "Okudum" }).click();
  await expect(shelf.getByRole("link", { name: BOOK, exact: true })).toBeVisible();

  // Dar ekran: Kitaplığım ve kitap sayfası yatay taşmasın.
  await page.setViewportSize({ width: 360, height: 800 });
  await expectNoHorizontalOverflow(page);
  await shelf.getByRole("link", { name: BOOK, exact: true }).click();
  await expect(page.getByRole("heading", { name: BOOK, level: 1 })).toBeVisible();
  await expectNoHorizontalOverflow(page);

  // Vazgeç sonrası sayfa tıklanabilir kalmalı: menüden açılan modal diyalog, kapanınca body'de
  // pointer-events: none bırakmamalı (Radix menü + diyalog etkileşimi). Ardından gerçekten çıkar.
  await moreButton(page).click();
  await page.getByRole("menuitem", { name: "Kitaplığımdan çıkar" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Vazgeç" }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expectShelfStatus(page, "Okudum");

  // Çıkar: kitap sayfasında, onayla. Dar ekranda Kitaplığım kullanıcı menüsünde.
  await removeFromLibrary(page);
  await openAccountMenu(page);
  await page.getByRole("link", { name: "Kitaplığım" }).click();
  await expect(page.getByRole("heading", { name: "Kitaplığın boş" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
});
