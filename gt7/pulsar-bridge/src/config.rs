use std::{env, time::Duration};

/// Parse heartbeat interval from environment variable with validation
pub fn parse_heartbeat_interval() -> Duration {
    let default_heartbeat_interval_seconds: f64 = 1.6;
    
    match env::var("HEARTBEAT_INTERVAL_SECONDS") {
        Ok(val_str) => match val_str.parse::<f64>() {
            Ok(seconds) if seconds > 0.0 => {
                log::info!("Heartbeat interval set to {} seconds (from HEARTBEAT_INTERVAL_SECONDS).", seconds);
                Duration::from_secs_f64(seconds)
            }
            _ => {
                log::info!("Invalid HEARTBEAT_INTERVAL_SECONDS ('{}'). Using default: {} seconds.", val_str, default_heartbeat_interval_seconds);
                Duration::from_secs_f64(default_heartbeat_interval_seconds)
            }
        },
        Err(_) => {
            log::info!("HEARTBEAT_INTERVAL_SECONDS not set. Using default: {} seconds.", default_heartbeat_interval_seconds);
            Duration::from_secs_f64(default_heartbeat_interval_seconds)
        }
    }
}

/// Parse log packet interval from environment variable with validation
pub fn parse_log_packet_interval() -> Option<Duration> {
    let default_log_interval_seconds: u64 = 5;
    
    match env::var("LOG_PACKET_INTERVAL_SECONDS") {
        Ok(val_str) => match val_str.parse::<u64>() {
            Ok(0) => {
                log::info!("Periodic packet detail logging is disabled (LOG_PACKET_INTERVAL_SECONDS=0).");
                None
            }
            Ok(seconds) if seconds > 0 => {
                log::info!("Periodic packet detail logging interval set to every {} seconds (from LOG_PACKET_INTERVAL_SECONDS).", seconds);
                Some(Duration::from_secs(seconds))
            }
            _ => {
                log::info!("Invalid or non-positive LOG_PACKET_INTERVAL_SECONDS ('{}'). Using default: {} seconds.", val_str, default_log_interval_seconds);
                Some(Duration::from_secs(default_log_interval_seconds))
            }
        },
        Err(_) => {
            log::info!("LOG_PACKET_INTERVAL_SECONDS not set. Using default: {} seconds.", default_log_interval_seconds);
            Some(Duration::from_secs(default_log_interval_seconds))
        }
    }
}

/// Validate required environment variables
pub fn validate_required_env_vars() -> Result<(String, String, String), String> {
    let ps5_ip = env::var("PS5_IP_ADDRESS")
        .map_err(|_| "Error: The PS5_IP_ADDRESS environment variable must be set.".to_string())?;
    
    let pulsar_url = env::var("PULSAR_SERVICE_URL")
        .map_err(|_| "Error: The PULSAR_SERVICE_URL environment variable must be set.".to_string())?;
    
    let pulsar_topic = env::var("PULSAR_TOPIC")
        .map_err(|_| "Error: The PULSAR_TOPIC environment variable must be set.".to_string())?;
    
    Ok((ps5_ip, pulsar_url, pulsar_topic))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    // Helper to safely set and unset environment variables for testing
    struct EnvVar {
        key: String,
        original_value: Option<String>,
    }

    impl EnvVar {
        fn set(key: &str, value: &str) -> Self {
            let original_value = env::var(key).ok();
            env::set_var(key, value);
            Self {
                key: key.to_string(),
                original_value,
            }
        }

        fn unset(key: &str) -> Self {
            let original_value = env::var(key).ok();
            env::remove_var(key);
            Self {
                key: key.to_string(),
                original_value,
            }
        }
    }

    impl Drop for EnvVar {
        fn drop(&mut self) {
            match &self.original_value {
                Some(value) => env::set_var(&self.key, value),
                None => env::remove_var(&self.key),
            }
        }
    }

    #[test]
    fn test_parse_heartbeat_interval_default() {
        let _env = EnvVar::unset("HEARTBEAT_INTERVAL_SECONDS");
        let interval = parse_heartbeat_interval();
        assert_eq!(interval, Duration::from_secs_f64(1.6));
    }

    #[test]
    fn test_parse_heartbeat_interval_valid_values() {
        let test_cases = vec![
            ("1.0", Duration::from_secs_f64(1.0)),
            ("2.5", Duration::from_secs_f64(2.5)),
            ("0.1", Duration::from_secs_f64(0.1)),
            ("10", Duration::from_secs_f64(10.0)),
        ];

        for (input, expected) in test_cases {
            let _env = EnvVar::set("HEARTBEAT_INTERVAL_SECONDS", input);
            let interval = parse_heartbeat_interval();
            assert_eq!(interval, expected, "Failed for input: {}", input);
        }
    }

    #[test]
    fn test_parse_heartbeat_interval_invalid_values() {
        let invalid_inputs = vec![
            "0",      // Zero
            "-1.5",   // Negative
            "abc",    // Non-numeric
            "",       // Empty
            "1.5.5",  // Invalid format
        ];

        for input in invalid_inputs {
            let _env = EnvVar::set("HEARTBEAT_INTERVAL_SECONDS", input);
            let interval = parse_heartbeat_interval();
            assert_eq!(interval, Duration::from_secs_f64(1.6), "Should use default for invalid input: {}", input);
        }
    }

    #[test]
    fn test_parse_log_packet_interval_default() {
        let _env = EnvVar::unset("LOG_PACKET_INTERVAL_SECONDS");
        let interval = parse_log_packet_interval();
        assert_eq!(interval, Some(Duration::from_secs(5)));
    }

    #[test]
    fn test_parse_log_packet_interval_disabled() {
        let _env = EnvVar::set("LOG_PACKET_INTERVAL_SECONDS", "0");
        let interval = parse_log_packet_interval();
        assert_eq!(interval, None);
    }

    #[test]
    fn test_parse_log_packet_interval_valid_values() {
        let test_cases = vec![
            ("1", Some(Duration::from_secs(1))),
            ("10", Some(Duration::from_secs(10))),
            ("60", Some(Duration::from_secs(60))),
        ];

        for (input, expected) in test_cases {
            let _env = EnvVar::set("LOG_PACKET_INTERVAL_SECONDS", input);
            let interval = parse_log_packet_interval();
            assert_eq!(interval, expected, "Failed for input: {}", input);
        }
    }

    #[test]
    fn test_parse_log_packet_interval_invalid_values() {
        let invalid_inputs = vec![
            "-1",     // Negative
            "abc",    // Non-numeric
            "",       // Empty
            "1.5",    // Float (expects integer)
        ];

        for input in invalid_inputs {
            let _env = EnvVar::set("LOG_PACKET_INTERVAL_SECONDS", input);
            let interval = parse_log_packet_interval();
            assert_eq!(interval, Some(Duration::from_secs(5)), "Should use default for invalid input: {}", input);
        }
    }

    #[test]
    fn test_validate_required_env_vars_success() {
        let _ps5_env = EnvVar::set("PS5_IP_ADDRESS", "192.168.1.100");
        let _pulsar_url_env = EnvVar::set("PULSAR_SERVICE_URL", "pulsar://localhost:6650");
        let _pulsar_topic_env = EnvVar::set("PULSAR_TOPIC", "test-topic");

        let result = validate_required_env_vars();
        assert!(result.is_ok());
        
        let (ps5_ip, pulsar_url, pulsar_topic) = result.unwrap();
        assert_eq!(ps5_ip, "192.168.1.100");
        assert_eq!(pulsar_url, "pulsar://localhost:6650");
        assert_eq!(pulsar_topic, "test-topic");
    }

    #[test]
    fn test_validate_required_env_vars_missing_ps5_ip() {
        let _ps5_env = EnvVar::unset("PS5_IP_ADDRESS");
        let _pulsar_url_env = EnvVar::set("PULSAR_SERVICE_URL", "pulsar://localhost:6650");
        let _pulsar_topic_env = EnvVar::set("PULSAR_TOPIC", "test-topic");

        let result = validate_required_env_vars();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("PS5_IP_ADDRESS"));
    }

    #[test]
    fn test_validate_required_env_vars_missing_pulsar_url() {
        let _ps5_env = EnvVar::set("PS5_IP_ADDRESS", "192.168.1.100");
        let _pulsar_url_env = EnvVar::unset("PULSAR_SERVICE_URL");
        let _pulsar_topic_env = EnvVar::set("PULSAR_TOPIC", "test-topic");

        let result = validate_required_env_vars();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("PULSAR_SERVICE_URL"));
    }

    #[test]
    fn test_validate_required_env_vars_missing_pulsar_topic() {
        let _ps5_env = EnvVar::set("PS5_IP_ADDRESS", "192.168.1.100");
        let _pulsar_url_env = EnvVar::set("PULSAR_SERVICE_URL", "pulsar://localhost:6650");
        let _pulsar_topic_env = EnvVar::unset("PULSAR_TOPIC");

        let result = validate_required_env_vars();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("PULSAR_TOPIC"));
    }

    #[test]
    fn test_validate_required_env_vars_all_missing() {
        let _ps5_env = EnvVar::unset("PS5_IP_ADDRESS");
        let _pulsar_url_env = EnvVar::unset("PULSAR_SERVICE_URL");
        let _pulsar_topic_env = EnvVar::unset("PULSAR_TOPIC");

        let result = validate_required_env_vars();
        assert!(result.is_err());
        // Should fail on the first missing variable (PS5_IP_ADDRESS)
        assert!(result.unwrap_err().contains("PS5_IP_ADDRESS"));
    }
}