use kafka::producer::{Producer, Record, RequiredAcks};
use log::{error, info, warn};
use std::sync::Arc;
use std::time::Duration;
use tokio::runtime::Runtime;
use tokio::sync::mpsc;

use gt7_telemetry_core::flags::PacketFlags;
use gt7_telemetry_core::packet::Packet;

type KafkaMessagePayload = Vec<u8>;

fn kafka_worker_task(
    mut producer: Producer,
    topic: String,
    mut receiver: mpsc::Receiver<KafkaMessagePayload>,
    runtime: Arc<Runtime>,
) {
    info!("KafkaWorker: Task started.");
    runtime.spawn(async move {
        while let Some(payload_bytes) = receiver.recv().await {
            let record = Record::from_key_value(&topic, "gt7-telemetry", payload_bytes);
            
            match producer.send(&record) {
                Ok(_) => {
                    info!("KafkaWorker: Sent payload to Kafka topic: {}", topic);
                }
                Err(e) => {
                    error!("KafkaWorker: Kafka send error: {}", e);
                }
            }
        }
        info!("KafkaWorker: Channel closed, task finishing.");
    });
}

pub struct KafkaHandler {
    message_sender: mpsc::Sender<KafkaMessagePayload>,
}

impl KafkaHandler {
    pub fn new(bootstrap_servers: String, topic: String, runtime: Arc<Runtime>) -> Result<Self, String> {
        let producer = Producer::from_hosts(vec![bootstrap_servers])
            .with_ack_timeout(Duration::from_secs(1))
            .with_required_acks(RequiredAcks::One)
            .create()
            .map_err(|e| format!("Could not create Kafka producer: {}", e))?;

        let (tx, rx) = mpsc::channel::<KafkaMessagePayload>(100);
        kafka_worker_task(producer, topic, rx, runtime);

        info!("KafkaHandler: Initialized successfully and worker task spawned.");
        Ok(KafkaHandler { message_sender: tx })
    }

    pub fn try_send_packet(&self, packet: &Packet) {
        if let Some(flags) = packet.flags {
            if !flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing)
                && packet.laps_in_race > 0
            {
                match serde_json::to_string(packet) {
                    Ok(json_payload) => {
                        let payload_bytes = json_payload.into_bytes();
                        match self.message_sender.try_send(payload_bytes) {
                            Ok(_) => {}
                            Err(mpsc::error::TrySendError::Full(_payload_bytes)) => {
                                warn!(
                                    "KafkaHandler: Worker channel full. Packet (ID: {}) dropped.",
                                    packet.packet_id
                                );
                            }
                            Err(mpsc::error::TrySendError::Closed(_payload_bytes)) => {
                                error!(
                                    "KafkaHandler: Worker channel closed. Packet (ID: {}) dropped. Worker may have terminated.",
                                    packet.packet_id
                                );
                            }
                        }
                    }
                    Err(e) => {
                        error!(
                            "KafkaHandler: Failed to serialize packet (ID: {}) to JSON: {}",
                            packet.packet_id, e
                        );
                    }
                }
            }
        } else {
            warn!(
                "KafkaHandler: Packet (ID: {}) not processed; Flags field is None.",
                packet.packet_id
            );
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use gt7_telemetry_core::flags::PacketFlags;
    use gt7_telemetry_core::packet::Packet;
    use tokio::sync::mpsc;

    impl KafkaHandler {
        fn new_for_test(message_sender: mpsc::Sender<KafkaMessagePayload>) -> Self {
            Self { message_sender }
        }
    }

    fn base_test_packet() -> Packet {
        Packet {
            packet_id: 0,
            flags: Some(PacketFlags::empty()),
            laps_in_race: 0,
            ..Default::default()
        }
    }

    #[tokio::test]
    async fn try_send_packet_sends_when_conditions_met() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.packet_id = 1;
        packet.flags = Some(PacketFlags::CarOnTrack);
        packet.laps_in_race = 1;
        handler.try_send_packet(&packet);
        let received_payload = rx.recv().await;
        assert!(received_payload.is_some(), "Expected a message to be sent");
        if let Some(payload_bytes) = received_payload {
            let sent_packet_json = serde_json::to_vec(&packet).unwrap();
            assert_eq!(
                payload_bytes, sent_packet_json,
                "Payload should match serialized original packet"
            );
        }
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_paused() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::CarOnTrack | PacketFlags::Paused);
        packet.laps_in_race = 1;
        handler.try_send_packet(&packet);
        assert!(
            rx.try_recv().is_err(),
            "Expected no message when packet is paused"
        );
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_loading() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::CarOnTrack | PacketFlags::LoadingOrProcessing);
        packet.laps_in_race = 1;
        handler.try_send_packet(&packet);
        assert!(
            rx.try_recv().is_err(),
            "Expected no message when packet is loading"
        );
    }

    #[tokio::test]
    async fn try_send_packet_sends_when_not_on_track() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::empty()); // Not on track
        packet.laps_in_race = 1;
        handler.try_send_packet(&packet);
        assert!(
            rx.try_recv().is_ok(),
            "Expected message to be sent even when not on track"
        );
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_laps_in_race_is_zero() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::CarOnTrack);
        packet.laps_in_race = 0;
        handler.try_send_packet(&packet);
        assert!(
            rx.try_recv().is_err(),
            "Expected no message when laps_in_race is 0"
        );
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_flags_is_none() {
        let (tx, mut rx) = mpsc::channel::<KafkaMessagePayload>(10);
        let handler = KafkaHandler::new_for_test(tx);
        let mut packet = base_test_packet();
        packet.flags = None;
        packet.laps_in_race = 1;
        handler.try_send_packet(&packet);
        assert!(
            rx.try_recv().is_err(),
            "Expected no message when flags is None"
        );
    }
}