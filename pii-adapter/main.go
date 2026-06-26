package main

import (
	"log"
	"net/http"
)

// server wires HTTP handlers to a Presidio client.
type server struct {
	client *presidioClient
}

func main() {
	cfg := loadConfig()
	srv := &server{client: newPresidioClient(cfg)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /request", srv.handleRequest)
	mux.HandleFunc("POST /response", srv.handleResponse)
	mux.HandleFunc("GET /health", srv.handleHealth)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	log.Printf("pii-adapter listening on %s (presidio: %s)", cfg.ListenAddr, cfg.PresidioURL)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
