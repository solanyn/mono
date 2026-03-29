package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/yield/api/internal/client"
	"github.com/solanyn/mono/yield/api/internal/config"
	"github.com/solanyn/mono/yield/api/internal/store"
)

type Scheduler struct {
	cron    *cron.Cron
	cfg     config.Config
	queries *store.Queries
	domain  *client.CachedDomainClient
}

func New(cfg config.Config, queries *store.Queries, domain *client.CachedDomainClient) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		cfg:     cfg,
		queries: queries,
		domain:  domain,
	}
}

func (s *Scheduler) Start() {
	s.cron.AddFunc("0 2 * * *", func() {
		log.Println("scheduler: ingesting NSW VG bulk sales")
	})

	s.cron.AddFunc("0 3 * * 0", func() {
		log.Println("scheduler: syncing school catchments")
	})

	s.cron.AddFunc("1 0 * * *", func() {
		log.Println("scheduler: draining Domain fetch queue (quota reset)")
	})

	s.cron.AddFunc("0 4 1 */3 *", func() {
		log.Println("scheduler: refreshing GNAF address data")
	})

	s.cron.Start()
	log.Println("scheduler: started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
