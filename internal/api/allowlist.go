package api

import (
	"database/sql"
	"errors"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func normalizePrefix(value string) (string, error) {
	value = strings.TrimSpace(value)
	if ip, err := netip.ParseAddr(value); err == nil && ip.Zone() == "" {
		ip = ip.Unmap()
		return netip.PrefixFrom(ip, ip.BitLen()).String(), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return "", errors.New("invalid IP or CIDR")
	}
	// Normalize IPv4-mapped IPv6 prefixes as IPv4 too.
	if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
	}
	return prefix.Masked().String(), nil
}

func (a *App) allowlist(c *gin.Context) {
	a.list(c, `SELECT id,cidr,notes,created_at FROM ip_allowlist ORDER BY created_at DESC,id LIMIT ? OFFSET ?`)
}

var errAllowlistConflict = errors.New("allowlist conflict")

func (a *App) saveAllowlist(c *gin.Context) {
	var in struct {
		CIDR  string `json:"cidr" binding:"required,max=64"`
		Notes string `json:"notes" binding:"max=256"`
	}
	if !bind(c, &in) {
		return
	}
	prefix, err := normalizePrefix(in.CIDR)
	if err != nil {
		bad(c, 400, "请输入有效的 IP 地址或 CIDR 网段")
		return
	}
	id := uuid.NewString()
	err = a.mutate(c, c.MustGet("user").(User).ID, "allowlist.create", id, func(tx *sql.Tx) error {
		var locked string
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&locked); e != nil {
			return e
		}
		var count int
		if e := tx.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM ip_allowlist`).Scan(&count); e != nil {
			return e
		}
		if count >= 500 {
			return errAllowlistConflict
		}
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT COUNT(*) FROM ip_allowlist WHERE cidr=?`), prefix).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return errAllowlistConflict
		}
		_, e := tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO ip_allowlist(id,cidr,notes) VALUES(?,?,?)`), id, prefix, strings.TrimSpace(in.Notes))
		return e
	})
	if errors.Is(err, errAllowlistConflict) {
		bad(c, 409, "地址已存在或白名单已达 500 条上限")
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"id": id, "cidr": prefix})
}

func (a *App) deleteAllowlist(c *gin.Context) {
	err := a.mutate(c, c.MustGet("user").(User).ID, "allowlist.delete", c.Param("id"), func(tx *sql.Tx) error {
		result, e := tx.ExecContext(c.Request.Context(), a.db.Bind(`DELETE FROM ip_allowlist WHERE id=?`), c.Param("id"))
		if e != nil {
			return e
		}
		if n, e := result.RowsAffected(); e != nil {
			return e
		} else if n == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}
