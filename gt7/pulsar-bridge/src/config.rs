use std::env;

#[derive(Debug, Clone)]
pub struct Config {
    pub ps5_ip_address: String,
    pub pulsar_service_url: String,
    pub pulsar_topic: String,
    pub http_bind_address: String,
    pub heartbeat_interval_seconds: f64,
    pub log_packet_interval_seconds: Option<u64>,
}

impl Config {
    pub fn from_env() -> Result<Self, String> {
        let ps5_ip_address = env::var("PS5_IP_ADDRESS")
            .map_err(|_| "PS5_IP_ADDRESS environment variable must be set".to_string())?;

        let pulsar_service_url = env::var("PULSAR_SERVICE_URL")
            .map_err(|_| "PULSAR_SERVICE_URL environment variable must be set".to_string())?;

        let pulsar_topic = env::var("PULSAR_TOPIC")
            .map_err(|_| "PULSAR_TOPIC environment variable must be set".to_string())?;

        let http_bind_address = env::var("HTTP_BIND_ADDRESS")
            .unwrap_or_else(|_| "0.0.0.0:8080".to_string());

        let heartbeat_interval_seconds = env::var("HEARTBEAT_INTERVAL_SECONDS")
            .unwrap_or_else(|_| "1.6".to_string())
            .parse::<f64>()
            .map_err(|_| "HEARTBEAT_INTERVAL_SECONDS must be a valid number".to_string())?;

        let log_packet_interval_seconds = match env::var("LOG_PACKET_INTERVAL_SECONDS") {
            Ok(val_str) => match val_str.parse::<u64>() {
                Ok(0) => None, // 0 means disabled
                Ok(seconds) => Some(seconds),
                Err(_) => Some(5), // Default to 5 seconds on parse error
            },
            Err(_) => Some(5), // Default to 5 seconds if not set
        };

        Ok(Config {
            ps5_ip_address,
            pulsar_service_url,
            pulsar_topic,
            http_bind_address,
            heartbeat_interval_seconds,
            log_packet_interval_seconds,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_config_creation() {
        // Test that Config struct can be created
        let config = Config {
            ps5_ip_address: "192.168.1.1".to_string(),
            pulsar_service_url: "pulsar://localhost:6650".to_string(),
            pulsar_topic: "test-topic".to_string(),
            http_bind_address: "0.0.0.0:8080".to_string(),
            heartbeat_interval_seconds: 1.6,
            log_packet_interval_seconds: Some(5),
        };

        assert_eq!(config.ps5_ip_address, "192.168.1.1");
        assert_eq!(config.heartbeat_interval_seconds, 1.6);
    }
}