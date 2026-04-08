// Package store provides SQLite persistence for scrib meetings, transcripts, and speakers.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection.
type DB struct {
	db *sql.DB
}

// Open opens or creates the scrib database at the given path.
func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS speakers (
			id        INTEGER PRIMARY KEY,
			name      TEXT NOT NULL,
			embedding BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS meetings (
			id           INTEGER PRIMARY KEY,
			name         TEXT NOT NULL,
			recorded_at  DATETIME NOT NULL,
			duration_s   REAL,
			template     TEXT DEFAULT 'standup',
			audio_path   TEXT,
			num_speakers INTEGER DEFAULT 0,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS segments (
			id         INTEGER PRIMARY KEY,
			meeting_id INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
			speaker_id INTEGER REFERENCES speakers(id),
			speaker_label TEXT,
			start_s    REAL NOT NULL,
			end_s      REAL NOT NULL,
			text       TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS summaries (
			id         INTEGER PRIMARY KEY,
			meeting_id INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
			template   TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS segments_fts USING fts5(
			text,
			content=segments,
			content_rowid=id
		);

		CREATE TRIGGER IF NOT EXISTS segments_ai AFTER INSERT ON segments BEGIN
			INSERT INTO segments_fts(rowid, text) VALUES (new.id, new.text);
		END;

		CREATE TRIGGER IF NOT EXISTS segments_ad AFTER DELETE ON segments BEGIN
			INSERT INTO segments_fts(segments_fts, rowid, text) VALUES('delete', old.id, old.text);
		END;

		CREATE TRIGGER IF NOT EXISTS segments_au AFTER UPDATE ON segments BEGIN
			INSERT INTO segments_fts(segments_fts, rowid, text) VALUES('delete', old.id, old.text);
			INSERT INTO segments_fts(rowid, text) VALUES (new.id, new.text);
		END;
	`)
	return err
}

// Meeting represents a recorded meeting.
type Meeting struct {
	ID          int64
	Name        string
	RecordedAt  time.Time
	DurationS   float64
	Template    string
	AudioPath   string
	NumSpeakers int
}

// Segment represents a diarised transcript segment.
type Segment struct {
	ID           int64
	MeetingID    int64
	SpeakerID    *int64
	SpeakerLabel string
	StartS       float64
	EndS         float64
	Text         string
}

// Speaker represents a known speaker with optional voiceprint.
type Speaker struct {
	ID        int64
	Name      string
	Embedding []byte
}

// Summary represents a meeting summary generated from a template.
type Summary struct {
	ID        int64
	MeetingID int64
	Template  string
	Content   string
	CreatedAt time.Time
}

// InsertMeeting creates a new meeting record.
func (d *DB) InsertMeeting(m *Meeting) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO meetings (name, recorded_at, duration_s, template, audio_path, num_speakers)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.Name, m.RecordedAt, m.DurationS, m.Template, m.AudioPath, m.NumSpeakers,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertSegment adds a transcript segment to a meeting.
func (d *DB) InsertSegment(s *Segment) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO segments (meeting_id, speaker_id, speaker_label, start_s, end_s, text)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.MeetingID, s.SpeakerID, s.SpeakerLabel, s.StartS, s.EndS, s.Text,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertSummary stores a summary for a meeting.
func (d *DB) InsertSummary(s *Summary) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO summaries (meeting_id, template, content) VALUES (?, ?, ?)`,
		s.MeetingID, s.Template, s.Content,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertSpeaker adds a known speaker.
func (d *DB) InsertSpeaker(name string, embedding []byte) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO speakers (name, embedding) VALUES (?, ?)`,
		name, embedding,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListMeetings returns meetings ordered by most recent.
func (d *DB) ListMeetings(limit int) ([]Meeting, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.Query(
		`SELECT id, name, recorded_at, duration_s, template, audio_path, num_speakers
		 FROM meetings ORDER BY recorded_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meetings []Meeting
	for rows.Next() {
		var m Meeting
		if err := rows.Scan(&m.ID, &m.Name, &m.RecordedAt, &m.DurationS, &m.Template, &m.AudioPath, &m.NumSpeakers); err != nil {
			return nil, err
		}
		meetings = append(meetings, m)
	}
	return meetings, nil
}

// GetMeeting returns a single meeting by ID.
func (d *DB) GetMeeting(id int64) (*Meeting, error) {
	var m Meeting
	err := d.db.QueryRow(
		`SELECT id, name, recorded_at, duration_s, template, audio_path, num_speakers
		 FROM meetings WHERE id = ?`, id,
	).Scan(&m.ID, &m.Name, &m.RecordedAt, &m.DurationS, &m.Template, &m.AudioPath, &m.NumSpeakers)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetSegments returns all segments for a meeting.
func (d *DB) GetSegments(meetingID int64) ([]Segment, error) {
	rows, err := d.db.Query(
		`SELECT id, meeting_id, speaker_id, speaker_label, start_s, end_s, text
		 FROM segments WHERE meeting_id = ? ORDER BY start_s`, meetingID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []Segment
	for rows.Next() {
		var s Segment
		if err := rows.Scan(&s.ID, &s.MeetingID, &s.SpeakerID, &s.SpeakerLabel, &s.StartS, &s.EndS, &s.Text); err != nil {
			return nil, err
		}
		segments = append(segments, s)
	}
	return segments, nil
}

// GetSummaries returns all summaries for a meeting.
func (d *DB) GetSummaries(meetingID int64) ([]Summary, error) {
	rows, err := d.db.Query(
		`SELECT id, meeting_id, template, content, created_at
		 FROM summaries WHERE meeting_id = ? ORDER BY created_at DESC`, meetingID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.MeetingID, &s.Template, &s.Content, &s.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// ListSpeakers returns all known speakers.
func (d *DB) ListSpeakers() ([]Speaker, error) {
	rows, err := d.db.Query(`SELECT id, name, embedding FROM speakers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var speakers []Speaker
	for rows.Next() {
		var s Speaker
		if err := rows.Scan(&s.ID, &s.Name, &s.Embedding); err != nil {
			return nil, err
		}
		speakers = append(speakers, s)
	}
	return speakers, nil
}

// Search performs full-text search across all transcript segments.
func (d *DB) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.Query(`
		SELECT s.id, s.meeting_id, s.speaker_label, s.start_s, s.end_s, s.text,
		       m.name, m.recorded_at
		FROM segments_fts fts
		JOIN segments s ON s.id = fts.rowid
		JOIN meetings m ON m.id = s.meeting_id
		WHERE segments_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SegmentID, &r.MeetingID, &r.SpeakerLabel, &r.StartS, &r.EndS, &r.Text, &r.MeetingName, &r.RecordedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// SearchResult is a transcript match with meeting context.
type SearchResult struct {
	SegmentID    int64
	MeetingID    int64
	SpeakerLabel string
	StartS       float64
	EndS         float64
	Text         string
	MeetingName  string
	RecordedAt   time.Time
}

// DefaultPath returns the default database path (~/.local/share/scrib/scrib.db).
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "scrib", "scrib.db")
}

// FormatTranscript renders segments as readable text.
func FormatTranscript(segments []Segment) string {
	var sb strings.Builder
	for _, seg := range segments {
		mins := int(seg.StartS) / 60
		secs := int(seg.StartS) % 60
		label := seg.SpeakerLabel
		if label == "" {
			label = "UNKNOWN"
		}
		fmt.Fprintf(&sb, "**%s** (%d:%02d): %s\n", label, mins, secs, seg.Text)
	}
	return sb.String()
}
