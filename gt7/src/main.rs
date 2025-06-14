use gt7::{
    constants::{PACKET_HEARTBEAT_DATA, PACKET_SIZE},
    errors::ParsePacketError,
    packet::Packet,
    flags::PacketFlags,
};
use std::net::{SocketAddr, UdpSocket};
use std::process;
use std::env;
use log::{info, warn, error};
use env_logger::Env;

mod pulsar_handler;
use pulsar_handler::PulsarHandler;

const TELEMETRY_SERVER_PORT: u16 = 33739;
const HEARTBEAT_INTERVAL_PACKETS: i32 = 100;
const BIND_ADDRESS: &str = "0.0.0.0:33740";

fn main() {
    env_logger::Builder::from_env(Env::default().default_filter_or("info")).init();

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

    let pulsar_handler = match PulsarHandler::new(pulsar_service_url, pulsar_topic) {
        Ok(handler) => handler,
        Err(e) => {
            error!("Failed to initialize PulsarHandler: {}", e);
            process::exit(1);
        }
    };

    info!("Attempting to bind to socket: {}", BIND_ADDRESS);
    let socket = match UdpSocket::bind(BIND_ADDRESS) {
        Ok(s) => {
            info!("Successfully bound to socket: {}", BIND_ADDRESS);
            s
        }
        Err(e) => {
            error!("Failed to bind socket {}: {}", BIND_ADDRESS, e);
            process::exit(1);
        }
    };

    let destination_str = format!("{}:{}", ps5_ip_address, TELEMETRY_SERVER_PORT);
    info!("Target telemetry server: {}", destination_str);
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

    info!("Sending initial heartbeat to {}", destination);
    if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
        error!("Failed to send initial heartbeat: {}", e);
        process::exit(1);
    }
    info!("Initial heartbeat sent.");

    loop {
        if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
            error!(
                "Failed to send heartbeat/request packet: {}. Continuing...",
                e
            );
            continue;
        }

        let mut buf = [0u8; PACKET_SIZE];
        match socket.recv_from(&mut buf) {
            Ok((number_of_bytes, src_addr)) => {
                if number_of_bytes == PACKET_SIZE {
                    match Packet::try_from(&buf) {
                        Ok(packet) => {
                            info!(
                                "Packet ID: {}, Lap: {}/{}, RPM: {:.0}, Speed: {:.1} m/s, Flags: {:?}",
                                packet.packet_id,
                                packet.lap_count,
                                packet.laps_in_race,
                                packet.engine_rpm,
                                packet.meters_per_second,
                                packet.flags
                            );

                            pulsar_handler.try_send_packet(&packet);

                            if let Some(flags) = packet.flags {
                                let is_paused_or_loading = flags.intersects(PacketFlags::Paused | PacketFlags::LoadingOrProcessing);
                                let is_not_on_track_or_race_not_started = !flags.contains(PacketFlags::CarOnTrack) || packet.laps_in_race <= 0;

                                if is_paused_or_loading || is_not_on_track_or_race_not_started {
                                    let mut reasons = Vec::new();
                                    if flags.contains(PacketFlags::Paused) { reasons.push("Paused"); }
                                    if flags.contains(PacketFlags::LoadingOrProcessing) { reasons.push("LoadingOrProcessing"); }
                                    if !flags.contains(PacketFlags::CarOnTrack) { reasons.push("NotOnTrack"); }
                                    if packet.laps_in_race <= 0 { reasons.push("RaceNotStarted"); }
                                    if !reasons.is_empty() {
                                        info!("Main: Packet {} conditions not met for Pulsar send ({}). Flags: {:?}, Laps: {}",
                                            packet.packet_id, reasons.join(", "), flags, packet.laps_in_race);
                                    }
                                }
                            }

                            if packet.packet_id > 0
                                && packet.packet_id % HEARTBEAT_INTERVAL_PACKETS == 0
                            {
                                info!(
                                    "Sending periodic heartbeat (Packet ID: {})",
                                    packet.packet_id
                                );
                                if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
                                    error!("Failed to send periodic heartbeat: {}", e);
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
                error!("Failed to receive packet: {}. Retrying...", e);
            }
        }
    }
}
