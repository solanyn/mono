package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
