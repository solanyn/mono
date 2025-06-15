package router

import (
	"net/http"
	"github.com/solanyn/goyangi/tldr/backend/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func New(cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	// TODO: Re-enable httplog.RequestLogger when httplog v3 is properly configured
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(middleware.Heartbeat("/readyz"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedCORS,
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
	}))

	r.Mount("/api", NewsRouter(cfg))

	return r
}
