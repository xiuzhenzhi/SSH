package storage

import (
	"context"
	"fmt"
	"sshguard/migrations"
	"strings"
)

// Hold SQLite's write reservation across version checks and schema changes so
// concurrent startup processes cannot both apply the same migration.
func migrateSQLite(ctx context.Context, d *DB) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	entries, err := migrations.Files.ReadDir("sqlite")
	if err != nil {
		return err
	}
	for _, f := range entries {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, f.Name()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		data, err := migrations.Files.ReadFile("sqlite/" + f.Name())
		if err != nil {
			return err
		}
		for _, statement := range strings.Split(string(data), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("%s: %w", f.Name(), err)
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES(?)`, f.Name()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
