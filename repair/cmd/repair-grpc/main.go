package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ext_proc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/solanyn/mono/repair"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type ExtProcServer struct {
	ext_proc.UnimplementedExternalProcessorServer
	engine *repair.Engine
}

type WebhookResponse struct {
	Action WebhookAction `json:"action"`
}

type WebhookAction struct {
	Body   map[string]any `json:"body"`
	Reason string         `json:"reason"`
}

func (s *ExtProcServer) Process(stream ext_proc.ExternalProcessor_ProcessServer) error {
	var (
		requestBuf  []byte
		responseBuf []byte
	)

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch v := req.Request.(type) {
		case *ext_proc.ProcessingRequest_RequestHeaders:
			slog.Info("ext_proc request headers", "headers", len(v.RequestHeaders.Headers.Headers))
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_RequestHeaders{
					RequestHeaders: &ext_proc.HeadersResponse{
						Response: &ext_proc.CommonResponse{},
					},
				},
			})

		case *ext_proc.ProcessingRequest_RequestBody:
			requestBuf = append(requestBuf, v.RequestBody.Body...)

			if !v.RequestBody.EndOfStream {
				_ = stream.Send(&ext_proc.ProcessingResponse{
					Response: &ext_proc.ProcessingResponse_RequestBody{
						RequestBody: &ext_proc.BodyResponse{
							Response: &ext_proc.CommonResponse{},
						},
					},
				})
				continue
			}

			slog.Info("request body complete", "bytes", len(requestBuf))
			repaired := repair.Repair(requestBuf, s.engine)
			slog.Info("repair complete", "originalBytes", len(requestBuf), "repairedBytes", len(repaired))
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_RequestBody{
					RequestBody: &ext_proc.BodyResponse{
						Response: &ext_proc.CommonResponse{
							BodyMutation: &ext_proc.BodyMutation{
								Mutation: &ext_proc.BodyMutation_StreamedResponse{
									StreamedResponse: &ext_proc.StreamedBodyResponse{
										Body:        repaired,
										EndOfStream: true,
									},
								},
							},
						},
					},
				},
			})
			requestBuf = requestBuf[:0]

		case *ext_proc.ProcessingRequest_RequestTrailers:
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_RequestTrailers{
					RequestTrailers: &ext_proc.TrailersResponse{
						HeaderMutation: &ext_proc.HeaderMutation{},
					},
				},
			})

		case *ext_proc.ProcessingRequest_ResponseHeaders:
			slog.Info("ext_proc response headers", "headers", len(v.ResponseHeaders.Headers.Headers))
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &ext_proc.HeadersResponse{
						Response: &ext_proc.CommonResponse{},
					},
				},
			})

		case *ext_proc.ProcessingRequest_ResponseBody:
			responseBuf = append(responseBuf, v.ResponseBody.Body...)

			if !v.ResponseBody.EndOfStream {
				_ = stream.Send(&ext_proc.ProcessingResponse{
					Response: &ext_proc.ProcessingResponse_ResponseBody{
						ResponseBody: &ext_proc.BodyResponse{
							Response: &ext_proc.CommonResponse{},
						},
					},
				})
				continue
			}

			slog.Info("response body complete", "bytes", len(responseBuf))
			repair.CacheToolCalls(responseBuf, s.engine)
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_ResponseBody{
					ResponseBody: &ext_proc.BodyResponse{
						Response: &ext_proc.CommonResponse{
							BodyMutation: &ext_proc.BodyMutation{
								Mutation: &ext_proc.BodyMutation_StreamedResponse{
									StreamedResponse: &ext_proc.StreamedBodyResponse{
										Body:        responseBuf,
										EndOfStream: true,
									},
								},
							},
						},
					},
				},
			})
			responseBuf = responseBuf[:0]

		case *ext_proc.ProcessingRequest_ResponseTrailers:
			_ = stream.Send(&ext_proc.ProcessingResponse{
				Response: &ext_proc.ProcessingResponse_ResponseTrailers{
					ResponseTrailers: &ext_proc.TrailersResponse{
						HeaderMutation: &ext_proc.HeaderMutation{},
					},
				},
			})

		default:
			slog.Warn("unknown ext_proc request type", "type", v)
			_ = stream.Send(&ext_proc.ProcessingResponse{})
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *ExtProcServer) repairWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("webhook read error", "err", err)
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	slog.Info("webhook request", "bytes", len(body))

	var guardrailReq struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(body, &guardrailReq); err != nil {
		slog.Error("webhook unmarshal guardrail body", "err", err)
		http.Error(w, "unmarshal error", http.StatusInternalServerError)
		return
	}

	repaired := repair.Repair(guardrailReq.Body, s.engine)
	slog.Info("webhook repair complete", "originalBytes", len(guardrailReq.Body), "repairedBytes", len(repaired))

	var bodyJSON map[string]any
	if err := json.Unmarshal(repaired, &bodyJSON); err != nil {
		slog.Error("webhook unmarshal repaired body", "err", err)
		http.Error(w, "unmarshal error", http.StatusInternalServerError)
		return
	}

	resp := WebhookResponse{
		Action: WebhookAction{
			Body:   bodyJSON,
			Reason: "repaired",
		},
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		slog.Error("webhook marshal error", "err", err)
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respJSON)
}

func main() {
	level := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", "repair-grpc"))

	mode := envOr("MODE", "both")
	slog.Info("starting", "mode", mode)

	grpcAddr := envOr("LISTEN_ADDR", "0.0.0.0:4444")
	metricsAddr := envOr("METRICS_ADDR", "0.0.0.0:9090")
	webhookAddr := envOr("WEBHOOK_ADDR", "0.0.0.0:8080")

	engine := repair.NewEngine()
	extProcSrv := &ExtProcServer{engine: engine}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	metricsSrv := &http.Server{
		Addr:              metricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("metrics listening", "addr", metricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "err", err)
			os.Exit(1)
		}
	}()

	var grpcServer *grpc.Server
	if mode == "extproc" || mode == "both" {
		grpcServer = grpc.NewServer()
		healthServer := health.NewServer()
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

		ext_proc.RegisterExternalProcessorServer(grpcServer, extProcSrv)
		grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			slog.Error("failed to listen", "addr", grpcAddr, "err", err)
			os.Exit(1)
		}

		go func() {
			slog.Info("gRPC listening", "addr", grpcAddr)
			if err := grpcServer.Serve(lis); err != nil {
				slog.Error("gRPC server error", "err", err)
				os.Exit(1)
			}
		}()
	}

	var webhookSrv *http.Server
	if mode == "webhook" || mode == "both" {
		webhookMux := http.NewServeMux()
		webhookMux.HandleFunc("/", extProcSrv.repairWebhook)
		webhookMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		})

		webhookSrv = &http.Server{
			Addr:              webhookAddr,
			Handler:           webhookMux,
			ReadHeaderTimeout: 30 * time.Second,
		}

		go func() {
			slog.Info("webhook listening", "addr", webhookAddr)
			if err := webhookSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("webhook server error", "err", err)
				os.Exit(1)
			}
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("shutting down")
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	if webhookSrv != nil {
		_ = webhookSrv.Close()
	}
	_ = metricsSrv.Close()
}
