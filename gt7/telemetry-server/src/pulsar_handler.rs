use log::{error, info, warn};
use pulsar::{Producer, Pulsar, TokioExecutor};
use std::sync::Arc;
use tokio::runtime::Runtime;
use tokio::sync::mpsc;

use gt7_telemetry_server::flags::PacketFlags;
use gt7_telemetry_server::packet::Packet;

type PulsarMessagePayload = Vec<u8>;

async fn pulsar_worker_task(
    mut producer: Producer<TokioExecutor>,
    mut receiver: mpsc::Receiver<PulsarMessagePayload>,
) {
    info!("PulsarWorker: Task started.");
    while let Some(payload_bytes) = receiver.recv().await {
        let initial_future_output = producer.send_non_blocking(payload_bytes).await;
        match initial_future_output {
            Ok(broker_ack_future) => match broker_ack_future.await {
                Ok(send_receipt) => {
                    if let Some(actual_message_id) = &send_receipt.message_id {
                        info!(
                            "PulsarWorker: Sent payload to Pulsar. LedgerID: {}, EntryID: {}",
                            actual_message_id.ledger_id, actual_message_id.entry_id
                        );
                    } else {
                        info!(
                            "PulsarWorker: Sent payload (no MessageId in receipt, ProducerID: {}, SequenceID: {})",
                            send_receipt.producer_id, send_receipt.sequence_id
                        );
                    }
                }
                Err(e_ack) => {
                    error!("PulsarWorker: Pulsar send ack error: {}", e_ack);
                }
            },
            Err(e_initial_send) => {
                error!(
                    "PulsarWorker: Pulsar failed to initiate send (send_non_blocking future error): {}",
                    e_initial_send
                );
            }
        }
    }
    info!("PulsarWorker: Channel closed, task finishing.");
    if let Err(e) = producer.close().await {
        error!("PulsarWorker: Failed to close producer: {}", e);
    }
    info!("PulsarWorker: Producer closed.");
}

pub struct PulsarHandler {
    message_sender: mpsc::Sender<PulsarMessagePayload>,
}

impl PulsarHandler {
    pub fn new(service_url: String, topic: String, runtime: Arc<Runtime>) -> Result<Self, String> {
        let producer = runtime.block_on(async {
            Pulsar::builder(service_url, TokioExecutor)
                .build()
                .await
                .map_err(|e| format!("Could not connect to Pulsar: {}", e))?
                .producer()
                .with_topic(topic.clone())
                .with_name("gt7-telemetry-producer")
                .build()
                .await
                .map_err(|e| {
                    format!(
                        "Failed to create Pulsar producer for topic '{}': {}",
                        topic, e
                    )
                })
        })?;

        let (tx, rx) = mpsc::channel::<PulsarMessagePayload>(100);
        runtime.spawn(pulsar_worker_task(producer, rx));

        info!("PulsarHandler: Initialized successfully and worker task spawned.");
        Ok(PulsarHandler { message_sender: tx })
    }

    pub fn try_send_packet(&self, packet: &Packet) {
        if let Some(flags) = packet.flags {
            // Send packets unless paused, loading, or race hasn't started
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
                                    "PulsarHandler: Worker channel full. Packet (ID: {}) dropped.",
                                    packet.packet_id
                                );
                            }
                            Err(mpsc::error::TrySendError::Closed(_payload_bytes)) => {
                                error!(
                                    "PulsarHandler: Worker channel closed. Packet (ID: {}) dropped. Worker may have terminated.",
                                    packet.packet_id
                                );
                            }
                        }
                    }
                    Err(e) => {
                        error!(
                            "PulsarHandler: Failed to serialize packet (ID: {}) to JSON: {}",
                            packet.packet_id, e
                        );
                    }
                }
            }
        } else {
            warn!(
                "PulsarHandler: Packet (ID: {}) not processed; Flags field is None.",
                packet.packet_id
            );
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use gt7_pulsar_bridge::flags::PacketFlags;
    use gt7_pulsar_bridge::packet::Packet;
    use tokio::sync::mpsc;

    impl PulsarHandler {
        fn new_for_test(message_sender: mpsc::Sender<PulsarMessagePayload>) -> Self {
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);
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
