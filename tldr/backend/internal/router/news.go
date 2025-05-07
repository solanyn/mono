package router

import (
	"fmt"
	"tldr/internal/config"
	"tldr/internal/handler"
	"tldr/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
)

func NewsRouter(cfg config.Config, logger *httplog.Logger) chi.Router {
	r := chi.NewRouter()

	client := storage.NewMinIOClient(cfg.MinIO)
	logger.Info(fmt.Sprintf("Connected to MinIO at %s", cfg.MinIO.Endpoint))
	newsHandler := handler.NewNewsHandler(client)

	r.Use(httplog.RequestLogger(logger))

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
