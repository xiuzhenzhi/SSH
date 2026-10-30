package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type collectorHealth struct {
	mu             sync.Mutex
	state, message string
	poll, event    time.Time
}

func (h *collectorHealth) update(e error, at time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.poll = time.Now().UTC()
	h.state, h.message = "healthy", ""
	if e != nil {
		h.state = "error"
		h.message = e.Error()
		if len(h.message) > 256 {
			end := 256
			for end > 0 && !utf8.RuneStart(h.message[end]) {
				end--
			}
			h.message = h.message[:end]
		}
	}
	if !at.IsZero() {
		h.event = at
	}
}
func (h *collectorHealth) heartbeat(s *spool) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	q, d := 0, 0
	if s != nil {
		q, d = s.counts()
	}
	state, message := h.state, h.message
	if state == "" {
		state = "starting"
	}
	if q >= queueLimit || d > 0 {
		state = "error"
		if message == "" {
			if q >= queueLimit {
				message = "本地待传队列已满，采集已暂停；事件上传后会自动恢复。"
			} else {
				message = "存在被服务端拒绝的事件，已保存在本地隔离队列。请检查 Agent 日志。"
			}
		}
	}
	payload := map[string]any{"version": version, "collector_state": state, "collector_message": message, "queue_depth": q, "rejected_events": d}
	if !h.poll.IsZero() {
		payload["last_poll_at"] = h.poll
	}
	if !h.event.IsZero() {
		payload["last_event_at"] = h.event
	}
	return payload
}
func runAgent(ctx context.Context, cfg Config, stateDir string) {
	interval := reportInterval(cfg.ReportIntervalSeconds)
	s, e := openSpool(stateDir)
	if e == nil {
		if e = s.bindIdentity(cfg); e != nil {
			s.db.Close()
			s = nil
		}
	}
	health := &collectorHealth{}
	if e != nil {
		health.update(fmt.Errorf("cannot initialize durable Agent state: %w", e), time.Time{})
		log.Printf("Cannot initialize durable Agent state; collection disabled until service restart: %v", e)
	} else {
		defer s.db.Close()
	}
	var wg sync.WaitGroup
	if s != nil {
		first, cancel := context.WithTimeout(ctx, 10*time.Second)
		at, err := collectPlatform(first, s, cfg.LogFile)
		cancel()
		health.update(err, at)
		wg.Add(1)
		go func() {
			defer wg.Done()
			runCollector(ctx, s, cfg.LogFile, interval, health)
		}()
	}
	defer wg.Wait()
	tr := &transport{cfg: cfg}
	defer tr.close()
	nextHeartbeat := time.Time{}
	nextUpload := time.Time{}
	uploadFailures := 0
	lastHeartbeatError := time.Time{}
	wakeup := time.NewTimer(time.Hour)
	wakeup.Stop()
	defer wakeup.Stop()
	if s != nil {
		log.Printf("SSHGuard %s: durable SSH collection and HTTP(S)/WebSocket reporting enabled", version)
	} else {
		log.Printf("SSHGuard %s: heartbeat reporting enabled; durable SSH collection unavailable", version)
	}
	for ctx.Err() == nil {
		if time.Now().After(nextHeartbeat) {
			err := sendHeartbeat(ctx, tr, health.heartbeat(s), interval)
			if err != nil && ctx.Err() == nil && (lastHeartbeatError.IsZero() || time.Since(lastHeartbeatError) >= time.Minute) {
				log.Print("Heartbeat unavailable; retrying without discarding queued events")
				lastHeartbeatError = time.Now()
			} else if err == nil && !lastHeartbeatError.IsZero() {
				log.Print("Heartbeat connection restored")
				lastHeartbeatError = time.Time{}
			}
			nextHeartbeat = time.Now().Add(interval)
		}
		if s != nil && !time.Now().Before(nextUpload) {
			// Bounded drain avoids starving heartbeats and shutdown during a backlog.
			for i := 0; i < 10; i++ {
				sent, e := uploadBatch(ctx, tr, s)
				if e != nil {
					if ctx.Err() == nil {
						log.Print("Event upload unconfirmed; queued events retained")
					}
					uploadFailures = min(uploadFailures+1, 6)
					delay := time.Duration(1<<uploadFailures) * time.Second
					nextUpload = time.Now().Add(delay/2 + time.Duration(rand.Int64N(int64(delay/2))))
					break
				}
				uploadFailures = 0
				nextUpload = time.Now().Add(interval)
				if !sent || time.Now().After(nextHeartbeat) {
					break
				}
			}
		}
		next := nextHeartbeat
		if s != nil && nextUpload.Before(next) {
			next = nextUpload
		}
		wakeup.Reset(max(10*time.Millisecond, time.Until(next)))
		select {
		case <-ctx.Done():
			return
		case <-wakeup.C:
		}
	}
}

func runCollector(ctx context.Context, s *spool, path string, interval time.Duration, health *collectorHealth) {
	base := min(interval, 2*time.Second)
	delay, maxIdle := base, min(interval, 10*time.Second)
	pollTimer := time.NewTimer(delay)
	defer pollTimer.Stop()
	queueBlocked := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTimer.C:
			if q, _ := s.counts(); queueBlocked && q > queueLimit-500 {
				health.update(errQueueFull, time.Time{})
				pollTimer.Reset(base)
				continue
			}
			before, _ := s.position()
			pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			at, err := collectPlatform(pollCtx, s, path)
			cancel()
			if ctx.Err() != nil {
				return
			}
			health.update(err, at)
			queueBlocked = errors.Is(err, errQueueFull)
			after, _ := s.position()
			switch {
			case queueBlocked:
				delay = base
			case err != nil:
				delay = min(delay*2, 30*time.Second)
			case before == after:
				delay = min(delay*2, maxIdle)
			default:
				delay = base
			}
			pollTimer.Reset(delay)
		}
	}
}

func logTimestamp(line string, now time.Time) (time.Time, bool) {
	if token, _, ok := strings.Cut(line, " "); ok {
		if at, e := time.Parse(time.RFC3339Nano, token); e == nil {
			return at, true
		}
	}
	if len(line) < 15 {
		return time.Time{}, false
	}
	at, e := time.ParseInLocation("Jan _2 15:04:05 2006", line[:15]+fmt.Sprintf(" %d", now.Year()), now.Location())
	if e != nil {
		return time.Time{}, false
	}
	if at.After(now.Add(24 * time.Hour)) {
		at = at.AddDate(-1, 0, 0)
	}
	return at, true
}

// Follow rename rotation by file identity. Queue and byte offset share a durable transaction.
func collectFile(ctx context.Context, s *spool, path string) (time.Time, error) {
	previous, e := s.position()
	if e != nil {
		return time.Time{}, e
	}
	f, e := os.Open(path)
	if e != nil {
		return time.Time{}, fmt.Errorf("SSH log is missing or unreadable: %s; check log source and permissions", path)
	}
	defer func() { f.Close() }()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return time.Time{}, errors.New("SSH log source is not a regular file")
	}
	id, e := fileIdentityFromInfo(f, info)
	if e != nil {
		return time.Time{}, e
	}
	if previous.Kind != "file:"+path {
		return time.Time{}, s.append(nil, cursor{Kind: "file:" + path, FileID: id, Offset: info.Size(), At: time.Now().UTC()})
	}
	if previous.FileID == id && previous.Offset == info.Size() {
		return time.Time{}, nil
	}
	rotated := false
	if previous.FileID != id {
		candidate, e := findRotatedLog(ctx, filepath.Dir(path), previous.FileID)
		if e != nil {
			return time.Time{}, e
		}
		if candidate == nil {
			// Record an explicit gap before resuming the current file on the next poll.
			e = s.append(nil, cursor{Kind: previous.Kind, FileID: id, At: previous.At})
			if e != nil {
				return time.Time{}, e
			}
			return time.Time{}, errors.New("SSH log rotation gap: previous file no longer exists; resuming current file")
		}
		f.Close()
		f = candidate
		rotated = true
	}
	info, e = f.Stat()
	if e != nil {
		return time.Time{}, e
	}
	if info.Size() < previous.Offset {
		previous.Offset = 0
		previous.FileID = id
		if e = s.append(nil, previous); e != nil {
			return time.Time{}, e
		}
		return time.Time{}, errors.New("SSH log was truncated; collection resumes at the beginning")
	}
	if _, e = f.Seek(previous.Offset, io.SeekStart); e != nil {
		return time.Time{}, e
	}
	reader := bufio.NewReaderSize(f, 65536)
	next := previous
	events := []SSHEvent{}
	last := time.Time{}
	eof := false
	for i := 0; i < 500; i++ {
		if e = ctx.Err(); e != nil {
			return time.Time{}, e
		}
		raw, e := reader.ReadSlice('\n')
		if e == bufio.ErrBufferFull {
			return time.Time{}, errors.New("SSH log line exceeds 64 KiB; inspect source")
		}
		line := string(raw)
		if e == io.EOF {
			eof = true
			break
		} // Keep partial lines until their newline arrives.
		if e != nil {
			return time.Time{}, e
		}
		offset := next.Offset
		next.Offset += int64(len(line))
		if !strings.Contains(line, "sshd") {
			continue
		}
		at, ok := logTimestamp(line, time.Now())
		if !ok {
			continue
		}
		next.At = at
		event, ok := parseSSH(line, fmt.Sprintf("file|%s|%d|%s", previous.FileID, offset, line), at)
		if ok {
			events = append(events, event)
			last = at
		}
	}
	if rotated && eof {
		next = cursor{Kind: previous.Kind, FileID: id, At: next.At}
	}
	if e = s.append(events, next); e != nil {
		return time.Time{}, e
	}
	return last, nil
}

// Scan directory entries in small batches; large log directories must not all
// be retained in memory. Ownership of a matching file transfers to the caller.
func findRotatedLog(ctx context.Context, dir, previousID string) (*os.File, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, errors.New("cannot inspect rotated SSH logs")
	}
	defer d.Close()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, readErr := d.ReadDir(128)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !entry.Type().IsRegular() {
				continue
			}
			candidate, err := os.Open(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			id, err := fileIdentity(candidate)
			if err == nil && id == previousID {
				return candidate, nil
			}
			candidate.Close()
		}
		if readErr == io.EOF {
			return nil, nil
		}
		if readErr != nil {
			return nil, errors.New("cannot inspect rotated SSH logs")
		}
	}
}
