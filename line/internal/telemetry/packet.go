package telemetry

import (
	"encoding/binary"
	"fmt"
	"math"

	"golang.org/x/crypto/salsa20"
)

const (
	SendPort    = 33739
	ReceivePort = 33740
	PacketSize  = 296
	magic       = 0x47375330
)

var key = []byte("Simulator Interface Packet GT7 v")

type Frame struct {
	PacketID    int32
	PosX        float32
	PosY        float32
	PosZ        float32
	VelX        float32
	VelY        float32
	VelZ        float32
	RotPitch    float32
	RotYaw      float32
	RotRoll     float32
	AngVelX     float32
	AngVelY     float32
	AngVelZ     float32
	RideHeight  float32
	RPM         float32
	FuelLevel   float32
	FuelCap     float32
	Speed       float32
	Boost       float32
	OilPressure float32
	WaterTemp   float32
	OilTemp     float32
	TireTempFL  float32
	TireTempFR  float32
	TireTempRL  float32
	TireTempRR  float32
	SuspFL      float32
	SuspFR      float32
	SuspRL      float32
	SuspRR      float32
	WheelFL     float32
	WheelFR     float32
	WheelRL     float32
	WheelRR     float32
	TireRadFL   float32
	TireRadFR   float32
	TireRadRL   float32
	TireRadRR   float32
	CurrentLap  int16
	TotalLaps   int16
	BestLap     int32
	LastLap     int32
	CurrentTime int32
	Throttle    uint8
	Brake       uint8
	Steering    int8
	Gear        uint8
	CarID       int32
	Flags       uint16
	TimeOfDay   int32
}

func Decrypt(data []byte) ([]byte, error) {
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

func Parse(d []byte) Frame {
	return Frame{
		PacketID:    int32(binary.LittleEndian.Uint32(d[0x70 : 0x70+4])),
		PosX:       f32(d, 0x04),
		PosY:       f32(d, 0x08),
		PosZ:       f32(d, 0x0C),
		VelX:       f32(d, 0x10),
		VelY:       f32(d, 0x14),
		VelZ:       f32(d, 0x18),
		RotPitch:   f32(d, 0x1C),
		RotYaw:     f32(d, 0x20),
		RotRoll:    f32(d, 0x24),
		AngVelX:    f32(d, 0x2C),
		AngVelY:    f32(d, 0x30),
		AngVelZ:    f32(d, 0x34),
		RideHeight: f32(d, 0x38),
		RPM:        f32(d, 0x3C),
		FuelLevel:  f32(d, 0x44),
		FuelCap:    f32(d, 0x48),
		Speed:      f32(d, 0x4C),
		Boost:      f32(d, 0x50),
		OilPressure: f32(d, 0x54),
		WaterTemp:  f32(d, 0x58),
		OilTemp:    f32(d, 0x5C),
		TireTempFL: f32(d, 0x60),
		TireTempFR: f32(d, 0x64),
		TireTempRL: f32(d, 0x68),
		TireTempRR: f32(d, 0x6C),
		SuspFL:     f32(d, 0xC4),
		SuspFR:     f32(d, 0xC8),
		SuspRL:     f32(d, 0xCC),
		SuspRR:     f32(d, 0xD0),
		WheelFL:    f32(d, 0xA4),
		WheelFR:    f32(d, 0xA8),
		WheelRL:    f32(d, 0xAC),
		WheelRR:    f32(d, 0xB0),
		TireRadFL:  f32(d, 0xB4),
		TireRadFR:  f32(d, 0xB8),
		TireRadRL:  f32(d, 0xBC),
		TireRadRR:  f32(d, 0xC0),
		CurrentLap: int16(binary.LittleEndian.Uint16(d[0x74 : 0x74+2])),
		TotalLaps:  int16(binary.LittleEndian.Uint16(d[0x76 : 0x76+2])),
		BestLap:    int32(binary.LittleEndian.Uint32(d[0x78 : 0x78+4])),
		LastLap:    int32(binary.LittleEndian.Uint32(d[0x7C : 0x7C+4])),
		CurrentTime: int32(binary.LittleEndian.Uint32(d[0x80 : 0x80+4])),
		Throttle:   d[0x91],
		Brake:      d[0x92],
		Steering:   int8(d[0x93]),
		Gear:       d[0x90] & 0x0F,
		CarID:      int32(binary.LittleEndian.Uint32(d[0x124 : 0x124+4])),
		Flags:      binary.LittleEndian.Uint16(d[0x8E : 0x8E+2]),
		TimeOfDay:  int32(binary.LittleEndian.Uint32(d[0x104 : 0x104+4])),
	}
}

func f32(d []byte, off int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(d[off : off+4]))
}

func (f Frame) IsOnTrack() bool {
	return f.Flags&0x01 != 0
}

func (f Frame) IsPaused() bool {
	return f.Flags&0x02 != 0
}
