package relaylog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxEntries = 500

// FileEntry is one file in a world directory snapshot.
type FileEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// PlayerInfo is a lightweight player record for the snapshot.
type PlayerInfo struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	IsHost bool   `json:"isHost"`
}

// Snapshot captures the state of a world directory at a point in time.
type Snapshot struct {
	IsHost        bool         `json:"isHost"`
	HasLocalData  bool         `json:"hasLocalData"`
	HasLevelMeta  bool         `json:"hasLevelMeta"`
	PlayerFiles   int          `json:"playerFiles"`
	Files         []FileEntry  `json:"files"`
	Players       []PlayerInfo `json:"players,omitempty"`
	PalCount      int          `json:"palCount,omitempty"`
}

// RepairInfo summarizes the repair applied during import/download.
type RepairInfo struct {
	ICH         bool   `json:"ich"`
	Pals        bool   `json:"pals"`
	Opaque      bool   `json:"opaque"`
	Builders    int    `json:"builders"`
	Slots       int    `json:"slots"`
	Ownerless   int    `json:"ownerless"`
	HostUID     string `json:"hostUid,omitempty"`
}

// Detail holds operation-specific information.
type Detail struct {
	ZipFiles       []string    `json:"zipFiles,omitempty"`
	ZipSize        int64       `json:"zipSize,omitempty"`
	LocalDataInZip bool        `json:"localDataInZip,omitempty"`
	ConvertFrom    string      `json:"convertFrom,omitempty"`
	ConvertTo      string      `json:"convertTo,omitempty"`
	Repair         *RepairInfo `json:"repair,omitempty"`
	Error          string      `json:"error,omitempty"`
}

// Entry is one relay history record.
type Entry struct {
	ID        string   `json:"id"`
	Time      string   `json:"time"`
	Op        string   `json:"op"`
	Version   string   `json:"version"`
	User      string   `json:"user"`
	GUID      string   `json:"guid"`
	WorldName string   `json:"worldName,omitempty"`
	SteamID   uint64   `json:"steamId,omitempty"`
	RealUID   string   `json:"realUid,omitempty"`
	Before    *Snapshot `json:"before,omitempty"`
	After     *Snapshot `json:"after,omitempty"`
	Detail    *Detail   `json:"detail,omitempty"`
}

// LogPath returns the local relay log path for a world GUID.
func LogPath(guid string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "PalSaveRelay", "relay-log-"+guid+".jsonl")
}

// Read reads the relay log for a GUID. Returns empty slice if not found.
func Read(guid string) []Entry {
	path := LogPath(guid)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries
}

// Write writes entries to the relay log for a GUID, capped at maxEntries.
func Write(guid string, entries []Entry) error {
	path := LogPath(guid)
	if path == "" {
		return fmt.Errorf("relaylog: no APPDATA")
	}
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for _, e := range entries {
		b, _ := json.Marshal(e)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// Append appends an entry to the relay log for a GUID.
func Append(guid string, e Entry) error {
	entries := Read(guid)
	entries = append(entries, e)
	return Write(guid, entries)
}

// Merge merges incoming entries into local, skipping duplicates by ID.
// Returns the merged list (sorted by time).
func Merge(local, incoming []Entry) []Entry {
	seen := make(map[string]bool, len(local))
	for _, e := range local {
		seen[e.ID] = true
	}
	for _, e := range incoming {
		if !seen[e.ID] {
			local = append(local, e)
			seen[e.ID] = true
		}
	}
	sort.Slice(local, func(i, j int) bool {
		return local[i].Time < local[j].Time
	})
	return local
}

// NewEntry creates a new Entry with a unique ID and current timestamp.
func NewEntry(op, version, user, guid string) Entry {
	return Entry{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Op:      op,
		Version: version,
		User:    user,
		GUID:    guid,
	}
}

// Serialize encodes entries to JSONL bytes for embedding in a zip.
func Serialize(entries []Entry) []byte {
	var sb strings.Builder
	for _, e := range entries {
		b, _ := json.Marshal(e)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}

// Deserialize decodes JSONL bytes into entries.
func Deserialize(data []byte) []Entry {
	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}
