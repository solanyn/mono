package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Close()
	CreateSession(ctx context.Context, s *Session) error
	EndSession(ctx context.Context, id string, lapCount int, bestLapMs *int32) error
	UpdateSessionTrack(ctx context.Context, id string, trackID, trackName *string) error
	InsertLap(ctx context.Context, l *Lap) error
	ListSessions(ctx context.Context, limit, offset int) ([]Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	GetProgression(ctx context.Context, trackID string, limit int) ([]ProgressionPoint, error)
	GetLapTimesForSession(ctx context.Context, sessionID string) ([]int32, error)
	ListLaps(ctx context.Context, sessionID string) ([]Lap, error)
	ListAnnotations(ctx context.Context, sessionID string, lapNumber int) ([]Annotation, error)
	CreateAnnotation(ctx context.Context, a *Annotation) error
	DeleteAnnotation(ctx context.Context, id int64) error
	SetReferenceLap(ctx context.Context, r *ReferenceLap) error
	GetReferenceLap(ctx context.Context, trackID string, carCode int32, label string) (*ReferenceLap, error)
	ListReferenceLaps(ctx context.Context, trackID string, carCode int32) ([]ReferenceLap, error)
	DeleteReferenceLap(ctx context.Context, id int64) error
	GetCarComparisons(ctx context.Context, trackID string) ([]CarComparison, error)
	GetTrackHistory(ctx context.Context, trackID string, limit int) ([]SessionHistory, error)
	GetJournal(ctx context.Context, sessionID string) (*Journal, error)
	SaveJournal(ctx context.Context, j *Journal) error
	GetDistinctTracks(ctx context.Context) ([]string, error)
	ListPushSubscriptions(ctx context.Context) ([]PushSubscription, error)
	SavePushSubscription(ctx context.Context, s *PushSubscription) error
	DeletePushSubscription(ctx context.Context, endpoint string) error
}

var _ Store = (*DB)(nil)

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

type Session struct {
	ID        string     `json:"id"`
	CarCode   int32      `json:"car_code"`
	TrackID   *string    `json:"track_id,omitempty"`
	TrackName *string    `json:"track_name,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	LapCount  int        `json:"lap_count"`
	BestLapMs *int32     `json:"best_lap_ms,omitempty"`
}

type Lap struct {
	ID        int64   `json:"id"`
	SessionID string  `json:"session_id"`
	LapNumber int     `json:"lap_number"`
	TimeMs    *int32  `json:"time_ms,omitempty"`
	Frames    int     `json:"frames"`
	TopSpeed  *float32 `json:"top_speed,omitempty"`
	S3Key     string  `json:"s3_key"`
}

func (d *DB) CreateSession(ctx context.Context, s *Session) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO sessions (id, car_code, track_id, track_name, started_at, lap_count)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		s.ID, s.CarCode, s.TrackID, s.TrackName, s.StartedAt, s.LapCount,
	)
	return err
}

func (d *DB) EndSession(ctx context.Context, id string, lapCount int, bestLapMs *int32) error {
	now := time.Now()
	_, err := d.pool.Exec(ctx,
		`UPDATE sessions SET ended_at = $1, lap_count = $2, best_lap_ms = $3 WHERE id = $4`,
		now, lapCount, bestLapMs, id,
	)
	return err
}

func (d *DB) UpdateSessionTrack(ctx context.Context, id string, trackID, trackName *string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE sessions SET track_id = $1, track_name = $2 WHERE id = $3`,
		trackID, trackName, id,
	)
	return err
}

func (d *DB) InsertLap(ctx context.Context, l *Lap) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO laps (session_id, lap_number, time_ms, frames, top_speed, s3_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (session_id, lap_number) DO UPDATE SET
		   time_ms = EXCLUDED.time_ms,
		   frames = EXCLUDED.frames,
		   top_speed = EXCLUDED.top_speed,
		   s3_key = EXCLUDED.s3_key`,
		l.SessionID, l.LapNumber, l.TimeMs, l.Frames, l.TopSpeed, l.S3Key,
	)
	return err
}

func (d *DB) ListSessions(ctx context.Context, limit, offset int) ([]Session, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.pool.Query(ctx,
		`SELECT id, car_code, track_id, track_name, started_at, ended_at, lap_count, best_lap_ms
		 FROM sessions ORDER BY started_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CarCode, &s.TrackID, &s.TrackName, &s.StartedAt, &s.EndedAt, &s.LapCount, &s.BestLapMs); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (d *DB) GetSession(ctx context.Context, id string) (*Session, error) {
	var s Session
	err := d.pool.QueryRow(ctx,
		`SELECT id, car_code, track_id, track_name, started_at, ended_at, lap_count, best_lap_ms
		 FROM sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.CarCode, &s.TrackID, &s.TrackName, &s.StartedAt, &s.EndedAt, &s.LapCount, &s.BestLapMs)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type ProgressionPoint struct {
	SessionID        string    `json:"session_id"`
	Date             time.Time `json:"date"`
	TrackID          *string   `json:"track_id,omitempty"`
	TrackName        *string   `json:"track_name,omitempty"`
	CarCode          int32     `json:"car_code"`
	BestLapMs        int32     `json:"best_lap_ms"`
	LapCount         int       `json:"lap_count"`
	ConsistencyScore *float64  `json:"consistency_score,omitempty"`
}

func (d *DB) GetProgression(ctx context.Context, trackID string, limit int) ([]ProgressionPoint, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT s.id, s.started_at, s.track_id, s.track_name, s.car_code, s.best_lap_ms, s.lap_count,
		CASE WHEN lap_stats.avg_time > 0 AND lap_stats.cnt > 1
			THEN GREATEST(0, 1.0 - (lap_stats.stddev_time / lap_stats.avg_time) * 10)
			ELSE NULL
		END as consistency_score
		FROM sessions s
		LEFT JOIN LATERAL (
			SELECT AVG(time_ms)::float8 as avg_time, STDDEV_POP(time_ms)::float8 as stddev_time, COUNT(*) as cnt
			FROM laps WHERE session_id = s.id AND time_ms IS NOT NULL AND time_ms > 0
		) lap_stats ON true
		WHERE s.best_lap_ms IS NOT NULL AND s.best_lap_ms > 0`
	args := []interface{}{}
	argIdx := 1

	if trackID != "" {
		query += fmt.Sprintf(" AND s.track_id = $%d", argIdx)
		args = append(args, trackID)
		argIdx++
	}

	query += " ORDER BY s.started_at ASC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []ProgressionPoint
	for rows.Next() {
		var p ProgressionPoint
		if err := rows.Scan(&p.SessionID, &p.Date, &p.TrackID, &p.TrackName, &p.CarCode, &p.BestLapMs, &p.LapCount, &p.ConsistencyScore); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (d *DB) GetLapTimesForSession(ctx context.Context, sessionID string) ([]int32, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT time_ms FROM laps WHERE session_id = $1 AND time_ms IS NOT NULL AND time_ms > 0 ORDER BY lap_number`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var times []int32
	for rows.Next() {
		var t int32
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		times = append(times, t)
	}
	return times, rows.Err()
}

func (d *DB) ListLaps(ctx context.Context, sessionID string) ([]Lap, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, session_id, lap_number, time_ms, frames, top_speed, s3_key
		 FROM laps WHERE session_id = $1 ORDER BY lap_number`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var laps []Lap
	for rows.Next() {
		var l Lap
		if err := rows.Scan(&l.ID, &l.SessionID, &l.LapNumber, &l.TimeMs, &l.Frames, &l.TopSpeed, &l.S3Key); err != nil {
			return nil, err
		}
		laps = append(laps, l)
	}
	return laps, rows.Err()
}

type Annotation struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	LapNumber int       `json:"lap_number"`
	FrameIdx  int       `json:"frame_idx"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *DB) ListAnnotations(ctx context.Context, sessionID string, lapNumber int) ([]Annotation, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, session_id, lap_number, frame_idx, text, created_at
		 FROM annotations WHERE session_id = $1 AND lap_number = $2 ORDER BY frame_idx`,
		sessionID, lapNumber,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []Annotation
	for rows.Next() {
		var a Annotation
		if err := rows.Scan(&a.ID, &a.SessionID, &a.LapNumber, &a.FrameIdx, &a.Text, &a.CreatedAt); err != nil {
			return nil, err
		}
		annotations = append(annotations, a)
	}
	return annotations, rows.Err()
}

func (d *DB) CreateAnnotation(ctx context.Context, a *Annotation) error {
	err := d.pool.QueryRow(ctx,
		`INSERT INTO annotations (session_id, lap_number, frame_idx, text)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		a.SessionID, a.LapNumber, a.FrameIdx, a.Text,
	).Scan(&a.ID, &a.CreatedAt)
	return err
}

func (d *DB) DeleteAnnotation(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM annotations WHERE id = $1`, id)
	return err
}

type ReferenceLap struct {
	ID        int64     `json:"id"`
	TrackID   string    `json:"track_id"`
	CarCode   int32     `json:"car_code"`
	SessionID string    `json:"session_id"`
	LapNumber int       `json:"lap_number"`
	TimeMs    int32     `json:"time_ms"`
	S3Key     string    `json:"s3_key"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *DB) SetReferenceLap(ctx context.Context, r *ReferenceLap) error {
	err := d.pool.QueryRow(ctx,
		`INSERT INTO reference_laps (track_id, car_code, session_id, lap_number, time_ms, s3_key, label)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (track_id, car_code, label) DO UPDATE SET
		   session_id = EXCLUDED.session_id,
		   lap_number = EXCLUDED.lap_number,
		   time_ms = EXCLUDED.time_ms,
		   s3_key = EXCLUDED.s3_key,
		   created_at = NOW()
		 RETURNING id, created_at`,
		r.TrackID, r.CarCode, r.SessionID, r.LapNumber, r.TimeMs, r.S3Key, r.Label,
	).Scan(&r.ID, &r.CreatedAt)
	return err
}

func (d *DB) GetReferenceLap(ctx context.Context, trackID string, carCode int32, label string) (*ReferenceLap, error) {
	var r ReferenceLap
	err := d.pool.QueryRow(ctx,
		`SELECT id, track_id, car_code, session_id, lap_number, time_ms, s3_key, label, created_at
		 FROM reference_laps WHERE track_id = $1 AND car_code = $2 AND label = $3`,
		trackID, carCode, label,
	).Scan(&r.ID, &r.TrackID, &r.CarCode, &r.SessionID, &r.LapNumber, &r.TimeMs, &r.S3Key, &r.Label, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *DB) ListReferenceLaps(ctx context.Context, trackID string, carCode int32) ([]ReferenceLap, error) {
	query := `SELECT id, track_id, car_code, session_id, lap_number, time_ms, s3_key, label, created_at FROM reference_laps WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if trackID != "" {
		query += fmt.Sprintf(" AND track_id = $%d", argIdx)
		args = append(args, trackID)
		argIdx++
	}
	if carCode > 0 {
		query += fmt.Sprintf(" AND car_code = $%d", argIdx)
		args = append(args, carCode)
		argIdx++
	}

	query += " ORDER BY track_id, car_code, label"

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ReferenceLap
	for rows.Next() {
		var r ReferenceLap
		if err := rows.Scan(&r.ID, &r.TrackID, &r.CarCode, &r.SessionID, &r.LapNumber, &r.TimeMs, &r.S3Key, &r.Label, &r.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func (d *DB) DeleteReferenceLap(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM reference_laps WHERE id = $1`, id)
	return err
}

type CarComparison struct {
	CarCode   int32  `json:"car_code"`
	TrackID   string `json:"track_id"`
	Sessions  int    `json:"sessions"`
	BestLapMs int32  `json:"best_lap_ms"`
	AvgLapMs  int32  `json:"avg_lap_ms"`
	TotalLaps int    `json:"total_laps"`
}

func (d *DB) GetCarComparisons(ctx context.Context, trackID string) ([]CarComparison, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT s.car_code, s.track_id,
		        COUNT(DISTINCT s.id) as sessions,
		        MIN(l.time_ms) as best_lap_ms,
		        AVG(l.time_ms)::INTEGER as avg_lap_ms,
		        COUNT(l.id) as total_laps
		 FROM sessions s
		 JOIN laps l ON l.session_id = s.id
		 WHERE s.track_id = $1 AND l.time_ms IS NOT NULL AND l.time_ms > 0
		 GROUP BY s.car_code, s.track_id
		 ORDER BY MIN(l.time_ms) ASC`,
		trackID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comps []CarComparison
	for rows.Next() {
		var c CarComparison
		if err := rows.Scan(&c.CarCode, &c.TrackID, &c.Sessions, &c.BestLapMs, &c.AvgLapMs, &c.TotalLaps); err != nil {
			return nil, err
		}
		comps = append(comps, c)
	}
	return comps, rows.Err()
}

type SessionHistory struct {
	SessionID string    `json:"session_id"`
	CarCode   int32     `json:"car_code"`
	BestLapMs *int32    `json:"best_lap_ms"`
	LapCount  int       `json:"lap_count"`
	StartedAt time.Time `json:"started_at"`
}

func (d *DB) GetTrackHistory(ctx context.Context, trackID string, limit int) ([]SessionHistory, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.pool.Query(ctx,
		`SELECT id, car_code, best_lap_ms, lap_count, started_at
		 FROM sessions
		 WHERE track_id = $1 AND ended_at IS NOT NULL
		 ORDER BY started_at DESC LIMIT $2`,
		trackID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []SessionHistory
	for rows.Next() {
		var h SessionHistory
		if err := rows.Scan(&h.SessionID, &h.CarCode, &h.BestLapMs, &h.LapCount, &h.StartedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

type Journal struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *DB) GetJournal(ctx context.Context, sessionID string) (*Journal, error) {
	var j Journal
	err := d.pool.QueryRow(ctx,
		`SELECT id, session_id, content, created_at FROM journals WHERE session_id = $1`,
		sessionID,
	).Scan(&j.ID, &j.SessionID, &j.Content, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (d *DB) SaveJournal(ctx context.Context, j *Journal) error {
	err := d.pool.QueryRow(ctx,
		`INSERT INTO journals (session_id, content)
		 VALUES ($1, $2)
		 ON CONFLICT (session_id) DO UPDATE SET content = EXCLUDED.content, created_at = NOW()
		 RETURNING id, created_at`,
		j.SessionID, j.Content,
	).Scan(&j.ID, &j.CreatedAt)
	return err
}

func (d *DB) GetDistinctTracks(ctx context.Context) ([]string, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT DISTINCT track_id FROM sessions WHERE track_id IS NOT NULL ORDER BY track_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

type PushSubscription struct {
	ID       int64  `json:"id"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (d *DB) ListPushSubscriptions(ctx context.Context) ([]PushSubscription, error) {
	rows, err := d.pool.Query(ctx, `SELECT id, endpoint, p256dh, auth FROM push_subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []PushSubscription
	for rows.Next() {
		var s PushSubscription
		if err := rows.Scan(&s.ID, &s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (d *DB) SavePushSubscription(ctx context.Context, s *PushSubscription) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO push_subscriptions (endpoint, p256dh, auth)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (endpoint) DO UPDATE SET p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth`,
		s.Endpoint, s.P256dh, s.Auth)
	return err
}

func (d *DB) DeletePushSubscription(ctx context.Context, endpoint string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}
