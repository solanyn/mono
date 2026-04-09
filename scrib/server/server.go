// Package server provides the scrib sync server — receives meetings, stores in postgres + S3.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	_ "github.com/lib/pq"
)

// Config holds server configuration.
type Config struct {
	ListenAddr string
	DatabaseURL string
	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
}

// Server is the scrib sync server.
type Server struct {
	cfg    Config
	db     *sql.DB
	s3     *minio.Client
	router chi.Router
}

// New creates a new scrib server.
func New(cfg Config) (*Server, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s3Client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}

	s := &Server{cfg: cfg, db: db, s3: s3Client}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s.router = s.routes()
	return s, nil
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/sync/push", s.handlePush)
		r.Get("/sync/pull", s.handlePull)

		r.Get("/meetings", s.handleListMeetings)
		r.Get("/meetings/{uuid}", s.handleGetMeeting)
		r.Get("/search", s.handleSearch)
		r.Get("/speakers", s.handleListSpeakers)
		r.Post("/speakers", s.handleCreateSpeaker)

		r.Post("/audio/{uuid}", s.handleUploadAudio)
		r.Get("/audio/{uuid}", s.handleDownloadAudio)
	})

	return r
}

// ListenAndServe starts the server.
func (s *Server) ListenAndServe() error {
	log.Printf("scrib-server listening on %s", s.cfg.ListenAddr)
	return http.ListenAndServe(s.cfg.ListenAddr, s.router)
}

func (s *Server) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS speakers (
			id         SERIAL PRIMARY KEY,
			uuid       TEXT NOT NULL UNIQUE,
			name       TEXT NOT NULL,
			embedding  BYTEA,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS meetings (
			id             SERIAL PRIMARY KEY,
			uuid           TEXT NOT NULL UNIQUE,
			name           TEXT NOT NULL,
			recorded_at    TIMESTAMPTZ NOT NULL,
			duration_s     DOUBLE PRECISION,
			template       TEXT DEFAULT 'standup',
			audio_blob_key TEXT,
			num_speakers   INTEGER DEFAULT 0,
			client_id      TEXT,
			created_at     TIMESTAMPTZ DEFAULT NOW(),
			updated_at     TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS segments (
			id            SERIAL PRIMARY KEY,
			uuid          TEXT NOT NULL UNIQUE,
			meeting_id    INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
			speaker_id    INTEGER REFERENCES speakers(id),
			speaker_label TEXT,
			start_s       DOUBLE PRECISION NOT NULL,
			end_s         DOUBLE PRECISION NOT NULL,
			text          TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS summaries (
			id         SERIAL PRIMARY KEY,
			uuid       TEXT NOT NULL UNIQUE,
			meeting_id INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
			template   TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_meetings_recorded ON meetings(recorded_at DESC);
		CREATE INDEX IF NOT EXISTS idx_segments_meeting ON segments(meeting_id);
		CREATE INDEX IF NOT EXISTS idx_summaries_meeting ON summaries(meeting_id);
	`)
	return err
}

// --- Sync types ---

// SyncPayload is what the client pushes.
type SyncPayload struct {
	ClientID string         `json:"client_id"`
	Meetings []SyncMeeting  `json:"meetings"`
	Speakers []SyncSpeaker  `json:"speakers,omitempty"`
}

type SyncMeeting struct {
	UUID        string        `json:"uuid"`
	Name        string        `json:"name"`
	RecordedAt  time.Time     `json:"recorded_at"`
	DurationS   float64       `json:"duration_s"`
	Template    string        `json:"template"`
	NumSpeakers int           `json:"num_speakers"`
	Segments    []SyncSegment `json:"segments"`
	Summaries   []SyncSummary `json:"summaries"`
}

type SyncSegment struct {
	UUID         string  `json:"uuid"`
	SpeakerLabel string  `json:"speaker_label"`
	StartS       float64 `json:"start_s"`
	EndS         float64 `json:"end_s"`
	Text         string  `json:"text"`
}

type SyncSummary struct {
	UUID     string `json:"uuid"`
	Template string `json:"template"`
	Content  string `json:"content"`
}

type SyncSpeaker struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Embedding []byte `json:"embedding,omitempty"`
}

type SyncResponse struct {
	Synced  int    `json:"synced"`
	Cursor  string `json:"cursor"`
	Message string `json:"message,omitempty"`
}

// --- Handlers ---

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var payload SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	synced := 0
	for _, sp := range payload.Speakers {
		_, err := s.db.Exec(
			`INSERT INTO speakers (uuid, name, embedding) VALUES ($1, $2, $3)
			 ON CONFLICT (uuid) DO UPDATE SET name=EXCLUDED.name, embedding=EXCLUDED.embedding, updated_at=NOW()`,
			sp.UUID, sp.Name, sp.Embedding,
		)
		if err != nil {
			log.Printf("sync speaker %s: %v", sp.UUID, err)
		}
	}

	for _, m := range payload.Meetings {
		var meetingID int
		err := s.db.QueryRow(
			`INSERT INTO meetings (uuid, name, recorded_at, duration_s, template, num_speakers, client_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (uuid) DO UPDATE SET
			   name=EXCLUDED.name, duration_s=EXCLUDED.duration_s, template=EXCLUDED.template,
			   num_speakers=EXCLUDED.num_speakers, updated_at=NOW()
			 RETURNING id`,
			m.UUID, m.Name, m.RecordedAt, m.DurationS, m.Template, m.NumSpeakers, payload.ClientID,
		).Scan(&meetingID)
		if err != nil {
			log.Printf("sync meeting %s: %v", m.UUID, err)
			continue
		}

		for _, seg := range m.Segments {
			_, err := s.db.Exec(
				`INSERT INTO segments (uuid, meeting_id, speaker_label, start_s, end_s, text)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT (uuid) DO NOTHING`,
				seg.UUID, meetingID, seg.SpeakerLabel, seg.StartS, seg.EndS, seg.Text,
			)
			if err != nil {
				log.Printf("sync segment %s: %v", seg.UUID, err)
			}
		}

		for _, sum := range m.Summaries {
			_, err := s.db.Exec(
				`INSERT INTO summaries (uuid, meeting_id, template, content)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (uuid) DO UPDATE SET content=EXCLUDED.content, template=EXCLUDED.template`,
				sum.UUID, meetingID, sum.Template, sum.Content,
			)
			if err != nil {
				log.Printf("sync summary %s: %v", sum.UUID, err)
			}
		}
		synced++
	}

	cursor := time.Now().UTC().Format(time.RFC3339Nano)
	json.NewEncoder(w).Encode(SyncResponse{Synced: synced, Cursor: cursor})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	var since time.Time
	if cursor != "" {
		since, _ = time.Parse(time.RFC3339Nano, cursor)
	}

	rows, err := s.db.Query(
		`SELECT uuid, name, recorded_at, duration_s, template, COALESCE(audio_blob_key,''), num_speakers
		 FROM meetings WHERE updated_at > $1 ORDER BY updated_at LIMIT $2`,
		since, limit,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var meetings []SyncMeeting
	for rows.Next() {
		var m SyncMeeting
		var blobKey string
		if err := rows.Scan(&m.UUID, &m.Name, &m.RecordedAt, &m.DurationS, &m.Template, &blobKey, &m.NumSpeakers); err != nil {
			continue
		}

		// Fetch segments
		segRows, _ := s.db.Query(
			`SELECT s.uuid, s.speaker_label, s.start_s, s.end_s, s.text
			 FROM segments s JOIN meetings m ON m.id = s.meeting_id
			 WHERE m.uuid = $1 ORDER BY s.start_s`, m.UUID,
		)
		if segRows != nil {
			for segRows.Next() {
				var seg SyncSegment
				segRows.Scan(&seg.UUID, &seg.SpeakerLabel, &seg.StartS, &seg.EndS, &seg.Text)
				m.Segments = append(m.Segments, seg)
			}
			segRows.Close()
		}

		// Fetch summaries
		sumRows, _ := s.db.Query(
			`SELECT s.uuid, s.template, s.content
			 FROM summaries s JOIN meetings m ON m.id = s.meeting_id
			 WHERE m.uuid = $1 ORDER BY s.created_at DESC`, m.UUID,
		)
		if sumRows != nil {
			for sumRows.Next() {
				var sum SyncSummary
				sumRows.Scan(&sum.UUID, &sum.Template, &sum.Content)
				m.Summaries = append(m.Summaries, sum)
			}
			sumRows.Close()
		}

		meetings = append(meetings, m)
	}

	newCursor := time.Now().UTC().Format(time.RFC3339Nano)
	json.NewEncoder(w).Encode(map[string]any{
		"meetings": meetings,
		"cursor":   newCursor,
	})
}

func (s *Server) handleUploadAudio(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	key := fmt.Sprintf("audio/%s.wav", uuid)

	_, err := s.s3.PutObject(context.Background(), s.cfg.S3Bucket, key, r.Body, r.ContentLength, minio.PutObjectOptions{
		ContentType: "audio/wav",
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("s3 upload: %v", err), http.StatusInternalServerError)
		return
	}

	// Update meeting blob key
	s.db.Exec(`UPDATE meetings SET audio_blob_key = $1 WHERE uuid = $2`, key, uuid)

	json.NewEncoder(w).Encode(map[string]string{"blob_key": key})
}

func (s *Server) handleDownloadAudio(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	key := fmt.Sprintf("audio/%s.wav", uuid)

	obj, err := s.s3.GetObject(context.Background(), s.cfg.S3Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", "audio/wav")
	io.Copy(w, obj)
}

func (s *Server) handleListMeetings(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	rows, err := s.db.Query(
		`SELECT uuid, name, recorded_at, duration_s, template, num_speakers
		 FROM meetings ORDER BY recorded_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var meetings []map[string]any
	for rows.Next() {
		var uuid, name, template string
		var recordedAt time.Time
		var durationS float64
		var numSpeakers int
		rows.Scan(&uuid, &name, &recordedAt, &durationS, &template, &numSpeakers)
		meetings = append(meetings, map[string]any{
			"uuid": uuid, "name": name, "recorded_at": recordedAt,
			"duration_s": durationS, "template": template, "num_speakers": numSpeakers,
		})
	}
	json.NewEncoder(w).Encode(meetings)
}

func (s *Server) handleGetMeeting(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")

	var name, template string
	var recordedAt time.Time
	var durationS float64
	var numSpeakers int
	err := s.db.QueryRow(
		`SELECT name, recorded_at, duration_s, template, num_speakers
		 FROM meetings WHERE uuid = $1`, uuid,
	).Scan(&name, &recordedAt, &durationS, &template, &numSpeakers)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Segments
	segRows, _ := s.db.Query(
		`SELECT s.uuid, s.speaker_label, s.start_s, s.end_s, s.text
		 FROM segments s JOIN meetings m ON m.id = s.meeting_id
		 WHERE m.uuid = $1 ORDER BY s.start_s`, uuid,
	)
	var segments []map[string]any
	if segRows != nil {
		for segRows.Next() {
			var su, sl, t string
			var ss, se float64
			segRows.Scan(&su, &sl, &ss, &se, &t)
			segments = append(segments, map[string]any{
				"uuid": su, "speaker": sl, "start": ss, "end": se, "text": t,
			})
		}
		segRows.Close()
	}

	// Summaries
	sumRows, _ := s.db.Query(
		`SELECT s.uuid, s.template, s.content
		 FROM summaries s JOIN meetings m ON m.id = s.meeting_id
		 WHERE m.uuid = $1 ORDER BY s.created_at DESC`, uuid,
	)
	var summaries []map[string]any
	if sumRows != nil {
		for sumRows.Next() {
			var su, st, sc string
			sumRows.Scan(&su, &st, &sc)
			summaries = append(summaries, map[string]any{
				"uuid": su, "template": st, "content": sc,
			})
		}
		sumRows.Close()
	}

	json.NewEncoder(w).Encode(map[string]any{
		"uuid": uuid, "name": name, "recorded_at": recordedAt,
		"duration_s": durationS, "template": template, "num_speakers": numSpeakers,
		"segments": segments, "summaries": summaries,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q required", http.StatusBadRequest)
		return
	}

	rows, err := s.db.Query(
		`SELECT s.uuid, s.speaker_label, s.start_s, s.end_s, s.text, m.uuid, m.name, m.recorded_at
		 FROM segments s JOIN meetings m ON m.id = s.meeting_id
		 WHERE s.text ILIKE '%' || $1 || '%'
		 ORDER BY m.recorded_at DESC, s.start_s
		 LIMIT 50`, q,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var su, sl, t, mu, mn string
		var ss, se float64
		var ra time.Time
		rows.Scan(&su, &sl, &ss, &se, &t, &mu, &mn, &ra)
		results = append(results, map[string]any{
			"segment_uuid": su, "speaker": sl, "start": ss, "end": se, "text": t,
			"meeting_uuid": mu, "meeting_name": mn, "recorded_at": ra,
		})
	}
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleListSpeakers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT uuid, name FROM speakers ORDER BY name`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var speakers []map[string]string
	for rows.Next() {
		var uuid, name string
		rows.Scan(&uuid, &name)
		speakers = append(speakers, map[string]string{"uuid": uuid, "name": name})
	}
	json.NewEncoder(w).Encode(speakers)
}

func (s *Server) handleCreateSpeaker(w http.ResponseWriter, r *http.Request) {
	var sp SyncSpeaker
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if sp.UUID == "" || sp.Name == "" {
		http.Error(w, "uuid and name required", http.StatusBadRequest)
		return
	}

	_, err := s.db.Exec(
		`INSERT INTO speakers (uuid, name, embedding) VALUES ($1, $2, $3)
		 ON CONFLICT (uuid) DO UPDATE SET name=EXCLUDED.name, embedding=EXCLUDED.embedding`,
		sp.UUID, sp.Name, sp.Embedding,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"uuid": sp.UUID, "name": sp.Name})
}
