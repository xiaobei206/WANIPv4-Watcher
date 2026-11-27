#!/bin/sh
# Install or update WANIPv4-Watcher on ASUSWRT-Merlin.
set -eu
umask 077

REPO_RAW="https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main"
SCRIPTS="/jffs/scripts"
CONFIG_DIR="/jffs/configs"
CONFIG="$CONFIG_DIR/wanipv4-watcher.json"
MANAGER="$SCRIPTS/wanipv4-watcher"
STARTED="$SCRIPTS/services-start"
MANAGER_TMP="/tmp/wanipv4-watcher-merlin.new"
APP_DIR="/jffs/wanipv4-watcher"
BIN="$APP_DIR/wanipv4-watcher"
BIN_DOWNLOAD="/tmp/wanipv4-watcher-arm64.new"
BIN_TMP="$APP_DIR/wanipv4-watcher.new"
CHECKSUMS_TMP="/tmp/wanipv4-watcher-checksums.txt"
ASSET="wanipv4-watcher-linux-arm64"
RELEASE_BASE="https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download"
START_TMP="/tmp/services-start.wanipv4-watcher.$$"
MARKER="# BEGIN WANIPv4-Watcher-Merlin"
END_MARKER="# END WANIPv4-Watcher-Merlin"

say() { printf '%s\n' "[WANIPv4-Watcher Merlin installer] $*"; }
fail() { say "ERROR: $*"; exit 1; }

fetch() {
    url="$1"
    dest="$2"
    name="$(basename "$dest")"
    rm -f "$dest"
    say "downloading $name from $url"
    if [ -n "${https_proxy:-}${HTTPS_PROXY:-}${http_proxy:-}${HTTP_PROXY:-}" ]; then
        say "proxy environment detected (proxy value is not printed)."
    else
        say "no HTTP(S) proxy environment detected; trying direct access."
    fi

    if command -v curl >/dev/null 2>&1; then
        if curl -fSL --show-error --connect-timeout 10 --max-time 180 "$url" -o "$dest"; then
            if [ -s "$dest" ]; then
                say "curl download succeeded: $name ($(wc -c < "$dest" | tr -d ' ') bytes)."
                return 0
            fi
            say "curl returned success but the downloaded file is empty."
        else
            curl_status=$?
            say "curl failed for $name (exit $curl_status); trying Wget."
        fi
        rm -f "$dest"
    else
        say "curl is not installed; using Wget."
    fi

    if command -v wget >/dev/null 2>&1; then
        if wget -T 180 -O "$dest" "$url"; then
            if [ -s "$dest" ]; then
                say "Wget download succeeded: $name ($(wc -c < "$dest" | tr -d ' ') bytes)."
                return 0
            fi
            say "Wget returned success but the downloaded file is empty."
        else
            wget_status=$?
            say "Wget failed for $name (exit $wget_status)."
        fi
        rm -f "$dest"
    else
        say "Wget is not installed."
    fi
    return 1
}

CURRENT_UID="$(awk '/^Uid:/ {print $2; exit}' /proc/self/status 2>/dev/null || true)"
if [ "$CURRENT_UID" != "0" ]; then
    if command -v id >/dev/null 2>&1; then CURRENT_UID="$(id -u 2>/dev/null || true)"; fi
fi
if [ "$CURRENT_UID" != "0" ]; then
    fail "please SSH into the router as the administrator/root account (UID 0 required)."
fi
ARCH="$(uname -m 2>/dev/null || true)"
case "$ARCH" in
    aarch64|arm64) ;;
    *) fail "this installer requires ARM64/AArch64; uname -m reported '$ARCH'." ;;
esac
if [ ! -d /jffs ] || [ ! -w /jffs ]; then
    fail "/jffs is not writable. Enable the JFFS partition in Merlin System settings first."
fi
NVRAM_BIN="$(command -v nvram 2>/dev/null || true)"
if [ -z "$NVRAM_BIN" ] && [ -x /bin/nvram ]; then NVRAM_BIN="/bin/nvram"; fi
if [ -z "$NVRAM_BIN" ]; then
    fail "nvram command not found (checked PATH and /bin/nvram)."
fi
JFFS_SCRIPTS="$("$NVRAM_BIN" get jffs2_scripts 2>/dev/null || true)"
if [ "$JFFS_SCRIPTS" != "1" ]; then
    fail "enable 'Enable JFFS custom scripts and configs' in Administration > System, apply, then rerun this installer."
fi
SHA256SUM_BIN="$(command -v sha256sum 2>/dev/null || true)"
OPENSSL_BIN="$(command -v openssl 2>/dev/null || true)"
if [ -z "$OPENSSL_BIN" ]; then
    for candidate in /usr/sbin/openssl /usr/bin/openssl /bin/openssl /koolshare/bin/openssl; do
        if [ -x "$candidate" ]; then OPENSSL_BIN="$candidate"; break; fi
    done
fi
if [ -z "$SHA256SUM_BIN" ] && [ -z "$OPENSSL_BIN" ]; then
    fail "sha256sum or OpenSSL is required for SHA-256 verification (checked common Merlin/KoolShare paths)."
fi

mkdir -p "$SCRIPTS" "$CONFIG_DIR" "$APP_DIR"

if [ -f "$STARTED" ] && grep -F "$MARKER" "$STARTED" >/dev/null 2>&1 &&
   ! grep -F "$END_MARKER" "$STARTED" >/dev/null 2>&1; then
    fail "the existing services-start script has an incomplete WANIPv4-Watcher marker; no startup file was changed."
fi

if [ ! -f "$CONFIG" ]; then
    cat > "$CONFIG" <<'CONFIG_JSON'
{
  "device_name": "",
  "device_note": "",
  "wan_interface": "auto",
  "check_times": ["07:00", "19:00"],
  "http_timeout_seconds": 8,
  "external_ip_urls": [
    "https://myip.ipip.net",
    "http://ip.3322.net",
    "https://cip.cc",
    "https://ip9.com.cn/get"
  ],
  "alert_on_private_wan": true,
  "notify_on_start": true,
  "notify_on_recovery": true,
  "log_file": "",
  "web_port": 10005,
  "smtp": {
    "host": "smtp.qq.com",
    "port": 465,
    "username": "",
    "password": "",
    "from": "",
    "to": []
  }
}
CONFIG_JSON
    chmod 600 "$CONFIG"
    say "created first-run config. All WAN, schedule and email settings are entered in the Web UI."
else
    chmod 600 "$CONFIG"
    say "existing config preserved at $CONFIG."
fi

# Download once, verify, then store the executable permanently in JFFS.
fetch "$RELEASE_BASE/$ASSET" "$BIN_DOWNLOAD" || fail "could not download the ARM64 Release binary. The lines above show which downloader failed; check whether the proxy can reach GitHub release-assets."
[ -s "$BIN_DOWNLOAD" ] || fail "downloaded ARM64 binary is empty."
fetch "$RELEASE_BASE/checksums.txt" "$CHECKSUMS_TMP" || fail "could not download checksums.txt. The lines above show which downloader failed."
EXPECTED="$(awk -v asset="$ASSET" '$2 == asset {print $1; exit}' "$CHECKSUMS_TMP")"
[ -n "$EXPECTED" ] || fail "checksums.txt does not contain $ASSET."
if [ -n "$SHA256SUM_BIN" ]; then
    ACTUAL="$("$SHA256SUM_BIN" "$BIN_DOWNLOAD" | awk '{print $1}')"
else
    ACTUAL="$("$OPENSSL_BIN" dgst -sha256 "$BIN_DOWNLOAD" | awk '{print $NF}')"
fi
[ "$EXPECTED" = "$ACTUAL" ] || fail "SHA-256 mismatch; existing JFFS binary was not replaced."
cp "$BIN_DOWNLOAD" "$BIN_TMP"
chmod 700 "$BIN_TMP"
mv -f "$BIN_TMP" "$BIN"
rm -f "$BIN_DOWNLOAD" "$CHECKSUMS_TMP"
say "verified binary installed permanently at $BIN."

fetch "$REPO_RAW/scripts/bootstrap-merlin.sh" "$MANAGER_TMP" || fail "could not download bootstrap-merlin.sh."
sh -n "$MANAGER_TMP" || fail "downloaded bootstrap-merlin.sh did not pass shell syntax validation."
chmod 755 "$MANAGER_TMP"
mv -f "$MANAGER_TMP" "$MANAGER"

if [ -f "$STARTED" ]; then
    awk '
        /# BEGIN WANIPv4-Watcher-Merlin/ { skip = 1; next }
        /# END WANIPv4-Watcher-Merlin/   { skip = 0; next }
        !skip { print }
    ' "$STARTED" > "$START_TMP"
else
    printf '#!/bin/sh\n' > "$START_TMP"
fi

cat >> "$START_TMP" <<'MERLIN_BOOT'

# BEGIN WANIPv4-Watcher-Merlin
(
    sleep 5
    /jffs/scripts/wanipv4-watcher install-cron || true
    /jffs/scripts/wanipv4-watcher start
) >/tmp/wanipv4-watcher-bootstrap.log 2>&1 &
# END WANIPv4-Watcher-Merlin
MERLIN_BOOT

chmod 755 "$START_TMP"
mv -f "$START_TMP" "$STARTED"
chmod 755 "$STARTED"

say "installed manager at $MANAGER and preserved other services-start content."
say "starting the permanent JFFS binary; normal boots will not download from GitHub."
"$MANAGER" stop >/dev/null 2>&1 || true
# Remove legacy file-based runtime logs left by v0.3.1; v0.3.2 logs only in RAM.
rm -f /tmp/wanipv4-watcher.log /tmp/wanipv4-watcher.log.1
if "$MANAGER" start; then
    say "installation complete. Open http://$("$NVRAM_BIN" get lan_ipaddr 2>/dev/null):10005 and complete configuration in the Web UI."
else
    say "the startup hook/config were installed, but the immediate service start failed."
    say "inspect /tmp/wanipv4-watcher-bootstrap.log and rerun: $MANAGER start"
    exit 1
fi
say "after saving the Web UI settings, the service activates the configured local check times."
say "config: $CONFIG ; runtime logs: RAM buffer (WebUI) ; startup diagnostics: /tmp/wanipv4-watcher-bootstrap.log"
