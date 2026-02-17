package sync

import (
	"time"

	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
)

// Reconciler manages history sync checkpoints.
type Reconciler struct {
	db     *store.DB
	logger *zap.Logger
}

// NewReconciler creates a new reconciler.
func NewReconciler(db *store.DB, logger *zap.Logger) *Reconciler {
	return &Reconciler{db: db, logger: logger}
}

// UpdateCheckpoint updates a sync checkpoint value.
func (r *Reconciler) UpdateCheckpoint(key, value string) error {
	now := time.Now().UnixMilli()
	_, err := r.db.Exec(`
		INSERT INTO sync_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now)
	return err
}

// GetCheckpoint retrieves a sync checkpoint value.
func (r *Reconciler) GetCheckpoint(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM sync_state WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}
