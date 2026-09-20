// playwright.config.ts — GERÇEK tarayıcıda uçtan uca testler (Faz 13).
//
// NEDEN vitest'in üstüne bir de bu: vitest jsdom'da koşar ve fetch sahtedir — orada ÇEREZ
// diye bir şey yok. Faz 13'ün bütün konusu (httpOnly + Secure + SameSite=Strict bir refresh
// çerezi, rotasyon, reuse tespiti, sekmeler arası Web Locks, cross-origin reddi) ancak
// çerez kavanozu olan gerçek bir tarayıcıda sınanabilir. Bu dosya o tarayıcıyı tarif eder.
//
// Testleri `make e2e` (scripts/e2e.sh) çalıştırır: yığını kaldırır, demo veriyi basar,
// rastgele parolalı bir admin açar ve çıkarken bu koşunun kullanıcılarını siler.
import { defineConfig } from "@playwright/test";

import { BASE_URL } from "./e2e/env";

export default defineConfig({
  testDir: "./e2e",
  // Paylaşılan "okur" hesabını bir kez açar ve parolasını env ile worker'lara geçirir.
  globalSetup: "./e2e/global-setup.ts",

  // Senaryolar birbirinin verisine dokunmuyor (her biri kendi kullanıcısını/kitabını
  // E2E_RUN_ID ile adlandırıyor) → paralel koşabilirler. Sınır rate limit'te, eşzamanlılıkta
  // değil: bütün koşu 11 login/register isteğinin altında (bkz. e2e/helpers.ts).
  fullyParallel: true,

  // CI'da .only ile yarım bırakılmış bir dosya sessizce "yeşil" görünmesin.
  forbidOnly: !!process.env.CI,

  // Lokalde YENİDEN DENEME YOK: kararsız bir test burada kırmızı kalsın, sebebi aransın.
  // CI'da 1 deneme (paylaşılan runner'ın ağ/zamanlama gürültüsü). DİKKAT: her yeniden
  // deneme, senaryonun girişlerini de TEKRAR yapar → rate limit bütçesinden yer.
  retries: process.env.CI ? 1 : 0,

  reporter: [["list"], ["html", { open: "never" }]],

  use: {
    baseURL: BASE_URL,
    // Trace = tıklama tıklama kayıt (DOM anlık görüntüleri, ağ istekleri, konsol).
    // 'retain-on-failure': yalnız düşen testte saklanır. NEDEN "hep açık" değil: trace
    // ÇEREZLERİ ve istek gövdelerini (parolalar!) içerir — test-results/ bu yüzden hem
    // .gitignore hem de frontend/.dockerignore içinde.
    trace: "retain-on-failure",
  },

  // YALNIZ chromium: burada sınanan şey uygulamanın davranışı, tarayıcı uyumluluğu değil.
  // Üç motoru birden kurmak CI'da dakikalar ve yüzlerce MB demekti.
  //
  // `devices['Desktop Chrome']` BİLEREK kullanılmıyor: o hazır ayar `channel: 'chromium'`
  // getirir ve Playwright'i TAM chromium derlemesine bağlar; CI'daki kurulum ise
  // `--only-shell` (yalnız chromium-headless-shell, ~100 MB yerine daha azı). Sade
  // browserName ile headless koşu doğrudan o kabuğu kullanır.
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
