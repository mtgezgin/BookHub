// cross-origin.spec.ts — CSRF koruması (http.CrossOriginProtection) NGINX'in arkasında.
// RATE LIMIT BÜTÇESİ: 0 login/register. Tablo: e2e/helpers.ts.
//
// Uç BİLEREK /api/auth/refresh: yazma method'u (POST) ama sınırlı kovada değil ve çerezsiz
// çağrıldığında zaten 401 veriyor — yani koruma katmanını, kotayı harcamadan ve kimseyi
// gerçekten giriş yaptırmadan sınayabiliyoruz.
//
// Çerez GÖNDERMİYORUZ: burada sorulan soru "bu istek kabul edilir mi", "bu oturum geçerli
// mi" değil. 403 ile 401'i ayırt etmek testin bütün noktası.
import { expect, test } from "@playwright/test";

import { BASE_URL } from "./env";

/** İsteğin kendi origin'i (şema + host + port). */
const ORIGIN = new URL(BASE_URL).origin;

interface Case {
  name: string;
  headers: Record<string, string>;
  status: number;
  code: string;
}

const cases: Case[] = [
  {
    name: "Sec-Fetch-Site: cross-site → 403",
    headers: { "Sec-Fetch-Site": "cross-site" },
    status: 403,
    code: "cross_origin_request",
    // Tarayıcının KENDİ yazdığı başlık (sayfadaki JS ne ekleyebilir ne değiştirebilir):
    // korumanın birinci ve asıl kapısı.
  },
  {
    name: "Sec-Fetch-Site yok, yabancı Origin → 403",
    headers: { Origin: "http://evil.example" },
    status: 403,
    code: "cross_origin_request",
    // Fetch Metadata göndermeyen istemci (eski tarayıcı / güvenli olmayan origin):
    // yedek kontrol Origin ile Host'u karşılaştırır.
  },
  {
    name: "Sec-Fetch-Site yok, Origin == Host → 401 (403 DEĞİL)",
    headers: { Origin: ORIGIN },
    status: 401,
    code: "invalid_token",
    // NGINX REGRESYON TESTİ: proxy, Host'u PORTUYLA iletmezse ($host, $http_host yerine)
    // API "localhost:3000" ≠ "localhost" görür ve MEŞRU isteğe 403 der. O hata bu satırda
    // yakalanır — 403 gelirse frontend/nginx/api-proxy.conf bozulmuş demektir.
  },
];

for (const c of cases) {
  test(c.name, async ({ request }) => {
    const res = await request.post("/api/auth/refresh", { headers: c.headers });

    expect(res.status(), `beklenmeyen durum — gövde: ${await res.text()}`).toBe(c.status);
    const body = await res.json();
    expect(body.error.code).toBe(c.code);
  });
}
