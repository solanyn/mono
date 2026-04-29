package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// OutboxEntry is a pending upload the TUI parked for later retry. One file
// per entry under <outbox>/*.json, written atomically via rename.
type OutboxEntry struct {
	UUID       string    `json:"uuid,omitempty"` // optional: set if metadata stage already succeeded
	Name       string    `json:"name"`
	Template   string    `json:"template,omitempty"`
	WAVPath    string    `json:"wav_path"`
	DurationS  float64   `json:"duration_s"`
	ServerURL  string    `json:"server_url"`
	RecordedAt time.Time `json:"recorded_at"`
	LastErr    string    `json:"last_err,omitempty"`
}

// OutboxWrite parks an entry for later retry. Returns the path written.
// outboxDir is created (0o700) if missing. Write is atomic via tmp+rename.
func OutboxWrite(outboxDir string, e OutboxEntry) (string, error) {
	if err := os.MkdirAll(outboxDir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir outbox: %w", err)
	}
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now()
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s.json", e.RecordedAt.UTC().Format("20060102T150405"), sanitize(e.Name))
	dst := filepath.Join(outboxDir, name)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// OutboxList returns all .json entries sorted by recorded_at ascending.
// Corrupt files are skipped (with their path returned in the errors slice).
func OutboxList(outboxDir string) ([]OutboxEntry, []string, error) {
	entries, err := os.ReadDir(outboxDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var out []OutboxEntry
	var bad []string
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		p := filepath.Join(outboxDir, de.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			bad = append(bad, p)
			continue
		}
		var e OutboxEntry
		if err := json.Unmarshal(data, &e); err != nil {
			bad = append(bad, p)
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.Before(out[j].RecordedAt) })
	return out, bad, nil
}

// OutboxDelete removes the on-disk entry matching name/recorded_at pair.
// Matches the filename scheme used by OutboxWrite.
func OutboxDelete(outboxDir string, e OutboxEntry) error {
	name := fmt.Sprintf("%s-%s.json", e.RecordedAt.UTC().Format("20060102T150405"), sanitize(e.Name))
	return os.Remove(filepath.Join(outboxDir, name))
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == '-' || r == '_' || r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "meeting"
	}
	return string(out)
}
