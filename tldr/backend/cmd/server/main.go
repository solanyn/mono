package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/solanyn/goyangi/tldr/backend/internal/config"
	"github.com/solanyn/goyangi/tldr/backend/internal/router"
)

func main() {
	cfg := config.Load()

	// TODO: Re-enable advanced logging with httplog v3
	r := router.New(cfg)

	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
