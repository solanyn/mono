package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/gen/linepb"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/telemetry"
)

type liveHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
	status  linepb.LiveStatus
}

func newLiveHub() *liveHub {
	return &liveHub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

func (h *liveHub) broadcast(frame *linepb.TelemetryFrame) {
	h.mu.Lock()
	data, _ := json.Marshal(frame)
	var failed []*websocket.Conn
	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			failed = append(failed, conn)
		}
	}
	for _, conn := range failed {
		conn.Close()
		delete(h.clients, conn)
	}
	h.mu.Unlock()
}

func (h *liveHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *liveHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *liveHub) setStatus(s linepb.LiveStatus) {
	h.mu.Lock()
	h.status = s
	h.mu.Unlock()
}

func (h *liveHub) getStatus() linepb.LiveStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func (h *liveHub) drain() {
	h.mu.Lock()
	defer h.mu.Unlock()
	msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
	for conn := range h.clients {
		conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
		conn.Close()
		delete(h.clients, conn)
	}
}

func (s *server) handleLiveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "err", err)
		return
	}
	defer conn.Close()

	status, _ := json.Marshal(s.live.getStatus())
	conn.WriteMessage(websocket.TextMessage, status)

	s.live.add(conn)
	defer s.live.remove(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *server) handleCoachWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("coach websocket upgrade", "err", err)
		return
	}
	defer conn.Close()

	s.coach.AddClient(conn)
	defer s.coach.RemoveClient(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *server) runLiveConsumer(ctx context.Context, consumer *kafka.Consumer, topic string) {
	slog.Info("live consumer started", "topic", topic)
	var currentCar int32
	var currentLap int32
	consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
		if len(record.Value) < telemetry.EncodedFrameSize {
			return nil
		}
		frame := telemetry.DecodeFrame(record.Value)
		if !frame.IsOnTrack() || frame.IsPaused() {
			return nil
		}

		if frame.CarID != currentCar || int32(frame.CurrentLap) != currentLap {
			currentCar = frame.CarID
			currentLap = int32(frame.CurrentLap)
			s.live.setStatus(linepb.LiveStatus{
				Active:     true,
				CarCode:    currentCar,
				CurrentLap: currentLap,
			})
		}

		tf := &linepb.TelemetryFrame{
			PacketID:    frame.PacketID,
			X:           frame.PosX,
			Y:           frame.PosY,
			Z:           frame.PosZ,
			Speed:       frame.Speed,
			Throttle:    float32(frame.Throttle) / 255.0,
			Brake:       float32(frame.Brake) / 255.0,
			Steering:    float32(frame.Steering) / 127.0,
			Rpm:         frame.RPM,
			Gear:        int32(frame.Gear),
			TireTempFL:  frame.TireTempFL,
			TireTempFR:  frame.TireTempFR,
			TireTempRL:  frame.TireTempRL,
			TireTempRR:  frame.TireTempRR,
			FuelLevel:   frame.FuelLevel,
			CurrentLap:  int32(frame.CurrentLap),
			CurrentTime: frame.CurrentTime,
			TimestampNs: record.Timestamp.UnixNano(),
		}
		s.live.broadcast(tf)
		s.coach.Detector().Process(ctx, frame)
		return nil
	})
}
