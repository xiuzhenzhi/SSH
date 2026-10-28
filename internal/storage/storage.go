package storage

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"sshguard/migrations"
	"strings"
	"time"
)

type DB struct {
	*sql.DB
	Driver string
}

func Open(ctx context.Context, driver, dsn string) (*DB, error) {
	name := driver
	if driver == "postgres" {
		name = "pgx"
	}
	p, e := sql.Open(name, dsn)
	if e != nil {
		return nil, e
	}
	p.SetMaxOpenConns(20)
	p.SetMaxIdleConns(5)
	p.SetConnMaxLifetime(5 * time.Minute)
	if e = p.PingContext(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return &DB{p, driver}, nil
}
func (d *DB) Bind(q string) string {
	if d.Driver != "postgres" {
		return q
	}
	n := 0
	var b strings.Builder
	for _, c := range q {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func (d *DB) Exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return d.ExecContext(ctx, d.Bind(q), args...)
}
func (d *DB) Row(ctx context.Context, q string, args ...any) *sql.Row {
	return d.QueryRowContext(ctx, d.Bind(q), args...)
}
func (d *DB) List(ctx context.Context, q string, args ...any) ([]map[string]any, error) {
	rows, e := d.QueryContext(ctx, d.Bind(q), args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	cols, e := rows.Columns()
	if e != nil {
		return nil, e
	}
	result := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			return nil, e
		}
		item := map[string]any{}
		for i, k := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			item[k] = v
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// MySQL DDL auto-commits: migrations must be idempotent for retry after interruption.
func Migrate(ctx context.Context, d *DB) error {
	conn, e := d.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	if d.Driver == "postgres" {
		if _, e = conn.ExecContext(ctx, `SELECT pg_advisory_lock(704201)`); e != nil {
			return e
		}
		defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(704201)`)
	} else {
		var ok int
		if e = conn.QueryRowContext(ctx, `SELECT GET_LOCK('sshguard_migrations',30)`).Scan(&ok); e != nil || ok != 1 {
			return fmt.Errorf("migration lock failed: %v", e)
		}
		defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK('sshguard_migrations')`)
	}
	if _, e = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version varchar(128) PRIMARY KEY, applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP)`); e != nil {
		return e
	}
	dir := d.Driver
	entries, e := migrations.Files.ReadDir(dir)
	if e != nil {
		return e
	}
	for _, f := range entries {
		var n int
		if e = conn.QueryRowContext(ctx, d.Bind(`SELECT COUNT(*) FROM schema_migrations WHERE version=?`), f.Name()).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			continue
		}
		b, e := migrations.Files.ReadFile(dir + "/" + f.Name())
		if e != nil {
			return e
		}
		tx, e := conn.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		for _, statement := range strings.Split(string(b), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, e = tx.ExecContext(ctx, statement); e != nil {
				tx.Rollback()
				return fmt.Errorf("%s: %w", f.Name(), e)
			}
		}
		if _, e = tx.ExecContext(ctx, d.Bind(`INSERT INTO schema_migrations(version) VALUES(?)`), f.Name()); e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}
