# GT7 Telemetry Server

Rust-based telemetry bridge that captures Gran Turismo 7 UDP packets and streams them to Apache Pulsar.

## Features

- UDP listener on port 33739 with Salsa20 decryption
- Heartbeat manager (1.6s intervals) for GT7 connection
- Pulsar producer client for streaming telemetry
- HTTP health endpoints and monitoring

## Development

```bash
# Build and run
bazelisk run //gt7/telemetry:telemetry_server

# Build container
bazelisk build //gt7/telemetry:image

# Tests
bazelisk test //gt7/telemetry:...
```

## Usage

Configure environment variables:

```bash
export PS5_IP_ADDRESS=192.168.1.100
export PULSAR_SERVICE_URL=pulsar://pulsar:6650
export PULSAR_TOPIC=persistent://public/default/gt7-telemetry
```

Server runs on `http://0.0.0.0:8080` with health check at `/healthz`

## Data Flow

GT7 Console → UDP (encrypted) → Telemetry Server → Apache Pulsar → Stream Processors
