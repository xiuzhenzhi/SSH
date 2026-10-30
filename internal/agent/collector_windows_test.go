package agent

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestWindowsXMLUnicode(t *testing.T) {
	input := "<Events><Event><EventData><Data>用户🔒</Data></EventData></Event></Events>"
	words := utf16.Encode([]rune(input))
	data := make([]byte, 2+len(words)*2)
	data[0], data[1] = 255, 254
	for i, v := range words {
		binary.LittleEndian.PutUint16(data[2+i*2:], v)
	}
	if string(decodeWindowsXML(data)) != input {
		t.Fatal("Unicode event text corrupted")
	}
}
func TestWindowsNativeEventReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	records, e := readWindowsEvents(ctx, "qe", "Application", "/f:RenderedXml", "/e:Events", "/uni:true", "/c:1", "/rd:true")
	if e != nil {
		t.Fatal(e)
	}
	if len(records) != 1 || records[0].System.RecordID == 0 || !strings.Contains(records[0].System.Time.Value, "T") {
		t.Fatal("native Windows XML record not decoded")
	}
}
