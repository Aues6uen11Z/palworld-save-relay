package main

import (
	"archive/zip"
	"bytes"
	"testing"

	"palworld-save-relay/internal/relaylog"
)

// buildZipWithRelayLog builds an in-memory zip containing a _relay_log.jsonl
// entry (and optionally extra files), for testing sourceGUIDFromZip.
func buildZipWithRelayLog(t *testing.T, guid string, extra map[string][]byte) []byte {
	t.Helper()
	entries := []relaylog.Entry{{ID: "1", Op: "export", GUID: guid}}
	body := relaylog.Serialize(entries)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(relayLogFilename)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(body)
	for name, data := range extra {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSourceGUIDFromZip_Present(t *testing.T) {
	zipBytes := buildZipWithRelayLog(t, "ABCDEF0123456789ABCDEF0123456789", nil)
	got, ok := sourceGUIDFromZip(zipBytes)
	if !ok {
		t.Fatal("expected ok=true when relay log present")
	}
	if got != "ABCDEF0123456789ABCDEF0123456789" {
		t.Fatalf("got guid %q", got)
	}
}

func TestSourceGUIDFromZip_Absent(t *testing.T) {
	// A zip with no relay log: identity cannot be verified.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("Level.sav")
	w.Write([]byte("not-a-real-save"))
	zw.Close()
	if _, ok := sourceGUIDFromZip(buf.Bytes()); ok {
		t.Fatal("expected ok=false for zip without relay log")
	}
}

func TestSourceGUIDFromZip_EmptyGUID(t *testing.T) {
	// Relay log present but no GUID recorded: treat as unverifiable.
	entries := []relaylog.Entry{{ID: "1", Op: "export", GUID: ""}}
	body := relaylog.Serialize(entries)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(relayLogFilename)
	w.Write(body)
	zw.Close()
	if _, ok := sourceGUIDFromZip(buf.Bytes()); ok {
		t.Fatal("expected ok=false when guid is empty")
	}
}
