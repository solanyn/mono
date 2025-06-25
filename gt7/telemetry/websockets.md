# GT7 Telemetry WebSocket Output Structure

WebSocket endpoint: `ws://localhost:8080/ws`

Each message is a JSON object with the following structure:

```typescript
interface GT7TelemetryPacket {
  // Position & Movement
  position: [number, number, number]; // [x, y, z] coordinates in meters
  velocity: [number, number, number]; // [x, y, z] velocity in m/s
  rotation: [number, number, number]; // [pitch, yaw, roll] in radians
  relative_orientation_to_north: number; // Heading relative to north in radians
  angular_velocity: [number, number, number]; // [pitch, yaw, roll] angular velocity
  body_height: number; // Vehicle body height above ground in meters
  meters_per_second: number; // Current speed in m/s

  // Engine
  engine_rpm: number;
  gas_level: number; // Current fuel level
  gas_capacity: number; // Maximum fuel capacity
  turbo_boost: number;
  oil_pressure: number;
  water_temperature: number;
  oil_temperature: number;

  // Tires
  tire_fl_surface_temperature: number; // Front-left
  tire_fr_surface_temperature: number; // Front-right
  tire_rl_surface_temperature: number; // Rear-left
  tire_rr_surface_temperature: number; // Rear-right
  tire_fl_radius: number;
  tire_fr_radius: number;
  tire_rl_radius: number;
  tire_rr_radius: number;
  tire_fl_suspension_height: number;
  tire_fr_suspension_height: number;
  tire_rl_suspension_height: number;
  tire_rr_suspension_height: number;

  // Wheels
  wheel_fl_rps: number; // Rotations per second
  wheel_fr_rps: number;
  wheel_rl_rps: number;
  wheel_rr_rps: number;

  // Race Data
  packet_id: number;
  lap_count: number;
  laps_in_race: number;
  best_lap_time: number; // Milliseconds
  last_lap_time: number; // Milliseconds
  time_of_day_progression: number;
  qualifying_position: number;
  num_cars_pre_race: number;

  // Controls
  current_gear: number; // 0 = reverse, 1-8 = forward gears
  suggested_gear: number;
  throttle: number; // 0-255
  brake: number; // 0-255
  clutch_pedal: number;
  clutch_engagement: number;

  // Performance
  alert_rpm_min: number;
  alert_rpm_max: number;
  calculated_max_speed: number;
  rpm_from_clutch_to_gearbox: number;
  transmission_top_speed: number;
  gear_ratios: [number, number, number, number, number, number, number]; // Gears 1-7

  // Track
  road_plane: [number, number, number]; // Surface normal vector
  road_plane_distance: number;

  // Vehicle
  car_code: number;

  // Status Flags
  flags: {
    CarOnTrack: boolean;
    Paused: boolean;
    LoadingOrProcessing: boolean;
    InGear: boolean;
    HasTurbo: boolean;
    RevLimiterAlertActive: boolean;
    HandBrakeActive: boolean;
    LightsActive: boolean;
    HighBeamActive: boolean;
    LowBeamActive: boolean;
    ASMActive: boolean;
    TCSActive: boolean;
  } | null;
}
```
