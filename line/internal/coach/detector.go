package coach

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/solanyn/mono/line/internal/telemetry"
)

type EventType string

const (
	EventLapComplete    EventType = "lap_complete"
	EventSectorComplete EventType = "sector_complete"
	EventConsistency    EventType = "consistency_alert"
	EventTyreDeg        EventType = "tyre_degradation"
	EventFuelLow        EventType = "fuel_low"
)

type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      map[string]interface{}
}

type LapStats struct {
	LapNumber  int16
	TimeMs     int32
	TopSpeed   float32
	AvgSpeed   float32
	Frames     int
	BrakeCount int
}

type Detector struct {
	mu          sync.Mutex
	events      chan Event
	currentLap  int16
	lapFrames   []telemetry.Frame
	lapHistory  []LapStats
	lastTireAvg float32
}

const maxLapHistory = 20

func NewDetector(bufSize int) *Detector {
	return &Detector{
		events: make(chan Event, bufSize),
	}
}

func (d *Detector) Events() <-chan Event {
	return d.events
}

func (d *Detector) Process(_ context.Context, frame telemetry.Frame) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !frame.IsOnTrack() || frame.IsPaused() {
		return
	}

	if frame.CurrentLap != d.currentLap && d.currentLap > 0 {
		d.onLapComplete(frame)
	}

	d.currentLap = frame.CurrentLap
	d.lapFrames = append(d.lapFrames, frame)

	d.checkTyreDegradation(frame)
	d.checkFuel(frame)
}

func (d *Detector) onLapComplete(newFrame telemetry.Frame) {
	if len(d.lapFrames) == 0 {
		return
	}

	stats := d.computeLapStats(d.currentLap, d.lapFrames)
	d.lapHistory = append(d.lapHistory, stats)
	if len(d.lapHistory) > maxLapHistory {
		d.lapHistory = d.lapHistory[len(d.lapHistory)-maxLapHistory:]
	}

	data := map[string]interface{}{
		"lap_number": stats.LapNumber,
		"time_ms":    stats.TimeMs,
		"top_speed":  stats.TopSpeed,
		"avg_speed":  stats.AvgSpeed,
		"frames":     stats.Frames,
	}

	if len(d.lapHistory) >= 2 {
		prev := d.lapHistory[len(d.lapHistory)-2]
		data["delta_ms"] = stats.TimeMs - prev.TimeMs
		data["prev_time_ms"] = prev.TimeMs
	}

	if len(d.lapHistory) >= 3 {
		d.checkConsistency(data)
	}

	d.emit(Event{
		Type:      EventLapComplete,
		Timestamp: time.Now(),
		Data:      data,
	})

	d.lapFrames = d.lapFrames[:0]
}

func (d *Detector) checkConsistency(data map[string]interface{}) {
	n := len(d.lapHistory)
	recent := d.lapHistory[n-3:]
	var times []int32
	for _, l := range recent {
		times = append(times, l.TimeMs)
	}

	mean := float64(times[0]+times[1]+times[2]) / 3.0
	var variance float64
	for _, t := range times {
		diff := float64(t) - mean
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / 3.0)
	cv := stddev / mean

	if cv > 0.02 {
		d.emit(Event{
			Type:      EventConsistency,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"cv":     cv,
				"stddev": stddev,
				"mean":   mean,
				"laps":   times,
			},
		})
	}
}

func (d *Detector) checkTyreDegradation(frame telemetry.Frame) {
	avg := (frame.TireTempFL + frame.TireTempFR + frame.TireTempRL + frame.TireTempRR) / 4.0
	if d.lastTireAvg > 0 && avg > d.lastTireAvg+15 {
		d.emit(Event{
			Type:      EventTyreDeg,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"current_avg": avg,
				"prev_avg":    d.lastTireAvg,
				"delta":       avg - d.lastTireAvg,
				"fl":          frame.TireTempFL,
				"fr":          frame.TireTempFR,
				"rl":          frame.TireTempRL,
				"rr":          frame.TireTempRR,
			},
		})
	}
	if len(d.lapFrames)%60 == 0 {
		d.lastTireAvg = avg
	}
}

func (d *Detector) checkFuel(frame telemetry.Frame) {
	if frame.FuelCap <= 0 {
		return
	}
	pct := frame.FuelLevel / frame.FuelCap
	if pct < 0.1 && pct > 0 {
		if len(d.lapFrames)%600 == 1 {
			d.emit(Event{
				Type:      EventFuelLow,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"fuel_pct":   pct,
					"fuel_level": frame.FuelLevel,
					"fuel_cap":   frame.FuelCap,
				},
			})
		}
	}
}

func (d *Detector) computeLapStats(lapNum int16, frames []telemetry.Frame) LapStats {
	var topSpeed, totalSpeed float32
	var brakeCount int
	var prevBrake uint8

	for _, f := range frames {
		if f.Speed > topSpeed {
			topSpeed = f.Speed
		}
		totalSpeed += f.Speed
		if f.Brake > 50 && prevBrake <= 50 {
			brakeCount++
		}
		prevBrake = f.Brake
	}

	timeMs := int32(0)
	if len(frames) > 0 {
		last := frames[len(frames)-1]
		if last.LastLap > 0 {
			timeMs = last.LastLap
		} else {
			timeMs = last.CurrentTime
		}
	}

	return LapStats{
		LapNumber:  lapNum,
		TimeMs:     timeMs,
		TopSpeed:   topSpeed,
		AvgSpeed:   totalSpeed / float32(len(frames)),
		Frames:     len(frames),
		BrakeCount: brakeCount,
	}
}

func (d *Detector) emit(e Event) {
	select {
	case d.events <- e:
	default:
	}
}

func FormatPrompt(e Event) string {
	switch e.Type {
	case EventLapComplete:
		timeMs := e.Data["time_ms"].(int32)
		lapNum := e.Data["lap_number"].(int16)
		mins := timeMs / 60000
		secs := (timeMs % 60000) / 1000
		millis := timeMs % 1000
		msg := fmt.Sprintf("Lap %d complete: %d:%02d.%03d.", lapNum, mins, secs, millis)
		if delta, ok := e.Data["delta_ms"]; ok {
			d := delta.(int32)
			if d < 0 {
				msg += fmt.Sprintf(" %.1fs faster than previous lap. Good improvement.", float64(-d)/1000.0)
			} else if d > 500 {
				msg += fmt.Sprintf(" %.1fs slower than previous lap. Focus on consistency.", float64(d)/1000.0)
			} else {
				msg += " Consistent pace."
			}
		}
		return msg

	case EventConsistency:
		cv := e.Data["cv"].(float64)
		return fmt.Sprintf("Consistency alert: your last 3 laps have %.1f%% variation. Focus on repeating the same braking points and turn-in.", cv*100)

	case EventTyreDeg:
		delta := e.Data["delta"].(float32)
		return fmt.Sprintf("Tyre temperatures rising quickly, up %.0f degrees. Consider smoother inputs to manage degradation.", delta)

	case EventFuelLow:
		pct := e.Data["fuel_pct"].(float32)
		return fmt.Sprintf("Fuel at %.0f%%. Consider pitting soon.", pct*100)

	default:
		return ""
	}
}
