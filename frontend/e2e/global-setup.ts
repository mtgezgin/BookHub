// e2e/global-setup.ts — tüm koşuda BİR KEZ çalışır.
//
// Dört senaryo aynı "okur" hesabını kullanıyor (her biri KENDİ girişini yaparak — bkz.
// playwright.config.ts'teki storageState notu). Hesabı senaryolar arayüzden kendileri
// açsaydı her biri 2 sınırlı istek (kayıt + otomatik giriş) daha harcardı; NGINX'in
// login|register kovası ise 11 istekte doluyor (bütçe tablosu: helpers.ts).
//
// DİKKAT: paylaşılan hesap, senaryoların "taze kullanıcı" varsaymamasını da şart koşar —
// CI'daki yeniden denemede aynı okur aynı kitabı zaten puanlamış olur (rating-comment.spec).
import { request } from "@playwright/test";
import { randomBytes } from "node:crypto";

import { DIRECT_API_URL, READER_EMAIL, READER_NAME, READER_PASSWORD_ENV } from "./env";

export default async function globalSetup(): Promise<void> {
  // Parola her koşuda rastgele ve YALNIZCA bellekte: hiçbir dosyaya, log'a ya da repoya
  // yazılmıyor. process.env üzerinden worker süreçlerine geçiyor (Playwright worker'ları
  // globalSetup'tan SONRA doğar ve ortamı miras alır).
  const password = `E2e-${randomBytes(15).toString("base64url")}`;
  process.env[READER_PASSWORD_ENV] = password;

  // NGINX'i ATLAYIP doğrudan API'ye: bu, tarayıcı/çerez semantiği olmayan saf bir API
  // çağrısı — sınırlı login|register kovasından (11 istek, bkz. helpers.ts) kredi yemesi
  // için hiçbir sebep yok. Kovayı testlerin GERÇEKTEN arayüzden yaptığı girişlere
  // bırakıyoruz; CI'daki bir yeniden deneme de bu sayede sığıyor.
  const api = await request.newContext({ baseURL: DIRECT_API_URL });
  try {
    const res = await api.post("/api/auth/register", {
      data: { email: READER_EMAIL, display_name: READER_NAME, password },
    });
    // Bu istek NGINX'i atladığı için kalan tek sınır API'nin kendi IP başına kovası
    // (AUTH_RATE_LIMIT_PER_MIN; scripts/e2e.sh onu 1000'e çıkarır). Guard yine de duruyor:
    // yığın yükseltilmiş sınır olmadan kaldırıldıysa hata "kayıt 429" diye açıkça çıksın.
    if (res.status() === 429) {
      throw new Error(
        "paylaşılan okur kaydı 429 aldı — API auth rate limit: yığın AUTH_RATE_LIMIT_PER_MIN " +
          "yükseltilmeden kalkmış olabilir (e2e'yi 'make e2e' ile koş, bkz. scripts/e2e.sh)",
      );
    }
    if (res.status() !== 201) {
      throw new Error(`paylaşılan okur kaydedilemedi (${res.status()}): ${await res.text()}`);
    }
  } finally {
    await api.dispose();
  }
}
