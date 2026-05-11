package main

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/solanyn/mono/line/internal/telemetry"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	track := generateTrack(100)
	pt := track[50]

	pkt := buildPacket(42, pt, 2, 5, 1234, 30000)
	encrypted := encrypt(pkt)

	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	frame, err := telemetry.Parse(decrypted)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if frame.PacketID != 42 {
		t.Errorf("PacketID = %d, want 42", frame.PacketID)
	}
	if frame.CurrentLap != 2 {
		t.Errorf("CurrentLap = %d, want 2", frame.CurrentLap)
	}
	if frame.TotalLaps != 5 {
		t.Errorf("TotalLaps = %d, want 5", frame.TotalLaps)
	}
	if frame.CarID != 1234 {
		t.Errorf("CarID = %d, want 1234", frame.CarID)
	}
	if !frame.IsOnTrack() {
		t.Error("expected IsOnTrack() = true")
	}
	if !approxEq(frame.PosX, pt.x) {
		t.Errorf("PosX = %f, want %f", frame.PosX, pt.x)
	}
	if !approxEq(frame.PosZ, pt.z) {
		t.Errorf("PosZ = %f, want %f", frame.PosZ, pt.z)
	}
	if !approxEq(frame.Speed, pt.speed) {
		t.Errorf("Speed = %f, want %f", frame.Speed, pt.speed)
	}
	if frame.Throttle != pt.throttle {
		t.Errorf("Throttle = %d, want %d", frame.Throttle, pt.throttle)
	}
	if frame.Brake != pt.brake {
		t.Errorf("Brake = %d, want %d", frame.Brake, pt.brake)
	}
	if frame.Gear != pt.gear {
		t.Errorf("Gear = %d, want %d", frame.Gear, pt.gear)
	}
	if !approxEq(frame.RPM, pt.rpm) {
		t.Errorf("RPM = %f, want %f", frame.RPM, pt.rpm)
	}
	if !approxEq(frame.FuelLevel, pt.fuelLevel) {
		t.Errorf("FuelLevel = %f, want %f", frame.FuelLevel, pt.fuelLevel)
	}
	if !approxEq(frame.TireTempFL, pt.tireTempFL) {
		t.Errorf("TireTempFL = %f, want %f", frame.TireTempFL, pt.tireTempFL)
	}
}

func TestMultipleLapsSequence(t *testing.T) {
	track := generateTrack(60)
	laps := 3
	var frames []telemetry.Frame

	var pktID int32
	for lap := int16(1); lap <= int16(laps); lap++ {
		for i := 0; i < len(track); i++ {
			pktID++
			pkt := buildPacket(pktID, track[i], lap, int16(laps), 5678, int32(i)*16)
			encrypted := encrypt(pkt)
			decrypted, err := telemetry.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("lap %d frame %d: decrypt failed: %v", lap, i, err)
			}
			frame, err := telemetry.Parse(decrypted)
			if err != nil {
				t.Fatalf("lap %d frame %d: parse failed: %v", lap, i, err)
			}
			frames = append(frames, frame)
		}
	}

	if len(frames) != laps*len(track) {
		t.Fatalf("got %d frames, want %d", len(frames), laps*len(track))
	}

	lapTransitions := 0
	for i := 1; i < len(frames); i++ {
		if frames[i].CurrentLap != frames[i-1].CurrentLap {
			lapTransitions++
		}
	}
	if lapTransitions != laps-1 {
		t.Errorf("lap transitions = %d, want %d", lapTransitions, laps-1)
	}

	for i := 1; i < len(frames); i++ {
		if frames[i].PacketID <= frames[i-1].PacketID {
			t.Errorf("frame %d: PacketID %d <= previous %d", i, frames[i].PacketID, frames[i-1].PacketID)
		}
	}
}

func TestEncodeDecodeIntegration(t *testing.T) {
	track := generateTrack(10)
	pt := track[0]
	pkt := buildPacket(1, pt, 1, 3, 9999, 500)
	encrypted := encrypt(pkt)

	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	frame, err := telemetry.Parse(decrypted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	encoded := frame.Encode()
	decoded := telemetry.DecodeFrame(encoded)

	if decoded.PacketID != frame.PacketID {
		t.Errorf("encode/decode PacketID mismatch: %d vs %d", decoded.PacketID, frame.PacketID)
	}
	if decoded.CarID != frame.CarID {
		t.Errorf("encode/decode CarID mismatch: %d vs %d", decoded.CarID, frame.CarID)
	}
	if !approxEq(decoded.Speed, frame.Speed) {
		t.Errorf("encode/decode Speed mismatch: %f vs %f", decoded.Speed, frame.Speed)
	}
	if decoded.CurrentLap != frame.CurrentLap {
		t.Errorf("encode/decode CurrentLap mismatch: %d vs %d", decoded.CurrentLap, frame.CurrentLap)
	}
}

func TestPacketSize(t *testing.T) {
	track := generateTrack(1)
	pkt := buildPacket(1, track[0], 1, 1, 1, 0)
	if len(pkt) != packetSize {
		t.Errorf("packet size = %d, want %d", len(pkt), packetSize)
	}
	encrypted := encrypt(pkt)
	if len(encrypted) != packetSize {
		t.Errorf("encrypted size = %d, want %d", len(encrypted), packetSize)
	}
}

func TestTrackGeneration(t *testing.T) {
	track := generateTrack(3600)
	if len(track) != 3600 {
		t.Fatalf("track length = %d, want 3600", len(track))
	}

	for i, pt := range track {
		if pt.speed < 0 || pt.speed > 400 {
			t.Errorf("point %d: speed %f out of range", i, pt.speed)
		}
		if pt.fuelLevel < 0 || pt.fuelLevel > 100 {
			t.Errorf("point %d: fuel %f out of range", i, pt.fuelLevel)
		}
		if pt.gear < 1 || pt.gear > 6 {
			t.Errorf("point %d: gear %d out of range", i, pt.gear)
		}
		dist := math.Sqrt(float64(pt.x*pt.x + pt.z*pt.z))
		if dist < 50 || dist > 500 {
			t.Errorf("point %d: distance from center %f out of range", i, dist)
		}
	}
}

func TestMagicAfterDecrypt(t *testing.T) {
	track := generateTrack(1)
	pkt := buildPacket(1, track[0], 1, 1, 1, 0)

	if binary.LittleEndian.Uint32(pkt[0:4]) != magicVal {
		t.Fatal("plaintext should have magic")
	}

	encrypted := encrypt(pkt)
	if binary.LittleEndian.Uint32(encrypted[0:4]) == magicVal {
		t.Error("encrypted should NOT have magic at offset 0")
	}

	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if binary.LittleEndian.Uint32(decrypted[0:4]) != magicVal {
		t.Error("decrypted should have magic restored")
	}
}

func approxEq(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.01
}
