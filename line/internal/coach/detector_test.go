package coach

import (
	"context"
	"testing"
	"time"

	"github.com/solanyn/mono/line/internal/telemetry"
)

func TestDetectorLapComplete(t *testing.T) {
	d := NewDetector(16)

	for i := 0; i < 100; i++ {
		d.Process(context.Background(), telemetry.Frame{
			CurrentLap: 1,
			Speed:      150,
			Throttle:   200,
			Flags:      0x01,
			LastLap:    62000,
			TireTempFL: 80, TireTempFR: 82, TireTempRL: 78, TireTempRR: 79,
		})
	}

	d.Process(context.Background(), telemetry.Frame{
		CurrentLap: 2,
		Speed:      140,
		Throttle:   255,
		Flags:      0x01,
		LastLap:    62000,
		TireTempFL: 80, TireTempFR: 82, TireTempRL: 78, TireTempRR: 79,
	})

	select {
	case e := <-d.Events():
		if e.Type != EventLapComplete {
			t.Fatalf("expected lap_complete, got %s", e.Type)
		}
		if e.Data["lap_number"].(int16) != 1 {
			t.Fatalf("expected lap 1, got %v", e.Data["lap_number"])
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestDetectorConsistencyAlert(t *testing.T) {
	d := NewDetector(16)

	laps := []struct {
		lapNum  int16
		lastLap int32
	}{
		{1, 62000},
		{2, 63500},
		{3, 68000},
	}

	for _, lap := range laps {
		for i := 0; i < 50; i++ {
			d.Process(context.Background(), telemetry.Frame{
				CurrentLap: lap.lapNum,
				Speed:      150,
				Throttle:   200,
				Flags:      0x01,
				LastLap:    lap.lastLap,
				TireTempFL: 80, TireTempFR: 82, TireTempRL: 78, TireTempRR: 79,
			})
		}
	}

	d.Process(context.Background(), telemetry.Frame{
		CurrentLap: 4,
		Speed:      140,
		Flags:      0x01,
		LastLap:    68000,
		TireTempFL: 80, TireTempFR: 82, TireTempRL: 78, TireTempRR: 79,
	})

	var gotLapComplete, gotConsistency bool
	timeout := time.After(time.Second)
	for {
		select {
		case e := <-d.Events():
			switch e.Type {
			case EventLapComplete:
				gotLapComplete = true
			case EventConsistency:
				gotConsistency = true
			}
			if gotLapComplete && gotConsistency {
				return
			}
		case <-timeout:
			if !gotLapComplete {
				t.Fatal("no lap_complete event")
			}
			if !gotConsistency {
				t.Fatal("no consistency_alert event")
			}
			return
		}
	}
}

func TestDetectorFuelLow(t *testing.T) {
	d := NewDetector(16)

	d.Process(context.Background(), telemetry.Frame{
		CurrentLap: 1,
		Speed:      150,
		Flags:      0x01,
		FuelLevel:  3,
		FuelCap:    50,
		TireTempFL: 80, TireTempFR: 82, TireTempRL: 78, TireTempRR: 79,
	})

	select {
	case e := <-d.Events():
		if e.Type != EventFuelLow {
			t.Fatalf("expected fuel_low, got %s", e.Type)
		}
		pct := e.Data["fuel_pct"].(float32)
		if pct > 0.1 {
			t.Fatalf("expected pct < 0.1, got %f", pct)
		}
	case <-time.After(time.Second):
		t.Fatal("no fuel event received")
	}
}

func TestFormatPrompt(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{
			Event{Type: EventLapComplete, Data: map[string]interface{}{
				"lap_number": int16(3),
				"time_ms":    int32(62450),
			}},
			"Lap 3 complete: 1:02.450.",
		},
		{
			Event{Type: EventFuelLow, Data: map[string]interface{}{
				"fuel_pct":   float32(0.08),
				"fuel_level": float32(4),
				"fuel_cap":   float32(50),
			}},
			"Fuel at 8%. Consider pitting soon.",
		},
	}

	for _, tt := range tests {
		got := FormatPrompt(tt.event)
		if got != tt.want {
			t.Errorf("FormatPrompt(%s) = %q, want %q", tt.event.Type, got, tt.want)
		}
	}
}
