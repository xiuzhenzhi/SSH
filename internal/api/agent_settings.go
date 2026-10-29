package api

import (
	"context"
	"database/sql"
)

// Call under the parent server lock, shared by edits and Agent transactions.
func (a *App) ensureServerSettings(ctx context.Context, tx *sql.Tx, id string) error {
	var n int
	if e := tx.QueryRowContext(ctx, a.db.Bind(`SELECT COUNT(*) FROM server_settings WHERE server_id=?`), id).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		_, e := tx.ExecContext(ctx, a.db.Bind(`INSERT INTO server_settings(server_id) VALUES(?)`), id)
		return e
	}
	return nil
}
