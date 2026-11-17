// resilience.spec.ts — sunucu geçici olarak cevap veremediğinde arayüzün davranışı
// (REVIEW_FEEDBACK F34, F36).
// RATE LIMIT BÜTÇESİ: 0 login/register (hatalar tarayıcıda taklit edilir). Tablo: e2e/helpers.ts.
import { expect, test } from "@playwright/test";

// NGINX'in ve ağ geçidinin kendi hata sayfası: HTML gövde, hata zarfı yok. Canlıda (HTTP/2) durum
// metni de boştur; arayüz mesajı durum kodundan üretmelidir.
const GATEWAY_ERROR = { status: 502, contentType: "text/html", body: "<html><body>502 Bad Gateway</body></html>" };

test("katalog 502 alırsa kullanıcı anlamlı bir mesaj okur (boş kutu değil)", async ({ page }) => {
  await page.route("**/api/books?**", (route) => route.fulfill(GATEWAY_ERROR));
  await page.goto("/");
  await expect(page.getByRole("alert").filter({ hasText: "Sunucuya şu an ulaşılamıyor" })).toBeVisible();
});

test("açılışta yenileme 502 dönerse giriş sayfasına atılmaz; sunucu reddedince atılır", async ({ page }) => {
  // Bu tarayıcıda oturum açılmış görünüyor (HINT), ama yenileme isteği geçici olarak başarısız.
  await page.addInitScript(() => localStorage.setItem("bookhub.session", "1"));
  let refreshCalls = 0;
  await page.route("**/api/auth/refresh", (route) => {
    refreshCalls++;
    return route.fulfill(GATEWAY_ERROR);
  });

  await page.goto("/library");
  await expect(page.getByRole("status").filter({ hasText: "Oturumun doğrulanamadı" })).toBeVisible();
  await expect(page).toHaveURL(/\/library$/);

  // Yeniden deneniyor: ilk denemeden 1 sn sonra ikinci istek gider.
  await expect.poll(() => refreshCalls, { timeout: 5_000 }).toBeGreaterThanOrEqual(2);
  await expect(page).toHaveURL(/\/library$/);

  // Sunucu geri geldi ve bu tarayıcının oturumu olmadığını söyledi (çerez yok → 401): sonuç artık
  // kesin, kullanıcı giriş sayfasına yönlendirilir.
  await page.unroute("**/api/auth/refresh");
  await expect(page).toHaveURL(/\/login$/, { timeout: 15_000 });
});
