package e2e

import (
	"encoding/binary"
	"testing"

	"github.com/solanyn/mono/line/internal/storage"
	"github.com/solanyn/mono/line/internal/telemetry"
)

func TestFullDataPath(t *testing.T) {
	track := generateTrack(600)
	laps := 3
	carID := int32(1234)

	var allRows []storage.TelemetryRow
	var pktID int32
	var lapBoundaries []int

	for lap := int16(1); lap <= int16(laps); lap++ {
		lapStart := len(allRows)
		for i := 0; i < len(track); i++ {
			pktID++
			pkt := buildPacket(pktID, track[i], lap, int16(laps), carID, int32(i)*16)
			encrypted := encrypt(pkt)

			decrypted, err := telemetry.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("lap %d frame %d: decrypt failed: %v", lap, i, err)
			}

			frame := telemetry.Parse(decrypted)

			if frame.PacketID != pktID {
				t.Fatalf("packet ID mismatch: got %d want %d", frame.PacketID, pktID)
			}
			if frame.CarID != carID {
				t.Fatalf("car ID mismatch: got %d want %d", frame.CarID, carID)
			}
			if frame.CurrentLap != lap {
				t.Fatalf("lap mismatch: got %d want %d", frame.CurrentLap, lap)
			}
			if !frame.IsOnTrack() {
				t.Fatal("expected on track")
			}

			encoded := frame.Encode()
			if len(encoded) != telemetry.EncodedFrameSize {
				t.Fatalf("encoded size %d != %d", len(encoded), telemetry.EncodedFrameSize)
			}

			decoded := telemetry.DecodeFrame(encoded)
			if decoded.PacketID != frame.PacketID {
				t.Fatalf("decode roundtrip: PacketID %d != %d", decoded.PacketID, frame.PacketID)
			}
			if decoded.CarID != frame.CarID {
				t.Fatalf("decode roundtrip: CarID %d != %d", decoded.CarID, frame.CarID)
			}

			row := decoded.ToRow()
			allRows = append(allRows, row)
		}
		lapBoundaries = append(lapBoundaries, lapStart)
	}

	if len(allRows) != laps*len(track) {
		t.Fatalf("total rows %d, want %d", len(allRows), laps*len(track))
	}

	for lap := 0; lap < laps; lap++ {
		start := lapBoundaries[lap]
		end := start + len(track)
		lapRows := allRows[start:end]

		data, err := storage.WriteParquet(lapRows)
		if err != nil {
			t.Fatalf("lap %d: write parquet: %v", lap+1, err)
		}

		if len(data) == 0 {
			t.Fatalf("lap %d: empty parquet", lap+1)
		}

		readBack, err := storage.ReadParquet(data)
		if err != nil {
			t.Fatalf("lap %d: read parquet: %v", lap+1, err)
		}

		if len(readBack) != len(lapRows) {
			t.Fatalf("lap %d: read %d rows, wrote %d", lap+1, len(readBack), len(lapRows))
		}

		for i, row := range readBack {
			if row.PacketID != lapRows[i].PacketID {
				t.Errorf("lap %d row %d: PacketID %d != %d", lap+1, i, row.PacketID, lapRows[i].PacketID)
			}
			if row.CurrentLap != lapRows[i].CurrentLap {
				t.Errorf("lap %d row %d: CurrentLap %d != %d", lap+1, i, row.CurrentLap, lapRows[i].CurrentLap)
			}
			if row.Speed != lapRows[i].Speed {
				t.Errorf("lap %d row %d: Speed %f != %f", lap+1, i, row.Speed, lapRows[i].Speed)
			}
		}

		bytesPerRow := float64(len(data)) / float64(len(lapRows))
		t.Logf("lap %d: %d rows, %d bytes (%.1f bytes/row)", lap+1, len(lapRows), len(data), bytesPerRow)
	}
}

func TestLapDetection(t *testing.T) {
	track := generateTrack(100)
	laps := 5
	carID := int32(5678)

	type lapData struct {
		frames []telemetry.Frame
	}
	var currentLap int16
	var sessions []lapData
	var current lapData

	var pktID int32
	for lap := int16(1); lap <= int16(laps); lap++ {
		for i := 0; i < len(track); i++ {
			pktID++
			pkt := buildPacket(pktID, track[i], lap, int16(laps), carID, int32(i)*16)
			encrypted := encrypt(pkt)
			decrypted, _ := telemetry.Decrypt(encrypted)
			frame := telemetry.Parse(decrypted)

			if currentLap != 0 && frame.CurrentLap > currentLap {
				sessions = append(sessions, current)
				current = lapData{}
			}
			currentLap = frame.CurrentLap
			current.frames = append(current.frames, frame)
		}
	}
	sessions = append(sessions, current)

	if len(sessions) != laps {
		t.Fatalf("detected %d laps, want %d", len(sessions), laps)
	}

	for i, s := range sessions {
		if len(s.frames) != len(track) {
			t.Errorf("lap %d: %d frames, want %d", i+1, len(s.frames), len(track))
		}
	}
}

func TestSessionBoundary(t *testing.T) {
	track := generateTrack(50)

	var pktID int32
	var frames []telemetry.Frame

	for i := 0; i < len(track); i++ {
		pktID++
		pkt := buildPacket(pktID, track[i], 1, 3, 1111, int32(i)*16)
		encrypted := encrypt(pkt)
		decrypted, _ := telemetry.Decrypt(encrypted)
		frames = append(frames, telemetry.Parse(decrypted))
	}

	for i := 0; i < len(track); i++ {
		pktID++
		pkt := buildPacket(pktID, track[i], 1, 3, 2222, int32(i)*16)
		encrypted := encrypt(pkt)
		decrypted, _ := telemetry.Decrypt(encrypted)
		frames = append(frames, telemetry.Parse(decrypted))
	}

	var sessionBreaks int
	for i := 1; i < len(frames); i++ {
		if frames[i].CarID != frames[i-1].CarID {
			sessionBreaks++
		}
	}

	if sessionBreaks != 1 {
		t.Errorf("session breaks = %d, want 1", sessionBreaks)
	}
}

func TestDataIntegrity(t *testing.T) {
	track := generateTrack(3600)
	pkt := buildPacket(1, track[1800], 3, 10, 9999, 45000)
	encrypted := encrypt(pkt)
	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	frame := telemetry.Parse(decrypted)
	encoded := frame.Encode()
	decoded := telemetry.DecodeFrame(encoded)
	row := decoded.ToRow()

	if row.PosX != track[1800].x {
		t.Errorf("PosX: %f != %f", row.PosX, track[1800].x)
	}
	if row.PosZ != track[1800].z {
		t.Errorf("PosZ: %f != %f", row.PosZ, track[1800].z)
	}
	if row.Speed != track[1800].speed {
		t.Errorf("Speed: %f != %f", row.Speed, track[1800].speed)
	}
	if row.Throttle != int32(track[1800].throttle) {
		t.Errorf("Throttle: %d != %d", row.Throttle, track[1800].throttle)
	}
	if row.Brake != int32(track[1800].brake) {
		t.Errorf("Brake: %d != %d", row.Brake, track[1800].brake)
	}
	if row.Gear != int32(track[1800].gear) {
		t.Errorf("Gear: %d != %d", row.Gear, track[1800].gear)
	}
	if row.CurrentLap != 3 {
		t.Errorf("CurrentLap: %d != 3", row.CurrentLap)
	}
	if row.CarID != 9999 {
		t.Errorf("CarID: %d != 9999", row.CarID)
	}
}

func TestOffTrackFramesSkipped(t *testing.T) {
	track := generateTrack(10)
	pkt := buildPacket(1, track[0], 1, 3, 1234, 0)
	binary.LittleEndian.PutUint16(pkt[0x8E:0x90], 0x00)
	encrypted := encrypt(pkt)
	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	frame := telemetry.Parse(decrypted)
	if frame.IsOnTrack() {
		t.Error("expected off-track frame")
	}
}

func TestPausedFrames(t *testing.T) {
	track := generateTrack(10)
	pkt := buildPacket(1, track[0], 1, 3, 1234, 0)
	binary.LittleEndian.PutUint16(pkt[0x8E:0x90], 0x03)
	encrypted := encrypt(pkt)
	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	frame := telemetry.Parse(decrypted)
	if !frame.IsOnTrack() {
		t.Error("expected on-track")
	}
	if !frame.IsPaused() {
		t.Error("expected paused")
	}
}

func TestDuplicatePacketIDs(t *testing.T) {
	track := generateTrack(10)
	var lastPktID int32
	var dropped int

	for i := 0; i < 20; i++ {
		pktID := int32(i/2 + 1)
		pkt := buildPacket(pktID, track[i%10], 1, 3, 1234, int32(i)*16)
		encrypted := encrypt(pkt)
		decrypted, _ := telemetry.Decrypt(encrypted)
		frame := telemetry.Parse(decrypted)

		if frame.PacketID <= lastPktID {
			dropped++
			continue
		}
		lastPktID = frame.PacketID
	}

	if dropped != 10 {
		t.Errorf("dropped %d duplicates, want 10", dropped)
	}
}

func TestPacketGaps(t *testing.T) {
	track := generateTrack(10)
	pktIDs := []int32{1, 2, 3, 10, 11, 12, 50, 51}
	var gaps int64
	var lastPktID int32

	for i, id := range pktIDs {
		pkt := buildPacket(id, track[i%10], 1, 3, 1234, int32(i)*16)
		encrypted := encrypt(pkt)
		decrypted, _ := telemetry.Decrypt(encrypted)
		frame := telemetry.Parse(decrypted)

		if lastPktID > 0 && frame.PacketID-lastPktID > 1 {
			gaps += int64(frame.PacketID - lastPktID - 1)
		}
		lastPktID = frame.PacketID
	}

	if gaps != 43 {
		t.Errorf("gap frames = %d, want 43 (6+37)", gaps)
	}
}

func TestCorruptedEncryption(t *testing.T) {
	track := generateTrack(1)
	pkt := buildPacket(1, track[0], 1, 1, 1234, 0)
	encrypted := encrypt(pkt)

	encrypted[0x40] ^= 0xFF
	encrypted[0x41] ^= 0xFF

	_, err := telemetry.Decrypt(encrypted)
	if err == nil {
		t.Error("expected decrypt error for corrupted IV")
	}
}

func TestUndersizedPacket(t *testing.T) {
	_, err := telemetry.Decrypt([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Error("expected error for undersized packet")
	}
}

func TestMaxFramesPerLapCap(t *testing.T) {
	track := generateTrack(100)
	const maxFrames = 30000

	var rows []storage.TelemetryRow
	var pktID int32
	for i := 0; i < maxFrames+500; i++ {
		pktID++
		pkt := buildPacket(pktID, track[i%100], 1, 3, 1234, int32(i)*16)
		encrypted := encrypt(pkt)
		decrypted, _ := telemetry.Decrypt(encrypted)
		frame := telemetry.Parse(decrypted)
		rows = append(rows, frame.ToRow())

		if len(rows) >= maxFrames {
			data, err := storage.WriteParquet(rows)
			if err != nil {
				t.Fatalf("write at cap: %v", err)
			}
			readBack, err := storage.ReadParquet(data)
			if err != nil {
				t.Fatalf("read at cap: %v", err)
			}
			if len(readBack) != maxFrames {
				t.Fatalf("read %d rows at cap, want %d", len(readBack), maxFrames)
			}
			rows = rows[:0]
		}
	}

	if len(rows) != 500 {
		t.Errorf("remaining rows = %d, want 500", len(rows))
	}
}

func TestZeroSpeedFrames(t *testing.T) {
	pt := trackPoint{x: 100, y: 0, z: 200, speed: 0, throttle: 0, brake: 0, gear: 1, rpm: 800, fuelLevel: 50}
	pkt := buildPacket(1, pt, 1, 3, 1234, 0)
	encrypted := encrypt(pkt)
	decrypted, err := telemetry.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	frame := telemetry.Parse(decrypted)
	if frame.Speed != 0 {
		t.Errorf("speed = %f, want 0", frame.Speed)
	}
	row := frame.ToRow()
	if row.Speed != 0 {
		t.Errorf("row speed = %f, want 0", row.Speed)
	}
}
