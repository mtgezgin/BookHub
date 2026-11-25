// library.spec.ts — kişisel kitaplık ve okuma takibi, İKİ senaryo TEK girişle:
//  1. Kitaplık (Faz 15; Faz 17A-1'de raf kontrolüyle): ekle → "Zaten okudum" → katalog kartında rozet →
//     Kitaplığım'da süz → çıkar; 360 px genişlikte yatay taşma yok (UX-02, F13).
//  2. Okuma (Faz 17A-1): başla → ilerle → İKİ SEKMEDE 409 → yarım bırak → devam et → Kitaplığım'da
//     ilerleme → bitir → "Puan ver" (spec Doğrulama 3, 6, 10'un yerel karşılığı).
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts. İkinci senaryo ayrı bir dosya olsaydı kendi
// girişini yapar ve bütçeyi tavana (11) dayardı; bu yüzden aynı dosyada, aynı sayfayı paylaşarak
// (describe.serial + beforeAll'da tek giriş). İkinci sekme aynı tarayıcı bağlamında açılır: oturum
// çerezle gelir, giriş harcamaz.
//
// Paylaşılan okur (e2e-reader-<RUN_ID>) her koşuda yeni açılır, yani kitaplığı boş başlar. CI'daki
// yeniden deneme ise AYNI hesabı kullanır: ilk deneme kitabı kitaplıkta bırakmış olabilir. Senaryolar
// bu yüzden önce kitabı (varsa) çıkarır ve sonunda kitaplığı yine boş bırakır. Okuma geçmişi çıkarmakla
// silinmez (spec § Kararlar): iddialar geçmişin tamamına değil, en yeni satıra bakar.
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

// İki senaryo aynı sayfayı paylaşır: biri düşerse öteki anlamsız (ve yeniden deneme ikisini birlikte
// koşar, girişi de bir kez tekrarlar; bütçe tablosundaki "yeniden deneme" satırıyla aynı hesap).
test.describe.configure({ mode: "serial" });

let page: Page;

test.beforeAll(async ({ browser }) => {
  // browser fixture'ı Playwright Test'te config'deki `use` ayarlarıyla (baseURL dahil) bağlam açar.
  // Bağlam AÇIKÇA: browser.newPage()'in kendi bağlamına ikinci sekme eklenemiyor (okuma senaryosu ekler).
  const context = await browser.newContext();
  page = await context.newPage();
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);
});

test.afterAll(async () => {
  await page.context().close();
});

test("kitabı ekle, durumunu değiştir, rozeti gör, Kitaplığım'da süz ve çıkar", async () => {
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

// ------------------------------------------------------------------------------------------------
// Okuma (Faz 17A-1)

const progressBar = (tab: Page) => shelfGroup(tab).getByRole("progressbar", { name: "Okuma ilerlemen" });
const progressForm = (tab: Page) => tab.getByRole("form", { name: "İlerleme" });
const historyRows = (tab: Page) => tab.getByRole("region", { name: "Okuma geçmişin" }).getByRole("listitem");

async function saveProgress(tab: Page, value: number) {
  await shelfGroup(tab).getByRole("button", { name: "İlerlemeyi güncelle" }).click();
  await progressForm(tab).getByLabel("Kaçıncı sayfadasın?").fill(String(value));
  await progressForm(tab).getByRole("button", { name: "Kaydet", exact: true }).click();
  await expect(progressForm(tab)).toBeHidden();
}

test("okuma: başla, ilerle, iki sekmede 409, yarım bırak, devam et, Kitaplığım'da ilerleme, bitir", async () => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/");
  await expectLoggedIn(page, READER_NAME);
  await openBookFromCatalog(page);
  await expect(moreButton(page)).toBeEnabled();
  if (!(await addButton(page).isVisible())) await removeFromLibrary(page);

  // Başla: tek tık, varsayılan baskı, birim sayfa (Dune'un sayfa toplamı biliniyor).
  await moreButton(page).click();
  await page.getByRole("menuitem", { name: "Okumaya başla" }).click();
  await expectShelfStatus(page, "Şu an okuyorum");
  await expect(progressBar(page)).toHaveAttribute("aria-valuetext", /^0 \/ \d+ sayfa$/);
  const total = Number((await progressBar(page).getAttribute("aria-valuetext"))!.match(/\/ (\d+)/)![1]);
  const pages = (n: number) => `${n} / ${total} sayfa`;

  await saveProgress(page, 42);
  await expect(progressBar(page)).toHaveAttribute("aria-valuetext", pages(42));

  // İKİ SEKME (spec Doğrulama 3): A formu açar ve yazar; B bu arada kaydeder; A gönderince sürümü eskidir.
  const second = await page.context().newPage();
  await second.goto(page.url());
  await expect(progressBar(second)).toHaveAttribute("aria-valuetext", pages(42));

  await shelfGroup(page).getByRole("button", { name: "İlerlemeyi güncelle" }).click();
  await progressForm(page).getByLabel("Kaçıncı sayfadasın?").fill("50");
  await saveProgress(second, 57);
  await expect(progressBar(second)).toHaveAttribute("aria-valuetext", pages(57));

  await progressForm(page).getByRole("button", { name: "Kaydet", exact: true }).click();
  // 409: satır değişmedi (B'nin 57'si duruyor), A öteki sekmenin değerini görür, kendi 50'si korunur.
  await expect(progressForm(page).getByRole("alert")).toContainText(`İlerlemen başka bir yerde güncellenmiş: ${pages(57)}`);
  await expect(progressForm(page).getByLabel("Kaçıncı sayfadasın?")).toHaveValue("50");
  await second.reload();
  await expect(progressBar(second)).toHaveAttribute("aria-valuetext", pages(57));

  // Kullanıcı karar verir: yeni sürümle yeniden gönderir.
  await progressForm(page).getByRole("button", { name: "Benimkini kaydet" }).click();
  await expect(progressBar(page)).toHaveAttribute("aria-valuetext", pages(50));
  await second.reload();
  await expect(progressBar(second)).toHaveAttribute("aria-valuetext", pages(50));
  await second.close();

  // Yarım bırak → Devam et (Doğrulama 6): sayfa geri gelir, günlük satırı kalmaz.
  await moreButton(page).click();
  await page.getByRole("menuitem", { name: "Yarım bıraktım" }).click();
  await expectShelfStatus(page, "Yarım bıraktım");
  await expect(shelfGroup(page)).toContainText(`Yarım bıraktın · ${pages(50)}`);
  await expect(historyRows(page).first()).toContainText(`Yarım bıraktın · ${pages(50)}`);

  await shelfGroup(page).getByRole("button", { name: "Devam et" }).click();
  await expectShelfStatus(page, "Şu an okuyorum");
  await expect(progressBar(page)).toHaveAttribute("aria-valuetext", pages(50));
  await expect(historyRows(page).filter({ hasText: `Yarım bıraktın · ${pages(50)}` })).toHaveCount(0);

  // Dar ekran: okurken (çubuk, açık form, geçmiş) kitap sayfası yatay taşmasın.
  await page.setViewportSize({ width: 360, height: 800 });
  await shelfGroup(page).getByRole("button", { name: "İlerlemeyi güncelle" }).click();
  await expectNoHorizontalOverflow(page);
  await progressForm(page).getByRole("button", { name: "Vazgeç" }).click();
  await page.setViewportSize({ width: 1280, height: 800 });

  // Kitaplığım: "Şu an okuyorum" kartında aynı ilerleme (adım 6).
  await topbar(page).getByRole("link", { name: "Kitaplığım" }).click();
  await page.getByRole("group", { name: "Durum" }).getByRole("button", { name: "Şu an okuyorum" }).click();
  const card = page.getByRole("list", { name: "Kitaplığım" }).getByRole("listitem").filter({ hasText: BOOK });
  await expect(card.getByRole("progressbar", { name: `${BOOK}: okuma ilerlemen` })).toHaveAttribute("aria-valuetext", pages(50));
  await card.getByRole("link", { name: BOOK, exact: true }).click();

  // Bitir: bugünün yerel günüyle; puanı olmayan kitapta "Puan ver" puan seçiciye götürür.
  await shelfGroup(page).getByRole("button", { name: "Bitirdim" }).click();
  await expectShelfStatus(page, "Okudum");
  await expect(shelfGroup(page)).toContainText("Bugün bitirdin");
  await expect(historyRows(page).first()).toContainText("Bitirdin");
  await shelfGroup(page).getByRole("button", { name: "Puan ver" }).click();
  await expect(page.getByRole("group", { name: "Senin puanın" }).getByRole("button").first()).toBeFocused();

  // Kitaplığı boş bırak (geçmiş kalır).
  await removeFromLibrary(page);
  await expect(historyRows(page).first()).toContainText("Bitirdin");
});
