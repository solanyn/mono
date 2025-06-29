use std::time::{Duration, Instant};

/// Heartbeat manager to handle GT7 connection timing requirements
pub struct HeartbeatManager {
    last_heartbeat_time: Instant,
    heartbeat_interval: Duration,
}

impl HeartbeatManager {
    pub fn new(heartbeat_interval: Duration) -> Self {
        Self {
            last_heartbeat_time: Instant::now(),
            heartbeat_interval,
        }
    }

    /// Check if a heartbeat is needed based on the configured interval
    pub fn is_heartbeat_needed(&self) -> bool {
        self.last_heartbeat_time.elapsed() >= self.heartbeat_interval
    }

    /// Record that a heartbeat was sent
    pub fn record_heartbeat_sent(&mut self) {
        self.last_heartbeat_time = Instant::now();
    }

    /// Get time since last heartbeat
    pub fn time_since_last_heartbeat(&self) -> Duration {
        self.last_heartbeat_time.elapsed()
    }

    /// Get the configured heartbeat interval
    pub fn heartbeat_interval(&self) -> Duration {
        self.heartbeat_interval
    }

    /// Update the heartbeat interval (useful for configuration changes)
    pub fn update_interval(&mut self, new_interval: Duration) {
        self.heartbeat_interval = new_interval;
    }

    /// Calculate time until next heartbeat is needed
    pub fn time_until_next_heartbeat(&self) -> Duration {
        let elapsed = self.time_since_last_heartbeat();
        if elapsed >= self.heartbeat_interval {
            Duration::ZERO
        } else {
            self.heartbeat_interval - elapsed
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;
    use std::time::Duration;

    #[test]
    fn test_heartbeat_manager_creation() {
        let interval = Duration::from_secs(2);
        let manager = HeartbeatManager::new(interval);

        assert_eq!(manager.heartbeat_interval(), interval);
        assert!(!manager.is_heartbeat_needed()); // Should not need heartbeat immediately
    }

    #[test]
    fn test_heartbeat_needed_after_interval() {
        let interval = Duration::from_millis(10);
        let manager = HeartbeatManager::new(interval);

        // Should not need heartbeat immediately
        assert!(!manager.is_heartbeat_needed());

        // Wait longer than the interval
        thread::sleep(Duration::from_millis(15));

        // Should now need a heartbeat
        assert!(manager.is_heartbeat_needed());
    }

    #[test]
    fn test_record_heartbeat_sent() {
        let interval = Duration::from_millis(50);
        let mut manager = HeartbeatManager::new(interval);

        // Wait a bit then record heartbeat
        thread::sleep(Duration::from_millis(10));
        manager.record_heartbeat_sent();

        // Should not need heartbeat immediately after recording
        assert!(!manager.is_heartbeat_needed());

        // Wait for interval and should need heartbeat again
        thread::sleep(Duration::from_millis(55));
        assert!(manager.is_heartbeat_needed());
    }

    #[test]
    fn test_time_since_last_heartbeat() {
        let interval = Duration::from_millis(100);
        let manager = HeartbeatManager::new(interval);

        // Should be very close to zero initially
        let initial_time = manager.time_since_last_heartbeat();
        assert!(initial_time < Duration::from_millis(5));

        // Wait and check again
        thread::sleep(Duration::from_millis(20));
        let later_time = manager.time_since_last_heartbeat();
        assert!(later_time >= Duration::from_millis(15));
        assert!(later_time < Duration::from_millis(50));
    }

    #[test]
    fn test_update_interval() {
        let initial_interval = Duration::from_secs(1);
        let new_interval = Duration::from_secs(2);
        let mut manager = HeartbeatManager::new(initial_interval);

        assert_eq!(manager.heartbeat_interval(), initial_interval);

        manager.update_interval(new_interval);
        assert_eq!(manager.heartbeat_interval(), new_interval);
    }

    #[test]
    fn test_time_until_next_heartbeat() {
        let interval = Duration::from_millis(100);
        let manager = HeartbeatManager::new(interval);

        // Initially should be close to the full interval
        let time_until = manager.time_until_next_heartbeat();
        assert!(time_until <= interval);
        assert!(time_until >= Duration::from_millis(95)); // Allow for small timing variations

        // After the interval passes, should be zero
        thread::sleep(Duration::from_millis(105));
        let time_until_after = manager.time_until_next_heartbeat();
        assert_eq!(time_until_after, Duration::ZERO);
    }

    #[test]
    fn test_heartbeat_timing_precision() {
        // Test with GT7's actual recommended interval
        let gt7_interval = Duration::from_secs_f64(1.6);
        let mut manager = HeartbeatManager::new(gt7_interval);

        // Should not need heartbeat initially
        assert!(!manager.is_heartbeat_needed());

        // Simulate sending heartbeat
        manager.record_heartbeat_sent();

        // Should still not need heartbeat
        assert!(!manager.is_heartbeat_needed());

        // The manager should correctly handle fractional seconds
        assert_eq!(manager.heartbeat_interval(), gt7_interval);
    }

    #[test]
    fn test_heartbeat_boundary_conditions() {
        let interval = Duration::from_millis(100);
        let mut manager = HeartbeatManager::new(interval);

        // Record heartbeat, then test well before the boundary
        manager.record_heartbeat_sent();
        thread::sleep(Duration::from_millis(50));
        assert!(!manager.is_heartbeat_needed());

        thread::sleep(Duration::from_millis(60)); // Now at ~110ms, well past boundary
        assert!(manager.is_heartbeat_needed());
    }

    #[test]
    fn test_zero_interval_edge_case() {
        let zero_interval = Duration::ZERO;
        let manager = HeartbeatManager::new(zero_interval);

        // With zero interval, should always need heartbeat
        assert!(manager.is_heartbeat_needed());

        let time_until = manager.time_until_next_heartbeat();
        assert_eq!(time_until, Duration::ZERO);
    }

    #[test]
    fn test_very_long_interval() {
        let long_interval = Duration::from_secs(3600); // 1 hour
        let manager = HeartbeatManager::new(long_interval);

        // Should not need heartbeat for a very long time
        assert!(!manager.is_heartbeat_needed());

        let time_until = manager.time_until_next_heartbeat();
        assert!(time_until > Duration::from_secs(3599));
    }
}
