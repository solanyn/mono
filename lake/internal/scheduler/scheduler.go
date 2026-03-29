package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/lake/internal/ingest"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Scheduler struct {
	cron       *cron.Cron
	s3         *storage.Client
	producer   *kafka.Producer
	lastIngest time.Time
}

func New(s3 *storage.Client, producer *kafka.Producer) *Scheduler {
	loc, _ := time.LoadLocation("Australia/Sydney")
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		s3:       s3,
		producer: producer,
	}
}

func (s *Scheduler) Start() {
	s.cron.AddFunc("0 7 * * *", s.wrapIngest("rba", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestRBA(ctx, s.s3)
	}))

	s.cron.AddFunc("0 8 * * *", s.wrapIngest("abs", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestABS(ctx, s.s3)
	}))

	s.cron.AddFunc("*/5 * * * *", s.wrapIngest("aemo", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestAEMO(ctx, s.s3)
	}))

	s.cron.AddFunc("*/15 * * * *", s.wrapIngest("rss", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestRSS(ctx, s.s3)
	}))

	s.cron.AddFunc("*/30 * * * *", s.wrapIngest("reddit", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestReddit(ctx, s.s3)
	}))

	s.cron.AddFunc("0 10 * * *", s.wrapIngest("domain", func(ctx context.Context) (ingest.Result, error) {
		return ingest.IngestDomain(ctx, s.s3)
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

		if s.producer != nil && result.Key != "" {
			event := kafka.BronzeWritten{
				Source:    result.Source,
				Bucket:    "bronze",
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
