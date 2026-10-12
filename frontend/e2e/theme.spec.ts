// theme.spec.ts — public/theme-init.js temayı ilk boyamadan önce <html data-theme>'ya yazar.
// RATE LIMIT BÜTÇESİ: 0 login/register (giriş yok). Tablo: e2e/helpers.ts.
import { expect, test } from "@playwright/test";

test("kayıtlı koyu tema açılışta <html data-theme=dark> olarak uygulanır", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.addInitScript(() => localStorage.setItem("bookhub.theme", "dark"));
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("kayıtlı seçim yokken sistemin koyu tercihi uygulanır", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});
