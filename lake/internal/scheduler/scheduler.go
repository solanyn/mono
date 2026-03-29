package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/lake/internal/ingest"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Scheduler struct {
	cron       *cron.Cron
	s3         *storage.Client
	lastIngest time.Time
}

func New(s3 *storage.Client) *Scheduler {
	loc, _ := time.LoadLocation("Australia/Sydney")
	return &Scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		s3:   s3,
	}
}

func (s *Scheduler) Start() {
	s.cron.AddFunc("0 7 * * *", s.wrap("rba", func(ctx context.Context) error {
		return ingest.IngestRBA(ctx, s.s3)
	}))

	s.cron.AddFunc("0 8 * * *", s.wrap("abs", func(ctx context.Context) error {
		return ingest.IngestABS(ctx, s.s3)
	}))

	s.cron.AddFunc("*/5 * * * *", s.wrap("aemo", func(ctx context.Context) error {
		return ingest.IngestAEMO(ctx, s.s3)
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

func (s *Scheduler) wrap(name string, fn func(context.Context) error) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("scheduler: running %s", name)
		if err := fn(ctx); err != nil {
			log.Printf("scheduler: %s failed: %v", name, err)
			return
		}
		s.lastIngest = time.Now()
		log.Printf("scheduler: %s completed", name)
	}
}
