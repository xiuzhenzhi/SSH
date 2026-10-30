package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"sort"
	"sshguard/internal/geoip"
	"sshguard/internal/ipaddress"
	"sshguard/internal/notify"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oschwald/maxminddb-golang/v2"
	"golang.org/x/sync/singleflight"
)

type geoCacheEntry struct {
	location geoip.Location
	err      error
	expires  time.Time
}
type geoIPState struct {
	mu         sync.Mutex
	cache      map[string]geoCacheEntry
	reader     *maxminddb.Reader
	revision   string
	flights    singleflight.Group
	updateMu   sync.Mutex
	client     *http.Client
	freeWindow time.Time
	freeCount  int
}

func (a *App) readGeoIP(ctx context.Context) (geoip.Config, error) {
	var raw string
	err := a.db.Row(ctx, `SELECT config_ciphertext FROM geoip_settings WHERE id=1`).Scan(&raw)
	if err == sql.ErrNoRows {
		return geoip.Config{Provider: "geojs"}, nil
	}
	if err != nil {
		return geoip.Config{}, err
	}
	plain, err := notify.Open(a.cfg.EncryptionKey, "geoip-settings", raw)
	if err != nil {
		return geoip.Config{}, err
	}
	var cfg geoip.Config
	err = json.Unmarshal(plain, &cfg)
	return cfg, err
}

func (a *App) geoIPSettings(c *gin.Context) {
	cfg, err := a.readGeoIP(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	var revision string
	var updated time.Time
	err = a.db.Row(c.Request.Context(), `SELECT revision,updated_at FROM geoip_database WHERE id=1`).Scan(&revision, &updated)
	if err != nil && err != sql.ErrNoRows {
		fail(c, err)
		return
	}
	var stamp any
	if err == nil {
		stamp = updated
	}
	ok(c, gin.H{"provider": cfg.Provider, "token_configured": cfg.Token != "", "database_ready": revision != "", "database_updated_at": stamp})
}

func (a *App) saveGeoIPSettings(c *gin.Context) {
	var in struct {
		geoip.Config
		ClearToken bool `json:"clear_token"`
	}
	if !bind(c, &in) {
		return
	}
	in.Token = strings.TrimSpace(in.Token)
	if strings.ContainsAny(in.Token, "\r\n\x00") {
		bad(c, 400, "凭证格式无效")
		return
	}
	err := a.mutate(c, c.MustGet("user").(User).ID, "geoip.settings.update", "geoip", func(tx *sql.Tx) error {
		// Serialize singleton creation and edits across database replicas.
		var locked string
		if err := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&locked); err != nil {
			return err
		}
		var previous string
		err := tx.QueryRowContext(c.Request.Context(), `SELECT config_ciphertext FROM geoip_settings WHERE id=1`).Scan(&previous)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && in.Token == "" && !in.ClearToken {
			plain, err := notify.Open(a.cfg.EncryptionKey, "geoip-settings", previous)
			if err != nil {
				return err
			}
			var old geoip.Config
			if err := json.Unmarshal(plain, &old); err != nil {
				return err
			}
			if old.Provider == in.Provider {
				in.Token = old.Token
			}
		}
		if in.ClearToken || (in.Provider != "ipinfo" && in.Provider != "ip-api") {
			in.Token = ""
		}
		raw, _ := json.Marshal(in.Config)
		sealed, err := notify.Seal(a.cfg.EncryptionKey, "geoip-settings", raw)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(c.Request.Context(), `DELETE FROM geoip_settings WHERE id=1`); err != nil {
			return err
		}
		_, err = tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO geoip_settings(id,config_ciphertext) VALUES(1,?)`), sealed)
		return err
	})
	if err != nil {
		fail(c, err)
		return
	}
	a.invalidateCountryRanking()
	a.hub.publish()
	a.geoIPSettings(c)
}

func (a *App) updateGeoIPDatabase(c *gin.Context) {
	if !a.geo.updateMu.TryLock() {
		bad(c, 409, "GeoIP 数据库正在更新")
		return
	}
	defer a.geo.updateMu.Unlock()
	cfg, err := a.readGeoIP(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	if cfg.Provider != "maxmind" {
		bad(c, 400, "仅 MaxMind 需要更新本地 GeoIP 数据库")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, geoip.DatabaseURL, nil)
	client := *a.geo.client
	client.Timeout = 15 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		bad(c, 502, "GeoIP 数据库下载失败，请检查主控网络后重试")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bad(c, 502, "GeoIP 数据库下载失败")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, geoip.MaxDatabaseBytes+1))
	if err != nil || len(raw) > geoip.MaxDatabaseBytes {
		bad(c, 502, "GeoIP 数据库过大或下载不完整")
		return
	}
	reader, err := geoip.ValidateDatabase(raw)
	if err != nil {
		bad(c, 502, err.Error())
		return
	}
	defer reader.Close()
	revision, updated := uuid.NewString(), time.Now().UTC()
	err = a.mutate(c, c.MustGet("user").(User).ID, "geoip.database.update", "geoip", func(tx *sql.Tx) error {
		var locked string
		if err := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&locked); err != nil {
			return err
		}
		if _, err := tx.ExecContext(c.Request.Context(), `DELETE FROM geoip_database WHERE id=1`); err != nil {
			return err
		}
		_, err := tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO geoip_database(id,revision,content,updated_at) VALUES(1,?,?,?)`), revision, raw, updated)
		return err
	})
	if err != nil {
		fail(c, err)
		return
	}
	a.invalidateCountryRanking()
	a.hub.publish()
	ok(c, gin.H{"database_ready": true, "database_updated_at": updated})
}

func (a *App) lookupGeoIP(ctx context.Context, cfg geoip.Config, address, revision string, cached bool) (geoip.Location, error) {
	if cfg.Provider == "none" {
		return geoip.Location{}, errors.New("GeoIP 已关闭")
	}
	if !ipaddress.Public(address) {
		return geoip.Location{}, errors.New("请输入公网 IPv4 或 IPv6 地址")
	}
	key := cfg.Provider + ":" + hash(cfg.Token) + ":" + revision + ":" + address
	if cached {
		a.geo.mu.Lock()
		entry, found := a.geo.cache[key]
		a.geo.mu.Unlock()
		if found && time.Now().Before(entry.expires) {
			return entry.location, entry.err
		}
	}
	pending := a.geo.flights.DoChan(key, func() (any, error) {
		// All callers share this lookup. One page changing or timing out must
		// not cancel another page's lookup or cache a caller-budget failure.
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 6*time.Second)
		defer cancel()
		if cfg.Provider == "ip-api" && cfg.Token == "" {
			a.geo.mu.Lock()
			now := time.Now()
			if !now.Before(a.geo.freeWindow) {
				a.geo.freeWindow = now.Add(time.Minute)
				a.geo.freeCount = 0
			}
			allowed := a.geo.freeCount < 40
			if allowed {
				a.geo.freeCount++
			}
			a.geo.mu.Unlock()
			if !allowed {
				return geoip.Location{}, errors.New("ip-api 免费接口查询频率已达上限，请稍后重试")
			}
		}
		var location geoip.Location
		var err error
		if cfg.Provider == "maxmind" {
			a.geo.mu.Lock()
			defer a.geo.mu.Unlock()
			if revision == "" {
				return location, errors.New("请先更新 GeoIP 数据库")
			}
			if a.geo.reader == nil || a.geo.revision != revision {
				var raw []byte
				var current string
				if err := a.db.Row(lookupCtx, `SELECT content,revision FROM geoip_database WHERE id=1`).Scan(&raw, &current); err != nil {
					return location, err
				}
				reader, err := geoip.ValidateDatabase(raw)
				if err != nil {
					return location, err
				}
				if a.geo.reader != nil {
					a.geo.reader.Close()
				}
				a.geo.reader, a.geo.revision = reader, current
			}
			return geoip.LookupDatabase(a.geo.reader, address)
		}
		location, err = geoip.Lookup(lookupCtx, a.geo.client, cfg, address)
		a.geo.mu.Lock()
		defer a.geo.mu.Unlock()
		if a.geo.cache == nil || len(a.geo.cache) >= 8192 {
			a.geo.cache = map[string]geoCacheEntry{}
		}
		ttl := 24 * time.Hour
		if err != nil {
			ttl = time.Minute
		}
		if lookupCtx.Err() == nil {
			a.geo.cache[key] = geoCacheEntry{location: location, err: err, expires: time.Now().Add(ttl)}
		}
		return location, err
	})
	select {
	case <-ctx.Done():
		return geoip.Location{}, ctx.Err()
	case result := <-pending:
		if result.Err != nil {
			return geoip.Location{}, result.Err
		}
		return result.Val.(geoip.Location), nil
	}
}

func (a *App) geoIPRevision(ctx context.Context, cfg geoip.Config) (string, error) {
	if cfg.Provider != "maxmind" {
		return "", nil
	}
	var revision string
	err := a.db.Row(ctx, `SELECT revision FROM geoip_database WHERE id=1`).Scan(&revision)
	if err == sql.ErrNoRows {
		err = nil
	}
	return revision, err
}

func (a *App) testGeoIP(c *gin.Context) {
	var in struct {
		IP string `json:"ip" binding:"required,ip"`
	}
	if !bind(c, &in) {
		return
	}
	if !ipaddress.Public(in.IP) {
		bad(c, 400, "请输入公网 IPv4 或 IPv6 地址")
		return
	}
	cfg, err := a.readGeoIP(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	if cfg.Provider == "none" {
		bad(c, 400, "请先选择地理位置数据提供商")
		return
	}
	revision, err := a.geoIPRevision(c.Request.Context(), cfg)
	if err != nil {
		fail(c, err)
		return
	}
	if cfg.Provider == "maxmind" && revision == "" {
		bad(c, 409, "请先更新 GeoIP 数据库")
		return
	}
	location, err := a.lookupGeoIP(c.Request.Context(), cfg, in.IP, revision, false)
	if err != nil {
		bad(c, 502, err.Error())
		return
	}
	ok(c, gin.H{"ip": in.IP, "provider": cfg.Provider, "country_code": location.Code, "country_name": location.Name})
}

// Keep the raw NIC snapshot in SQL, but expose only public display addresses,
// with the authenticated connection's source as a NAT fallback.
func serverAddresses(row map[string]any) []string {
	var raw []string
	if value, ok := row["ip_addresses"].(string); ok {
		_ = json.Unmarshal([]byte(value), &raw)
	}
	public := []string{}
	seen, families := map[string]bool{}, map[bool]bool{}
	for _, value := range raw {
		ip, err := netip.ParseAddr(value)
		if err != nil {
			continue
		}
		value = ip.Unmap().String()
		if seen[value] {
			continue
		}
		seen[value] = true
		if ipaddress.Public(value) {
			public = append(public, value)
			families[ip.Unmap().Is4()] = true
		}
	}
	if observed, ok := row["observed_public_ip"].(string); ok && ipaddress.Public(observed) {
		ip := netip.MustParseAddr(observed).Unmap()
		if !families[ip.Is4()] {
			public = append(public, ip.String())
		}
	}
	sort.Slice(public, func(i, j int) bool {
		a, b := netip.MustParseAddr(public[i]), netip.MustParseAddr(public[j])
		if a.Is4() != b.Is4() {
			return a.Is4()
		}
		return a.Less(b)
	})
	rawPublic, _ := json.Marshal(public)
	row["public_ip_addresses"], row["ip_addresses"] = public, string(rawPublic)
	delete(row, "observed_public_ip")
	row["country_code"], row["country_name"] = "", ""
	return public
}

func (a *App) decorateServers(ctx context.Context, rows []map[string]any) {
	addresses := make([][]string, len(rows))
	for i, row := range rows {
		addresses[i] = serverAddresses(row)
	}
	cfg, err := a.readGeoIP(ctx)
	if err != nil || cfg.Provider == "none" {
		return
	}
	revision, err := a.geoIPRevision(ctx, cfg)
	if err != nil {
		return
	}
	// A provider outage must not prevent browsing or delay every live refresh.
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	var group sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i, row := range rows {
		if len(addresses[i]) == 0 {
			continue
		}
		group.Add(1)
		go func(row map[string]any, ips []string) {
			defer group.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				return
			}
			for _, address := range ips {
				location, err := a.lookupGeoIP(ctx, cfg, address, revision, true)
				if err == nil {
					row["country_code"], row["country_name"] = location.Code, location.Name
					return
				}
				if ctx.Err() != nil {
					return
				}
			}
		}(row, addresses[i])
	}
	group.Wait()
}
