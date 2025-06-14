use pulsar::{Pulsar, TokioExecutor, Producer};
use tokio::runtime::Runtime;
use tokio::sync::mpsc;
use log::{info, error, warn};
use serde_json;

use gt7::packet::Packet;
use gt7::flags::PacketFlags;

// Type of message sent over the channel to the worker
type PulsarMessagePayload = Vec<u8>;

// The dedicated worker task that owns the Producer
async fn pulsar_worker_task(
    mut producer: Producer<TokioExecutor>, // Producer is owned by this task
    mut receiver: mpsc::Receiver<PulsarMessagePayload>,
) {
    info!("PulsarWorker: Task started.");
    while let Some(payload_bytes) = receiver.recv().await {
        // Using send_non_blocking, matching the example's presumed success with owned producer
        let initial_future_output = producer.send_non_blocking(payload_bytes).await; // First await
        match initial_future_output { // This is Result<SendFutureActual, PulsarError>
            Ok(broker_ack_future) => { // broker_ack_future is the actual SendFuture for broker ack
                match broker_ack_future.await { // Second await
                    Ok(send_receipt) => { // send_receipt is CommandSendReceipt
                        if let Some(actual_message_id) = &send_receipt.message_id {
                            info!("PulsarWorker: Sent payload to Pulsar. LedgerID: {}, EntryID: {}",
                                  actual_message_id.ledger_id, actual_message_id.entry_id);
                        } else {
                            // This case might occur if message deduplication is involved or other specific scenarios
                            info!("PulsarWorker: Sent payload (no MessageId in receipt, ProducerID: {}, SequenceID: {})",
                                  send_receipt.producer_id, send_receipt.sequence_id);
                        }
                    }
                    Err(e_ack) => {
                        error!("PulsarWorker: Pulsar send ack error: {}", e_ack);
                    }
                }
            }
            Err(e_initial_send) => {
                error!("PulsarWorker: Pulsar failed to initiate send (send_non_blocking future error): {}", e_initial_send);
            }
        }
    }
    info!("PulsarWorker: Channel closed, task finishing.");
    if let Err(e) = producer.close().await { // Gracefully close producer
        error!("PulsarWorker: Failed to close producer: {}", e);
    }
    info!("PulsarWorker: Producer closed.");
}

pub struct PulsarHandler {
    message_sender: mpsc::Sender<PulsarMessagePayload>,
}

impl PulsarHandler {
    pub fn new(service_url: String, topic: String) -> Result<Self, String> {
        let runtime = Runtime::new().map_err(|e| format!("Failed to create Tokio V1 runtime: {}", e))?;
        
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
                .map_err(|e| format!("Failed to create Pulsar producer for topic '{}': {}", topic, e))
        })?;

        let (tx, rx) = mpsc::channel::<PulsarMessagePayload>(100);

        runtime.spawn(pulsar_worker_task(producer, rx));

        info!("PulsarHandler: Initialized successfully and worker task spawned.");
        Ok(PulsarHandler {
            message_sender: tx,
        })
    }

    // This method remains synchronous from the caller's perspective.
    pub fn try_send_packet(&self, packet: &Packet) {
        if let Some(flags) = packet.flags {
            // Conditional logic for sending (same as before)
            if !flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing) &&
               flags.contains(PacketFlags::CarOnTrack) &&
               packet.laps_in_race > 0 {
                match serde_json::to_string(packet) {
                    Ok(json_payload) => {
                        let payload_bytes = json_payload.into_bytes();
                        
                        // Use try_send for a non-blocking attempt to queue the message.
                        // This avoids making try_send_packet async and doesn't block the caller
                        // if the channel is full (it will drop the message instead).
                        match self.message_sender.try_send(payload_bytes) {
                            Ok(_) => {
                                // Optionally log success, but can be too verbose.
                                // info!("PulsarHandler: Packet (ID: {}) queued for sending.", packet.packet_id);
                            }
                            Err(mpsc::error::TrySendError::Full(_payload_bytes)) => {
                                warn!("PulsarHandler: Worker channel full. Packet (ID: {}) dropped.", packet.packet_id);
                            }
                            Err(mpsc::error::TrySendError::Closed(_payload_bytes)) => {
                                // This implies the worker task has panicked or shut down.
                                error!("PulsarHandler: Worker channel closed. Packet (ID: {}) dropped. Worker may have terminated.", packet.packet_id);
                            }
                        }
                    }
                    Err(e) => {
                        error!("PulsarHandler: Failed to serialize packet (ID: {}) to JSON: {}", packet.packet_id, e);
                    }
                }
            }
            // No explicit "else" logging here; main.rs can log if conditions aren't met.
        } else {
            warn!("PulsarHandler: Packet (ID: {}) not processed; Flags field is None.", packet.packet_id);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*; // Imports PulsarHandler, PulsarMessagePayload from parent module
    use tokio::sync::mpsc;
    // Packet and PacketFlags are part of the gt7 library crate
    use gt7::packet::Packet;
    use gt7::flags::PacketFlags;

    // Test-only constructor for PulsarHandler to inject the MPSC sender
    impl PulsarHandler {
        fn new_for_test(message_sender: mpsc::Sender<PulsarMessagePayload>) -> Self {
            Self { message_sender }
        }
    }

    // Helper to create a base packet for tests.
    // It uses Packet::default() which should be available from your gt7::packet module.
    fn base_test_packet() -> Packet {
        Packet {
            // Ensure packet_id is distinct for better test debugging if needed
            packet_id: 0, 
            flags: Some(PacketFlags::empty()),
            laps_in_race: 0,
            ..Default::default() // Fills other fields from Packet's Default impl
        }
    }

    #[tokio::test]
    async fn try_send_packet_sends_when_conditions_met() {
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);

        let mut packet = base_test_packet();
        packet.packet_id = 1;
        packet.flags = Some(PacketFlags::CarOnTrack); // Not paused, not loading
        packet.laps_in_race = 1; // Race is active

        handler.try_send_packet(&packet);

        let received_payload = rx.recv().await;
        assert!(received_payload.is_some(), "Expected a message to be sent");

        if let Some(payload_bytes) = received_payload {
            // Verify the payload can be deserialized back to the packet sent
            let sent_packet_json = serde_json::to_vec(&packet).unwrap();
            assert_eq!(payload_bytes, sent_packet_json, "Payload should match serialized original packet");
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
        // try_recv is non-blocking, suitable for checking emptiness after a sync call
        assert!(rx.try_recv().is_err(), "Expected no message when packet is paused");
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_loading() {
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);

        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::CarOnTrack | PacketFlags::LoadingOrProcessing);
        packet.laps_in_race = 1;

        handler.try_send_packet(&packet);
        assert!(rx.try_recv().is_err(), "Expected no message when packet is loading");
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_not_on_track() {
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);

        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::empty()); // CarOnTrack is not set
        packet.laps_in_race = 1;

        handler.try_send_packet(&packet);
        assert!(rx.try_recv().is_err(), "Expected no message when not on track");
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_laps_in_race_is_zero() {
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);

        let mut packet = base_test_packet();
        packet.flags = Some(PacketFlags::CarOnTrack);
        packet.laps_in_race = 0; // Race not active

        handler.try_send_packet(&packet);
        assert!(rx.try_recv().is_err(), "Expected no message when laps_in_race is 0");
    }

    #[tokio::test]
    async fn try_send_packet_does_not_send_when_flags_is_none() {
        let (tx, mut rx) = mpsc::channel::<PulsarMessagePayload>(10);
        let handler = PulsarHandler::new_for_test(tx);

        let mut packet = base_test_packet();
        packet.flags = None; // Flags struct is not present
        packet.laps_in_race = 1;

        handler.try_send_packet(&packet);
        assert!(rx.try_recv().is_err(), "Expected no message when flags is None");
    }
}
