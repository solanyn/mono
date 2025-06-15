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
