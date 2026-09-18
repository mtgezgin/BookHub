// Package audit, denetim kaydı (audit_logs) yazımının tek kapısı. Kayıt, işlemin kendisiyle
// AYNI transaction'da yazılsın diye çağıran, tx'e bağlı *db.Queries'i verir.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Varlık tipleri (audit_logs.entity_type).
const (
	EntityBook    = "book"
	EntityComment = "comment"
)

// Write, bir denetim satırı ekler. details JSON'a çevrilir (jsonb kolonu).
func Write(ctx context.Context, q *db.Queries, actorID int64, action, entityType string, entityID int64, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("denetim detayı kodlanamadı: %w", err)
	}
	err = q.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ActorID:    &actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    raw,
	})
	if err != nil {
		return fmt.Errorf("denetim kaydı yazılamadı: %w", err)
	}
	return nil
}
