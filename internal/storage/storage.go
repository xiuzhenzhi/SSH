package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"regexp"
	"sshguard/migrations"
	"strings"
	"time"
)

type DB struct {
	*sql.DB
	Driver string
}

func Open(ctx context.Context, driver, dsn string) (*DB, error) {
	if driver != "mysql" && driver != "postgres" {
		return nil, fmt.Errorf("unsupported database driver")
	}
	name := driver
	if driver == "postgres" {
		name = "pgx"
	}
	var p *sql.DB
	var e error
	if driver == "mysql" {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("invalid MySQL DSN")
		}
		cfg.ParseTime = true
		if err := cfg.Apply(mysql.Charset("utf8mb4", "utf8mb4_unicode_ci")); err != nil {
			return nil, err
		}
		cfg.Loc = time.UTC
		if cfg.Params == nil {
			cfg.Params = map[string]string{}
		}
		cfg.Params["time_zone"] = "'+00:00'"
		cfg.MultiStatements = false
		cfg.Timeout = 10 * time.Second
		cfg.ReadTimeout = 15 * time.Second
		cfg.WriteTimeout = 15 * time.Second
		connector, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, fmt.Errorf("invalid MySQL connection settings")
		}
		p = sql.OpenDB(connector)
	} else {
		p, e = sql.Open(name, dsn)
	}
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
		if d.Driver == "mysql" && f.Name() == "004_alerting.sql" {
			// Existing installs may use utf8mb4_bin or 0900 collations. Foreign
			// key columns must match their actual referenced columns exactly.
			for _, ref := range []struct{ table, column string }{{"servers", "server_id"}, {"users", "acknowledged_by"}} {
				var charset, collation string
				if e = conn.QueryRowContext(ctx, `SELECT CHARACTER_SET_NAME,COLLATION_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME='id'`, ref.table).Scan(&charset, &collation); e != nil {
					return e
				}
				valid := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
				if !valid.MatchString(charset) || !valid.MatchString(collation) {
					return fmt.Errorf("unsupported legacy identifier collation")
				}
				old := ref.column + " varchar(36)"
				b = []byte(strings.ReplaceAll(string(b), old, old+" CHARACTER SET "+charset+" COLLATE "+collation))
			}
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
				// MySQL DDL commits independently. An interrupted index migration
				// can be retried without marking partially applied DDL as complete.
				var mysqlErr *mysql.MySQLError
				if d.Driver == "mysql" && strings.HasPrefix(strings.TrimSpace(statement), "CREATE INDEX ") && errors.As(e, &mysqlErr) && mysqlErr.Number == 1061 {
					continue
				}
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

// Cleanup only removes expired authentication metadata, never events/audit logs.
// Each pass is bounded to avoid large transactions and long row locks.
func Cleanup(ctx context.Context, d *DB) error {
	now := time.Now().UTC()
	for _, item := range []struct {
		table, key, predicate string
		cutoff                time.Time
	}{
		{"sessions", "token_hash", "expires_at", now},
		{"agent_enrollments", "token_hash", "expires_at", now},
		{"login_attempts", "bucket_key", "window_start", now.Add(-24 * time.Hour)},
	} {
		rows, e := d.QueryContext(ctx, d.Bind("SELECT "+item.key+" FROM "+item.table+" WHERE "+item.predicate+"<? LIMIT 1000"), item.cutoff)
		if e != nil {
			return e
		}
		ids := []any{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(ids) == 0 {
			continue
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		if _, e = d.Exec(ctx, "DELETE FROM "+item.table+" WHERE "+item.key+" IN ("+marks+")", ids...); e != nil {
			return e
		}
	}
	return nil
}
