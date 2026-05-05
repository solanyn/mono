package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/salsa20"
)

const (
	sendPort    = 33739
	receivePort = 33740
	magic       = 0x47375330
)

var key = []byte("Simulator Interface Packet GT7 v")

type Telemetry struct {
	PacketID  int32
	PosX      float32
	PosY      float32
	PosZ      float32
	VelX      float32
	VelY      float32
	VelZ      float32
	RotPitch  float32
	RotYaw    float32
	RotRoll   float32
	AngVelX   float32
	AngVelY   float32
	AngVelZ   float32
	RideHeight float32
	RPM       float32
	FuelLevel float32
	FuelCap   float32
	Speed     float32
	Boost     float32
	OilPressure float32
	WaterTemp float32
	OilTemp   float32
	TireTempFL float32
	TireTempFR float32
	TireTempRL float32
	TireTempRR float32
	CurrentLap int16
	TotalLaps  int16
	BestLap   int32
	LastLap   int32
	Throttle  uint8
	Brake     uint8
	Gear      uint8
	CarID     int32
}

func decrypt(data []byte) ([]byte, error) {
	if len(data) < 0x44+4 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	oiv := binary.LittleEndian.Uint32(data[0x40:0x44])
	iv2 := oiv ^ 0xDEADBEAF

	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[0:4], iv2)
	binary.LittleEndian.PutUint32(nonce[4:8], oiv)

	var k [32]byte
	copy(k[:], key)

	out := make([]byte, len(data))
	salsa20.XORKeyStream(out, data, nonce[:], &k)

	if binary.LittleEndian.Uint32(out[0:4]) != magic {
		return nil, fmt.Errorf("bad magic: %08x", binary.LittleEndian.Uint32(out[0:4]))
	}

	return out, nil
}

func parse(d []byte) Telemetry {
	return Telemetry{
		PacketID:   int32(binary.LittleEndian.Uint32(d[0x70 : 0x70+4])),
		PosX:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x04 : 0x04+4])),
		PosY:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x08 : 0x08+4])),
		PosZ:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x0C : 0x0C+4])),
		VelX:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x10 : 0x10+4])),
		VelY:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x14 : 0x14+4])),
		VelZ:       math.Float32frombits(binary.LittleEndian.Uint32(d[0x18 : 0x18+4])),
		RotPitch:   math.Float32frombits(binary.LittleEndian.Uint32(d[0x1C : 0x1C+4])),
		RotYaw:     math.Float32frombits(binary.LittleEndian.Uint32(d[0x20 : 0x20+4])),
		RotRoll:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x24 : 0x24+4])),
		AngVelX:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x2C : 0x2C+4])),
		AngVelY:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x30 : 0x30+4])),
		AngVelZ:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x34 : 0x34+4])),
		RideHeight: math.Float32frombits(binary.LittleEndian.Uint32(d[0x38 : 0x38+4])),
		RPM:        math.Float32frombits(binary.LittleEndian.Uint32(d[0x3C : 0x3C+4])),
		FuelLevel:  math.Float32frombits(binary.LittleEndian.Uint32(d[0x44 : 0x44+4])),
		FuelCap:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x48 : 0x48+4])),
		Speed:      math.Float32frombits(binary.LittleEndian.Uint32(d[0x4C : 0x4C+4])),
		Boost:      math.Float32frombits(binary.LittleEndian.Uint32(d[0x50 : 0x50+4])),
		OilPressure: math.Float32frombits(binary.LittleEndian.Uint32(d[0x54 : 0x54+4])),
		WaterTemp:  math.Float32frombits(binary.LittleEndian.Uint32(d[0x58 : 0x58+4])),
		OilTemp:    math.Float32frombits(binary.LittleEndian.Uint32(d[0x5C : 0x5C+4])),
		TireTempFL: math.Float32frombits(binary.LittleEndian.Uint32(d[0x60 : 0x60+4])),
		TireTempFR: math.Float32frombits(binary.LittleEndian.Uint32(d[0x64 : 0x64+4])),
		TireTempRL: math.Float32frombits(binary.LittleEndian.Uint32(d[0x68 : 0x68+4])),
		TireTempRR: math.Float32frombits(binary.LittleEndian.Uint32(d[0x6C : 0x6C+4])),
		CurrentLap: int16(binary.LittleEndian.Uint16(d[0x74 : 0x74+2])),
		TotalLaps:  int16(binary.LittleEndian.Uint16(d[0x76 : 0x76+2])),
		BestLap:    int32(binary.LittleEndian.Uint32(d[0x78 : 0x78+4])),
		LastLap:    int32(binary.LittleEndian.Uint32(d[0x7C : 0x7C+4])),
		Throttle:   d[0x91],
		Brake:      d[0x92],
		Gear:       d[0x90] & 0x0F,
		CarID:      int32(binary.LittleEndian.Uint32(d[0x124 : 0x124+4])),
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <ps5-ip>\n", os.Args[0])
		os.Exit(1)
	}
	ps5IP := os.Args[1]

	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", receivePort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	ps5Addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ps5IP, sendPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve: %v\n", err)
		os.Exit(1)
	}

	sendHeartbeat := func() {
		conn.WriteTo([]byte("A"), ps5Addr)
	}

	sendHeartbeat()
	fmt.Printf("Sent heartbeat to %s:%d, waiting for telemetry on :%d...\n", ps5IP, sendPort, receivePort)

	buf := make([]byte, 4096)
	pktCount := 0
	lastPktID := int32(0)

	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			fmt.Printf("timeout, resending heartbeat...\n")
			sendHeartbeat()
			continue
		}

		pktCount++
		if pktCount > 100 {
			sendHeartbeat()
			pktCount = 0
		}

		decrypted, err := decrypt(buf[:n])
		if err != nil {
			continue
		}

		t := parse(decrypted)
		if t.PacketID <= lastPktID {
			continue
		}
		lastPktID = t.PacketID

		fmt.Printf("\033[2J\033[H")
		fmt.Printf("=== GT7 Telemetry POC === pkt:%d\n\n", t.PacketID)
		fmt.Printf("Position:  X=%.2f  Y=%.2f  Z=%.2f\n", t.PosX, t.PosY, t.PosZ)
		fmt.Printf("Velocity:  X=%.2f  Y=%.2f  Z=%.2f\n", t.VelX, t.VelY, t.VelZ)
		fmt.Printf("Speed:     %.1f km/h\n", t.Speed*3.6)
		fmt.Printf("RPM:       %.0f\n", t.RPM)
		fmt.Printf("Gear:      %d\n", t.Gear)
		fmt.Printf("Throttle:  %.0f%%\n", float64(t.Throttle)/2.55)
		fmt.Printf("Brake:     %.0f%%\n", float64(t.Brake)/2.55)
		fmt.Printf("Lap:       %d/%d\n", t.CurrentLap, t.TotalLaps)
		if t.BestLap > 0 {
			fmt.Printf("Best Lap:  %d:%06.3f\n", t.BestLap/60000, float64(t.BestLap%60000)/1000)
		}
		if t.LastLap > 0 {
			fmt.Printf("Last Lap:  %d:%06.3f\n", t.LastLap/60000, float64(t.LastLap%60000)/1000)
		}
		fmt.Printf("Fuel:      %.1f/%.1f\n", t.FuelLevel, t.FuelCap)
		fmt.Printf("Car ID:    %d\n", t.CarID)
		fmt.Printf("Tires:     FL=%.1f FR=%.1f RL=%.1f RR=%.1f °C\n",
			t.TireTempFL, t.TireTempFR, t.TireTempRL, t.TireTempRR)
		fmt.Printf("Rotation:  P=%.3f Y=%.3f R=%.3f\n", t.RotPitch, t.RotYaw, t.RotRoll)
	}
}
