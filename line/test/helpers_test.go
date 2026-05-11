package e2e

import (
	"encoding/binary"
	"math"
	"time"

	"golang.org/x/crypto/salsa20"
)

const (
	packetSize = 296
	magicVal   = 0x47375330
)

var encKey = []byte("Simulator Interface Packet GT7 v")

type trackPoint struct {
	x, y, z    float32
	speed      float32
	throttle   uint8
	brake      uint8
	steering   int8
	rpm        float32
	gear       uint8
	tireTempFL float32
	tireTempFR float32
	tireTempRL float32
	tireTempRR float32
	fuelLevel  float32
}

func generateTrack(numPoints int) []trackPoint {
	points := make([]trackPoint, numPoints)
	fuelStart := float32(100.0)
	fuelPerFrame := fuelStart / float32(numPoints*5)

	for i := range points {
		t := float64(i) / float64(numPoints) * 2 * math.Pi
		r := 200.0 + 60.0*math.Sin(t*2) + 30.0*math.Cos(t*5)
		x := float32(math.Cos(t) * r)
		z := float32(math.Sin(t) * r)
		y := float32(5.0*math.Sin(t*3) + 2.0*math.Cos(t*7))

		curvature := math.Abs(60.0*2.0*math.Cos(t*2) + 30.0*-5.0*math.Sin(t*5))
		speed := float32(math.Max(40, 280-curvature*0.8))

		var throttle uint8
		var brake uint8
		if speed > 200 {
			throttle = 255
		} else if speed > 100 {
			throttle = uint8(200 + (speed-100)*0.55)
		} else {
			throttle = 100
			brake = uint8((100 - speed) * 2.5)
		}

		steering := int8(math.Sin(t*3) * 60)
		rpm := 3000 + speed*20
		gear := uint8(1)
		switch {
		case speed > 220:
			gear = 6
		case speed > 180:
			gear = 5
		case speed > 140:
			gear = 4
		case speed > 100:
			gear = 3
		case speed > 60:
			gear = 2
		}

		baseTemp := float32(75.0 + float64(brake)*0.05)
		fuel := fuelStart - fuelPerFrame*float32(i)
		if fuel < 0 {
			fuel = 0
		}

		points[i] = trackPoint{
			x: x, y: y, z: z,
			speed:      speed,
			throttle:   throttle,
			brake:      brake,
			steering:   steering,
			rpm:        rpm,
			gear:       gear,
			tireTempFL: baseTemp + float32(math.Abs(float64(steering))*0.1),
			tireTempFR: baseTemp + float32(math.Abs(float64(steering))*0.08),
			tireTempRL: baseTemp - 2,
			tireTempRR: baseTemp - 2,
			fuelLevel:  fuel,
		}
	}
	return points
}

func buildPacket(pktID int32, pt trackPoint, currentLap, totalLaps int16, carID, currentTimeMs int32) []byte {
	d := make([]byte, packetSize)

	binary.LittleEndian.PutUint32(d[0:4], magicVal)
	putF32(d, 0x04, pt.x)
	putF32(d, 0x08, pt.y)
	putF32(d, 0x0C, pt.z)
	putF32(d, 0x3C, pt.rpm)
	putF32(d, 0x44, pt.fuelLevel)
	putF32(d, 0x48, 100.0)
	putF32(d, 0x4C, pt.speed)
	putF32(d, 0x60, pt.tireTempFL)
	putF32(d, 0x64, pt.tireTempFR)
	putF32(d, 0x68, pt.tireTempRL)
	putF32(d, 0x6C, pt.tireTempRR)

	binary.LittleEndian.PutUint32(d[0x70:0x74], uint32(pktID))
	binary.LittleEndian.PutUint16(d[0x74:0x76], uint16(currentLap))
	binary.LittleEndian.PutUint16(d[0x76:0x78], uint16(totalLaps))
	binary.LittleEndian.PutUint32(d[0x80:0x84], uint32(currentTimeMs))

	d[0x90] = pt.gear
	d[0x91] = pt.throttle
	d[0x92] = pt.brake
	d[0x93] = byte(pt.steering)

	binary.LittleEndian.PutUint16(d[0x8E:0x90], 0x01)
	binary.LittleEndian.PutUint32(d[0x124:0x128], uint32(carID))

	return d
}

func encrypt(plaintext []byte) []byte {
	iv := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
	iv2 := iv ^ 0xDEADBEAF

	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[0:4], iv2)
	binary.LittleEndian.PutUint32(nonce[4:8], iv)

	var k [32]byte
	copy(k[:], encKey)

	out := make([]byte, len(plaintext))
	salsa20.XORKeyStream(out, plaintext, nonce[:], &k)
	binary.LittleEndian.PutUint32(out[0x40:0x44], iv)

	return out
}

func putF32(d []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(d[off:off+4], math.Float32bits(v))
}
