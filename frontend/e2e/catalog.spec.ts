// catalog.spec.ts — anonim ziyaretçinin gördüğü katalog.
// RATE LIMIT BÜTÇESİ: 0 login/register (giriş yok). Tablo: e2e/helpers.ts.
//
// İddialar yalnız İÇERME üzerine ("Dune listede var"), eşitlik üzerine değil: demo veri
// zamanla büyüyor ve moderasyon senaryosu her koşuda katalogda yeni bir kitap bırakıyor.
// "tam 12 kitap" gibi bir iddia, ilgisiz bir değişiklikte kırmızı yanardı.
import { expect, test } from "@playwright/test";

/** Sonuç ızgarası: yan taraftaki "En iyiler" listesinde de aynı başlıklar geçiyor. */
const results = ".grid";

/**
 * Süzmenin SUNUCUDA uygulandığını kanıtlayan "olmaması gereken" başlık. Üç şartı var:
 *  1) ne `q=Dune`'a ne de `genre=bilim-kurgu`'ya uyar (demo_seed'de türü "roman"),
 *  2) FİLTRESİZ ilk sayfada VARDIR — yoksa iddia hiçbir şey kanıtlamadan yeşil kalırdı,
 *  3) demo katalog büyüdükçe orada kalır: liste `created_at DESC, id DESC` sıralı ve bu,
 *     demo_seed'in EN SON eklediği kitap — yani filtresiz ilk sayfanın en başındaki demo
 *     kaydı. (Kontrol: demo veride sıra Aylak Adam → Fahrenheit 451 → … → Küçük Prens;
 *     "Tutunamayanlar" ilk EKLENEN kitap olduğu için 12'lik ilk sayfaya hiç girmez.)
 */
const ABSENT = "Aylak Adam";

test("arama başlığa göre süzer", async ({ page }) => {
  await page.goto("/");
  await page.getByPlaceholder("Başlık ya da yazar ara…").fill("Dune");
  await page.getByRole("button", { name: "Ara" }).click();

  // Arama URL'e yazılır (paylaşılabilir/yer imlenebilir bağlantı) — SPA gezintisi.
  await expect(page).toHaveURL(/\?q=Dune/);

  const grid = page.locator(results);
  await expect(grid.getByRole("link", { name: "Dune", exact: true })).toBeVisible();
  // Süzme gerçekten SUNUCUDA mı? Taze demo veride (CI'ın durumu) filtresiz ilk sayfa demo
  // kataloğun neredeyse tamamını taşır — "Dune var" iddiası, sunucu q'yu yok saysa bile
  // yeşil kalırdı. O yüzden bir de YOK OLMASI gerekeni şart koşuyoruz. Olumlu iddiadan
  // SONRA geliyor: ızgara doluyken, yani boş ızgarada boşuna geçmesin.
  await expect(grid).not.toContainText(ABSENT);
});

test("tür filtresi URL'den çalışır", async ({ page }) => {
  await page.goto("/?genre=bilim-kurgu");

  const grid = page.locator(results);
  await expect(grid.getByRole("link", { name: "Dune", exact: true })).toBeVisible();
  await expect(grid.getByRole("link", { name: "Vakıf", exact: true })).toBeVisible();
  await expect(grid).not.toContainText(ABSENT); // aynı gerekçe: süzme sunucuda mı?
});

test("kitap detayında yazar ve tür etiketleri görünür", async ({ page }) => {
  await page.goto("/?genre=bilim-kurgu");
  await page.locator(results).getByRole("link", { name: "Dune", exact: true }).click();

  await expect(page.getByRole("heading", { name: "Dune", level: 1 })).toBeVisible();
  await expect(page.locator("article.detail header")).toContainText("Frank Herbert");
  await expect(page.locator("article.detail header .tag")).toContainText("Bilim Kurgu");
});
