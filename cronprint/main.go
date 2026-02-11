package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

type Config struct {
	Host       string
	Port       string
	PrinterURI string
	Timezone   string
	Jobs       []JobConfig
}

type JobConfig struct {
	Name     string
	Schedule string
	FilePath string
	TestPage bool
}

func loadConfig() Config {
	cfg := Config{
		Host:       envOr("CRONPRINT_HOST", "0.0.0.0"),
		Port:       envOr("CRONPRINT_PORT", "8080"),
		PrinterURI: os.Getenv("CRONPRINT_PRINTER_URI"),
		Timezone:   envOr("CRONPRINT_TIMEZONE", "UTC"),
	}

	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "CRONPRINT_JOB_") || !strings.Contains(kv, "_SCHEDULE=") {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		key := parts[0]
		name := strings.TrimSuffix(strings.TrimPrefix(key, "CRONPRINT_JOB_"), "_SCHEDULE")
		name = strings.ToLower(name)
		prefix := fmt.Sprintf("CRONPRINT_JOB_%s", strings.ToUpper(name))

		job := JobConfig{
			Name:     name,
			Schedule: parts[1],
			FilePath: os.Getenv(prefix + "_FILE"),
			TestPage: os.Getenv(prefix+"_TESTPAGE") == "true",
		}
		cfg.Jobs = append(cfg.Jobs, job)
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()

	if cfg.PrinterURI == "" {
		log.Fatal("CRONPRINT_PRINTER_URI is required")
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("invalid timezone %q: %v", cfg.Timezone, err)
	}

	printer := NewIPPPrinter(cfg.PrinterURI)

	c := cron.New(cron.WithLocation(loc))

	for _, job := range cfg.Jobs {
		j := job
		_, err := c.AddFunc(j.Schedule, func() {
			log.Printf("executing job %q", j.Name)
			var printErr error
			if j.TestPage {
				printErr = printer.PrintTestPage(j.Name)
			} else {
				printErr = printer.PrintFile(j.FilePath, j.Name)
			}
			if printErr != nil {
				log.Printf("job %q failed: %v", j.Name, printErr)
			} else {
				log.Printf("job %q completed", j.Name)
			}
		})
		if err != nil {
			log.Fatalf("invalid schedule for job %q: %v", j.Name, err)
		}
		log.Printf("scheduled job %q: %s", j.Name, j.Schedule)
	}

	c.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		attrs, err := printer.GetPrinterAttributes()
		status := "healthy"
		if err != nil {
			status = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":     status,
			"service":    "cronprint",
			"printer":    cfg.PrinterURI,
			"printer_ok": err == nil,
			"attributes": attrs,
			"jobs":       len(c.Entries()),
		})
	})

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		entries := c.Entries()
		jobs := make([]map[string]any, len(entries))
		for i, e := range entries {
			jobs[i] = map[string]any{
				"id":       e.ID,
				"next_run": e.Next,
				"prev_run": e.Prev,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
	})

	mux.HandleFunc("/print-test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		err := printer.PrintTestPage("manual-test")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")
	c.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
