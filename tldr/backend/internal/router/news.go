package router

import (
	"log"
	"github.com/solanyn/goyangi/tldr/backend/internal/config"
	"github.com/solanyn/goyangi/tldr/backend/internal/handler"
	"github.com/solanyn/goyangi/tldr/backend/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func NewsRouter(cfg config.Config) chi.Router {
	r := chi.NewRouter()

	client := storage.NewMinIOClient(cfg.MinIO)
	log.Printf("Connected to MinIO at %s", cfg.MinIO.Endpoint)
	newsHandler := handler.NewNewsHandler(client)

	// TODO: Re-enable httplog.RequestLogger when httplog v3 is properly configured

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedCORS,
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
	}))

	r.Get("/news", newsHandler.ListNews)
	r.Get("/news/{id}", newsHandler.GetNews)

	return r
}
