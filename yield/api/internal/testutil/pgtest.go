package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/solanyn/mono/yield/api/migrations"
)

type TestDB struct {
	Pool    *pgxpool.Pool
	ConnStr string
	dataDir string
	port    int
	pgBin   string
	t       testing.TB
}

func NewTestDB(t testing.TB) *TestDB {
	t.Helper()

	pgBin := findPgBin(t)

	dataDir, err := os.MkdirTemp("", "yield-pgtest-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}

	port := freePort(t)

	initdb := exec.Command(filepath.Join(pgBin, "initdb"),
		"-D", dataDir,
		"--no-locale",
		"-E", "UTF8",
		"-A", "trust",
	)
	initdb.Env = append(os.Environ(), "TZ=UTC", "LC_ALL=C")
	if out, err := initdb.CombinedOutput(); err != nil {
		os.RemoveAll(dataDir)
		t.Fatalf("initdb: %v\n%s", err, out)
	}

	logFile := filepath.Join(dataDir, "pg.log")
	pgCtl := exec.Command(filepath.Join(pgBin, "pg_ctl"),
		"start",
		"-D", dataDir,
		"-w",
		"-l", logFile,
		"-o", fmt.Sprintf("-p %d -k %s -h 127.0.0.1", port, dataDir),
	)
	pgCtl.Env = append(os.Environ(), "TZ=UTC", "LC_ALL=C")
	pgCtl.Stdout = nil
	pgCtl.Stderr = nil
	if err := pgCtl.Run(); err != nil {
		logBytes, _ := os.ReadFile(logFile)
		os.RemoveAll(dataDir)
		t.Fatalf("pg_ctl start: %v\n%s", err, logBytes)
	}

	connStr := fmt.Sprintf("postgres://127.0.0.1:%d/postgres?sslmode=disable", port)

	db := &TestDB{
		dataDir: dataDir,
		port:    port,
		pgBin:   pgBin,
		ConnStr: connStr,
		t:       t,
	}

	if err := db.waitReady(); err != nil {
		db.Cleanup()
		t.Fatalf("pg not ready: %v", err)
	}

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		db.Cleanup()
		t.Fatalf("sql.Open: %v", err)
	}

	if _, err := sqlDB.Exec("CREATE DATABASE yield_test"); err != nil {
		sqlDB.Close()
		db.Cleanup()
		t.Fatalf("create db: %v", err)
	}
	sqlDB.Close()

	testConnStr := fmt.Sprintf("postgres://127.0.0.1:%d/yield_test?sslmode=disable", port)
	db.ConnStr = testConnStr

	testSQL, err := sql.Open("pgx", testConnStr)
	if err != nil {
		db.Cleanup()
		t.Fatalf("sql.Open test db: %v", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.Up(testSQL, "."); err != nil {
		t.Logf("goose migrations (some may fail without PostGIS): %v", err)
		testSQL.Close()
		testSQL, _ = sql.Open("pgx", testConnStr)
		runCoreMigrations(t, testSQL)
	}
	testSQL.Close()

	pool, err := pgxpool.New(context.Background(), testConnStr)
	if err != nil {
		db.Cleanup()
		t.Fatalf("pgxpool: %v", err)
	}
	db.Pool = pool

	return db
}

func runCoreMigrations(t testing.TB, db *sql.DB) {
	t.Helper()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sales (
			id              BIGSERIAL PRIMARY KEY,
			district        TEXT NOT NULL,
			property_id     TEXT,
			unit_number     TEXT,
			house_number    TEXT,
			street          TEXT,
			suburb          TEXT NOT NULL,
			postcode        TEXT,
			area            NUMERIC,
			area_type       TEXT,
			contract_date   DATE,
			settlement_date DATE,
			price           BIGINT,
			zone            TEXT,
			nature          TEXT,
			purpose         TEXT,
			strata_lot      TEXT,
			dealing_number  TEXT,
			source          TEXT NOT NULL DEFAULT 'nsw_vg',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(dealing_number, property_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_suburb ON sales(suburb)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_contract_date ON sales(contract_date)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_postcode ON sales(postcode)`,
		`CREATE TABLE IF NOT EXISTS listing_snapshots (
			id              BIGSERIAL PRIMARY KEY,
			listing_id      BIGINT NOT NULL,
			snapshot_at     TIMESTAMPTZ NOT NULL,
			blob_key        TEXT NOT NULL,
			listing_type    TEXT NOT NULL,
			status          TEXT,
			suburb          TEXT NOT NULL,
			postcode        TEXT,
			price_display   TEXT,
			price_numeric   NUMERIC,
			bedrooms        SMALLINT,
			bathrooms       SMALLINT,
			carspaces       SMALLINT,
			property_type   TEXT,
			land_area       NUMERIC,
			description     TEXT,
			headline        TEXT,
			photos_count    SMALLINT,
			agent_name      TEXT,
			agent_id        INT,
			date_listed     DATE,
			days_listed     INT,
			lat             DOUBLE PRECISION,
			lon             DOUBLE PRECISION,
			UNIQUE(listing_id, snapshot_at)
		)`,
		`CREATE TABLE IF NOT EXISTS property_snapshots (
			id              BIGSERIAL PRIMARY KEY,
			property_id     TEXT NOT NULL,
			snapshot_at     TIMESTAMPTZ NOT NULL,
			blob_key        TEXT NOT NULL,
			suburb          TEXT NOT NULL,
			sale_count      SMALLINT,
			last_sale_price NUMERIC,
			last_sale_date  DATE,
			UNIQUE(property_id, snapshot_at)
		)`,
		`CREATE TABLE IF NOT EXISTS suburb_stats (
			id                BIGSERIAL PRIMARY KEY,
			suburb            TEXT NOT NULL,
			state             TEXT NOT NULL,
			median_price      BIGINT,
			mean_price        BIGINT,
			sale_count        INT,
			median_yield_pct  DOUBLE PRECISION,
			auction_clearance DOUBLE PRECISION,
			days_on_market    INT,
			school_score      DOUBLE PRECISION,
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(suburb, state)
		)`,
		`CREATE TABLE IF NOT EXISTS domain_api_cache (
			id          BIGSERIAL PRIMARY KEY,
			endpoint    TEXT NOT NULL,
			params_hash TEXT NOT NULL,
			response    BYTEA,
			fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(endpoint, params_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS portfolio (
			id              BIGSERIAL PRIMARY KEY,
			address         TEXT NOT NULL,
			suburb          TEXT NOT NULL,
			postcode        TEXT,
			property_type   TEXT,
			bedrooms        SMALLINT,
			bathrooms       SMALLINT,
			purchase_price  BIGINT,
			purchase_date   DATE,
			current_rent_pw INT,
			notes           TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("migration stmt: %v", err)
		}
	}
}

func (db *TestDB) Cleanup() {
	if db.Pool != nil {
		db.Pool.Close()
	}

	pgCtl := exec.Command(filepath.Join(db.pgBin, "pg_ctl"),
		"stop",
		"-D", db.dataDir,
		"-m", "immediate",
		"-w",
	)
	pgCtl.Env = append(os.Environ(), "LC_ALL=C")
	pgCtl.Run()

	os.RemoveAll(db.dataDir)
}

func (db *TestDB) waitReady() error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", db.port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			sqlDB, err := sql.Open("pgx", db.ConnStr)
			if err == nil {
				if err := sqlDB.Ping(); err == nil {
					sqlDB.Close()
					return nil
				}
				sqlDB.Close()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("postgres not ready after 10s on port %d", db.port)
}

func findPgBin(t testing.TB) string {
	t.Helper()

	if v := os.Getenv("PGBIN"); v != "" {
		return v
	}

	searchPaths := []string{
		"/opt/homebrew/opt/postgresql@17/bin",
		"/usr/local/opt/postgresql@17/bin",
		"/usr/lib/postgresql/17/bin",
	}

	nixProfile := os.Getenv("HOME") + "/.nix-profile/bin"
	if _, err := os.Stat(filepath.Join(nixProfile, "pg_ctl")); err == nil {
		return nixProfile
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(filepath.Join(p, "pg_ctl")); err == nil {
			return p
		}
	}

	if path, err := exec.LookPath("pg_ctl"); err == nil {
		return filepath.Dir(path)
	}

	t.Skip("pg_ctl not found — set PGBIN or install postgresql")
	return ""
}

func freePort(t testing.TB) int {
	t.Helper()
	for i := 0; i < 10; i++ {
		port := 15432 + rand.Intn(10000)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	t.Fatal("could not find free port")
	return 0
}
