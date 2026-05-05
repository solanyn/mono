package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/telemetry"
)

func main() {
	ps5IP := envOrDefault("PS5_IP", "")
	if ps5IP == "" {
		slog.Error("PS5_IP environment variable required")
		os.Exit(1)
	}

	brokers := strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ",")
	topic := envOrDefault("KAFKA_TOPIC", "line.telemetry.raw")
	heartbeatInterval, _ := strconv.Atoi(envOrDefault("HEARTBEAT_INTERVAL", "100"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	producer, err := kafka.NewProducer(brokers, topic)
	if err != nil {
		slog.Error("failed to create producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", telemetry.ReceivePort))
	if err != nil {
		slog.Error("failed to listen", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	ps5Addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ps5IP, telemetry.SendPort))
	if err != nil {
		slog.Error("failed to resolve PS5 address", "err", err)
		os.Exit(1)
	}

	sendHeartbeat := func() {
		if _, err := conn.WriteTo([]byte("A"), ps5Addr); err != nil {
			slog.Warn("heartbeat send failed", "err", err)
		}
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	sendHeartbeat()
	slog.Info("capture started", "ps5", ps5IP, "brokers", brokers, "topic", topic)

	buf := make([]byte, 4096)
	var pktCount int
	var lastPktID int32
	var produced atomic.Int64
	var errors atomic.Int64

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.Info("stats", "produced", produced.Load(), "errors", errors.Load())
			}
		}
	}()

	for ctx.Err() == nil {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Debug("read timeout, resending heartbeat")
			sendHeartbeat()
			continue
		}

		pktCount++
		if pktCount >= heartbeatInterval {
			sendHeartbeat()
			pktCount = 0
		}

		decrypted, err := telemetry.Decrypt(buf[:n])
		if err != nil {
			continue
		}

		frame := telemetry.Parse(decrypted)
		if frame.PacketID <= lastPktID {
			continue
		}
		lastPktID = frame.PacketID

		if !frame.IsOnTrack() {
			continue
		}

		encoded := frame.Encode()
		key := []byte(fmt.Sprintf("%d", frame.CarID))

		producer.ProduceAsync(ctx, key, encoded, func(err error) {
			if err != nil {
				errors.Add(1)
			} else {
				produced.Add(1)
			}
		})
	}

	slog.Info("shutting down, flushing producer...")
	if err := producer.Flush(context.Background()); err != nil {
		slog.Error("flush failed", "err", err)
	}
	slog.Info("capture stopped", "total_produced", produced.Load(), "total_errors", errors.Load())
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
