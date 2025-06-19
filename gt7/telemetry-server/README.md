# GT7 Telemetry Server

Rust-based telemetry bridge that captures Gran Turismo 7 UDP packets and streams them to Apache Pulsar.

## System Architecture

```
GT7 Console → UDP (encrypted) → Telemetry Server → Apache Pulsar → Stream Processors
```

**Core Components:**
- UDP listener on port 33739
- Salsa20 decryption engine
- Heartbeat manager (1.6s intervals)
- Pulsar producer client
- HTTP health endpoints

**Data Flow:**
- GT7 transmits 296-byte encrypted packets at 60Hz
- Server maintains connection via heartbeat protocol
- Packets are decrypted and forwarded to Pulsar topic
- Stream processors consume telemetry for analysis

### Data Contents

The telemetry packets include extensive vehicle and race data:

- **Vehicle Physics**: 3-axis position, velocity, rotation, acceleration
- **Engine Data**: RPM, fuel levels, throttle/brake input
- **Performance**: Speed, gear information, tire temperatures and wear
- **Race Information**: Lap times, lap count, race position
- **Track Data**: Position on track, distance traveled
- **Simulator State**: Flags indicating car on track, paused state, in gear, etc.

### Connection Requirements

GT7 sends encrypted telemetry packets to any device that requests them via heartbeat. The console will only send packets to the IP address that initiated the heartbeat request. The connection requires:

- **Heartbeat Transmission**: Must send heartbeat packets regularly (approximately every 1.6 seconds) or the connection will cease
- **IP-based Connection**: GT7 only sends data to the IP address that sent the heartbeat
- **Packet Decryption**: All packets are encrypted with Salsa20 stream cipher

This bridge automatically handles:

- Initial connection establishment via heartbeat
- Continuous heartbeat transmission (configurable interval, defaults to 1.6 seconds)
- Packet decryption and parsing
- Data validation and error handling

## Quick Start

```bash
# Start services
docker-compose up --build

# Health check
curl http://localhost:8080/healthz

# Pulsar admin UI
open http://localhost:8081
```

**Configuration:**
- `PS5_IP_ADDRESS`: GT7 console IP (default: `host.docker.internal`)
- `PULSAR_SERVICE_URL`: `pulsar://pulsar:6650`
- `PULSAR_TOPIC`: `persistent://public/default/gt7-telemetry`

## Development & Testing with Docker Compose

### Prerequisites

- **Docker**: Ensure Docker is installed and running on your system
- **Docker Compose**: Typically included with Docker Desktop

### Configuration

Key environment variables in `docker-compose.yml`:

- `PS5_IP_ADDRESS`: IP address of your PlayStation 5 on your local network
- `PULSAR_SERVICE_URL`: Set to `pulsar://pulsar:6650`
- `PULSAR_TOPIC`: Defaults to `persistent://public/default/gt7-telemetry`
- `RUST_LOG`: Controls logging (defaults to `info,gt7_pulsar_bridge=debug`)
- `HTTP_BIND_ADDRESS`: Set to `0.0.0.0:8080`
- `HEARTBEAT_INTERVAL_SECONDS`: Interval for heartbeat packets (defaults to `1.6` seconds)

### Troubleshooting

- **Pulsar Initialization Time**: Apache Pulsar standalone can take several minutes to initialize completely. Check logs with `docker-compose logs pulsar`.
- **PS5 Connectivity**: Ensure the `PS5_IP_ADDRESS` is correctly configured and that your Docker container's network can reach the PS5.
