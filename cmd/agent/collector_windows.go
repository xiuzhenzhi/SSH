package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func defaultStateDir() string { return filepath.Join(filepath.Dir(defaultConfigPath()), "state") }
func fileIdentity(f *os.File) (string, error) {
	var info windows.ByHandleFileInformation
	if e := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); e != nil {
		return "", e
	}
	return fmt.Sprintf("%d:%d:%d", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}

type windowsEvent struct {
	System struct {
		RecordID uint64 `xml:"EventRecordID"`
		Time     struct {
			Value string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	Data struct {
		Values []string `xml:"Data"`
	} `xml:"EventData"`
	Render struct {
		Message string `xml:"Message"`
	} `xml:"RenderingInfo"`
}

func decodeWindowsXML(data []byte) []byte {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || (len(data) > 1 && data[0] == '<' && data[1] == 0) {
		data = bytes.TrimPrefix(data, []byte{0xff, 0xfe})
		words := make([]uint16, len(data)/2)
		for i := range words {
			words[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		return []byte(string(utf16.Decode(words)))
	}
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
}

// The single collector goroutine owns this timer; avoid two wevtutil processes per idle poll.
var nextLogResetCheck time.Time

func readWindowsEvents(ctx context.Context, args ...string) ([]windowsEvent, error) {
	cmd := exec.CommandContext(ctx, "wevtutil.exe", args...)
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	if e = cmd.Start(); e != nil {
		return nil, errors.New("Windows OpenSSH event reader could not start")
	}
	// The input is bounded; no shell interpolation and no provider output in logs.
	data, e := io.ReadAll(io.LimitReader(pipe, (4<<20)+1))
	if e != nil || len(data) > 4<<20 {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, errors.New("Windows OpenSSH event output exceeds limit")
	}
	if e = cmd.Wait(); e != nil {
		return nil, errors.New("OpenSSH/Operational unavailable; enable OpenSSH logging and grant the service read access")
	}
	data = decodeWindowsXML(data)
	// /uni:true avoids machine-code-page loss in usernames. /e:Events supplies one XML root.
	var root struct {
		Events []windowsEvent `xml:"Event"`
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	if e = xml.Unmarshal(data, &root); e != nil {
		return nil, errors.New("invalid Windows OpenSSH event XML")
	}
	return root.Events, nil
}
func collectPlatform(ctx context.Context, s *spool, explicit string) (time.Time, error) {
	if explicit != "" {
		return collectFile(ctx, s, explicit)
	}
	previous, e := s.position()
	if e != nil {
		return time.Time{}, e
	}
	base := []string{"qe", "OpenSSH/Operational", "/f:RenderedXml", "/e:Events", "/uni:true"}
	if previous.Kind != "windows" {
		records, e := readWindowsEvents(ctx, append(base, "/rd:true", "/c:1")...)
		if e != nil {
			return time.Time{}, e
		}
		initial := cursor{Kind: "windows", Position: "0", At: time.Now().UTC()}
		if len(records) > 0 {
			initial.Position = strconv.FormatUint(records[0].System.RecordID, 10)
		}
		return time.Time{}, s.append(nil, initial)
	}
	position, e := strconv.ParseUint(previous.Position, 10, 64)
	if e != nil {
		return time.Time{}, errors.New("invalid Windows event cursor")
	}
	records, e := readWindowsEvents(ctx, append(base, "/rd:false", "/c:100", fmt.Sprintf("/q:*[System[EventRecordID > %d]]", position))...)
	if e != nil {
		return time.Time{}, e
	}
	if len(records) == 0 && position > 0 && time.Now().After(nextLogResetCheck) {
		nextLogResetCheck = time.Now().Add(time.Minute)
		latest, e := readWindowsEvents(ctx, append(base, "/rd:true", "/c:1")...)
		if e != nil {
			return time.Time{}, e
		}
		if len(latest) == 0 || latest[0].System.RecordID < position {
			previous.Position = "0"
			if e = s.append(nil, previous); e != nil {
				return time.Time{}, e
			}
			return time.Time{}, errors.New("OpenSSH event log was cleared; cursor reset with a possible collection gap")
		}
	}
	next := previous
	events := []SSHEvent{}
	last := time.Time{}
	for _, record := range records {
		at, e := time.Parse(time.RFC3339Nano, record.System.Time.Value)
		if e != nil {
			return time.Time{}, errors.New("invalid OpenSSH event time")
		}
		message := record.Render.Message
		if message == "" {
			message = strings.Join(record.Data.Values, " ")
		}
		next.Position, next.At = strconv.FormatUint(record.System.RecordID, 10), at
		if event, ok := parseSSH(message, fmt.Sprintf("windows|%d|%s", record.System.RecordID, record.System.Time.Value), at); ok {
			events = append(events, event)
			last = at
		}
	}
	if e = s.append(events, next); e != nil {
		return time.Time{}, e
	}
	return last, nil
}
