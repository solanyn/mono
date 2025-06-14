use env_logger::Env;
use gt7_pulsar_bridge::{
    constants::{PACKET_HEARTBEAT_DATA, PACKET_SIZE},
    errors::ParsePacketError,
    flags::PacketFlags,
    packet::Packet,
};
use log::{debug, error, info, warn};
use std::env;
use std::net::{SocketAddr, UdpSocket};
use std::process;
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};
use tokio::runtime::Runtime;

use axum::{Router, routing::get};

mod pulsar_handler;
use pulsar_handler::PulsarHandler;

const TELEMETRY_SERVER_PORT: u16 = 33739;
const BIND_ADDRESS: &str = "0.0.0.0:33740";
const DEFAULT_HTTP_BIND_ADDRESS: &str = "0.0.0.0:8080";

async fn health_check_handler() -> &'static str {
    "OK"
}

async fn run_http_server(_runtime: Arc<Runtime>) {
    let http_bind_address =
        env::var("HTTP_BIND_ADDRESS").unwrap_or_else(|_| DEFAULT_HTTP_BIND_ADDRESS.to_string());

    let app = Router::new().route("/healthz", get(health_check_handler));

    info!("HTTP server listening on {}", http_bind_address);

    let listener = match tokio::net::TcpListener::bind(&http_bind_address).await {
        Ok(l) => l,
        Err(e) => {
            error!("Failed to bind HTTP server to {}: {}", http_bind_address, e);
            process::exit(1);
        }
    };

    if let Err(e) = axum::serve(listener, app).await {
        error!("HTTP server error: {}", e);
    }
}

fn main() {
    env_logger::Builder::from_env(Env::default().default_filter_or("info")).init();

    // Log configuration at startup
    info!("=== GT7 Pulsar Bridge Configuration ===");
    info!(
        "PS5_IP_ADDRESS: {}",
        env::var("PS5_IP_ADDRESS").unwrap_or_else(|_| "NOT SET".to_string())
    );
    info!(
        "PULSAR_SERVICE_URL: {}",
        env::var("PULSAR_SERVICE_URL").unwrap_or_else(|_| "NOT SET".to_string())
    );
    info!(
        "PULSAR_TOPIC: {}",
        env::var("PULSAR_TOPIC").unwrap_or_else(|_| "NOT SET".to_string())
    );
    info!(
        "HTTP_BIND_ADDRESS: {}",
        env::var("HTTP_BIND_ADDRESS").unwrap_or_else(|_| DEFAULT_HTTP_BIND_ADDRESS.to_string())
    );
    info!(
        "LOG_PACKET_INTERVAL_SECONDS: {}",
        env::var("LOG_PACKET_INTERVAL_SECONDS").unwrap_or_else(|_| "5 (default)".to_string())
    );
    info!(
        "RUST_LOG: {}",
        env::var("RUST_LOG").unwrap_or_else(|_| "info (default)".to_string())
    );
    info!(
        "HEARTBEAT_INTERVAL_SECONDS: {}",
        env::var("HEARTBEAT_INTERVAL_SECONDS").unwrap_or_else(|_| "1.6 (default)".to_string())
    );
    info!("UDP_BIND_ADDRESS: {}", BIND_ADDRESS);
    info!("TELEMETRY_SERVER_PORT: {}", TELEMETRY_SERVER_PORT);
    info!("======================================");

    let default_log_interval_seconds: u64 = 5;
    let log_interval_duration: Option<Duration> = match env::var("LOG_PACKET_INTERVAL_SECONDS") {
        Ok(val_str) => match val_str.parse::<u64>() {
            Ok(0) => {
                // 0 means disable this periodic log
                info!(
                    "Periodic packet detail logging is disabled (LOG_PACKET_INTERVAL_SECONDS=0)."
                );
                None
            }
            Ok(seconds) if seconds > 0 => {
                info!(
                    "Periodic packet detail logging interval set to every {} seconds (from LOG_PACKET_INTERVAL_SECONDS).",
                    seconds
                );
                Some(Duration::from_secs(seconds))
            }
            _ => {
                // Parsed to a non-positive number (other than 0), or failed to parse, or other invalid u64
                info!(
                    "Invalid or non-positive LOG_PACKET_INTERVAL_SECONDS ('{}'). Using default: {} seconds.",
                    val_str, default_log_interval_seconds
                );
                Some(Duration::from_secs(default_log_interval_seconds))
            }
        },
        Err(_) => Some(Duration::from_secs(default_log_interval_seconds)),
    };

    // Initialize last_periodic_log_time. If logging is enabled, set it to the past
    // so that the first eligible packet triggers a log if enough time has passed since theoretical epoch start.
    let mut last_periodic_log_time: Instant = if let Some(interval) = log_interval_duration {
        Instant::now()
            .checked_sub(interval)
            .unwrap_or_else(Instant::now)
    } else {
        Instant::now()
    };

    let runtime = Arc::new(Runtime::new().expect("Failed to create Tokio runtime"));

    let ps5_ip_address = match env::var("PS5_IP_ADDRESS") {
        Ok(val) => val,
        Err(_) => {
            error!("Error: The PS5_IP_ADDRESS environment variable must be set.");
            process::exit(1);
        }
    };

    let pulsar_service_url = match env::var("PULSAR_SERVICE_URL") {
        Ok(val) => val,
        Err(_) => {
            error!("Error: The PULSAR_SERVICE_URL environment variable must be set.");
            process::exit(1);
        }
    };

    let pulsar_topic = match env::var("PULSAR_TOPIC") {
        Ok(val) => val,
        Err(_) => {
            error!("Error: The PULSAR_TOPIC environment variable must be set.");
            process::exit(1);
        }
    };

    let pulsar_handler_runtime_clone = Arc::clone(&runtime);
    let pulsar_handler = match PulsarHandler::new(
        pulsar_service_url,
        pulsar_topic,
        pulsar_handler_runtime_clone,
    ) {
        Ok(handler) => handler,
        Err(e) => {
            error!("Failed to initialize PulsarHandler: {}", e);
            process::exit(1);
        }
    };

    let http_server_runtime_clone = Arc::clone(&runtime);
    http_server_runtime_clone.spawn(run_http_server(http_server_runtime_clone.clone()));

    info!(
        "Attempting to bind UDP telemetry socket to: {}",
        BIND_ADDRESS
    );
    let socket = match UdpSocket::bind(BIND_ADDRESS) {
        Ok(s) => {
            info!(
                "Successfully bound UDP telemetry socket to: {}",
                BIND_ADDRESS
            );
            s
        }
        Err(e) => {
            error!(
                "Failed to bind UDP telemetry socket {}: {}",
                BIND_ADDRESS, e
            );
            process::exit(1);
        }
    };

    // Set a read timeout on the socket
    if let Err(e) = socket.set_read_timeout(Some(Duration::from_secs(1))) {
        warn!(
            "Failed to set read timeout on UDP socket: {}. Proceeding with blocking socket.",
            e
        );
    } else {
        info!("UDP socket read timeout set to 1 second.");
    }

    // Enable broadcast reception
    if let Err(e) = socket.set_broadcast(true) {
        warn!(
            "Failed to enable broadcast on UDP socket: {}. May not receive broadcast packets.",
            e
        );
    } else {
        info!("UDP socket configured to receive broadcast packets.");
    }

    let destination_str = format!("{}:{}", ps5_ip_address, TELEMETRY_SERVER_PORT);
    info!("Target UDP telemetry server: {}", destination_str);
    let destination: SocketAddr = match destination_str.parse() {
        Ok(addr) => addr,
        Err(e) => {
            error!(
                "Invalid IP address or port format for PS5_IP_ADDRESS ('{}'): {}",
                ps5_ip_address, e
            );
            process::exit(1);
        }
    };

    info!("Sending initial UDP heartbeat to {}", destination);
    if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
        error!("Failed to send UDP heartbeat: {}. Will retry.", e);
    } else {
        info!("UDP heartbeat sent successfully.");
    }

    loop {
        // Send heartbeat to maintain connection
        if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
            error!("Failed to send UDP heartbeat: {}. Retrying...", e);
            thread::sleep(Duration::from_millis(100));
            continue;
        }

        let mut buf = [0u8; PACKET_SIZE];
        match socket.recv_from(&mut buf) {
            Ok((number_of_bytes, src_addr)) => {
                if number_of_bytes == PACKET_SIZE {
                    match Packet::try_from(&buf) {
                        Ok(packet) => {
                            info!(
                                "Received packet ID: {}, Flags: {:?}, Laps: {}, On Track: {}",
                                packet.packet_id,
                                packet.flags,
                                packet.laps_in_race,
                                packet
                                    .flags
                                    .map(|f| f.contains(PacketFlags::CarOnTrack))
                                    .unwrap_or(false)
                            );
                            pulsar_handler.try_send_packet(&packet);

                            if let Some(interval) = log_interval_duration {
                                if last_periodic_log_time.elapsed() >= interval {
                                    info!(
                                        "[Periodic Packet Detail] ID: {}, Lap: {}/{}, RPM: {:.0}, Speed: {:.1} m/s, Flags: {:?}",
                                        packet.packet_id,
                                        packet.lap_count,
                                        packet.laps_in_race,
                                        packet.engine_rpm,
                                        packet.meters_per_second,
                                        packet.flags
                                    );
                                    last_periodic_log_time = Instant::now();
                                }
                            }

                            if let Some(flags) = packet.flags {
                                let is_paused_or_loading = flags.intersects(
                                    PacketFlags::Paused | PacketFlags::LoadingOrProcessing,
                                );
                                let is_not_on_track_or_race_not_started = !flags
                                    .contains(PacketFlags::CarOnTrack)
                                    || packet.laps_in_race <= 0;

                                if is_paused_or_loading || is_not_on_track_or_race_not_started {
                                    let mut reasons = Vec::new();
                                    if flags.contains(PacketFlags::Paused) {
                                        reasons.push("Paused");
                                    }
                                    if flags.contains(PacketFlags::LoadingOrProcessing) {
                                        reasons.push("LoadingOrProcessing");
                                    }
                                    if !flags.contains(PacketFlags::CarOnTrack) {
                                        reasons.push("NotOnTrack");
                                    }
                                    if packet.laps_in_race <= 0 {
                                        reasons.push("RaceNotStarted");
                                    }
                                    if !reasons.is_empty() {
                                        info!(
                                            "Main: Packet {} conditions not met for Pulsar send ({}). Flags: {:?}, Laps: {}",
                                            packet.packet_id,
                                            reasons.join(", "),
                                            flags,
                                            packet.laps_in_race
                                        );
                                    }
                                }
                            }
                        }
                        Err(e) => match e {
                            ParsePacketError::InvalidMagicValue(val) => {
                                error!(
                                    "Packet parse error from {}: Invalid magic value {:#X}",
                                    src_addr, val
                                );
                            }
                            ParsePacketError::ReadError(io_err) => {
                                error!(
                                    "Packet parse error from {}: Read error: {}",
                                    src_addr, io_err
                                );
                            }
                            ParsePacketError::DecryptionError(dec_err) => {
                                error!(
                                    "Packet parse error from {}: Decryption error: {}",
                                    src_addr, dec_err
                                );
                            }
                        },
                    }
                } else {
                    warn!(
                        "Received packet of unexpected size: {} bytes from {}",
                        number_of_bytes, src_addr
                    );
                }
            }
            Err(e) => {
                // This will now also catch timeout errors if set_read_timeout was successful
                if e.kind() == std::io::ErrorKind::WouldBlock
                    || e.kind() == std::io::ErrorKind::TimedOut
                {
                    debug!("Socket receive timeout. Retrying..."); // Log timeouts at debug level
                } else {
                    error!("Failed to receive packet: {}. Retrying...", e);
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::{
        body::Body,
        http::{Request, StatusCode},
    };
    use http_body_util::BodyExt;
    use tower::util::ServiceExt;

    // Helper function to create the app for testing
    fn test_app() -> Router {
        Router::new().route("/healthz", get(health_check_handler))
    }

    #[tokio::test]
    async fn health_check_works() {
        let app = test_app();

        // `Router` implements `tower::Service<Request<Body>>` so we can
        // call it like any tower service, no need to run an HTTP server.
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/healthz")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);

        let body = response.into_body().collect().await.unwrap().to_bytes();
        assert_eq!(&body[..], b"OK");
    }
}
