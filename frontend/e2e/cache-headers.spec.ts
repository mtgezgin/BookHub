// cache-headers.spec.ts — NGINX'in üç önbellek sözleşmesi (REVIEW_FEEDBACK F35).
// RATE LIMIT BÜTÇESİ: 0 login/register. Tablo: e2e/helpers.ts.
//
// Adında içerik özeti olan dosya kalıcıdır (içerik değişirse ad değişir), index.html her açılışta
// sorulur (yeni sürümün dosya adlarını o taşır), BULUNAMAYAN dosya ise saklanmaz: "yok" kalıcı bir
// bilgi değildir. Üçüncüsü bir kez bozuldu: `add_header … always` 404'e de "bir yıl sakla" yazıyordu.
import { expect, test } from "@playwright/test";

import { BASE_URL } from "./env";

test("index.html önbelleklenmez", async ({ request }) => {
  const res = await request.get(`${BASE_URL}/`);
  expect(res.status()).toBe(200);
  expect(res.headers()["cache-control"]).toBe("no-cache");
});

test("adında özet olan dosya kalıcıdır", async ({ request }) => {
  const html = await (await request.get(`${BASE_URL}/`)).text();
  const asset = /\/assets\/[^"]+\.js/.exec(html)?.[0];
  expect(asset, "index.html bir /assets/ betiğine bağlanmalı").toBeTruthy();
  const res = await request.get(`${BASE_URL}${asset}`);
  expect(res.status()).toBe(200);
  expect(res.headers()["cache-control"]).toContain("immutable");
});

test("bulunamayan /assets/ dosyası önbelleklenmez", async ({ request }) => {
  const res = await request.get(`${BASE_URL}/assets/yok-boyle-bir-dosya.js`);
  expect(res.status()).toBe(404);
  expect(res.headers()["cache-control"] ?? "").not.toContain("immutable");
  expect(res.headers()["cache-control"] ?? "").not.toContain("max-age=31536000");
  // Güvenlik başlıkları hata cevabında da durmalı (onlarda `always` doğru).
  expect(res.headers()["x-content-type-options"]).toBe("nosniff");
});
