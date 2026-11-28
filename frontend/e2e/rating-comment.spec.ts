// rating-comment.spec.ts — kitap sayfasında topluluk, İKİ senaryo TEK girişle:
//  1. Puan ve yorum (Faz 5).
//  2. İnceleme (Faz 17B): yaz, düzenle, iki sekmede 409, gizlenince gerekçe ve gizli kalan düzenleme,
//     spoiler'ın klavyeyle açılması ve axe taraması, silince puanın kalması (spec Doğrulama 12, 14–16).
//     Faz 17C: zengin metin düzenleyicisi yalnız klavyeyle (araç çubuğu dahil, klavye tuzağı yok),
//     Markdown kısayolu, yapıştırılan HTML'in düz metne inmesi, parça parça spoiler (#128 kriter 2, 7).
// RATE LIMIT BÜTÇESİ: 1 login. Tablo: e2e/helpers.ts. İnceleme senaryosu ayrı bir dosya olsaydı kendi
// girişini yapar ve bütçeyi tavana (11) dayardı; library.spec'teki gibi aynı sayfayı paylaşır. Yönetici
// tarafı (öteki okurun spoiler'lı incelemesi, gizleme, geri açma) NGINX'i atlayan doğrudan API
// çağrılarıyla kurulur (global-setup'taki gibi): sınırlı kovadan kredi yemez. Gizleme diyaloğunun kendisi
// bileşen testinde (ReviewSection.test.tsx) ve canlı doğrulamada.
//
// Kitaba aramayla gidiyoruz (goto ile doğrudan /books/<id> yerine): id demo veriye göre
// değişir, ayrıca SPA gezintisi olduğu için bellekteki access token korunur — tam sayfa
// yüklemesi olsaydı önce restore'un bitmesini beklemek gerekirdi.
import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type APIRequestContext, type APIResponse, type Page } from "@playwright/test";

import { DIRECT_API_URL, READER_EMAIL, READER_NAME, RUN_ID, adminEmail, adminPassword, readerPassword } from "./env";
import { expectLoggedIn, topbar, uiLogin } from "./helpers";

/** Demo katalogdaki (backend/scripts/demo_seed.sql) puanlanmış bir kitap. */
const BOOK = "Vakıf";

// İki senaryo aynı sayfayı paylaşır; yeniden deneme ikisini birlikte koşar ve girişi bir kez tekrarlar.
test.describe.configure({ mode: "serial" });

let page: Page;

test.beforeAll(async ({ browser }) => {
  // Bağlam AÇIKÇA: inceleme senaryosu aynı bağlamda ikinci sekme açar (oturum çerezle gelir).
  const context = await browser.newContext();
  page = await context.newPage();
  await uiLogin(page, READER_EMAIL, readerPassword());
  await expectLoggedIn(page, READER_NAME);
});

test.afterAll(async () => {
  await page?.context().close(); // giriş öncesinde düştüyse sayfa hiç açılmamış olabilir
});

test("puan özeti anında güncellenir, yorum listenin başında görünür", async () => {
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
  // Bunu basılı puan düğmesine (aria-pressed) bakarak anlamak YANLIŞ olurdu: `mine` sayfaya
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
  await page.getByRole("list", { name: "Kitaplar" }).getByRole("link", { name: BOOK, exact: true }).click();
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
  // "Puan" bölümünde "(n oy)" yalnız özette geçer; dağılımın ekran okuyucu metni "8 puan: 3 oy" biçiminde.
  const summary = page.getByRole("region", { name: "Puan" });
  const before = voteCount(await summary.innerText());
  const expected = before + (alreadyRated ? 0 : 1); // UPSERT: zaten puanlıysa sayı artmaz

  const picker = page.getByRole("group", { name: "Senin puanın" });
  await picker.getByRole("button", { name: String(score), exact: true }).click();
  // Puan yazıldıktan sonra kitap + özet kaynaktan yeniden çekilir (ortalama türetilmiş veri).
  // Bu iddia artık yeniden denemede de AYIRT EDİCİ: tıklama/POST hiçbir şey yapmazsa
  // "active" eski puanda (8) kalır, beklenen ise 9'dur.
  await expect(picker.locator("button[aria-pressed=true]")).toHaveText(String(score));
  await expect(summary).toContainText(`(${expected} oy)`);
  await expect(page.getByRole("region", { name: "Bildirimler" })).toContainText(`Puanın kaydedildi: ${score}/10`);

  // Yorum: içerik REACT'te metin olarak basılır — köşeli parantezli bir şey yazsak bile
  // çalışmaz, görünür (XSS'e karşı asıl duvar).
  const comment = `E2E yorumu ${RUN_ID}: puan verdim, yorumu da bırakıyorum.`;
  await page.getByPlaceholder("Düşüncelerini yaz…").fill(comment);
  await page.getByRole("form", { name: "Yorum yaz" }).getByRole("button", { name: "Gönder" }).click();

  // Yorumlar en yeniden eskiye sıralı → yeni yorum ilk sayfanın başında.
  await expect(page.getByRole("list", { name: "Yorumlar" }).getByRole("listitem").first()).toContainText(comment);
  await expect(page.getByPlaceholder("Düşüncelerini yaz…")).toHaveValue("");
});

/** "★ 8.12 (7 oy)" → 7 */
function voteCount(text: string): number {
  const match = /\((\d+) oy\)/.exec(text);
  expect(match, `puan özetinden oy sayısı okunamadı: ${text}`).not.toBeNull();
  return Number(match![1]);
}

// ---------------------------------------------------------------------------------------------
// İnceleme (Faz 17B)
// ---------------------------------------------------------------------------------------------

/** Öteki okurun (yönetici hesabı) spoiler'lı incelemesinin metni: açılana kadar sayfada OLMAMALI. */
const SPOILER = `E2E spoiler ${RUN_ID}: sonunda herkes ölüyor`;
const HIDE_REASON = `E2E gizleme gerekçesi ${RUN_ID}`;
/** Öteki okurun incelemesindeki TEK spoiler parçası (Faz 17C): gerisi açık, bu açılana kadar sayfada yok. */
const PIECE = `E2E parça ${RUN_ID}: katil uşak`;

test("inceleme: yaz, düzenle, iki sekmede 409, gizlenince gerekçe, spoiler klavyeyle ve axe, sil", async () => {
  await openBook(page);
  const bookId = Number(new URL(page.url()).pathname.split("/").pop());
  const admin = await adminApi();
  // Düzenleyici CSP'ye takılmamalı (Faz 17C): Tiptap'in <style> etiketi kapalı (injectCSS: false).
  const styleViolations: string[] = [];
  page.on("console", (m) => {
    if (m.text().includes("Content Security Policy") && m.text().includes("style-src")) styleViolations.push(m.text());
  });
  try {
    // Yeniden denemede ilk denemenin incelemeleri kalmış olabilir: ikisi de baştan silinir.
    await admin.deleteReview(bookId);
    await deleteMyReviewIfAny(page);
    await admin.saveReview(bookId, { body: SPOILER, has_spoiler: true });
    await page.reload();
    await expectLoggedIn(page, READER_NAME);

    await spoilerIsKeyboardAccessible(page);
    await spoilerPieceOpensAlone(page, admin, bookId);
    await writeAndEdit(page);
    await twoTabConflict(page);
    await hiddenReviewShowsReason(page, admin, bookId);

    // Silme onayı puanın kalacağını söyler; silince yazma düğmesi döner, puan yerinde (Doğrulama 14).
    await myReview(page).getByRole("button", { name: "Sil" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("Puanın silinmez.");
    await dialog.getByRole("button", { name: "Sil", exact: true }).click();
    await expect(page.getByRole("button", { name: "İnceleme yaz" })).toBeVisible();
    await expect(page.getByRole("group", { name: "Senin puanın" }).locator("button[aria-pressed=true]")).toHaveCount(1);
    expect(styleViolations, "düzenleyici stil yüzünden CSP ihlali üretmemeli").toEqual([]);
  } finally {
    await admin.deleteReview(bookId);
    await admin.dispose();
  }
});

const reviews = (p: Page) => p.getByRole("region", { name: /^İncelemeler/ });
const myReview = (p: Page) => p.getByRole("region", { name: "Senin incelemen" });
const editor = (p: Page) => p.getByRole("form", { name: "İncelemeni yaz" });
/** Zengin metin alanı (Faz 17C). Ayrı parçada yüklenir; görünmesini beklemek yüklenmesini beklemektir. */
const field = (p: Page) => editor(p).getByRole("textbox", { name: "İncelemen" });
const tool = (p: Page, name: string) => editor(p).getByRole("toolbar", { name: "Biçim" }).getByRole("button", { name, exact: true });

/** Kitaba katalogdan (SPA gezintisi, bellekteki oturum korunur); önceki senaryo sayfayı kitapta bırakır. */
async function openBook(p: Page) {
  await topbar(p).getByRole("link", { name: "BookHub" }).click();
  await p.getByPlaceholder("Başlık ya da yazar ara…").fill(BOOK);
  await p.getByRole("button", { name: "Ara" }).click();
  await p.getByRole("list", { name: "Kitaplar" }).getByRole("link", { name: BOOK, exact: true }).click();
  await expect(p.getByRole("heading", { name: BOOK, level: 1 })).toBeVisible();
}

/** Okurun incelemesi varsa arayüzden siler (yeniden deneme). Benim incelemem gelene kadar bekler. */
async function deleteMyReviewIfAny(p: Page) {
  const write = p.getByRole("button", { name: "İnceleme yaz" });
  await expect(write.or(myReview(p))).toBeVisible();
  if (await write.isVisible()) return;
  await myReview(p).getByRole("button", { name: "Sil" }).click();
  await p.getByRole("dialog").getByRole("button", { name: "Sil", exact: true }).click();
  await expect(write).toBeVisible();
}

/** axe: İncelemeler bölümünde WCAG 2.1 A/AA ihlali yok. */
async function expectNoA11yViolations(p: Page, what: string) {
  const result = await new AxeBuilder({ page: p })
    .include("section[aria-labelledby=reviews-heading]")
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  const found = result.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(found, what).toEqual([]);
}

/**
 * Spoiler (Doğrulama 16, kavram 8): metin açılana kadar DOM'da yok; düğmeye odaklanıp Enter'la açılır,
 * aria-expanded değişir. axe kapalı ve açık hâlde.
 */
async function spoilerIsKeyboardAccessible(p: Page) {
  const card = reviews(p).getByRole("listitem").filter({ hasText: "E2E Admin" });
  const toggle = card.getByRole("button", { name: "Spoiler'ı göster" });
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  expect(await p.content(), "spoiler metni açılmadan sayfada olmamalı").not.toContain(SPOILER);
  await expectNoA11yViolations(p, "spoiler kapalı");

  await toggle.focus();
  await p.keyboard.press("Enter");
  await expect(card.getByText(SPOILER)).toBeVisible();
  await expect(card.getByRole("button", { name: "Spoiler'ı gizle" })).toHaveAttribute("aria-expanded", "true");
  await expectNoA11yViolations(p, "spoiler açık");
}

/**
 * Parça parça spoiler (Faz 17C, #128 kriter 2). Öteki okur incelemesini belgeyle düzenler: yalnız bir
 * parça spoiler. Gerisi açık okunur; parçanın metni kendi düğmesiyle açılana kadar sayfada yok.
 */
async function spoilerPieceOpensAlone(p: Page, admin: AdminApi, bookId: number) {
  await admin.saveReview(bookId, {
    version: 1,
    doc: { type: "doc", content: [
      { type: "paragraph", content: [
        { type: "text", text: "Sonunu söylüyorum: " },
        { type: "text", text: PIECE, marks: [{ type: "bold" }, { type: "spoiler" }] },
        { type: "text", text: " Yine de okuyun." },
      ] },
      { type: "bulletList", content: [{ type: "listItem", content: [{ type: "paragraph", content: [{ type: "text", text: "E2E madde" }] }] }] },
    ] },
  });
  await p.reload();
  await expectLoggedIn(p, READER_NAME);

  const card = reviews(p).getByRole("listitem").filter({ hasText: "E2E Admin" });
  await expect(card).toContainText("Sonunu söylüyorum:");
  await expect(card).toContainText("Yine de okuyun.");
  await expect(card.locator("ul > li")).toHaveText("E2E madde");
  await expect(card.getByText("Spoiler içeriyor")).toHaveCount(0); // tamamı spoiler değil: kart düğmesi yok
  expect(await p.content(), "spoiler parçası açılmadan sayfada olmamalı").not.toContain(PIECE);
  const toggle = card.getByRole("button", { name: "Spoiler'ı göster" });
  await expect(toggle).toHaveCount(1);
  await expectNoA11yViolations(p, "spoiler parçası kapalı");

  await toggle.focus();
  await p.keyboard.press("Enter");
  await expect(card.locator("strong", { hasText: PIECE })).toBeVisible();
  const hide = card.getByRole("button", { name: "Spoiler'ı gizle" });
  await expect(hide).toBeFocused(); // düğme yerinde kaldı: odak kaybolmadı
  await expect(hide).toHaveAttribute("aria-expanded", "true");
  await expectNoA11yViolations(p, "spoiler parçası açık");
}

/**
 * İlk yazım YALNIZ KLAVYEYLE (#128 kriter 7) ve düzenleme (Doğrulama 12). Araç çubuğuna Shift+Tab ile
 * girilir, ok tuşlarıyla gezilir; düğme odağı editöre geri verir. Tab editörden çıkar (klavye tuzağı yok).
 */
async function writeAndEdit(p: Page) {
  // Editör kütüphanesi ayrı parça (#128 kriter 8): inceleme okuyan sayfa onu istemez, "İnceleme yaz" ister.
  const editorChunk = () => p.evaluate(() => performance.getEntriesByType("resource").filter((r) => /\/assets\/ReviewRichEditor-.*\.js$/.test(r.name)).length);
  expect(await editorChunk(), "düzenleyici parçası istenmeden inmemeli").toBe(0);
  await p.getByRole("button", { name: "İnceleme yaz" }).click();
  await expect(field(p)).toBeVisible();
  expect(await editorChunk()).toBe(1);
  await field(p).focus();
  await p.keyboard.type(`E2E incelemesi ${RUN_ID} `);

  await p.keyboard.press("Shift+Tab");
  await expect(tool(p, "Kalın")).toBeFocused();
  await p.keyboard.press("ArrowRight");
  await p.keyboard.press("ArrowRight");
  await expect(tool(p, "Spoiler")).toBeFocused();
  await p.keyboard.press("Enter");
  await expect(field(p)).toBeFocused();
  await expect(tool(p, "Spoiler")).toHaveAttribute("aria-pressed", "true");
  await p.keyboard.type("gizli son");

  await p.keyboard.press("Shift+Tab"); // çubuk kaldığı düğmeyi hatırlar
  await expect(tool(p, "Spoiler")).toBeFocused();
  await p.keyboard.press("Enter");
  await expect(tool(p, "Spoiler")).toHaveAttribute("aria-pressed", "false");
  await p.keyboard.type(" **kalın** biter"); // Markdown kısayolu: yazarken kalına döner
  await expect(field(p).locator("strong")).toHaveText("kalın");
  await expect(field(p).locator("span[data-spoiler]")).toHaveText("gizli son");
  // İtalik, liste ve alıntı klavye kısayollarıyla (#128 kriter 1).
  await p.keyboard.press("Control+i");
  await p.keyboard.type(" eğik");
  await p.keyboard.press("Control+i");
  await p.keyboard.press("Enter");
  await p.keyboard.press("Control+Shift+8");
  await p.keyboard.type("E2E madde");
  await p.keyboard.press("Enter");
  await p.keyboard.press("Enter"); // boş maddede Enter listeden çıkarır
  await p.keyboard.press("Control+Shift+b");
  await p.keyboard.type("E2E alıntı");
  await expect(tool(p, "Alıntı")).toHaveAttribute("aria-pressed", "true");
  await expectNoA11yViolations(p, "düzenleyici açık");
  await expect(p.locator("style[data-tiptap-style]")).toHaveCount(0);

  await p.keyboard.press("Tab"); // editörden çıkış: sıradaki düğme
  await expect(editor(p).getByRole("button", { name: "Vazgeç" })).toBeFocused();
  await p.keyboard.press("Tab");
  await expect(editor(p).getByRole("button", { name: "Kaydet" })).toBeFocused();
  await p.keyboard.press("Enter");

  const formatted = async () => {
    await expect(myReview(p)).toContainText(`E2E incelemesi ${RUN_ID} gizli son kalın biter eğik`);
    await expect(myReview(p).locator("mark")).toHaveText("gizli son");
    await expect(myReview(p).locator("strong")).toHaveText("kalın");
    await expect(myReview(p).locator("em")).toHaveText(" eğik");
    await expect(myReview(p).locator("ul > li")).toHaveText("E2E madde");
    await expect(myReview(p).locator("blockquote")).toHaveText("E2E alıntı");
  };
  await formatted();
  await expect(myReview(p)).toContainText("spoiler içeriyor olarak işaretli");
  await expect(myReview(p)).not.toContainText("düzenlendi");
  await p.reload(); // sunucudan okununca da aynı biçim (#128 kriter 1)
  await expectLoggedIn(p, READER_NAME);
  await formatted();

  await myReview(p).getByRole("button", { name: "Düzenle" }).click();
  await expect(field(p)).toContainText(`E2E incelemesi ${RUN_ID} gizli son kalın biter eğik`); // belge biçimiyle açılır
  await expect(field(p).locator("blockquote")).toHaveText("E2E alıntı");
  await expect(field(p).locator("span[data-spoiler]")).toHaveText("gizli son");
  await pasteHostileHtml(p);
  await field(p).fill(`E2E düzenlenmiş inceleme ${RUN_ID}`);
  await editor(p).getByRole("button", { name: "Kaydet" }).click();
  await expect(myReview(p)).toContainText(`E2E düzenlenmiş inceleme ${RUN_ID}`);
  await expect(myReview(p)).toContainText("düzenlendi");
}

/**
 * Yapıştırılan HTML şemadan geçer: başlık, bağlantı ve resim öğe olarak giremez, metinleri kalır; olay
 * işleyicisi çalışmaz. (Sunucu zaten HTML kabul etmiyor; burada bakılan editörün ne ürettiği.)
 */
async function pasteHostileHtml(p: Page) {
  await field(p).focus();
  await p.keyboard.press("Control+End");
  await field(p).evaluate((el) => {
    const data = new DataTransfer();
    data.setData("text/html", `<h1>E2E başlık</h1><p><a href="https://kotu.example" onclick="window.__pwned=1">E2E bağlantı</a></p><img src="x" onerror="window.__pwned=1"><script>window.__pwned=1</script>`);
    data.setData("text/plain", "düz");
    el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
  });
  await expect(field(p)).toContainText("E2E bağlantı");
  await expect(field(p)).toContainText("E2E başlık");
  await expect(field(p).locator("a, h1, img, script")).toHaveCount(0);
  expect(await p.evaluate(() => (window as { __pwned?: number }).__pwned ?? null)).toBeNull();
}

/** İki sekme aynı incelemeyi düzenler: sonra kaydeden 409 alır, ötekinin metnini görür, "Benimkini kaydet" yazar. */
async function twoTabConflict(p: Page) {
  const second = await p.context().newPage();
  try {
    await second.goto(p.url());
    await expectLoggedIn(second, READER_NAME);
    await myReview(p).getByRole("button", { name: "Düzenle" }).click();
    await myReview(second).getByRole("button", { name: "Düzenle" }).click();
    await field(second).fill(`E2E ikinci sekme ${RUN_ID}`);
    await editor(second).getByRole("button", { name: "Kaydet" }).click();
    await expect(myReview(second)).toContainText(`E2E ikinci sekme ${RUN_ID}`);
  } finally {
    await second.close();
  }

  await field(p).fill(`E2E birinci sekme ${RUN_ID}`);
  await editor(p).getByRole("button", { name: "Kaydet" }).click();
  const note = editor(p).getByRole("alert");
  await expect(note).toContainText("İncelemen başka bir yerde değişmiş");
  await expect(note).toContainText(`E2E ikinci sekme ${RUN_ID}`);
  await expect(field(p)).toHaveText(`E2E birinci sekme ${RUN_ID}`);
  await editor(p).getByRole("button", { name: "Benimkini kaydet" }).click();
  await expect(myReview(p)).toContainText(`E2E birinci sekme ${RUN_ID}`);
}

/**
 * Yönetici gizler: okur gerekçeyi görür, düzenleyince inceleme gizli kalır; geri açınca not kalkar
 * (Doğrulama 15'in arayüz yarısı). Gizleme ve geri açma doğrudan API'den (dosya başındaki not).
 */
async function hiddenReviewShowsReason(p: Page, admin: AdminApi, bookId: number) {
  const id = await admin.reviewIdOf(bookId, READER_NAME);
  await admin.moderate(id, "hide", HIDE_REASON);
  await p.reload();
  await expectLoggedIn(p, READER_NAME);
  await expect(myReview(p)).toContainText(`Bu inceleme yönetici tarafından gizlendi: ${HIDE_REASON}`);

  await myReview(p).getByRole("button", { name: "Düzenle" }).click();
  await field(p).fill(`E2E gizliyken düzenlendi ${RUN_ID}`);
  await editor(p).getByRole("button", { name: "Kaydet" }).click();
  await expect(p.getByRole("region", { name: "Bildirimler" })).toContainText("gizli kalmaya devam ediyor");
  await expect(myReview(p)).toContainText(HIDE_REASON);

  await admin.moderate(id, "unhide");
  await p.reload();
  await expectLoggedIn(p, READER_NAME);
  await expect(myReview(p)).toContainText(`E2E gizliyken düzenlendi ${RUN_ID}`);
  await expect(myReview(p)).not.toContainText("gizlendi");
}

type AdminApi = Awaited<ReturnType<typeof adminApi>>;

/**
 * Yönetici, NGINX'i atlayan doğrudan API'den: sınırlı kovayı harcamaz (API'nin kendi sınırı e2e'de
 * yükseltilmiş). Tarayıcı ve çerez yok: token Authorization başlığında.
 */
async function adminApi() {
  const api: APIRequestContext = await request.newContext({ baseURL: DIRECT_API_URL });
  const login = await api.post("/api/auth/login", { data: { email: adminEmail(), password: adminPassword() } });
  expect(login.status(), "yönetici doğrudan API'den giriş yapamadı").toBe(200);
  const headers = { Authorization: `Bearer ${(await login.json()).access_token}` };
  const ok = async (call: Promise<APIResponse>, what: string, allowed = [200, 201, 204]) => {
    const res = await call;
    expect(allowed, `${what}: ${res.status()} ${await res.text()}`).toContain(res.status());
    return res;
  };
  return {
    // İki gövde de (spec karar 7): eski düz metin + kutu, ya da belge.
    saveReview: (bookId: number, data: { body: string; has_spoiler: boolean } | { doc: object; version?: number }) =>
      ok(api.put(`/api/books/${bookId}/review`, { headers, data }), "yöneticinin incelemesi"),
    deleteReview: (bookId: number) =>
      ok(api.delete(`/api/books/${bookId}/review`, { headers }), "yöneticinin incelemesini silme", [204, 404]),
    moderate: (id: number, action: "hide" | "unhide", reason?: string) =>
      ok(api.post(`/api/admin/reviews/${id}/${action}`, { headers, data: reason === undefined ? undefined : { reason } }), action),
    reviewIdOf: async (bookId: number, author: string): Promise<number> => {
      const res = await ok(api.get(`/api/books/${bookId}/reviews?limit=50`), "inceleme listesi");
      const items: { id: number; author: { display_name: string } }[] = (await res.json()).items;
      const item = items.find((r) => r.author.display_name === author);
      expect(item, `${author} incelemesi listede yok`).toBeTruthy();
      return item!.id;
    },
    dispose: () => api.dispose(),
  };
}
