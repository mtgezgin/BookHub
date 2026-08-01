// Package database, Postgres bağlantı HAVUZUNU (pool) kurar.
// Domain bilgisi içermez; tek işi "çalışan bir bağlantı havuzu ver"dir.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New, verilen DSN ile bir bağlantı havuzu açar ve bağlantıyı doğrular.
//
// NEDEN HAVUZ: her HTTP isteğinde sıfırdan bağlantı açmak (TCP + auth el sıkışması)
// çok pahalı. Havuz, açık bağlantıları hazır tutar ve eşzamanlı isteklere dağıtır;
// iş bitince bağlantı kapanmaz, havuza geri döner. (JS'teki pg.Pool'un karşılığı.)
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		// Buraya genelde DSN parse hatasıyla düşülür (bozuk URL vb.).
		return nil, fmt.Errorf("havuz yapılandırılamadı: %w", err)
	}

	// pgxpool.New TEMBELDİR: gerçek bağlantıyı ilk sorguya kadar ertelemesi mümkün.
	// Fail-fast felsefemiz (config'teki gibi) gereği boot'ta Ping ile
	// "gerçekten bağlanabiliyor muyuz?" sorusunu ŞİMDİ cevaplıyoruz —
	// yanlış şifreyi ilk müşteri isteğinde değil, açılışta öğren.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // yarım kalan havuzu bırakma
		return nil, fmt.Errorf("veritabanına ulaşılamadı (DATABASE_URL doğru mu? docker compose up -d çalıştı mı?): %w", err)
	}
	return pool, nil
}
