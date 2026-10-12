// e2e/env.ts — koşunun DIŞARIDAN gelen ayarları tek yerde.
//
// NEDEN AYRI DOSYA: playwright.config.ts de, senaryolar da aynı adresi/koşu kimliğini
// kullanıyor. İki yerde ayrı ayrı hesaplansaydı biri E2E_BASE_URL'i okurken diğeri sabit
// bir adrese giderdi ve "neden çerez gelmiyor" diye saatlerce bakılırdı.

/**
 * Uygulamanın (NGINX + API tek origin) adresi. Senaryoların TAMAMI buradan geçer;
 * tarayıcı için frontend ve API aynı origin'dedir.
 */
export const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

// HOSTNAME "localhost" OLMAK ZORUNDA — yoksa testler sessizce ANLAMSIZLAŞIR.
// Refresh çerezi Secure (COOKIE_SECURE=true): tarayıcı onu yalnız "güvenli bağlam"a
// yazar/geri gönderir ve düz http'de güvenli sayılan TEK host localhost'tur (127.0.0.1
// tarayıcıda güvenli sayılsa da Playwright'in kendi çerez kavanozu Secure çerezi http://
// şemasında localhost dışında düşürür). Böyle bir kurulumda her senaryo "oturum yok" gibi
// davranır ve hatanın nedeni testin kendisinde aranır. Erken ve yüksek sesle patlıyoruz.
const hostname = new URL(BASE_URL).hostname;
if (hostname !== "localhost") {
  throw new Error(
    `E2E_BASE_URL'in hostname'i "localhost" olmalı (gelen: ${hostname}). ` +
      `Secure refresh çerezi düz http'de yalnız localhost'ta taşınır; başka bir host'ta ` +
      `tüm oturum senaryoları sahte bir "oturum yok" hâline düşerdi.`,
  );
}

/**
 * NGINX'i ATLAYAN doğrudan API adresi. Yalnızca sınırlı kovayı harcamaması gereken
 * "bunun başarısız olması lazım" kontrolleri için (ör. eski parolayla giriş denemesi):
 * NGINX'in login|register kovası 3 sn'de 1 dolar, testin bütçesi ise 11 istek (bkz. helpers.ts).
 */
export const DIRECT_API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";

/**
 * Koşu kimliği (scripts/e2e.sh: date +%s; CI: run_id-run_attempt). Bu koşunun ürettiği
 * kullanıcı/kitap adlarının İÇİNDE yer alır → paralel koşular ve arka arkaya koşular
 * birbirinin verisini görmez, temizlik de tam olarak bu koşuyu siler.
 */
export const RUN_ID = process.env.E2E_RUN_ID ?? "local";

/**
 * Refresh çerezinin adı. Sunucu tarafı ayarlanabilir (COOKIE_NAME; aynı makinede ikinci
 * bir yığın çalışırken çakışmasın diye) — scripts/e2e.sh .env'i yüklediği için test ve
 * konteyner AYNI değeri görür.
 */
export const REFRESH_COOKIE = process.env.COOKIE_NAME ?? "bookhub_refresh";

/**
 * e2eEmail: temizlik deseniyle UYUMLU bir e-posta üretir.
 * scripts/e2e.sh çıkarken `DELETE FROM users WHERE email LIKE 'e2e-%-<RUN_ID>@example.com'`
 * koşuyor; desen dışına çıkan bir adres veritabanında sonsuza kadar kalırdı.
 */
export function e2eEmail(purpose: string): string {
  return `e2e-${purpose}-${randomSlug()}-${RUN_ID}@example.com`;
}

/** Testlerin ürettiği kullanıcıların parolası: her koşuda rastgele, hiçbir dosyaya yazılmaz. */
export function randomPassword(): string {
  return `E2e-${randomSlug()}${randomSlug()}`; // en az 8 karakter (politika)
}

/**
 * Testlerin önerdiği kitapların başlık öneki: koşu kimliği + HER ÇAĞRIDA rastgelelik.
 *
 * NEDEN rastgele kısım: `books.slug` tablonun tek UNIQUE kolonu ve E2E_RUN_ID tüm koşu
 * boyunca SABİT. CI'da `retries: 1` var (playwright.config.ts) — yeniden deneme aynı
 * başlığı gönderseydi API 409 (slug çakışması) döner, arayüz başarı mesajı yerine hata
 * basar ve test "başarı kutusu bulunamadı" gibi gerçek sebebi hiç söylemeyen bir yerde
 * düşerdi. Aynı şey `--repeat-each` için de geçerli.
 *
 * RUN_ID BAŞTA kalıyor: temizlik/arama desenleri ("bu koşunun kitapları") bozulmasın.
 */
export function e2eTitlePrefix(): string {
  return `E2E ${RUN_ID}-${randomSlug()}`;
}

function randomSlug(): string {
  return crypto.randomUUID().replaceAll("-", "").slice(0, 8);
}

/**
 * PAYLAŞILAN "okur": globalSetup bir kez kaydeder, dört senaryo onunla giriş yapar
 * (her biri KENDİ girişini yapar — storageState paylaşılmaz, bkz. helpers.ts).
 */
export const READER_EMAIL = `e2e-reader-${RUN_ID}@example.com`;
export const READER_NAME = "E2E Okur";

/** Okurun parolası globalSetup'ta üretilir ve env üzerinden worker'lara geçer. */
export const READER_PASSWORD_ENV = "E2E_READER_PASSWORD";

/** scripts/e2e.sh'ın `manage create-admin` ile açtığı admin. */
export const ADMIN_NAME = "E2E Admin";

export function readerPassword(): string {
  return requiredEnv(READER_PASSWORD_ENV);
}

export function adminEmail(): string {
  return requiredEnv("E2E_ADMIN_EMAIL");
}

export function adminPassword(): string {
  return requiredEnv("E2E_ADMIN_PASSWORD");
}

// requiredEnv: eksik ayarı testin ORTASINDA anlaşılmaz bir 401 olarak değil, burada
// adıyla bildirir. (Playwright'i elle koşturan biri en sık bu tuzağa düşer.)
function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(
      `${name} tanımlı değil. E2E testleri yığını ve admin'i kendisi kuran scripts/e2e.sh ` +
        `ile koşar: repo kökünden "make e2e".`,
    );
  }
  return value;
}
