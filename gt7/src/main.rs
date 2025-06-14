use gt7::{
    constants::{PACKET_HEARTBEAT_DATA, PACKET_SIZE},
    errors::ParsePacketError,
    packet::Packet,
};
use std::net::{SocketAddr, UdpSocket};
use std::process;
use std::env;

const TELEMETRY_SERVER_PORT: u16 = 33739;
const HEARTBEAT_INTERVAL_PACKETS: i32 = 100;
const BIND_ADDRESS: &str = "0.0.0.0:33740";

fn main() {
    let ps5_ip_address = match env::var("PS5_IP_ADDRESS") {
        Ok(val) => val,
        Err(_) => {
            eprintln!("Error: The PS5_IP_ADDRESS environment variable must be set.");
            process::exit(1);
        }
    };

    println!("Attempting to bind to socket: {}", BIND_ADDRESS);
    let socket = match UdpSocket::bind(BIND_ADDRESS) {
        Ok(s) => {
            println!("Successfully bound to socket: {}", BIND_ADDRESS);
            s
        }
        Err(e) => {
            eprintln!("Failed to bind socket {}: {}", BIND_ADDRESS, e);
            process::exit(1);
        }
    };

    let destination_str = format!("{}:{}", ps5_ip_address, TELEMETRY_SERVER_PORT);
    println!("Target telemetry server: {}", destination_str);
    let destination: SocketAddr = match destination_str.parse() {
        Ok(addr) => addr,
        Err(e) => {
            eprintln!(
                "Invalid IP address or port format for PS5_IP_ADDRESS ('{}'): {}",
                ps5_ip_address, e
            );
            process::exit(1);
        }
    };

    println!("Sending initial heartbeat to {}", destination);
    if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
        eprintln!("Failed to send initial heartbeat: {}", e);
        process::exit(1);
    }
    println!("Initial heartbeat sent.");

    loop {
        if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
            eprintln!(
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
                            println!(
                                "Packet ID: {}, Lap: {}/{}, RPM: {:.0}, Speed: {:.1} m/s, Flags: {:?}",
                                packet.packet_id,
                                packet.lap_count,
                                packet.laps_in_race,
                                packet.engine_rpm,
                                packet.meters_per_second,
                                packet.flags
                            );
                            if packet.packet_id > 0
                                && packet.packet_id % HEARTBEAT_INTERVAL_PACKETS == 0
                            {
                                println!(
                                    "Sending periodic heartbeat (Packet ID: {})",
                                    packet.packet_id
                                );
                                if let Err(e) = socket.send_to(PACKET_HEARTBEAT_DATA, destination) {
                                    eprintln!("Failed to send periodic heartbeat: {}", e);
                                }
                            }
                        }
                        Err(e) => match e {
                            ParsePacketError::InvalidMagicValue(val) => {
                                eprintln!(
                                    "Packet parse error from {}: Invalid magic value {:#X}",
                                    src_addr, val
                                );
                            }
                            ParsePacketError::ReadError(io_err) => {
                                eprintln!(
                                    "Packet parse error from {}: Read error: {}",
                                    src_addr, io_err
                                );
                            }
                            ParsePacketError::DecryptionError(dec_err) => {
                                eprintln!(
                                    "Packet parse error from {}: Decryption error: {}",
                                    src_addr, dec_err
                                );
                            }
                        },
                    }
                } else {
                    eprintln!(
                        "Received packet of unexpected size: {} bytes from {}",
                        number_of_bytes, src_addr
                    );
                }
            }
            Err(e) => {
                eprintln!("Failed to receive packet: {}. Retrying...", e);
            }
        }
    }
}
