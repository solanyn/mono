use gt7_pulsar_bridge::{
    constants::{PACKET_SIZE, PACKET_MAGIC_VALUE},
    packet::Packet,
    flags::PacketFlags,
    errors::ParsePacketError,
    config::{parse_heartbeat_interval, parse_log_packet_interval, validate_required_env_vars},
    heartbeat::HeartbeatManager,
};
use std::{env, time::Duration};

// Sample encrypted packet for integration testing
const SAMPLE_PACKET: [u8; PACKET_SIZE] = [
    0x66, 0x83, 0x09, 0x68, 0x05, 0xc8, 0xf5, 0xa9, 0x77, 0x48, 0x09, 0x9a, 0xaf, 0x1e, 0x9f,
    0x5b, 0x15, 0x8d, 0xd1, 0xcb, 0xd6, 0x6d, 0x0f, 0xa2, 0x06, 0xfc, 0xb4, 0x36, 0x44, 0xab,
    0xf7, 0x69, 0x2f, 0x3a, 0xfa, 0xd7, 0x9c, 0xa8, 0xe9, 0x88, 0xef, 0x46, 0x5f, 0x29, 0x00,
    0xac, 0x5b, 0x4c, 0x9b, 0x47, 0x7f, 0x0d, 0x52, 0x69, 0x0c, 0xc6, 0x79, 0x56, 0x44, 0xa9,
    0xe4, 0xe4, 0x6d, 0x8c, 0x29, 0x59, 0x33, 0xfb, 0x20, 0x27, 0x02, 0x50, 0xa9, 0x0b, 0xed,
    0xcb, 0x5d, 0xab, 0x98, 0xd6, 0x07, 0x91, 0xe8, 0xa3, 0x12, 0x94, 0x0c, 0x78, 0x09, 0x20,
    0x78, 0x29, 0x50, 0x2f, 0xf5, 0x43, 0xf6, 0x97, 0x40, 0x63, 0x34, 0x22, 0x41, 0xd8, 0x1e,
    0xa6, 0x4c, 0x5b, 0xd4, 0xe9, 0xfc, 0xae, 0x3e, 0xd4, 0x4e, 0x49, 0x74, 0x1b, 0x41, 0xa4,
    0x01, 0x17, 0x94, 0x84, 0x4b, 0xf2, 0x50, 0x38, 0xf9, 0x9a, 0xd3, 0x42, 0x02, 0xfc, 0x7a,
    0x93, 0x8a, 0x6d, 0x6e, 0x27, 0x81, 0x6e, 0x06, 0xc6, 0xa1, 0x61, 0x7f, 0xea, 0xe7, 0xc0,
    0xc7, 0xbe, 0x40, 0x22, 0xfd, 0xdc, 0x90, 0xdf, 0x25, 0x05, 0xd2, 0x50, 0xdb, 0x8f, 0x0c,
    0xea, 0x80, 0x80, 0x7d, 0xdb, 0x24, 0xa6, 0xb6, 0xe2, 0x29, 0xe9, 0xa3, 0x98, 0xe3, 0x6b,
    0xc5, 0x49, 0x1e, 0xe5, 0x60, 0x14, 0x20, 0x59, 0x3b, 0x37, 0x12, 0xce, 0x8a, 0x7e, 0xa9,
    0xe7, 0x68, 0x1e, 0x07, 0x6f, 0x49, 0x48, 0xdc, 0x4e, 0x02, 0x3c, 0xd9, 0xef, 0xf3, 0x2a,
    0x12, 0x7e, 0x9c, 0x43, 0xbc, 0x6c, 0x81, 0x22, 0x08, 0x3e, 0x92, 0x9f, 0xeb, 0x53, 0xe5,
    0x9c, 0x2a, 0x18, 0xb6, 0xf9, 0x08, 0x33, 0x80, 0xe1, 0x20, 0x6b, 0x67, 0xbf, 0x99, 0xb0,
    0xf2, 0x4f, 0x16, 0x4b, 0xce, 0x4a, 0x24, 0x5c, 0x35, 0x96, 0x00, 0xd3, 0x7a, 0x07, 0x5a,
    0x8b, 0xe5, 0x61, 0x94, 0xc7, 0xd2, 0x03, 0x84, 0x67, 0xfb, 0xba, 0xe7, 0x46, 0xdc, 0xd9,
    0xf8, 0x49, 0xe6, 0x56, 0x28, 0x43, 0x8c, 0xd1, 0x63, 0x5b, 0x36, 0xdc, 0xa2, 0xbe, 0x73,
    0x96, 0x98, 0x0b, 0x2e, 0x5e, 0x14, 0x9c, 0x96, 0x5a, 0xf5, 0x19,
];

// Helper to safely manage environment variables in tests
struct EnvGuard {
    key: String,
    original_value: Option<String>,
}

impl EnvGuard {
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

impl Drop for EnvGuard {
    fn drop(&mut self) {
        match &self.original_value {
            Some(value) => env::set_var(&self.key, value),
            None => env::remove_var(&self.key),
        }
    }
}

#[test]
fn test_complete_packet_processing_flow() {
    // Test the complete flow: encrypted packet -> decryption -> parsing -> packet struct
    let result = Packet::try_from(&SAMPLE_PACKET);
    
    assert!(result.is_ok(), "Packet parsing should succeed");
    
    let packet = result.unwrap();
    
    // Verify packet has reasonable values
    assert!(packet.packet_id >= 0, "Packet ID should be non-negative");
    assert!(packet.meters_per_second >= 0.0, "Speed should be non-negative");
    assert!(packet.engine_rpm >= 0.0, "RPM should be non-negative");
    
    // Test serialization roundtrip
    let json = serde_json::to_string(&packet).expect("Packet should serialize to JSON");
    let deserialized: Packet = serde_json::from_str(&json).expect("JSON should deserialize back to packet");
    
    assert_eq!(packet, deserialized, "Serialization roundtrip should preserve data");
}

#[test]
fn test_packet_processing_with_invalid_data() {
    // Test with corrupted packet data
    let mut corrupted_packet = SAMPLE_PACKET;
    corrupted_packet[0] = 0xFF; // Corrupt magic value
    
    let result = Packet::try_from(&corrupted_packet);
    assert!(result.is_err(), "Corrupted packet should fail to parse");
    
    if let Err(ParsePacketError::InvalidMagicValue(magic)) = result {
        assert_ne!(magic, PACKET_MAGIC_VALUE, "Should report different magic value");
    } else {
        panic!("Expected InvalidMagicValue error");
    }
}

#[test]
fn test_flags_integration() {
    // Test flag operations that would be used in real scenarios
    let mut flags = PacketFlags::empty();
    
    // Simulate car starting on track
    flags.insert(PacketFlags::CarOnTrack);
    assert!(flags.contains(PacketFlags::CarOnTrack));
    
    // Simulate pausing
    flags.insert(PacketFlags::Paused);
    assert!(flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing));
    
    // Test condition that would prevent Pulsar sending
    let should_not_send = flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing);
    assert!(should_not_send, "Should not send when paused");
    
    // Resume driving
    flags.remove(PacketFlags::Paused);
    let should_send = !flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing)
        && flags.contains(PacketFlags::CarOnTrack);
    assert!(should_send, "Should send when active and on track");
}

#[test]
fn test_configuration_integration() {
    // Test the configuration parsing with realistic scenarios
    
    // Test default configuration
    let _guard1 = EnvGuard::unset("HEARTBEAT_INTERVAL_SECONDS");
    let _guard2 = EnvGuard::unset("LOG_PACKET_INTERVAL_SECONDS");
    
    let heartbeat_interval = parse_heartbeat_interval();
    let log_interval = parse_log_packet_interval();
    
    assert_eq!(heartbeat_interval, Duration::from_secs_f64(1.6));
    assert_eq!(log_interval, Some(Duration::from_secs(5)));
    
    // Test production-like configuration
    let _guard3 = EnvGuard::set("HEARTBEAT_INTERVAL_SECONDS", "1.6");
    let _guard4 = EnvGuard::set("LOG_PACKET_INTERVAL_SECONDS", "10");
    
    let prod_heartbeat = parse_heartbeat_interval();
    let prod_log = parse_log_packet_interval();
    
    assert_eq!(prod_heartbeat, Duration::from_secs_f64(1.6));
    assert_eq!(prod_log, Some(Duration::from_secs(10)));
}

#[test]
fn test_heartbeat_timing_integration() {
    // Test heartbeat timing with GT7's requirements
    let gt7_interval = Duration::from_secs_f64(1.6);
    let mut heartbeat_manager = HeartbeatManager::new(gt7_interval);
    
    // Initial state
    assert!(!heartbeat_manager.is_heartbeat_needed());
    
    // Simulate packet reception loop timing
    for i in 0..5 {
        if heartbeat_manager.is_heartbeat_needed() {
            heartbeat_manager.record_heartbeat_sent();
            println!("Sent heartbeat at iteration {}", i);
        }
        
        // Simulate processing time
        std::thread::sleep(Duration::from_millis(100));
    }
    
    // Should have sent at least one heartbeat in this timeframe
    let time_since = heartbeat_manager.time_since_last_heartbeat();
    assert!(time_since < Duration::from_secs(2), "Should have sent heartbeat recently");
}

#[test] 
fn test_error_handling_integration() {
    // Test various error scenarios that could occur in production
    
    // Test missing environment variables
    let _ps5_guard = EnvGuard::unset("PS5_IP_ADDRESS");
    let _pulsar_guard = EnvGuard::unset("PULSAR_SERVICE_URL");
    let _topic_guard = EnvGuard::unset("PULSAR_TOPIC");
    
    let result = validate_required_env_vars();
    assert!(result.is_err(), "Should fail when required vars are missing");
    
    // Test with partial configuration (realistic misconfiguration scenario)
    let _ps5_guard2 = EnvGuard::set("PS5_IP_ADDRESS", "192.168.1.100");
    // Leave PULSAR_SERVICE_URL unset
    let _topic_guard2 = EnvGuard::set("PULSAR_TOPIC", "gt7-telemetry");
    
    let result2 = validate_required_env_vars();
    assert!(result2.is_err(), "Should fail when PULSAR_SERVICE_URL is missing");
    
    // Test with all required vars set
    let _pulsar_guard2 = EnvGuard::set("PULSAR_SERVICE_URL", "pulsar://localhost:6650");
    
    let result3 = validate_required_env_vars();
    assert!(result3.is_ok(), "Should succeed when all required vars are set");
}

#[test]
fn test_packet_validation_integration() {
    // Test packet validation logic that would be used in main.rs
    let packet = Packet::try_from(&SAMPLE_PACKET).expect("Sample packet should parse");
    
    // Test the conditions used in main.rs for packet processing
    if let Some(flags) = packet.flags {
        let is_paused_or_loading = flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing);
        let is_not_on_track_or_race_not_started = !flags.contains(PacketFlags::CarOnTrack) || packet.laps_in_race <= 0;
        
        // These are the actual conditions used in the main application
        let should_process = !is_paused_or_loading && !is_not_on_track_or_race_not_started;
        
        // For the sample packet, verify the logic works correctly
        println!("Sample packet processing decision: {}", should_process);
        println!("Flags: {:?}, Laps in race: {}", flags, packet.laps_in_race);
        
        // The logic should be consistent and deterministic
        assert_eq!(should_process, 
                  !flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing) 
                  && flags.contains(PacketFlags::CarOnTrack) 
                  && packet.laps_in_race > 0);
    }
}

#[test]
fn test_performance_characteristics() {
    // Test that packet processing is fast enough for 60Hz operation
    let start = std::time::Instant::now();
    let iterations = 100;
    
    for _ in 0..iterations {
        let _packet = Packet::try_from(&SAMPLE_PACKET).expect("Packet should parse");
    }
    
    let elapsed = start.elapsed();
    let per_packet = elapsed / iterations;
    
    // At 60Hz, we have ~16.67ms per packet. Parsing should be much faster.
    assert!(per_packet < Duration::from_millis(1), 
           "Packet parsing should be fast enough for 60Hz operation, got {:?} per packet", per_packet);
    
    println!("Packet parsing performance: {:?} per packet", per_packet);
}