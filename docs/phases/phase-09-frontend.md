# Faz 9 — Frontend (React + Vite)

> Durum: ✅ TAMAM · İlgili: [client.ts](../../frontend/src/api/client.ts)

## 🎯 Amaç
API'yi kullanan bir arayüz: katalog, detay (puan + yorum), giriş/kayıt, öneri, admin moderasyonu.

## 🧠 Yeni kavram(lar)
- SPA + geliştirmede **Vite proxy** (aynı origin → CORS yok).
- Token yönetimi: access bellekte, refresh `localStorage`'da (ödünleşim yazılı), **tek uçuşlu yenileme**.
- Rota koruması yalnız UX içindir; yetki API'de.

## ⚠️ Önemli noktalar
- Refresh token tek kullanımlık (Faz 3) → eşzamanlı 401'ler tek bir `/refresh` paylaşmalı; yoksa
  ikinci yenileme geçersiz token alır ve kullanıcı sebepsiz düşer (vitest ile kilitli).
- Kullanıcı içeriği metin olarak basılır; `dangerouslySetInnerHTML` yok (XSS).

## ✅ Tamamlanma
- [x] 6 sayfa, `tsc --noEmit` temiz, vitest 6/6, üretim paketi 87 kB gzip.
- [x] Demo verisi (`make demo-seed`) ve headless Chrome ekran görüntüleriyle doğrulandı.
