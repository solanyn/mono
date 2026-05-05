package linepb

type Session struct {
	Id        string `json:"id"`
	StartedAt int64  `json:"started_at"`
	EndedAt   int64  `json:"ended_at"`
	CarCode   int32  `json:"car_code"`
	TrackId   string `json:"track_id"`
	LapCount  int32  `json:"lap_count"`
	BestLapMs int32  `json:"best_lap_ms"`
}

type Lap struct {
	SessionId string  `json:"session_id"`
	LapNumber int32   `json:"lap_number"`
	TimeMs    int32   `json:"time_ms"`
	Frames    int32   `json:"frames"`
	TopSpeed  float32 `json:"top_speed"`
	S3Key     string  `json:"s3_key"`
}

type TelemetryFrame struct {
	PacketID    int32   `json:"packet_id"`
	X           float32 `json:"x"`
	Y           float32 `json:"y"`
	Z           float32 `json:"z"`
	Speed       float32 `json:"speed"`
	Throttle    float32 `json:"throttle"`
	Brake       float32 `json:"brake"`
	Steering    float32 `json:"steering"`
	Rpm         float32 `json:"rpm"`
	Gear        int32   `json:"gear"`
	TireTempFL  float32 `json:"tire_temp_fl,omitempty"`
	TireTempFR  float32 `json:"tire_temp_fr,omitempty"`
	TireTempRL  float32 `json:"tire_temp_rl,omitempty"`
	TireTempRR  float32 `json:"tire_temp_rr,omitempty"`
	SuspFL      float32 `json:"susp_fl,omitempty"`
	SuspFR      float32 `json:"susp_fr,omitempty"`
	SuspRL      float32 `json:"susp_rl,omitempty"`
	SuspRR      float32 `json:"susp_rr,omitempty"`
	FuelLevel   float32 `json:"fuel_level,omitempty"`
	CurrentLap  int32   `json:"current_lap"`
	CurrentTime int32   `json:"current_time_ms"`
	TimestampNs int64   `json:"timestamp_ns"`
}

type LiveStatus struct {
	Active    bool   `json:"active"`
	SessionId string `json:"session_id,omitempty"`
	CarCode   int32  `json:"car_code,omitempty"`
	TrackId   string `json:"track_id,omitempty"`
	CurrentLap int32 `json:"current_lap,omitempty"`
}
