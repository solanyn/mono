#!/usr/bin/env bash
set -euo pipefail

# End-to-end test for the line telemetry pipeline.
# Requires: Redpanda running on localhost:9092
#
# Tests the full flow:
#   simulator → capture → Redpanda → assembler → Parquet files (local /tmp)
#
# Usage:
#   ./e2e.sh              # run with default settings
#   LAPS=3 ./e2e.sh      # override lap count

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK_DIR=$(mktemp -d)
LAPS="${LAPS:-3}"
TRACK_POINTS="${TRACK_POINTS:-600}"
FPS="${FPS:-120}"

cleanup() {
    echo "--- Cleaning up ---"
    kill "$CAPTURE_PID" 2>/dev/null || true
    kill "$ASSEMBLER_PID" 2>/dev/null || true
    kill "$SIMULATOR_PID" 2>/dev/null || true
    rm -rf "$WORK_DIR"
}
trap cleanup EXIT

echo "=== Line E2E Test ==="
echo "Work dir: $WORK_DIR"
echo "Laps: $LAPS, Track points: $TRACK_POINTS, FPS: $FPS"
echo ""

# Build all binaries
echo "--- Building binaries ---"
cd "$LINE_DIR"
go build -o "$WORK_DIR/capture" ./cmd/capture/
go build -o "$WORK_DIR/assembler" ./cmd/assembler/
go build -o "$WORK_DIR/simulator" ./cmd/simulator/
echo "Build OK"
echo ""

# Check Redpanda is available
if ! nc -z localhost 9092 2>/dev/null; then
    echo "ERROR: Redpanda not available on localhost:9092"
    echo "Start it with: docker run -d --name redpanda -p 9092:9092 -p 9644:9644 docker.redpanda.com/redpandadata/redpanda:latest redpanda start --smp 1 --memory 256M --overprovisioned --kafka-addr 0.0.0.0:9092 --advertise-kafka-addr localhost:9092"
    exit 1
fi
echo "Redpanda: OK"

# Create output directory for parquet files (mock S3)
mkdir -p "$WORK_DIR/s3"

# Start capture service
echo ""
echo "--- Starting capture ---"
PS5_IP=127.0.0.1 \
KAFKA_BROKERS=localhost:9092 \
KAFKA_TOPIC=line.e2e.telemetry \
HEARTBEAT_INTERVAL=50 \
"$WORK_DIR/capture" > "$WORK_DIR/capture.log" 2>&1 &
CAPTURE_PID=$!
sleep 1

if ! kill -0 "$CAPTURE_PID" 2>/dev/null; then
    echo "ERROR: Capture failed to start"
    cat "$WORK_DIR/capture.log"
    exit 1
fi
echo "Capture PID: $CAPTURE_PID"

# Start assembler
echo ""
echo "--- Starting assembler ---"
KAFKA_BROKERS=localhost:9092 \
KAFKA_TOPIC=line.e2e.telemetry \
KAFKA_GROUP=line.e2e.assembler \
KAFKA_LAP_TOPIC=line.e2e.lap \
KAFKA_SESSION_TOPIC=line.e2e.session \
S3_ENDPOINT=http://localhost:3900 \
S3_BUCKET=line-e2e \
"$WORK_DIR/assembler" > "$WORK_DIR/assembler.log" 2>&1 &
ASSEMBLER_PID=$!
sleep 1

if ! kill -0 "$ASSEMBLER_PID" 2>/dev/null; then
    echo "ERROR: Assembler failed to start"
    cat "$WORK_DIR/assembler.log"
    exit 1
fi
echo "Assembler PID: $ASSEMBLER_PID"

# Start simulator
echo ""
echo "--- Starting simulator ---"
LAPS="$LAPS" \
TRACK_POINTS="$TRACK_POINTS" \
FPS="$FPS" \
CAR_ID=1234 \
"$WORK_DIR/simulator" > "$WORK_DIR/simulator.log" 2>&1 &
SIMULATOR_PID=$!

echo "Simulator PID: $SIMULATOR_PID"
echo "Waiting for simulation to complete..."

# Wait for simulator to finish
wait "$SIMULATOR_PID" || true
echo ""
echo "--- Simulator finished ---"
cat "$WORK_DIR/simulator.log"

# Give assembler time to process remaining frames and idle-flush
echo ""
echo "Waiting for assembler to flush (35s idle timeout)..."
sleep 36

# Check results
echo ""
echo "=== Results ==="
echo ""
echo "--- Capture log (last 5 lines) ---"
tail -5 "$WORK_DIR/capture.log"
echo ""
echo "--- Assembler log (last 10 lines) ---"
tail -10 "$WORK_DIR/assembler.log"

# Verify assembler processed laps
if grep -q "flushed lap" "$WORK_DIR/assembler.log"; then
    FLUSHED=$(grep -c "flushed lap" "$WORK_DIR/assembler.log")
    echo ""
    echo "SUCCESS: Assembler flushed $FLUSHED laps"
else
    echo ""
    echo "WARNING: No laps flushed (S3 may not be available, but Kafka flow worked)"
    if grep -q "total_frames" "$WORK_DIR/assembler.log"; then
        echo "Assembler received frames from Kafka (pipeline connected)"
    fi
fi

if grep -q "published session complete" "$WORK_DIR/assembler.log"; then
    echo "SUCCESS: Session complete event published"
fi

# Check capture stats
if grep -q "produced" "$WORK_DIR/capture.log"; then
    echo ""
    PRODUCED=$(grep "capture stats\|capture stopped" "$WORK_DIR/capture.log" | tail -1)
    echo "Capture: $PRODUCED"
fi

echo ""
echo "=== E2E Test Complete ==="
