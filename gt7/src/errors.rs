use thiserror::Error;

#[derive(Error, Debug)]
pub enum ParsePacketError {
    #[error("Invalid magic value: expected {expected:#X}, got {actual:#X}", expected = crate::constants::PACKET_MAGIC_VALUE, actual = .0)]
    InvalidMagicValue(u32),
    #[error("Failed to read data from packet")]
    ReadError(#[from] std::io::Error),
    #[error("Failed to decrypt packet data")]
    DecryptionError(#[from] CypherError),
}

#[derive(Error, Debug, PartialEq, Eq)]
pub enum CypherError {
    #[error("Input data too short (length {0}) to extract IV (requires at least 68 bytes)")]
    InputTooShortForIv(usize),
    #[error("Failed to parse IV structure from packet data")]
    InvalidIvStructure,
    #[error("Decryption key is invalid or has incorrect length (expected 32 bytes, key length {0})")]
    InvalidKeyLength(usize),
    #[error("Salsa20 cipher operation failed")]
    CipherOperationFailed,
}
