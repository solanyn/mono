use gt7_telemetry_server::{
    constants::{PACKET_HEARTBEAT_DATA, PACKET_SIZE},
    errors::ParsePacketError,
    flags::PacketFlags,
    packet::Packet,
    config::Config,
};
use crate::pulsar_handler::PulsarHandler;
use log::{debug, error, info, warn};
use std::net::{SocketAddr, UdpSocket};
use std::process;
use std::thread;
use std::time::{Duration, Instant};
use tokio::sync::broadcast;

pub fn run_telemetry_loop(
    config: Config,
    ws_tx: broadcast::Sender<String>,
    pulsar_handler: PulsarHandler,
) {
    let mut last_periodic_log_time = if let Some(interval) = config.log_packet_interval_duration {
        Instant::now()
            .checked_sub(interval)
            .unwrap_or_else(Instant::now)
    } else {
        Instant::now()
    };

    let socket = setup_udp_socket(&config.udp_bind_address);
    let destination = parse_destination(&config.target_address());

    send_initial_heartbeat(&socket, destination);

    loop {
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
                            handle_packet(
                                packet,
                                &ws_tx,
                                &pulsar_handler,
                                &config.log_packet_interval_duration,
                                &mut last_periodic_log_time,
                            );
                        }
                        Err(e) => handle_packet_error(e, src_addr),
                    }
                } else {
                    warn!(
                        "Received packet of unexpected size: {} bytes from {}",
                        number_of_bytes, src_addr
                    );
                }
            }
            Err(e) => {
                if e.kind() == std::io::ErrorKind::WouldBlock
                    || e.kind() == std::io::ErrorKind::TimedOut
                {
                    debug!("Socket receive timeout. Retrying...");
                } else {
                    error!("Failed to receive packet: {}. Retrying...", e);
                }
            }
        }
    }
}

fn setup_udp_socket(bind_address: &str) -> UdpSocket {
    info!("Attempting to bind UDP telemetry socket to: {}", bind_address);
    
    let socket = match UdpSocket::bind(bind_address) {
        Ok(s) => {
            info!("Successfully bound UDP telemetry socket to: {}", bind_address);
            s
        }
        Err(e) => {
            error!("Failed to bind UDP telemetry socket {}: {}", bind_address, e);
            process::exit(1);
        }
    };

    if let Err(e) = socket.set_read_timeout(Some(Duration::from_secs(1))) {
        warn!(
            "Failed to set read timeout on UDP socket: {}. Proceeding with blocking socket.",
            e
        );
    } else {
        info!("UDP socket read timeout set to 1 second.");
    }

    if let Err(e) = socket.set_broadcast(true) {
        warn!(
            "Failed to enable broadcast on UDP socket: {}. May not receive broadcast packets.",
            e
        );
    } else {
        info!("UDP socket configured to receive broadcast packets.");
    }

    socket
}

fn parse_destination(target_address: &str) -> SocketAddr {
    info!("Target UDP telemetry server: {}", target_address);
    
    match target_address.parse() {
        Ok(addr) => addr,
        Err(e) => {
            error!("Invalid target address format ('{}'): {}", target_address, e);
            process::exit(1);
        }
    }
}

fn send_initial_heartbeat(socket: &UdpSocket, destination: SocketAddr) {
    info!("Sending initial UDP heartbeat to {}", destination);
    if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
        error!("Failed to send UDP heartbeat: {}. Will retry.", e);
    } else {
        info!("UDP heartbeat sent successfully.");
    }
}

fn handle_packet(
    packet: Packet,
    ws_tx: &broadcast::Sender<String>,
    pulsar_handler: &PulsarHandler,
    log_interval: &Option<Duration>,
    last_periodic_log_time: &mut Instant,
) {
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
    
    let packet_json = serde_json::to_string(&packet).unwrap_or_else(|_| "{}".to_string());
    let _ = ws_tx.send(packet_json);

    if let Some(interval) = log_interval {
        if last_periodic_log_time.elapsed() >= *interval {
            info!(
                "[Periodic Packet Detail] ID: {}, Lap: {}/{}, RPM: {:.0}, Speed: {:.1} m/s, Flags: {:?}",
                packet.packet_id,
                packet.lap_count,
                packet.laps_in_race,
                packet.engine_rpm,
                packet.meters_per_second,
                packet.flags
            );
            *last_periodic_log_time = Instant::now();
        }
    }

    if let Some(flags) = packet.flags {
        check_packet_conditions(packet, flags);
    }
}

fn check_packet_conditions(packet: Packet, flags: PacketFlags) {
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
                "Packet {} conditions not met for Pulsar send ({}). Flags: {:?}, Laps: {}",
                packet.packet_id,
                reasons.join(", "),
                flags,
                packet.laps_in_race
            );
        }
    }
}

fn handle_packet_error(e: ParsePacketError, src_addr: SocketAddr) {
    match e {
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
    }
}