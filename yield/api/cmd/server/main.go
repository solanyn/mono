package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/solanyn/mono/yield/api/internal/config"
	"github.com/solanyn/mono/yield/api/internal/handler"
	"github.com/solanyn/mono/yield/api/internal/metrics"
	"github.com/solanyn/mono/yield/api/internal/scheduler"
)

func main() {
	cfg := config.Load()

	h := handler.New()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Get("/property/{id}", h.GetProperty)
		r.Post("/rent-check", h.RentCheck)
		r.Get("/suburb/{slug}", h.SuburbStats)
		r.Post("/analyze", h.Analyze)
		r.Get("/search", h.Search)
		r.Get("/portfolio", h.Portfolio)
	})
	r.Get("/metrics", metrics.Handler())

	sched := scheduler.New(cfg, nil, nil)
	sched.Start()
	defer sched.Stop()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("yield api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
