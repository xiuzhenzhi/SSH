package api

import (
	"context"
	"sort"
	"sync"
	"time"

	"sshguard/internal/ipaddress"
)

// Enrichment is bounded independently of SQL: an unavailable location provider
// must not prevent viewing events. Repeated addresses share the existing cache.
func (a *App) decorateEventCountries(ctx context.Context, rows []map[string]any) string {
	groups := make(map[string][]map[string]any)
	for _, row := range rows {
		address, _ := row["source_ip"].(string)
		public := ipaddress.Public(address)
		row["country_code"], row["country_name"], row["source_is_public"] = "", "", public
		if public {
			groups[address] = append(groups[address], row)
		}
	}
	cfg, err := a.readGeoIP(ctx)
	if err != nil {
		return "unavailable"
	}
	if cfg.Provider == "none" {
		return "disabled"
	}
	revision, err := a.geoIPRevision(ctx, cfg)
	if err != nil || (cfg.Provider == "maxmind" && revision == "") {
		return "unavailable"
	}
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	addresses := make([]string, 0, len(groups))
	for address := range groups {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool {
		left, _ := groups[addresses[i]][0]["attempts"].(int64)
		right, _ := groups[addresses[j]][0]["attempts"].(int64)
		if left != right {
			return left > right
		}
		return addresses[i] < addresses[j]
	})
	jobs := make(chan string)
	var workers sync.WaitGroup
	for i := 0; i < min(8, len(addresses)); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for address := range jobs {
				location, err := a.lookupGeoIP(ctx, cfg, address, revision, true)
				if err == nil {
					for _, row := range groups[address] {
						row["country_code"], row["country_name"] = location.Code, location.Name
					}
				}
			}
		}()
	}
send:
	for _, address := range addresses {
		select {
		case jobs <- address:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	workers.Wait()
	return "ready"
}
