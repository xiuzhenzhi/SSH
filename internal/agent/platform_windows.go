package agent

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const serviceName = "SSHGuardAgent"

func configureService(action, path string) error {
	if action != "install" && action != "uninstall" {
		return fmt.Errorf("service action must be install or uninstall")
	}
	manager, e := mgr.Connect()
	if e != nil {
		return e
	}
	defer manager.Disconnect()
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	path, e = filepath.Abs(path)
	if e != nil {
		return e
	}
	expected := syscall.EscapeArg(exe) + " -config " + syscall.EscapeArg(path)
	s, e := manager.OpenService(serviceName)
	if e == nil {
		defer s.Close()
		cfg, e := s.Config()
		if e != nil {
			return e
		}
		if !strings.EqualFold(cfg.BinaryPathName, expected) {
			return fmt.Errorf("existing service points to a different installation; refusing to replace it")
		}
		status, e := s.Query()
		if e != nil {
			return e
		}
		if status.State != svc.Stopped {
			return fmt.Errorf("stop SSHGuardAgent before changing the service")
		}
		if action == "uninstall" {
			return s.Delete()
		}
		cfg.StartType = mgr.StartAutomatic
		cfg.ServiceStartName = `NT AUTHORITY\LocalService`
		cfg.DelayedAutoStart = true
		cfg.SidType = windows.SERVICE_SID_TYPE_UNRESTRICTED
		if e = s.UpdateConfig(cfg); e != nil {
			return e
		}
	} else {
		if !errors.Is(e, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return e
		}
		if action == "uninstall" {
			return nil
		}
		if _, e = loadConfig(path); e != nil {
			return e
		}
		s, e = manager.CreateService(serviceName, exe, mgr.Config{StartType: mgr.StartAutomatic, ServiceStartName: `NT AUTHORITY\LocalService`, DisplayName: "SSHGuard Agent", Description: "SSHGuard durable SSH login collection and reporting", DelayedAutoStart: true, SidType: windows.SERVICE_SID_TYPE_UNRESTRICTED}, "-config", path)
		if e != nil {
			return e
		}
		defer s.Close()
	}
	if e = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 10 * time.Second}}, 86400); e != nil {
		return e
	}
	key, e := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\EventLog\Application\`+serviceName, registry.QUERY_VALUE)
	if e == nil {
		return key.Close()
	}
	if !errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
		return e
	}
	return eventlog.InstallAsEventCreate(serviceName, eventlog.Info|eventlog.Warning|eventlog.Error)
}
func defaultConfigPath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "SSHGuard", "agent.json")
}

type serviceHandler struct{ run func(context.Context) }

func (s *serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.run(ctx) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	status <- current
	for {
		select {
		case <-done:
			return false, 0
		case r, open := <-requests:
			if !open {
				cancel()
				<-done
				return false, 0
			}
			switch r.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending, WaitHint: 20000}
				cancel()
				<-done
				return false, 0
			}
		}
	}
}

type eventWriter struct{ log *eventlog.Log }

func (w eventWriter) Write(p []byte) (int, error) {
	e := w.log.Info(1, strings.TrimSpace(string(p)))
	if e != nil {
		return 0, e
	}
	return len(p), nil
}
func runPlatform(run func(context.Context)) error {
	isService, e := svc.IsWindowsService()
	if e != nil {
		return e
	}
	if !isService {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		run(ctx)
		return nil
	}
	events, e := eventlog.Open(serviceName)
	if e != nil {
		return e
	}
	defer events.Close()
	log.SetOutput(eventWriter{events})
	return svc.Run(serviceName, &serviceHandler{run: run})
}
