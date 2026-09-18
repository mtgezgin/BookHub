# Faz 10 — CI/CD

> Durum: ✅ TAMAM (deploy hariç — hesap gerektirir) · İlgili: [ci.yml](../../.github/workflows/ci.yml), [DEPLOYMENT.md](../DEPLOYMENT.md)

## 🎯 Amaç
Her push'ta otomatik doğrulama; imaj yayınlamayı kontrollü bir adıma bağlamak.

## 🧠 Yeni kavram(lar)
- GitHub Actions **servis konteynerleri** (gerçek Postgres/Redis ile entegrasyon testi).
- Uçtan uca iş: aynı `docker-compose.yml` + MVP smoke.
- Yayın ayrı iş akışında (`release.yml`): yalnız `v*` etiketi ya da elle; `GITHUB_TOKEN` ile ghcr.io.

## ✅ Tamamlanma
- [x] `backend` (birim + entegrasyon + lint), `frontend` (tsc + vitest + build), `e2e` (compose + smoke).
- [x] Release iş akışı + dağıtım rehberi. **Gerçek bulut deploy'u yapılmadı** (Supabase/Upstash/platform hesabı gerekir).
