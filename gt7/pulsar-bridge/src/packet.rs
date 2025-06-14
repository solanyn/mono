use std::{
    io::{Cursor, Seek, SeekFrom},
    mem,
};

use byteorder::{LittleEndian, ReadBytesExt};

use crate::{
    constants::{PACKET_MAGIC_VALUE, PACKET_SIZE},
    cypher, 
    errors::ParsePacketError,
    flags::PacketFlags,
};

#[derive(serde::Serialize, serde::Deserialize, Debug, Clone, Copy, PartialEq)]
pub struct Packet {
    pub position: [f32; 3],
    pub velocity: [f32; 3],
    pub rotation: [f32; 3],
    pub relative_orientation_to_north: f32,
    pub angular_velocity: [f32; 3],
    pub body_height: f32,
    pub engine_rpm: f32,
    pub gas_level: f32,
    pub gas_capacity: f32,
    pub meters_per_second: f32,
    pub turbo_boost: f32,
    pub oil_pressure: f32,
    pub water_temperature: f32,
    pub oil_temperature: f32,
    pub tire_fl_surface_temperature: f32,
    pub tire_fr_surface_temperature: f32,
    pub tire_rl_surface_temperature: f32,
    pub tire_rr_surface_temperature: f32,
    pub packet_id: i32,
    pub lap_count: i16,
    pub laps_in_race: i16,
    pub best_lap_time: i32,
    pub last_lap_time: i32,
    pub time_of_day_progression: i32,
    pub qualifying_position: i16,
    pub num_cars_pre_race: i16,
    pub alert_rpm_min: i16,
    pub alert_rpm_max: i16,
    pub calculated_max_speed: i16,
    pub flags: Option<PacketFlags>,
    pub current_gear: u8,
    pub suggested_gear: u8,
    pub throttle: u8,
    pub brake: u8,
    pub road_plane: [f32; 3],
    pub road_plane_distance: f32,
    pub wheel_fl_rps: f32,
    pub wheel_fr_rps: f32,
    pub wheel_rl_rps: f32,
    pub wheel_rr_rps: f32,
    pub tire_fl_radius: f32,
    pub tire_fr_radius: f32,
    pub tire_rl_radius: f32,
    pub tire_rr_radius: f32,
    pub tire_fl_suspension_height: f32,
    pub tire_fr_suspension_height: f32,
    pub tire_rl_suspension_height: f32,
    pub tire_rr_suspension_height: f32,
    pub clutch_pedal: f32,
    pub clutch_engagement: f32,
    pub rpm_from_clutch_to_gearbox: f32,
    pub transmission_top_speed: f32,
    pub gear_ratios: [f32; 7],
    pub car_code: i32,
}

impl Default for Packet {
    fn default() -> Self {
        Packet {
            position: [0.0; 3],
            velocity: [0.0; 3],
            rotation: [0.0; 3],
            relative_orientation_to_north: 0.0,
            angular_velocity: [0.0; 3],
            body_height: 0.0,
            engine_rpm: 0.0,
            gas_level: 0.0,
            gas_capacity: 0.0,
            meters_per_second: 0.0,
            turbo_boost: 0.0,
            oil_pressure: 0.0,
            water_temperature: 0.0,
            oil_temperature: 0.0,
            tire_fl_surface_temperature: 0.0,
            tire_fr_surface_temperature: 0.0,
            tire_rl_surface_temperature: 0.0,
            tire_rr_surface_temperature: 0.0,
            packet_id: 0,
            lap_count: 0,
            laps_in_race: 0,
            best_lap_time: 0,
            last_lap_time: 0,
            time_of_day_progression: 0,
            qualifying_position: 0,
            num_cars_pre_race: 0,
            alert_rpm_min: 0,
            alert_rpm_max: 0,
            calculated_max_speed: 0,
            flags: None,
            current_gear: 0,
            suggested_gear: 0,
            throttle: 0,
            brake: 0,
            road_plane: [0.0; 3],
            road_plane_distance: 0.0,
            wheel_fl_rps: 0.0,
            wheel_fr_rps: 0.0,
            wheel_rl_rps: 0.0,
            wheel_rr_rps: 0.0,
            tire_fl_radius: 0.0,
            tire_fr_radius: 0.0,
            tire_rl_radius: 0.0,
            tire_rr_radius: 0.0,
            tire_fl_suspension_height: 0.0,
            tire_fr_suspension_height: 0.0,
            tire_rl_suspension_height: 0.0,
            tire_rr_suspension_height: 0.0,
            clutch_pedal: 0.0,
            clutch_engagement: 0.0,
            rpm_from_clutch_to_gearbox: 0.0,
            transmission_top_speed: 0.0,
            gear_ratios: [0.0; 7],
            car_code: 0,
        }
    }
}

impl TryFrom<&[u8; PACKET_SIZE]> for Packet {
    type Error = ParsePacketError;

    fn try_from(data: &[u8; PACKET_SIZE]) -> Result<Self, Self::Error> {
        let decrypted_data = cypher::decrypt(data)?;
        let mut cursor = Cursor::new(decrypted_data);

        let magic = cursor.read_u32::<LittleEndian>()?;
        verify_magic_value(magic)?;

        let position = [
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
        ];
        let velocity = [
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
        ];
        let rotation = [
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
        ];
        let relative_orientation_to_north = cursor.read_f32::<LittleEndian>()?;
        let angular_velocity = [
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
        ];
        let body_height = cursor.read_f32::<LittleEndian>()?;
        let engine_rpm = cursor.read_f32::<LittleEndian>()?;

        cursor.seek(SeekFrom::Current(mem::size_of::<i32>() as i64))?;

        let gas_level = cursor.read_f32::<LittleEndian>()?;
        let gas_capacity = cursor.read_f32::<LittleEndian>()?;
        let meters_per_second = cursor.read_f32::<LittleEndian>()?;
        let turbo_boost = cursor.read_f32::<LittleEndian>()?;
        let oil_pressure = cursor.read_f32::<LittleEndian>()?;
        let water_temperature = cursor.read_f32::<LittleEndian>()?;
        let oil_temperature = cursor.read_f32::<LittleEndian>()?;
        let tire_fl_surface_temperature = cursor.read_f32::<LittleEndian>()?;
        let tire_fr_surface_temperature = cursor.read_f32::<LittleEndian>()?;
        let tire_rl_surface_temperature = cursor.read_f32::<LittleEndian>()?;
        let tire_rr_surface_temperature = cursor.read_f32::<LittleEndian>()?;
        let packet_id = cursor.read_i32::<LittleEndian>()?;
        let lap_count = cursor.read_i16::<LittleEndian>()?;
        let laps_in_race = cursor.read_i16::<LittleEndian>()?;
        let best_lap_time = cursor.read_i32::<LittleEndian>()?;
        let last_lap_time = cursor.read_i32::<LittleEndian>()?;
        let time_of_day_progression = cursor.read_i32::<LittleEndian>()?;
        let qualifying_position = cursor.read_i16::<LittleEndian>()?;
        let num_cars_pre_race = cursor.read_i16::<LittleEndian>()?;
        let alert_rpm_min = cursor.read_i16::<LittleEndian>()?;
        let alert_rpm_max = cursor.read_i16::<LittleEndian>()?;
        let calculated_max_speed = cursor.read_i16::<LittleEndian>()?;

        let flag_bits = cursor.read_u16::<LittleEndian>()?;
        let flags = PacketFlags::from_bits(flag_bits);

        let bits = cursor.read_u8()?;
        let current_gear = bits & 0b1111;
        let suggested_gear = bits >> 4;

        let throttle = cursor.read_u8()?;
        let brake = cursor.read_u8()?;

        cursor.read_u8()?; // Skip an unused byte

        let road_plane = [
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
            cursor.read_f32::<LittleEndian>()?,
        ];
        let road_plane_distance = cursor.read_f32::<LittleEndian>()?;
        let wheel_fl_rps = cursor.read_f32::<LittleEndian>()?;
        let wheel_fr_rps = cursor.read_f32::<LittleEndian>()?;
        let wheel_rl_rps = cursor.read_f32::<LittleEndian>()?;
        let wheel_rr_rps = cursor.read_f32::<LittleEndian>()?;
        let tire_fl_radius = cursor.read_f32::<LittleEndian>()?;
        let tire_fr_radius = cursor.read_f32::<LittleEndian>()?;
        let tire_rl_radius = cursor.read_f32::<LittleEndian>()?;
        let tire_rr_radius = cursor.read_f32::<LittleEndian>()?;
        let tire_fl_suspension_height = cursor.read_f32::<LittleEndian>()?;
        let tire_fr_suspension_height = cursor.read_f32::<LittleEndian>()?;
        let tire_rl_suspension_height = cursor.read_f32::<LittleEndian>()?;
        let tire_rr_suspension_height = cursor.read_f32::<LittleEndian>()?;

        cursor.seek(SeekFrom::Current(mem::size_of::<i32>() as i64 * 8))?;

        let clutch_pedal = cursor.read_f32::<LittleEndian>()?;
        let clutch_engagement = cursor.read_f32::<LittleEndian>()?;
        let rpm_from_clutch_to_gearbox = cursor.read_f32::<LittleEndian>()?;
        let transmission_top_speed = cursor.read_f32::<LittleEndian>()?;

        let mut gear_ratios: [f32; 7] = [0f32; 7];
        for gear_ratio_val in gear_ratios.iter_mut() {
            *gear_ratio_val = cursor.read_f32::<LittleEndian>()?;
        }

        cursor.read_f32::<LittleEndian>()?; // Skip 8th gear
        let car_code = cursor.read_i32::<LittleEndian>()?;

        Ok(Self {
            position,
            velocity,
            rotation,
            relative_orientation_to_north,
            angular_velocity,
            body_height,
            engine_rpm,
            gas_level,
            gas_capacity,
            meters_per_second,
            turbo_boost,
            oil_pressure,
            water_temperature,
            oil_temperature,
            tire_fl_surface_temperature,
            tire_fr_surface_temperature,
            tire_rl_surface_temperature,
            tire_rr_surface_temperature,
            packet_id,
            lap_count,
            laps_in_race,
            best_lap_time,
            last_lap_time,
            time_of_day_progression,
            qualifying_position,
            num_cars_pre_race,
            alert_rpm_min,
            alert_rpm_max,
            calculated_max_speed,
            flags,
            current_gear,
            suggested_gear,
            throttle,
            brake,
            road_plane,
            road_plane_distance,
            wheel_fl_rps,
            wheel_fr_rps,
            wheel_rl_rps,
            wheel_rr_rps,
            tire_fl_radius,
            tire_fr_radius,
            tire_rl_radius,
            tire_rr_radius,
            tire_fl_suspension_height,
            tire_fr_suspension_height,
            tire_rl_suspension_height,
            tire_rr_suspension_height,
            clutch_pedal,
            clutch_engagement,
            rpm_from_clutch_to_gearbox,
            transmission_top_speed,
            gear_ratios,
            car_code,
        })
    }
}

fn verify_magic_value(magic: u32) -> Result<(), ParsePacketError> {
    if magic != PACKET_MAGIC_VALUE {
        return Err(ParsePacketError::InvalidMagicValue(magic));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::constants::PACKET_DECRYPTION_KEY;
    use crate::errors::ParsePacketError; 
    use salsa20::cipher::{generic_array::GenericArray, KeyIvInit, StreamCipher};

    const SAMPLE_ENCRYPTED_PACKET: [u8; PACKET_SIZE] = [
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

    #[test]
    fn test_parse_sample_encrypted_packet() {
        match Packet::try_from(&SAMPLE_ENCRYPTED_PACKET) {
            Ok(parsed_packet) => {
                println!("Successfully parsed sample packet: {:#?}", parsed_packet);
                // TODO: Add assertions here once ground truth for SAMPLE_ENCRYPTED_PACKET is known.
            }
            Err(e) => {
                panic!("Parsing failed for sample encrypted packet: {:?}", e);
            }
        }
    }

    #[test]
    fn test_parse_invalid_magic_value_with_sample_structure() {
        let decrypted_payload = match crate::cypher::decrypt(&SAMPLE_ENCRYPTED_PACKET) {
            Ok(payload) => payload,
            Err(e) => panic!(
                "Failed to decrypt sample packet for magic value corruption test: {:?}",
                e
            ),
        };

        let mut re_encrypt_source = decrypted_payload;
        let bad_magic_val: u32 = 0xDEADBEEF;
        re_encrypt_source[0..4].copy_from_slice(&bad_magic_val.to_le_bytes());

        // .unwrap() is safe here as slice length is known and matches IV_SIZE.
        let original_iv_seed_from_sample: [u8; crate::cypher::IV_SIZE] = SAMPLE_ENCRYPTED_PACKET
            [crate::cypher::IV_OFFSET..(crate::cypher::IV_OFFSET + crate::cypher::IV_SIZE)]
            .try_into()
            .unwrap();

        let mut final_encrypted_corrupted_packet = re_encrypt_source;

        let key_bytes_slice = &PACKET_DECRYPTION_KEY[0..crate::cypher::EXPECTED_KEY_LEN];
        let key_slice = GenericArray::from_slice(key_bytes_slice);

        let iv1 = u32::from_le_bytes(original_iv_seed_from_.try_into().unwrap());
        let iv2 = iv1 ^ 0xDEADBEAF;
        let mut iv_salsa20_bytes: [u8; 8] = [0u8; 8];
        iv_salsa20_bytes[0..4].copy_from_slice(&iv2.to_le_bytes());
        iv_salsa20_bytes[4..].copy_from_slice(&iv1.to_le_bytes());
        let iv_generic_array = GenericArray::from_slice(&iv_salsa20_bytes);

        let mut cipher = salsa20::Salsa20::new(key_slice, iv_generic_array);
        cipher.apply_keystream(&mut final_encrypted_corrupted_packet[..]);

        final_encrypted_corrupted_packet
            [crate::cypher::IV_OFFSET..(crate::cypher::IV_OFFSET + crate::cypher::IV_SIZE)]
            .copy_from_slice(
                &SAMPLE_ENCRYPTED_PACKET
                    [crate::cypher::IV_OFFSET..(crate::cypher::IV_OFFSET + crate::cypher::IV_SIZE)],
            );

        match Packet::try_from(&final_encrypted_corrupted_packet) {
            Err(ParsePacketError::InvalidMagicValue(val)) => {
                assert_eq!(
                    val, bad_magic_val,
                    "The magic value reported in the error should be the corrupted one."
                );
            }
            Ok(p) => panic!(
                "Expected InvalidMagicValue error, but parsing succeeded with packet: {:#?}",
                p
            ),
            Err(e) => panic!(
                "Expected InvalidMagicValue error, got different error: {:?}",
                e
            ),
        }
    }
}
