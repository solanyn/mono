package scheduler

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/lake/internal/config"
	icebergw "github.com/solanyn/mono/lake/internal/iceberg"
	"github.com/solanyn/mono/lake/internal/ingest"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Scheduler struct {
	cron       *cron.Cron
	s3         *storage.Client
	iceberg    *icebergw.Writer
	cfg        config.Config
	producer   *kafka.Producer
	ctx        context.Context
	lastIngest atomic.Int64
}

type ingestFn func(context.Context) (ingest.Result, error)

type job struct {
	name string
	spec string
	fn   ingestFn
}

func New(ctx context.Context, cfg config.Config, s3 *storage.Client, iceberg *icebergw.Writer, producer *kafka.Producer) *Scheduler {
	loc, _ := time.LoadLocation("Australia/Sydney")
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		s3:       s3,
		iceberg:  iceberg,
		cfg:      cfg,
		producer: producer,
		ctx:      ctx,
	}
}

func (s *Scheduler) Start() {
	jobs := []job{
		{"rba", s.cfg.CronRBA, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestRBA(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"abs", s.cfg.CronABS, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestABS(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"aemo", s.cfg.CronAEMO, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestAEMO(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"reddit", s.cfg.CronReddit, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestReddit(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"domain", s.cfg.CronDomain, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestDomain(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"nsw_vg", s.cfg.CronNSWVG, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestNSWVG(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"asx", s.cfg.CronASX, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestASX(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"abs_ba", s.cfg.CronABSBA, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestABSBuildingApprovals(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"abs_migration", s.cfg.CronABSMig, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestABSMigration(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"rba_credit", s.cfg.CronRBACredit, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestRBACredit(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"weather", s.cfg.CronWeather, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestWeather(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"github_trending", s.cfg.CronGitHub, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestGitHubTrending(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"pypi_stats", s.cfg.CronPyPI, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestPyPIStats(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"npm_stats", s.cfg.CronNpm, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestNpmStats(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"hn_stories", s.cfg.CronHN, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestHN(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"nsw_fuel", s.cfg.CronNSWFuel, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestNSWFuel(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"nsw_property_licences", s.cfg.CronNSWProperty, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestNSWPropertyLicences(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"nsw_trades_licences", s.cfg.CronNSWTrades, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestNSWTradesLicences(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"vic_vg", s.cfg.CronVicVG, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestVicVG(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"sqm_research", s.cfg.CronSQM, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestSQM(ctx, s.s3, s.cfg.BronzeBucket) }},
		{"bank", s.cfg.CronBank, func(ctx context.Context) (ingest.Result, error) { return ingest.IngestBank(ctx, s.s3, s.cfg.BronzeBucket) }},
	}

	for _, j := range jobs {
		if j.spec == "" {
			slog.Info("scheduler: skipping job (no cron spec)", "job", j.name)
			continue
		}
		if _, err := s.cron.AddFunc(j.spec, s.wrapIngest(j.name, j.fn)); err != nil {
			slog.Error("scheduler: invalid cron spec", "job", j.name, "spec", j.spec, "err", err)
			os.Exit(1)
		}
	}

	s.cron.Start()
	slog.Info("scheduler: started")
}

func (s *Scheduler) Stop() {
	<-s.cron.Stop().Done()
}

func (s *Scheduler) LastIngest() time.Time {
	v := s.lastIngest.Load()
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

func (s *Scheduler) wrapIngest(name string, fn ingestFn) func() {
	return func() {
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()

		start := time.Now()
		slog.Info("scheduler: running", "job", name)
		result, err := fn(ctx)
		metrics.IngestDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
		if err != nil {
			metrics.IngestErrors.WithLabelValues(name).Inc()
			slog.Error("scheduler: job failed", "job", name, "err", err)
			return
		}

		now := time.Now()
		s.lastIngest.Store(now.UnixNano())
		metrics.IngestTotal.WithLabelValues(name).Inc()
		metrics.LastIngestTimestamp.WithLabelValues(name).Set(float64(now.Unix()))

		if s.iceberg != nil && result.Key != "" {
			bronzeData, err := s.s3.GetObject(ctx, s.cfg.BronzeBucket, result.Key)
			if err == nil {
				rows, err := storage.ReadBronze(bronzeData)
				if err == nil {
					maps := bronzeRowsToMaps(rows)
					if err := s.iceberg.AppendBronze(ctx, name, maps, result.Source, result.Key); err != nil {
						slog.Error("scheduler: iceberg append", "job", name, "err", err)
					}
				}
			}
		}

		if s.producer != nil && result.Key != "" {
			event := kafka.BronzeWritten{
				Source:    result.Source,
				Bucket:    s.cfg.BronzeBucket,
				Key:       result.Key,
				Timestamp: time.Now().UTC(),
				RowCount:  result.RowCount,
			}
			if err := s.producer.PublishBronzeWritten(ctx, event); err != nil {
				slog.Error("scheduler: kafka publish", "job", name, "err", err)
			}
		}

		slog.Info("scheduler: completed", "job", name, "rows", result.RowCount, "dur_sec", time.Since(start).Seconds())
	}
}

func bronzeRowsToMaps(rows []storage.BronzeRow) []map[string]interface{} {
	maps := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		maps = append(maps, map[string]interface{}{
			"_source":      row.Source,
			"_ingested_at": row.IngestedAt,
			"_raw_payload": row.RawPayload,
			"_batch_id":    row.BatchID,
		})
	}
	return maps
}
