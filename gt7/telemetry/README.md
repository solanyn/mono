# GT7 Telemetry Server

Rust-based telemetry bridge that captures Gran Turismo 7 UDP packets and streams them to a Kafka topic.

## Development

```bash
# Build and run
bazel run //gt7/telemetry:telemetry_server

# Build container
bazel build //gt7/telemetry:image

# Tests
bazel test //gt7/telemetry:...
```
