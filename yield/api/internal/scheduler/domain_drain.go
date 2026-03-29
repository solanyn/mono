package scheduler

import (
	"context"
	"log"
	"time"
)

func (s *Scheduler) drainDomainQueue() {
	if s.domain == nil {
		log.Println("domain_drain: domain client not configured, skipping")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	maxRequests := 450
	drained, err := s.domain.DrainQueue(ctx, maxRequests)
	if err != nil {
		log.Printf("domain_drain: %v", err)
	}
	log.Printf("domain_drain: processed %d queued requests", drained)
}
