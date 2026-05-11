package storage

import (
	"bytes"
	"fmt"
	"io"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/snappy"
)

type TelemetryRow struct {
	PacketID    int32   `parquet:"packet_id"`
	PosX        float32 `parquet:"pos_x"`
	PosY        float32 `parquet:"pos_y"`
	PosZ        float32 `parquet:"pos_z"`
	VelX        float32 `parquet:"vel_x"`
	VelY        float32 `parquet:"vel_y"`
	VelZ        float32 `parquet:"vel_z"`
	RotPitch    float32 `parquet:"rot_pitch"`
	RotYaw      float32 `parquet:"rot_yaw"`
	RotRoll     float32 `parquet:"rot_roll"`
	AngVelX     float32 `parquet:"ang_vel_x"`
	AngVelY     float32 `parquet:"ang_vel_y"`
	AngVelZ     float32 `parquet:"ang_vel_z"`
	RideHeight  float32 `parquet:"ride_height"`
	RPM         float32 `parquet:"rpm"`
	FuelLevel   float32 `parquet:"fuel_level"`
	FuelCap     float32 `parquet:"fuel_cap"`
	Speed       float32 `parquet:"speed"`
	Boost       float32 `parquet:"boost"`
	OilPressure float32 `parquet:"oil_pressure"`
	WaterTemp   float32 `parquet:"water_temp"`
	OilTemp     float32 `parquet:"oil_temp"`
	TireTempFL  float32 `parquet:"tire_temp_fl"`
	TireTempFR  float32 `parquet:"tire_temp_fr"`
	TireTempRL  float32 `parquet:"tire_temp_rl"`
	TireTempRR  float32 `parquet:"tire_temp_rr"`
	SuspFL      float32 `parquet:"susp_fl"`
	SuspFR      float32 `parquet:"susp_fr"`
	SuspRL      float32 `parquet:"susp_rl"`
	SuspRR      float32 `parquet:"susp_rr"`
	WheelFL     float32 `parquet:"wheel_fl"`
	WheelFR     float32 `parquet:"wheel_fr"`
	WheelRL     float32 `parquet:"wheel_rl"`
	WheelRR     float32 `parquet:"wheel_rr"`
	TireRadFL   float32 `parquet:"tire_rad_fl"`
	TireRadFR   float32 `parquet:"tire_rad_fr"`
	TireRadRL   float32 `parquet:"tire_rad_rl"`
	TireRadRR   float32 `parquet:"tire_rad_rr"`
	CurrentLap  int32   `parquet:"current_lap"`
	TotalLaps   int32   `parquet:"total_laps"`
	BestLap     int32   `parquet:"best_lap_ms"`
	LastLap     int32   `parquet:"last_lap_ms"`
	CurrentTime int32   `parquet:"current_time_ms"`
	Throttle    int32   `parquet:"throttle"`
	Brake       int32   `parquet:"brake"`
	Steering    int32   `parquet:"steering"`
	Gear        int32   `parquet:"gear"`
	CarID       int32   `parquet:"car_id"`
	Flags       int32   `parquet:"flags"`
	TimeOfDay   int32   `parquet:"time_of_day"`
}

func WriteParquet(rows []TelemetryRow) ([]byte, error) {
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[TelemetryRow](&buf,
		parquet.Compression(&snappy.Codec{}),
	)
	if _, err := w.Write(rows); err != nil {
		return nil, fmt.Errorf("parquet write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("parquet close: %w", err)
	}
	return buf.Bytes(), nil
}

func ReadParquet(data []byte) ([]TelemetryRow, error) {
	r := parquet.NewGenericReader[TelemetryRow](bytes.NewReader(data))
	defer r.Close()

	rows := make([]TelemetryRow, r.NumRows())
	n, err := r.Read(rows)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("parquet read: %w", err)
	}
	return rows[:n], nil
}
