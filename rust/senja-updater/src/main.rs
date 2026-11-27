#![cfg(windows)]
#![windows_subsystem = "windows"]

use reqwest::blocking::Client;
use reqwest::header::USER_AGENT;
use senja_update_protocol::{download_wire, newer};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    env,
    error::Error,
    fs,
    io::{Read, Write},
    os::windows::fs::MetadataExt,
    path::{Path, PathBuf},
    process::Command,
    time::{Duration, Instant},
};
use windows_sys::Win32::UI::WindowsAndMessaging::{FindWindowW, MessageBoxW, PostMessageW};

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const REPOSITORY: &str = "FyarrAja/Senja-Bar";
const INSTALLER: &str = "Senja-Bar.exe";
const MAX_EXE: u64 = 64 << 20;
const MAX_METADATA: u64 = 1 << 20;
const CHECKING: u32 = 1;
const DOWNLOADING: u32 = 2;
const INSTALLING: u32 = 3;
const CURRENT: u32 = 4;
const FAILED: u32 = 5;
const AVAILABLE: u32 = 6;
const READY: u32 = 7;

#[derive(Deserialize)]
struct Asset {
    name: String,
    browser_download_url: String,
}
#[derive(Deserialize)]
struct Release {
    tag_name: String,
    draft: bool,
    prerelease: bool,
    assets: Vec<Asset>,
}
#[derive(Serialize)]
struct State<'a> {
    latest: &'a str,
    checked_at: String,
}
#[derive(Serialize, Deserialize)]
struct Pending {
    version: String,
    directory: String,
    sha256: String,
}

fn message(kind: u32, wparam: usize, lparam: isize) {
    let name: Vec<u16> = "SenjaHostWindow\0".encode_utf16().collect();
    unsafe {
        let window = FindWindowW(name.as_ptr(), std::ptr::null());
        if !window.is_null() {
            PostMessageW(window, 0x8000 + kind, wparam, lparam);
        }
    }
}
fn phase(value: u32) {
    message(61, value as usize, 0);
}
fn progress(done: u64, total: u64, started: Instant) {
    message(
        67,
        download_wire(done as i64, total as i64) as usize,
        started.elapsed().as_millis().min(isize::MAX as u128) as isize,
    );
}

fn client(timeout: Duration) -> Result<Client> {
    Ok(Client::builder()
        .timeout(timeout)
        .redirect(reqwest::redirect::Policy::custom(|attempt| {
            let url = attempt.url();
            let trusted = matches!(
                url.host_str(),
                Some(
                    "github.com"
                        | "release-assets.githubusercontent.com"
                        | "objects.githubusercontent.com"
                )
            );
            if attempt.previous().len() >= 8
                || url.scheme() != "https"
                || !trusted
                || !url.username().is_empty()
                || url.password().is_some()
            {
                attempt.error("untrusted download redirect")
            } else {
                attempt.follow()
            }
        }))
        .build()?)
}

fn get(
    client: &Client,
    url: &str,
    limit: u64,
    mut output: impl Write,
    on_progress: impl Fn(u64, u64),
) -> Result<()> {
    let mut response = client
        .get(url)
        .header(USER_AGENT, "Senja-Bar-Updater")
        .send()?;
    if response.status() != reqwest::StatusCode::OK {
        return Err(format!("download returned HTTP {}", response.status()).into());
    }
    let total = response.content_length().unwrap_or(0);
    if total > limit {
        return Err("download exceeds size limit".into());
    }
    on_progress(0, total);
    let mut done = 0_u64;
    let mut buffer = [0_u8; 32 * 1024];
    let mut last = Instant::now();
    loop {
        let n = response.read(&mut buffer)?;
        if n == 0 {
            break;
        }
        done += n as u64;
        if done > limit {
            return Err("download exceeds size limit".into());
        }
        output.write_all(&buffer[..n])?;
        if last.elapsed() >= Duration::from_millis(100) {
            on_progress(done, total);
            last = Instant::now();
        }
    }
    on_progress(done, total);
    Ok(())
}

fn trusted_asset(raw: &str, tag: &str, name: &str) -> bool {
    let Ok(url) = reqwest::Url::parse(raw) else {
        return false;
    };
    url.scheme() == "https"
        && url.host_str() == Some("github.com")
        && url.port().is_none()
        && url.username().is_empty()
        && url.password().is_none()
        && url.query().is_none()
        && url.fragment().is_none()
        && url.path() == format!("/{REPOSITORY}/releases/download/{tag}/{name}")
}
fn asset_urls(release: &Release) -> Result<(&str, &str)> {
    let mut exe = None;
    let mut sum = None;
    for asset in &release.assets {
        if asset.name == INSTALLER || asset.name == "SHA256SUMS.txt" {
            if !trusted_asset(&asset.browser_download_url, &release.tag_name, &asset.name) {
                return Err("untrusted release asset".into());
            }
            let slot = if asset.name == INSTALLER {
                &mut exe
            } else {
                &mut sum
            };
            if slot.is_some() {
                return Err("duplicate release asset".into());
            }
            *slot = Some(asset.browser_download_url.as_str());
        }
    }
    Ok((
        exe.ok_or("installer unavailable")?,
        sum.ok_or("checksums unavailable")?,
    ))
}
fn check(client: &Client, current: &str) -> Result<(Release, bool)> {
    let mut bytes = Vec::new();
    get(
        client,
        &format!("https://api.github.com/repos/{REPOSITORY}/releases/latest"),
        MAX_METADATA,
        &mut bytes,
        |_, _| {},
    )?;
    let release: Release = serde_json::from_slice(&bytes)?;
    if release.draft || release.prerelease {
        return Err("release is not stable".into());
    }
    let is_newer = newer(&release.tag_name, current).map_err(|_| "invalid release version")?;
    if is_newer {
        asset_urls(&release)?;
    }
    Ok((release, is_newer))
}
fn expected_hash(bytes: &[u8]) -> Result<String> {
    let content = std::str::from_utf8(bytes)?;
    let mut expected = None;
    for line in content.lines() {
        let fields: Vec<_> = line.split_whitespace().collect();
        if fields.len() == 2 && fields[1].trim_start_matches('*') == INSTALLER {
            if expected.is_some() {
                return Err("duplicate installer checksum".into());
            }
            if fields[0].len() != 64 {
                return Err("invalid installer checksum".into());
            }
            hex::decode(fields[0])?;
            expected = Some(fields[0].to_ascii_lowercase());
        }
    }
    expected.ok_or_else(|| "installer checksum missing".into())
}
fn hash_file(path: &Path) -> Result<String> {
    let metadata = fs::symlink_metadata(path)?;
    if !metadata.is_file()
        || metadata.file_attributes() & 0x400 != 0
        || metadata.len() == 0
        || metadata.len() > MAX_EXE
    {
        return Err("invalid staged installer".into());
    }
    let mut file = fs::File::open(path)?;
    let mut hash = Sha256::new();
    std::io::copy(&mut file, &mut hash)?;
    Ok(hex::encode(hash.finalize()))
}
fn atomic_json<T: Serialize>(path: &Path, value: &T) -> Result<()> {
    let parent = path.parent().ok_or("invalid state path")?;
    fs::create_dir_all(parent)?;
    let mut file = tempfile::NamedTempFile::new_in(parent)?;
    serde_json::to_writer(&mut file, value)?;
    file.flush()?;
    file.persist(path)?;
    Ok(())
}
fn stage(client: &Client, current: &str, root: &Path) -> Result<()> {
    phase(CHECKING);
    let (release, is_newer) = check(client, current)?;
    if !is_newer {
        phase(CURRENT);
        return Ok(());
    }
    let (exe, sum) = asset_urls(&release)?;
    phase(DOWNLOADING);
    stage_download(client, root, &release.tag_name, exe, sum, progress)?;
    phase(READY);
    Ok(())
}
fn stage_download(
    client: &Client,
    root: &Path,
    tag: &str,
    exe: &str,
    sum: &str,
    on_progress: impl Fn(u64, u64, Instant),
) -> Result<()> {
    let mut sums = Vec::new();
    get(client, sum, 64 << 10, &mut sums, |_, _| {})?;
    let expected = expected_hash(&sums)?;
    fs::create_dir_all(root)?;
    let mut temp = tempfile::NamedTempFile::new_in(root)?;
    let started = Instant::now();
    get(client, exe, MAX_EXE, &mut temp, |done, total| {
        on_progress(done, total, started)
    })?;
    temp.flush()?;
    let actual = hash_file(temp.path())?;
    if actual != expected {
        return Err("installer checksum mismatch".into());
    }
    let stage = tempfile::Builder::new()
        .prefix(&format!("{tag}-"))
        .tempdir_in(root)?;
    let directory = stage
        .path()
        .file_name()
        .ok_or("invalid stage path")?
        .to_string_lossy()
        .into_owned();
    let path = stage.path().join(INSTALLER);
    temp.persist(&path)?;
    let pending = Pending {
        version: tag.to_owned(),
        directory,
        sha256: actual,
    };
    atomic_json(&root.join("pending.json"), &pending)?;
    let _ = stage.keep();
    Ok(())
}
fn pending_path(current: &str, root: &Path) -> Result<PathBuf> {
    let metadata = fs::symlink_metadata(root.join("pending.json"))?;
    if !metadata.is_file() || metadata.len() > 4096 {
        return Err("invalid pending metadata".into());
    }
    let pending: Pending = serde_json::from_slice(&fs::read(root.join("pending.json"))?)?;
    if !pending.version.starts_with('v')
        || !newer(&pending.version, current).map_err(|_| "invalid staged version")?
        || !pending
            .directory
            .starts_with(&format!("{}-", pending.version))
        || pending.directory.contains(['/', '\\', ':'])
        || pending.directory == "."
        || pending.directory == ".."
        || pending.sha256.len() != 64
        || hex::decode(&pending.sha256).is_err()
    {
        return Err("invalid pending record".into());
    }
    let path = root.join(&pending.directory).join(INSTALLER);
    let directory = root.join(&pending.directory);
    for target in [root, directory.as_path(), path.as_path()] {
        if fs::symlink_metadata(target)?.file_attributes() & 0x400 != 0 {
            return Err("staged update is a link".into());
        }
    }
    if hash_file(&path)? != pending.sha256 {
        return Err("staged installer checksum mismatch".into());
    }
    Ok(path)
}
fn run() -> Result<()> {
    let mut args: Vec<String> = env::args().skip(1).collect();
    let quiet = args.last().is_some_and(|arg| arg == "--quiet");
    if quiet {
        args.pop();
    }
    if args.len() != 1
        && (args.len() != 2 || !matches!(args[1].as_str(), "--check-only" | "--apply-ready"))
    {
        return Err("current version required".into());
    }
    let base = PathBuf::from(env::var("LOCALAPPDATA")?);
    if args.get(1).is_some_and(|arg| arg == "--apply-ready") {
        let root = base.join("Senja").join("updates");
        let path = match pending_path(&args[0], &root) {
            Ok(path) => path,
            Err(error) => {
                let _ = fs::remove_file(root.join("pending.json"));
                return Err(error);
            }
        };
        phase(INSTALLING);
        return if Command::new(&path)
            .current_dir(path.parent().ok_or("invalid installer path")?)
            .status()?
            .success()
        {
            Ok(())
        } else {
            Err("installer failed".into())
        };
    }
    if args.get(1).is_some_and(|arg| arg == "--check-only") {
        let (release, is_newer) = check(&client(Duration::from_secs(20))?, &args[0])?;
        let time = chrono::Utc::now().to_rfc3339_opts(chrono::SecondsFormat::Nanos, true);
        atomic_json(
            &base.join("Senja").join("update-state.json"),
            &State {
                latest: &release.tag_name,
                checked_at: time,
            },
        )?;
        phase(if is_newer { AVAILABLE } else { CURRENT });
        return Ok(());
    }
    stage(
        &client(Duration::from_secs(180))?,
        &args[0],
        &base.join("Senja").join("updates"),
    )
}

fn main() {
    let quiet = env::args().next_back().is_some_and(|arg| arg == "--quiet");
    let check_only = env::args().any(|arg| arg == "--check-only");
    if let Err(error) = run() {
        phase(FAILED);
        if !quiet && !check_only {
            let message: Vec<u16> = format!("Pembaruan gagal. Versi lama tetap dapat digunakan.\nCoba lagi saat koneksi tersedia.\n\n{error}\0").encode_utf16().collect();
            let title: Vec<u16> = "Senja — Pembaruan\0".encode_utf16().collect();
            unsafe {
                MessageBoxW(std::ptr::null_mut(), message.as_ptr(), title.as_ptr(), 0x10);
            }
        }
        std::process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpListener;
    use std::thread;

    fn local_response(response: &'static [u8]) -> (String, thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}/fixture", listener.local_addr().unwrap());
        let task = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut request = [0; 1024];
            let _ = stream.read(&mut request);
            stream.write_all(response).unwrap();
        });
        (url, task)
    }

    fn local_stage(
        checksum: String,
        executable: Vec<u8>,
        declared_size: usize,
    ) -> (String, thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let task = thread::spawn(move || {
            for _ in 0..2 {
                let (mut stream, _) = listener.accept().unwrap();
                let mut request = [0; 1024];
                let n = stream.read(&mut request).unwrap();
                let request = String::from_utf8_lossy(&request[..n]);
                let (body, length) = if request.starts_with("GET /SHA256SUMS.txt ") {
                    (checksum.as_bytes(), checksum.len())
                } else {
                    (executable.as_slice(), declared_size)
                };
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {length}\r\nConnection: close\r\n\r\n"
                )
                .unwrap();
                stream.write_all(body).unwrap();
            }
        });
        (base, task)
    }

    #[test]
    fn stages_verified_bytes_and_rejects_mismatch_or_interruption() {
        let client = Client::new();
        for (actual, declared, should_pass) in [
            (b"MZ-good".to_vec(), 7, true),
            (b"MZ-bad!".to_vec(), 7, false),
            (b"MZ-good".to_vec(), 12, false),
        ] {
            let expected = hex::encode(Sha256::digest(b"MZ-good"));
            let (base, task) = local_stage(format!("{expected}  {INSTALLER}\n"), actual, declared);
            let root = tempfile::tempdir().unwrap();
            let result = stage_download(
                &client,
                root.path(),
                "v1.3.1",
                &format!("{base}/{INSTALLER}"),
                &format!("{base}/SHA256SUMS.txt"),
                |_, _, _| {},
            );
            task.join().unwrap();
            assert_eq!(result.is_ok(), should_pass);
            assert_eq!(root.path().join("pending.json").exists(), should_pass);
            if should_pass {
                let path = pending_path("1.3.0", root.path()).unwrap();
                assert_eq!(fs::read(path).unwrap(), b"MZ-good");
            }
        }
    }

    #[test]
    fn download_accepts_only_http_200_with_bounded_body() {
        let client = Client::new();
        let (url, task) = local_response(b"HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello");
        let mut body = Vec::new();
        get(&client, &url, 5, &mut body, |_, _| {}).unwrap();
        task.join().unwrap();
        assert_eq!(body, b"hello");

        let (url, task) =
            local_response(b"HTTP/1.1 206 Partial Content\r\nContent-Length: 5\r\n\r\nhello");
        assert!(get(&client, &url, 5, Vec::new(), |_, _| {}).is_err());
        task.join().unwrap();

        let (url, task) = local_response(b"HTTP/1.1 200 OK\r\nContent-Length: 6\r\n\r\nhello!");
        assert!(get(&client, &url, 5, Vec::new(), |_, _| {}).is_err());
        task.join().unwrap();
    }

    #[test]
    fn assets_reject_foreign_hosts_and_duplicate_checksums() {
        let tag = "v1.3.1";
        let base = format!("https://github.com/{REPOSITORY}/releases/download/{tag}/");
        assert!(trusted_asset(&(base.clone() + INSTALLER), tag, INSTALLER));
        assert!(!trusted_asset(
            "https://evil.example/Senja-Bar.exe",
            tag,
            INSTALLER
        ));
        assert!(!trusted_asset(
            &(base.clone() + INSTALLER + "?download=1"),
            tag,
            INSTALLER
        ));
        let release = Release {
            tag_name: tag.into(),
            draft: false,
            prerelease: false,
            assets: vec![
                Asset {
                    name: INSTALLER.into(),
                    browser_download_url: base.clone() + INSTALLER,
                },
                Asset {
                    name: "SHA256SUMS.txt".into(),
                    browser_download_url: base.clone() + "SHA256SUMS.txt",
                },
                Asset {
                    name: "SHA256SUMS.txt".into(),
                    browser_download_url: base + "SHA256SUMS.txt",
                },
            ],
        };
        assert!(asset_urls(&release).is_err());
    }

    #[test]
    fn checksum_and_pending_tamper_are_rejected() {
        let expected = "a".repeat(64);
        assert_eq!(
            expected_hash(format!("{expected}  {INSTALLER}\n").as_bytes()).unwrap(),
            expected
        );
        assert!(
            expected_hash(format!("{expected}  {INSTALLER}\n{expected}  {INSTALLER}\n").as_bytes())
                .is_err()
        );
        let root = tempfile::tempdir().unwrap();
        let directory = root.path().join("v1.3.1-test");
        fs::create_dir(&directory).unwrap();
        fs::write(directory.join(INSTALLER), b"MZ-good").unwrap();
        let hash = hash_file(&directory.join(INSTALLER)).unwrap();
        atomic_json(
            &root.path().join("pending.json"),
            &Pending {
                version: "v1.3.1".into(),
                directory: "v1.3.1-test".into(),
                sha256: hash,
            },
        )
        .unwrap();
        assert!(pending_path("1.3.0", root.path()).is_ok());
        fs::write(directory.join(INSTALLER), b"MZ-bad").unwrap();
        assert!(pending_path("1.3.0", root.path()).is_err());
    }
}
