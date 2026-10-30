package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

type maintenanceConnector struct{ conn *maintenanceConn }

func (c maintenanceConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c maintenanceConnector) Driver() driver.Driver                        { return maintenanceDriver(c) }

type maintenanceDriver struct{ conn *maintenanceConn }

func (d maintenanceDriver) Open(string) (driver.Conn, error) { return d.conn, nil }

type maintenanceConn struct {
	queries []string
	failure string
}

func (c *maintenanceConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}
func (c *maintenanceConn) Close() error { return nil }
func (c *maintenanceConn) Begin() (driver.Tx, error) {
	return nil, errors.New("maintenance must not start a transaction")
}
func (c *maintenanceConn) ExecContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.queries = append(c.queries, q)
	if c.failure == "permission" {
		return nil, errors.New("permission denied")
	}
	return driver.RowsAffected(0), nil
}
func (c *maintenanceConn) QueryContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.queries = append(c.queries, q)
	if c.failure == "permission" {
		return nil, errors.New("permission denied")
	}
	return &maintenanceRows{failure: c.failure}, nil
}

type maintenanceRows struct {
	step    int
	failure string
}

func (r *maintenanceRows) Columns() []string { return []string{"Table", "Op", "Msg_type", "Msg_text"} }
func (r *maintenanceRows) Close() error      { return nil }
func (r *maintenanceRows) Next(out []driver.Value) error {
	r.step++
	if r.step > 2 {
		return io.EOF
	}
	out[0] = "guard.ssh_events"
	out[1] = "optimize"
	out[2] = "note"
	out[3] = "recreate + analyze instead"
	if r.step == 2 {
		out[2] = "status"
		out[3] = "OK"
		if r.failure == "error" {
			out[2] = "error"
			out[3] = "operation rejected"
		}
		if r.failure == "missing" {
			return io.EOF
		}
	}
	return nil
}

func TestExternalMaintenanceUsesOnlyOwnedTablesAndChecksResults(t *testing.T) {
	for _, dialect := range []string{"postgres", "mysql"} {
		t.Run(dialect, func(t *testing.T) {
			for _, failure := range []string{"", "permission", "error", "missing"} {
				if dialect == "postgres" && (failure == "error" || failure == "missing") {
					continue
				}
				conn := &maintenanceConn{failure: failure}
				pool := sql.OpenDB(maintenanceConnector{conn})
				db := &DB{pool, dialect}
				err := db.Reclaim(context.Background())
				pool.Close()
				if failure != "" {
					if err == nil {
						t.Fatalf("accepted %s failure", failure)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(conn.queries) != len(maintenanceTables) {
					t.Fatal("missing tables")
				}
				for i, query := range conn.queries {
					expected := "OPTIMIZE TABLE `" + maintenanceTables[i] + "`"
					if dialect == "postgres" {
						expected = `VACUUM (ANALYZE) "` + maintenanceTables[i] + `"`
					}
					if query != expected || strings.Contains(query, "FULL") {
						t.Fatalf("unexpected maintenance: %s", query)
					}
				}
			}
		})
	}
}
