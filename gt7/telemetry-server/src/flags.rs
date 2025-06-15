use bitflags::bitflags;
use serde::{Deserialize, Deserializer, Serialize, Serializer};

bitflags! {
    #[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
    pub struct PacketFlags: u16 {
        const None = 0;
        const CarOnTrack = 1 << 0;
        const Paused = 1 << 1;
        const LoadingOrProcessing = 1 << 2;
        const InGear = 1 << 3;
        const HasTurbo = 1 << 4;
        const RevLimiterBlinkAlertActive = 1 << 5;
        const HandBrakeActive = 1 << 6;
        const LightsActive = 1 << 7; // Value is 128
        const HighBeamActive = 1 << 8;
        const LowBeamActive = 1 << 9;
        const ASMActive = 1 << 10;
        const TCSActive = 1 << 11;
    }
}

impl Serialize for PacketFlags {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_u16(self.bits())
    }
}

impl<'de> Deserialize<'de> for PacketFlags {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        let bits = u16::deserialize(deserializer)?;
        PacketFlags::from_bits(bits).ok_or_else(|| {
            serde::de::Error::invalid_value(
                serde::de::Unexpected::Unsigned(u64::from(bits)),
                &"valid PacketFlags bits",
            )
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_flag_operations() {
        let mut flags = PacketFlags::empty();
        assert_eq!(flags, PacketFlags::None);
        assert!(!flags.contains(PacketFlags::CarOnTrack));

        flags.insert(PacketFlags::CarOnTrack);
        assert!(flags.contains(PacketFlags::CarOnTrack));
        assert!(flags.intersects(PacketFlags::CarOnTrack | PacketFlags::Paused));
        assert!(!flags.contains(PacketFlags::Paused));

        flags.insert(PacketFlags::Paused);
        assert!(flags.contains(PacketFlags::CarOnTrack | PacketFlags::Paused));
        assert!(flags.contains(PacketFlags::CarOnTrack));
        assert!(flags.contains(PacketFlags::Paused));

        flags.remove(PacketFlags::CarOnTrack);
        assert!(!flags.contains(PacketFlags::CarOnTrack));
        assert!(flags.contains(PacketFlags::Paused));

        flags.toggle(PacketFlags::Paused);
        assert!(!flags.contains(PacketFlags::Paused));
        flags.toggle(PacketFlags::Paused);
        assert!(flags.contains(PacketFlags::Paused));
    }

    #[test]
    fn test_from_bits_and_bits() {
        let raw_value: u16 = PacketFlags::CarOnTrack.bits() | PacketFlags::InGear.bits();
        let flags = PacketFlags::from_bits_truncate(raw_value);

        assert!(flags.contains(PacketFlags::CarOnTrack));
        assert!(flags.contains(PacketFlags::InGear));
        assert!(!flags.contains(PacketFlags::Paused));
        assert_eq!(flags.bits(), raw_value);

        let all_defined_flags = PacketFlags::CarOnTrack
            | PacketFlags::Paused
            | PacketFlags::LoadingOrProcessing
            | PacketFlags::InGear
            | PacketFlags::HasTurbo
            | PacketFlags::RevLimiterBlinkAlertActive
            | PacketFlags::HandBrakeActive
            | PacketFlags::LightsActive
            | PacketFlags::HighBeamActive
            | PacketFlags::LowBeamActive
            | PacketFlags::ASMActive
            | PacketFlags::TCSActive;
        assert_eq!(PacketFlags::all().bits(), all_defined_flags.bits());

        let truncated = PacketFlags::from_bits_truncate(0xFFFF);
        assert_eq!(truncated, PacketFlags::all());

        let invalid_bits = PacketFlags::from_bits(0xF000);
        assert!(
            invalid_bits.is_none(),
            "Expected None for unrecognised high bits if not truncating/retaining"
        );

        let valid_bits_some = PacketFlags::from_bits(PacketFlags::CarOnTrack.bits());
        assert_eq!(valid_bits_some, Some(PacketFlags::CarOnTrack));
    }

    #[test]
    fn test_serde_flags() {
        let flags = PacketFlags::CarOnTrack | PacketFlags::LightsActive; // 1 | 128 = 129
        let serialized = serde_json::to_string(&flags).unwrap();

        let expected_numeric_value = flags.bits().to_string(); // "129"
        assert_eq!(serialized, expected_numeric_value);

        let deserialized: PacketFlags = serde_json::from_str(&serialized).unwrap();
        assert_eq!(deserialized, flags);

        let empty_flags = PacketFlags::empty();
        let serialized_empty = serde_json::to_string(&empty_flags).unwrap();
        assert_eq!(serialized_empty, empty_flags.bits().to_string()); // "0"
        let deserialized_empty: PacketFlags = serde_json::from_str(&serialized_empty).unwrap();
        assert_eq!(deserialized_empty, empty_flags);
    }
}
