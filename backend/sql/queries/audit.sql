-- audit.sql — denetim kaydı (Faz 4). Yalnızca INSERT ve SELECT: tablo append-only
-- (UPDATE/DELETE'i DB trigger'ı reddeder), burada da o iki sorgu hiç yok.

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, details)
VALUES ($1, $2, $3, $4, $5);

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 100;
