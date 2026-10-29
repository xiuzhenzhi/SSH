package api

import "time"

type dashboardSnapshot struct {
	at   time.Time
	data any
}

// Call with dashboardMu held. Keys are a fixed set of supported ranges.
func (a *App) cachedDashboard(key string) (any, bool) {
	v, ok := a.dashboardCache[key]
	return v.data, ok && time.Since(v.at) < 5*time.Second
}
func (a *App) saveDashboard(key string, data any) {
	if a.dashboardCache == nil {
		a.dashboardCache = make(map[string]dashboardSnapshot)
	}
	a.dashboardCache[key] = dashboardSnapshot{time.Now(), data}
}
