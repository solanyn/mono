# GT7 Pulsar Bridge

This project bridges telemetry data from Gran Turismo 7 to an Apache Pulsar topic. It captures real-time UDP telemetry packets transmitted by GT7 and forwards them to Apache Pulsar for further processing and analysis.

## About GT7 Telemetry

Gran Turismo 7 always transmits UDP telemetry data - there are no game settings to enable or disable it. The game continuously sends encrypted packets that provide comprehensive real-time information about the vehicle and race state.

### Telemetry Specifications

- **Packet Size**: 296 bytes
- **Transmission Rate**: 60Hz (60 packets per second)
- **Protocol**: UDP
- **Encryption**: Salsa20 stream cipher with 32-byte key and 8-byte nonce
- **Port**: 33739 (default GT7 telemetry port)

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
- Periodic heartbeat transmission (every 100 packets received)
- Packet decryption and parsing
- Data validation and error handling

## Local Development & Testing with Docker Compose

This guide explains how to run the application and an Apache Pulsar instance locally using Docker Compose for development and testing purposes.

### Prerequisites

*   **Docker**: Ensure Docker is installed and running on your system. You can download it from [Docker's website](https://www.docker.com/products/docker-desktop).
*   **Docker Compose**: Docker Compose is typically included with Docker Desktop. If not, follow the [official installation guide](https://docs.docker.com/compose/install/).

### Configuration

The Docker Compose setup (`docker-compose.yml`) includes the `gt7-pulsar-bridge` application and an Apache Pulsar standalone service.

Key environment variables for the `gt7-pulsar-bridge` service are configured in `docker-compose.yml`:

*   `PS5_IP_ADDRESS`:
    *   This needs to be the IP address of your PlayStation 5 on your local network, or an address that your Docker container can use to reach the PS5 (e.g., your host's IP if the PS5 sends data there and port forwarding is set up from the container to the host for the telemetry UDP port if needed, though the current setup assumes direct UDP send from the container).
    *   By default, it's set to `"host.docker.internal"`, which often works on Docker Desktop to refer to the host machine. If your PS5 sends telemetry data to your host machine (where Docker is running), this might work.
    *   **You will likely need to change this value** in `docker-compose.yml` to match your specific network setup and how the PS5's telemetry data is being directed.
*   `PULSAR_SERVICE_URL`: Set to `pulsar://pulsar:6650` to connect to the Pulsar container.
*   `PULSAR_TOPIC`: Defaults to `persistent://public/default/gt7-telemetry`.
*   `RUST_LOG`: Controls logging. Defaults to `info,gt7_pulsar_bridge=debug`. You can adjust this for more or less verbose logging.
*   `HTTP_BIND_ADDRESS`: Set to `0.0.0.0:8080` for the container.
*   `HEARTBEAT_INTERVAL_SECONDS`: Controls how often heartbeat packets are sent to GT7. Defaults to `1.6` seconds, which is the recommended interval to maintain the connection. You can adjust this value, but intervals too long (>1.6s) may cause GT7 to stop sending telemetry data.

### Running the Services

1.  **Navigate to the project directory**:
    Open your terminal and change to the root directory of this project (where the `docker-compose.yml` file is located).

    ```bash
    cd path/to/gt7-pulsar-bridge
    ```

2.  **Start the services**:
    Run the following command:

    ```bash
    docker-compose up --build
    ```
    *   `--build`: This flag tells Docker Compose to rebuild the `gt7-pulsar-bridge` image if there have been any changes to its source code or `Dockerfile`.
    *   The first time you run this, Docker will download the `apachepulsar/pulsar` image, which might take some time. Pulsar itself also takes a minute or two to initialize fully. The application is configured to wait for Pulsar to be healthy before starting.

### Accessing Services

*   **GT7 Pulsar Bridge Health Check**:
    Once the services are running, you can check if the application is up by accessing its health check endpoint in your browser or with `curl`:
    `http://localhost:8080/healthz`

*   **Apache Pulsar Dashboard/Admin UI**:
    The Pulsar service's HTTP admin interface (which is on port `8080` inside its container) is mapped to port `8081` on your host machine to avoid conflicts. You can access it at:
    `http://localhost:8081`
    (e.g., `http://localhost:8081/admin/v2/brokers/health` to check broker health)

### Stopping the Services

To stop the services, press `Ctrl+C` in the terminal where `docker-compose up` is running.

To stop and remove the containers, you can run:

```bash
docker-compose down
```
If you defined and used a named volume for Pulsar data (currently commented out in `docker-compose.yml`) and want to remove it as well, you can use `docker-compose down -v`.

### Troubleshooting

*   **Pulsar Initialization Time**: Apache Pulsar standalone can take a significant amount of time (sometimes a few minutes, especially on the first run) to initialize completely. The `gt7-pulsar-bridge` service is set to depend on Pulsar's health check. If you see connection errors from the bridge to Pulsar, it might be that Pulsar hasn't fully started yet. Check the logs from the `pulsar` container: `docker-compose logs pulsar`.
*   **PS5 Connectivity**: Ensure the `PS5_IP_ADDRESS` is correctly configured and that your Docker container's network can reach the PS5 or the designated UDP telemetry endpoint. UDP networking with Docker can sometimes require specific host network configurations depending on your OS and Docker setup if the telemetry source is external to the Docker host.
