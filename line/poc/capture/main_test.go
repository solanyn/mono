package main

import (
	"encoding/binary"
	"math"
	"testing"

	"golang.org/x/crypto/salsa20"
)

func encrypt(data []byte) []byte {
	oiv := binary.LittleEndian.Uint32(data[0x40:0x44])
	iv2 := oiv ^ 0xDEADBEAF

	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[0:4], iv2)
	binary.LittleEndian.PutUint32(nonce[4:8], oiv)

	var k [32]byte
	copy(k[:], key)

	out := make([]byte, len(data))
	salsa20.XORKeyStream(out, data, nonce[:], &k)
	binary.LittleEndian.PutUint32(out[0x40:0x44], oiv)
	return out
}

func TestDecryptAndParse(t *testing.T) {
	plain := make([]byte, 296)

	binary.LittleEndian.PutUint32(plain[0:4], magic)

	binary.LittleEndian.PutUint32(plain[0x40:0x44], 0x12345678)

	binary.LittleEndian.PutUint32(plain[0x04:0x08], math.Float32bits(100.5))
	binary.LittleEndian.PutUint32(plain[0x08:0x0C], math.Float32bits(50.25))
	binary.LittleEndian.PutUint32(plain[0x0C:0x10], math.Float32bits(-30.0))

	binary.LittleEndian.PutUint32(plain[0x3C:0x40], math.Float32bits(7500.0))

	binary.LittleEndian.PutUint32(plain[0x4C:0x50], math.Float32bits(55.56))

	plain[0x91] = 255
	plain[0x92] = 0
	plain[0x90] = 4

	binary.LittleEndian.PutUint16(plain[0x74:0x76], 3)
	binary.LittleEndian.PutUint16(plain[0x76:0x78], 10)
	binary.LittleEndian.PutUint32(plain[0x78:0x7C], 85432)
	binary.LittleEndian.PutUint32(plain[0x7C:0x80], 87654)

	binary.LittleEndian.PutUint32(plain[0x70:0x74], 42)

	binary.LittleEndian.PutUint32(plain[0x124:0x128], 1234)

	encrypted := encrypt(plain)

	decrypted, err := decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	tel := parse(decrypted)

	if tel.PacketID != 42 {
		t.Errorf("PacketID = %d, want 42", tel.PacketID)
	}
	if !approx(tel.PosX, 100.5) {
		t.Errorf("PosX = %f, want 100.5", tel.PosX)
	}
	if !approx(tel.PosY, 50.25) {
		t.Errorf("PosY = %f, want 50.25", tel.PosY)
	}
	if !approx(tel.PosZ, -30.0) {
		t.Errorf("PosZ = %f, want -30.0", tel.PosZ)
	}
	if !approx(tel.RPM, 7500.0) {
		t.Errorf("RPM = %f, want 7500", tel.RPM)
	}
	if !approx(tel.Speed, 55.56) {
		t.Errorf("Speed = %f, want 55.56", tel.Speed)
	}
	if tel.Throttle != 255 {
		t.Errorf("Throttle = %d, want 255", tel.Throttle)
	}
	if tel.Brake != 0 {
		t.Errorf("Brake = %d, want 0", tel.Brake)
	}
	if tel.Gear != 4 {
		t.Errorf("Gear = %d, want 4", tel.Gear)
	}
	if tel.CurrentLap != 3 {
		t.Errorf("CurrentLap = %d, want 3", tel.CurrentLap)
	}
	if tel.TotalLaps != 10 {
		t.Errorf("TotalLaps = %d, want 10", tel.TotalLaps)
	}
	if tel.BestLap != 85432 {
		t.Errorf("BestLap = %d, want 85432", tel.BestLap)
	}
	if tel.LastLap != 87654 {
		t.Errorf("LastLap = %d, want 87654", tel.LastLap)
	}
	if tel.CarID != 1234 {
		t.Errorf("CarID = %d, want 1234", tel.CarID)
	}
}

func TestDecryptBadMagic(t *testing.T) {
	plain := make([]byte, 296)
	binary.LittleEndian.PutUint32(plain[0:4], 0xDEADDEAD)
	binary.LittleEndian.PutUint32(plain[0x40:0x44], 0xAABBCCDD)

	encrypted := encrypt(plain)
	_, err := decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestDecryptShortPacket(t *testing.T) {
	_, err := decrypt(make([]byte, 10))
	if err == nil {
		t.Fatal("expected error for short packet")
	}
}

func approx(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.01
}
