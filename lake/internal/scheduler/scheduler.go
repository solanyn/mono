package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/lake/internal/config"
	icebergw "github.com/solanyn/mono/lake/internal/iceberg"
	"github.com/solanyn/mono/lake/internal/ingest"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Scheduler struct {
	cron       *cron.Cron
	s3         *storage.Client
	iceberg    *icebergw.Writer
	cfg        config.Config
	producer   *kafka.Producer
	lastIngest time.Time
}

func New(cfg config.Config, s3 *storage.Client, iceberg *icebergw.Writer, producer *kafka.Producer) *Scheduler {
	loc, _ := time.LoadLocation("Australia/Sydney")
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		s3:       s3,
		iceberg:  iceberg,
		cfg:      cfg,
		producer: producer,
	}
}

func (s *Scheduler) Start() {
	s.cron.AddFunc(s.cfg.CronRBA, s.wrapIngest("rba", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestRBA(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronABS, s.wrapIngest("abs", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestABS(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronAEMO, s.wrapIngest("aemo", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestAEMO(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronRSS, s.wrapIngest("rss", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestRSS(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronReddit, s.wrapIngest("reddit", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestReddit(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronDomain, s.wrapIngest("domain", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestDomain(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.AddFunc(s.cfg.CronNSWVG, s.wrapIngest("nsw_vg", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestNSWVG(ctx, s.s3, s.cfg.BronzeBucket)
	}))

	s.cron.Start()
	log.Println("scheduler: started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) LastIngest() time.Time {
	return s.lastIngest
}

func (s *Scheduler) wrapIngest(name string, fn func(context.Context) (ingest.Result, error)) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("scheduler: running %s", name)
		result, err := fn(ctx)
		if err != nil {
			log.Printf("scheduler: %s failed: %v", name, err)
			return
		}
		s.lastIngest = time.Now()

		if s.iceberg != nil && result.Key != "" {
			bronzeData, err := s.s3.GetObject(ctx, s.cfg.BronzeBucket, result.Key)
			if err == nil {
				rows, err := storage.ReadBronze(bronzeData)
				if err == nil {
					maps := bronzeRowsToMaps(rows)
					if err := s.iceberg.AppendBronze(ctx, name, maps, result.Source, result.Key); err != nil {
						log.Printf("scheduler: iceberg append %s: %v", name, err)
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
				log.Printf("scheduler: kafka publish %s: %v", name, err)
			}
		}

		log.Printf("scheduler: %s completed", name)
	}
}

func bronzeRowsToMaps(rows []storage.BronzeRow) []map[string]interface{} {
	var maps []map[string]interface{}
	for _, row := range rows {
		m := map[string]interface{}{
			"_source":      row.Source,
			"_ingested_at": row.IngestedAt,
			"_raw_payload": row.RawPayload,
			"_batch_id":    row.BatchID,
		}
		maps = append(maps, m)
	}
	return maps
}
