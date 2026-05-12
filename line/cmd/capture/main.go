package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/solanyn/mono/line/internal/config"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/telemetry"
)

func main() {
	ps5IP := config.Env("PS5_IP", "")
	if ps5IP == "" {
		slog.Error("PS5_IP environment variable required")
		os.Exit(1)
	}

	brokers := config.EnvList("KAFKA_BROKERS", "localhost:9092", ",")
	topic := config.Env("KAFKA_TOPIC", "line.telemetry.raw")
	heartbeatInterval := config.EnvInt("HEARTBEAT_INTERVAL", 100)

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
	var dropped atomic.Int64
	var decryptErrors atomic.Int64
	var gapFrames atomic.Int64

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.Info("capture stats",
					"produced", produced.Load(),
					"dropped_dup", dropped.Load(),
					"decrypt_errors", decryptErrors.Load(),
					"gap_frames", gapFrames.Load(),
					"last_pkt_id", atomic.LoadInt32(&lastPktID),
				)
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
			errCount := decryptErrors.Add(1)
			if errCount%100 == 1 {
				slog.Warn("decrypt failed", "err", err, "total_errors", errCount, "pkt_size", n)
			}
			continue
		}

		frame, err := telemetry.Parse(decrypted)
		if err != nil {
			slog.Warn("parse failed", "err", err)
			continue
		}
		if frame.PacketID <= lastPktID {
			dropped.Add(1)
			continue
		}

		gap := frame.PacketID - lastPktID
		if lastPktID > 0 && gap > 1 {
			gapFrames.Add(int64(gap - 1))
			if gap > 10 {
				slog.Warn("packet gap detected", "expected", lastPktID+1, "got", frame.PacketID, "missed", gap-1)
			}
		}
		atomic.StoreInt32(&lastPktID, frame.PacketID)

		if !frame.IsOnTrack() {
			continue
		}

		encoded := frame.Encode()

		producer.ProduceAsync(ctx, topic, fmt.Sprintf("%d", frame.CarID), encoded)
		produced.Add(1)
	}

	slog.Info("shutting down, flushing producer...")
	if err := producer.Flush(context.Background()); err != nil {
		slog.Error("flush failed", "err", err)
	}
	slog.Info("capture stopped", "total_produced", produced.Load(), "total_dropped", dropped.Load(), "total_gaps", gapFrames.Load(), "total_decrypt_errors", decryptErrors.Load())
}

