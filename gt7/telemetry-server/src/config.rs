use std::env;
use std::time::Duration;
use log::info;

const DEFAULT_HTTP_PORT: &str = "8080";
const DEFAULT_HEARTBEAT_INTERVAL: f64 = 1.6;
const DEFAULT_LOG_INTERVAL: u64 = 5;
const TELEMETRY_SERVER_PORT: u16 = 33739;
const BIND_ADDRESS: &str = "0.0.0.0:33740";

#[derive(Debug, Clone)]
pub struct Config {
    pub ps5_ip_address: String,
    pub pulsar_service_url: String,
    pub pulsar_topic: String,
    pub http_port: String,
    pub udp_bind_address: String,
    pub telemetry_server_port: u16,
    pub heartbeat_interval_seconds: f64,
    pub log_packet_interval_duration: Option<Duration>,
}

impl Config {
    pub fn load() -> Result<Self, String> {
        let ps5_ip_address = env::var("PS5_IP_ADDRESS")
            .map_err(|_| "PS5_IP_ADDRESS environment variable must be set".to_string())?;

        let pulsar_service_url = env::var("PULSAR_SERVICE_URL")
            .map_err(|_| "PULSAR_SERVICE_URL environment variable must be set".to_string())?;

        let pulsar_topic = env::var("PULSAR_TOPIC")
            .map_err(|_| "PULSAR_TOPIC environment variable must be set".to_string())?;

        let http_port = env::var("HTTP_PORT").unwrap_or_else(|_| DEFAULT_HTTP_PORT.to_string());

        let heartbeat_interval_seconds = env::var("HEARTBEAT_INTERVAL_SECONDS")
            .unwrap_or_else(|_| DEFAULT_HEARTBEAT_INTERVAL.to_string())
            .parse::<f64>()
            .unwrap_or(DEFAULT_HEARTBEAT_INTERVAL);

        let log_packet_interval_duration = match env::var("LOG_PACKET_INTERVAL_SECONDS") {
            Ok(val_str) => match val_str.parse::<u64>() {
                Ok(0) => {
                    info!("Periodic packet detail logging is disabled (LOG_PACKET_INTERVAL_SECONDS=0).");
                    None
                }
                Ok(seconds) if seconds > 0 => {
                    info!("Periodic packet detail logging interval set to every {} seconds.", seconds);
                    Some(Duration::from_secs(seconds))
                }
                _ => {
                    info!("Invalid LOG_PACKET_INTERVAL_SECONDS ('{}'), using default: {} seconds.", val_str, DEFAULT_LOG_INTERVAL);
                    Some(Duration::from_secs(DEFAULT_LOG_INTERVAL))
                }
            },
            Err(_) => Some(Duration::from_secs(DEFAULT_LOG_INTERVAL)),
        };

        let config = Config {
            ps5_ip_address,
            pulsar_service_url,
            pulsar_topic,
            http_port,
            udp_bind_address: BIND_ADDRESS.to_string(),
            telemetry_server_port: TELEMETRY_SERVER_PORT,
            heartbeat_interval_seconds,
            log_packet_interval_duration,
        };

        config.log_configuration();
        Ok(config)
    }

    pub fn log_configuration(&self) {
        info!("=== GT7 Pulsar Bridge Configuration ===");
        info!("PS5_IP_ADDRESS: {}", self.ps5_ip_address);
        info!("PULSAR_SERVICE_URL: {}", self.pulsar_service_url);
        info!("PULSAR_TOPIC: {}", self.pulsar_topic);
        info!("HTTP_PORT: {}", self.http_port);
        info!("HEARTBEAT_INTERVAL_SECONDS: {}", self.heartbeat_interval_seconds);
        info!("UDP_BIND_ADDRESS: {}", self.udp_bind_address);
        info!("TELEMETRY_SERVER_PORT: {}", self.telemetry_server_port);
        info!("RUST_LOG: {}", env::var("RUST_LOG").unwrap_or_else(|_| "info (default)".to_string()));
        if let Some(duration) = self.log_packet_interval_duration {
            info!("LOG_PACKET_INTERVAL_SECONDS: {}", duration.as_secs());
        } else {
            info!("LOG_PACKET_INTERVAL_SECONDS: disabled");
        }
        info!("======================================");
    }

    pub fn target_address(&self) -> String {
        format!("{}:{}", self.ps5_ip_address, self.telemetry_server_port)
    }

    pub fn http_bind_address(&self) -> String {
        format!("0.0.0.0:{}", self.http_port)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_config_creation() {
        let config = Config {
            ps5_ip_address: "192.168.1.1".to_string(),
            pulsar_service_url: "pulsar://localhost:6650".to_string(),
            pulsar_topic: "test-topic".to_string(),
            http_port: "8080".to_string(),
            udp_bind_address: "0.0.0.0:33740".to_string(),
            telemetry_server_port: 33739,
            heartbeat_interval_seconds: 1.6,
            log_packet_interval_duration: Some(Duration::from_secs(5)),
        };

        assert_eq!(config.ps5_ip_address, "192.168.1.1");
        assert_eq!(config.target_address(), "192.168.1.1:33739");
        assert_eq!(config.http_bind_address(), "0.0.0.0:8080");
    }
}
