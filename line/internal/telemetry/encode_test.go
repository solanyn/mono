package telemetry

import (
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	original := Frame{
		PacketID:    12345,
		PosX:        100.5,
		PosY:        -50.3,
		PosZ:        200.7,
		VelX:        10.1,
		VelY:        -2.5,
		VelZ:        30.0,
		RotPitch:    0.1,
		RotYaw:      1.57,
		RotRoll:     -0.05,
		AngVelX:     0.01,
		AngVelY:     0.02,
		AngVelZ:     0.03,
		RideHeight:  0.12,
		RPM:         7500.0,
		FuelLevel:   45.2,
		FuelCap:     100.0,
		Speed:       55.5,
		Boost:       1.2,
		OilPressure: 3.5,
		WaterTemp:   85.0,
		OilTemp:     95.0,
		TireTempFL:  80.0,
		TireTempFR:  82.0,
		TireTempRL:  78.0,
		TireTempRR:  79.0,
		SuspFL:      0.05,
		SuspFR:      0.06,
		SuspRL:      0.04,
		SuspRR:      0.04,
		WheelFL:     50.0,
		WheelFR:     50.1,
		WheelRL:     49.9,
		WheelRR:     50.0,
		TireRadFL:   0.33,
		TireRadFR:   0.33,
		TireRadRL:   0.34,
		TireRadRR:   0.34,
		CurrentLap:  3,
		TotalLaps:   10,
		BestLap:     95000,
		LastLap:     96500,
		CurrentTime: 45000,
		Throttle:    255,
		Brake:       0,
		Steering:    -15,
		Gear:        4,
		CarID:       1234,
		Flags:       0x01,
		TimeOfDay:   720,
	}

	encoded := original.Encode()
	if len(encoded) != EncodedFrameSize {
		t.Fatalf("encoded size = %d, want %d", len(encoded), EncodedFrameSize)
	}

	decoded := DecodeFrame(encoded)

	if decoded.PacketID != original.PacketID {
		t.Errorf("PacketID = %d, want %d", decoded.PacketID, original.PacketID)
	}
	if decoded.PosX != original.PosX {
		t.Errorf("PosX = %f, want %f", decoded.PosX, original.PosX)
	}
	if decoded.Speed != original.Speed {
		t.Errorf("Speed = %f, want %f", decoded.Speed, original.Speed)
	}
	if decoded.RPM != original.RPM {
		t.Errorf("RPM = %f, want %f", decoded.RPM, original.RPM)
	}
	if decoded.CurrentLap != original.CurrentLap {
		t.Errorf("CurrentLap = %d, want %d", decoded.CurrentLap, original.CurrentLap)
	}
	if decoded.Throttle != original.Throttle {
		t.Errorf("Throttle = %d, want %d", decoded.Throttle, original.Throttle)
	}
	if decoded.Steering != original.Steering {
		t.Errorf("Steering = %d, want %d", decoded.Steering, original.Steering)
	}
	if decoded.Gear != original.Gear {
		t.Errorf("Gear = %d, want %d", decoded.Gear, original.Gear)
	}
	if decoded.CarID != original.CarID {
		t.Errorf("CarID = %d, want %d", decoded.CarID, original.CarID)
	}
	if decoded.Flags != original.Flags {
		t.Errorf("Flags = %d, want %d", decoded.Flags, original.Flags)
	}
	if decoded.TimeOfDay != original.TimeOfDay {
		t.Errorf("TimeOfDay = %d, want %d", decoded.TimeOfDay, original.TimeOfDay)
	}
}

func TestTimeOfDayLargeValue(t *testing.T) {
	f := Frame{TimeOfDay: 86400000}
	encoded := f.Encode()
	decoded := DecodeFrame(encoded)
	if decoded.TimeOfDay != 86400000 {
		t.Errorf("TimeOfDay = %d, want 86400000 (large value truncated)", decoded.TimeOfDay)
	}
}

func TestTimeOfDayMaxInt32(t *testing.T) {
	f := Frame{TimeOfDay: 2147483647}
	encoded := f.Encode()
	decoded := DecodeFrame(encoded)
	if decoded.TimeOfDay != 2147483647 {
		t.Errorf("TimeOfDay = %d, want 2147483647", decoded.TimeOfDay)
	}
}

func TestParseShortBuffer(t *testing.T) {
	_, err := Parse(make([]byte, 10))
	if err == nil {
		t.Error("expected error for short buffer")
	}

	_, err = Parse(make([]byte, PacketSize-1))
	if err == nil {
		t.Error("expected error for buffer one byte short")
	}

	_, err = Parse(make([]byte, PacketSize))
	if err != nil {
		t.Errorf("unexpected error for valid-size buffer: %v", err)
	}
}

func TestFrameFlags(t *testing.T) {
	f := Frame{Flags: 0x01}
	if !f.IsOnTrack() {
		t.Error("expected IsOnTrack=true")
	}
	if f.IsPaused() {
		t.Error("expected IsPaused=false")
	}

	f.Flags = 0x02
	if f.IsOnTrack() {
		t.Error("expected IsOnTrack=false")
	}
	if !f.IsPaused() {
		t.Error("expected IsPaused=true")
	}

	f.Flags = 0x03
	if !f.IsOnTrack() {
		t.Error("expected IsOnTrack=true")
	}
	if !f.IsPaused() {
		t.Error("expected IsPaused=true")
	}
}
