//! Wire and version rules shared with the existing Senja updater.

const MAX_DOWNLOAD: i64 = 64 << 20;
pub const PENDING_FLAG: u32 = 0x100;
pub const READY: u32 = 7;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum VersionError {
    Invalid,
}

pub fn version(input: &str) -> Result<[i64; 3], VersionError> {
    let value = input.trim().strip_prefix('v').unwrap_or(input.trim());
    let parts: Vec<_> = value.split('.').collect();
    if parts.len() != 3 {
        return Err(VersionError::Invalid);
    }
    let mut parsed = [0; 3];
    for (index, part) in parts.iter().enumerate() {
        if part.is_empty() || !part.bytes().all(|byte| byte.is_ascii_digit()) {
            return Err(VersionError::Invalid);
        }
        parsed[index] = part.parse().map_err(|_| VersionError::Invalid)?;
    }
    Ok(parsed)
}

pub fn newer(next: &str, current: &str) -> Result<bool, VersionError> {
    Ok(version(next)? > version(current)?)
}

pub fn encode_phase(phase: u32, pending: bool) -> u32 {
    let phase = if phase > READY { 0 } else { phase };
    phase | if pending { PENDING_FLAG } else { 0 }
}

pub fn progress(done: i64, total: i64) -> u32 {
    let done = done.clamp(0, MAX_DOWNLOAD);
    let percent = if total > 0 {
        (done * 100 / total).min(100) as u32 + 1
    } else {
        0
    };
    ((done / 1024) as u32) << 8 | percent
}

pub fn download_wire(done: i64, total: i64) -> u64 {
    let done = done.clamp(0, MAX_DOWNLOAD) as u64;
    let total = total.clamp(0, MAX_DOWNLOAD) as u64;
    total << 32 | done
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_matches_go_release_rules() {
        assert_eq!(version(" v01.2.003 "), Ok([1, 2, 3]));
        assert_eq!(newer("v1.2.0", "1.1.9"), Ok(true));
        assert_eq!(newer("1.2.0", "v1.2.0"), Ok(false));
        for invalid in [
            "",
            "1.2",
            "1.2.3-beta",
            "v-1.2.3",
            "1. 2.3",
            "1.2.9223372036854775808",
        ] {
            assert_eq!(version(invalid), Err(VersionError::Invalid));
        }
    }

    #[test]
    fn wire_values_match_go_protocol() {
        assert_eq!(encode_phase(7, true), 0x107);
        assert_eq!(encode_phase(8, false), 0);
        assert_eq!(progress(0, 0), 0);
        assert_eq!(progress(512, 1024), 51);
        assert_eq!(progress(2048, 1024), 0x265);
        assert_eq!(download_wire(-1, 1024), 1024_u64 << 32);
        assert_eq!(
            download_wire(i64::MAX, i64::MAX),
            (64_u64 << 20) << 32 | (64_u64 << 20)
        );
    }
}
