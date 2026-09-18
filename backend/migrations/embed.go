// Package migrations, goose şema migration'larını Go binary'sine GÖMER.
//
// Neden gömülü: binary kendi şemasını taşısın. Böylece testler, Docker imajı ve taze
// bir klon "önce goose CLI'ı kur, sonra elle up çalıştır" adımına bağımlı kalmaz —
// dağıtılan artefakt, beklediği şemayı da yanında getirir. goose CLI hâlâ geçerli
// (status/down için), ama doğruluk kaynağı bu klasördeki .sql dosyaları.
package migrations

import "embed"

// FS, bu klasördeki tüm .sql migration'ları (dosya adı sırası = uygulama sırası).
//
//go:embed *.sql
var FS embed.FS
