package telemetry

import (
	"encoding/binary"
	"math"
)

const EncodedFrameSize = 182

func (f *Frame) Encode() []byte {
	b := make([]byte, EncodedFrameSize)
	binary.LittleEndian.PutUint32(b[0:], uint32(f.PacketID))
	putF32(b[4:], f.PosX)
	putF32(b[8:], f.PosY)
	putF32(b[12:], f.PosZ)
	putF32(b[16:], f.VelX)
	putF32(b[20:], f.VelY)
	putF32(b[24:], f.VelZ)
	putF32(b[28:], f.RotPitch)
	putF32(b[32:], f.RotYaw)
	putF32(b[36:], f.RotRoll)
	putF32(b[40:], f.AngVelX)
	putF32(b[44:], f.AngVelY)
	putF32(b[48:], f.AngVelZ)
	putF32(b[52:], f.RideHeight)
	putF32(b[56:], f.RPM)
	putF32(b[60:], f.FuelLevel)
	putF32(b[64:], f.FuelCap)
	putF32(b[68:], f.Speed)
	putF32(b[72:], f.Boost)
	putF32(b[76:], f.OilPressure)
	putF32(b[80:], f.WaterTemp)
	putF32(b[84:], f.OilTemp)
	putF32(b[88:], f.TireTempFL)
	putF32(b[92:], f.TireTempFR)
	putF32(b[96:], f.TireTempRL)
	putF32(b[100:], f.TireTempRR)
	putF32(b[104:], f.SuspFL)
	putF32(b[108:], f.SuspFR)
	putF32(b[112:], f.SuspRL)
	putF32(b[116:], f.SuspRR)
	putF32(b[120:], f.WheelFL)
	putF32(b[124:], f.WheelFR)
	putF32(b[128:], f.WheelRL)
	putF32(b[132:], f.WheelRR)
	putF32(b[136:], f.TireRadFL)
	putF32(b[140:], f.TireRadFR)
	putF32(b[144:], f.TireRadRL)
	putF32(b[148:], f.TireRadRR)
	binary.LittleEndian.PutUint16(b[152:], uint16(f.CurrentLap))
	binary.LittleEndian.PutUint16(b[154:], uint16(f.TotalLaps))
	binary.LittleEndian.PutUint32(b[156:], uint32(f.BestLap))
	binary.LittleEndian.PutUint32(b[160:], uint32(f.LastLap))
	binary.LittleEndian.PutUint32(b[164:], uint32(f.CurrentTime))
	b[168] = f.Throttle
	b[169] = f.Brake
	b[170] = byte(f.Steering)
	b[171] = f.Gear
	binary.LittleEndian.PutUint32(b[172:], uint32(f.CarID))
	binary.LittleEndian.PutUint16(b[176:], f.Flags)
	binary.LittleEndian.PutUint32(b[178:], uint32(f.TimeOfDay))
	return b
}

func DecodeFrame(b []byte) Frame {
	return Frame{
		PacketID:    int32(binary.LittleEndian.Uint32(b[0:])),
		PosX:        getF32(b[4:]),
		PosY:        getF32(b[8:]),
		PosZ:        getF32(b[12:]),
		VelX:        getF32(b[16:]),
		VelY:        getF32(b[20:]),
		VelZ:        getF32(b[24:]),
		RotPitch:    getF32(b[28:]),
		RotYaw:      getF32(b[32:]),
		RotRoll:     getF32(b[36:]),
		AngVelX:     getF32(b[40:]),
		AngVelY:     getF32(b[44:]),
		AngVelZ:     getF32(b[48:]),
		RideHeight:  getF32(b[52:]),
		RPM:         getF32(b[56:]),
		FuelLevel:   getF32(b[60:]),
		FuelCap:     getF32(b[64:]),
		Speed:       getF32(b[68:]),
		Boost:       getF32(b[72:]),
		OilPressure: getF32(b[76:]),
		WaterTemp:   getF32(b[80:]),
		OilTemp:     getF32(b[84:]),
		TireTempFL:  getF32(b[88:]),
		TireTempFR:  getF32(b[92:]),
		TireTempRL:  getF32(b[96:]),
		TireTempRR:  getF32(b[100:]),
		SuspFL:      getF32(b[104:]),
		SuspFR:      getF32(b[108:]),
		SuspRL:      getF32(b[112:]),
		SuspRR:      getF32(b[116:]),
		WheelFL:     getF32(b[120:]),
		WheelFR:     getF32(b[124:]),
		WheelRL:     getF32(b[128:]),
		WheelRR:     getF32(b[132:]),
		TireRadFL:   getF32(b[136:]),
		TireRadFR:   getF32(b[140:]),
		TireRadRL:   getF32(b[144:]),
		TireRadRR:   getF32(b[148:]),
		CurrentLap:  int16(binary.LittleEndian.Uint16(b[152:])),
		TotalLaps:   int16(binary.LittleEndian.Uint16(b[154:])),
		BestLap:     int32(binary.LittleEndian.Uint32(b[156:])),
		LastLap:     int32(binary.LittleEndian.Uint32(b[160:])),
		CurrentTime: int32(binary.LittleEndian.Uint32(b[164:])),
		Throttle:    b[168],
		Brake:       b[169],
		Steering:    int8(b[170]),
		Gear:        b[171],
		CarID:       int32(binary.LittleEndian.Uint32(b[172:])),
		Flags:       binary.LittleEndian.Uint16(b[176:]),
		TimeOfDay:   int32(binary.LittleEndian.Uint32(b[178:])),
	}
}

func putF32(b []byte, v float32) {
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
}

func getF32(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}
