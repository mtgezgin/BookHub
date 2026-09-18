# Faz 12 — Kubernetes

> Durum: ✅ TAMAM (yerel kind kümesinde) · İlgili: [deploy/k8s/](../../deploy/k8s/)

## 🎯 Amaç
Aynı uygulamayı Kubernetes'te çalıştırmak: 2 API kopyası, ayrı migration Job'ı, güvenli pod ayarları.

## 🧠 Yeni kavram(lar)
- Deployment vs StatefulSet vs Job; Service = küme içi DNS; ConfigMap vs Secret.
- readiness vs liveness probe; `rollingUpdate.maxUnavailable: 0`.
- Pod güvenliği: `runAsNonRoot` (SAYISAL uid şart), salt okunur kök dosya sistemi, capability'siz.

## 🪜 Çalıştırma
```bash
make k8s-up      # kind kümesi + imajlar + sırlar (rastgele) + apply + migrate Job + rollout
make k8s-port    # http://localhost:3001
make k8s-down    # kümeyi sil
```

## ⚠️ Deploy ederken bulunanlar
- `USER nonroot` (adla) → kubelet pod'u başlatmadı; `USER 65532:65532` gerekli.
- `kind load docker-image` Docker Desktop'ta çok platformlu imajlarda düşüyor → tek platformlu arşiv.

## ✅ Tamamlanma
- [x] Job Completed, tüm pod'lar Running; smoke 21/21; yük iki API kopyasına dağıldı (28/30).
