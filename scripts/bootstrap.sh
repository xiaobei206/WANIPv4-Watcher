#!/bin/sh
# Runtime bootstrap for Padavan. The executable and logs stay under /tmp (RAM).
set -eu
umask 077

REPO="xiaobei206/WANIPv4-Watcher"
CONFIG="${WANIPV4_CONFIG:-/tmp/wanipv4-watcher.json}"
BIN="/tmp/wanipv4-watcher"
NEW_BIN="/tmp/wanipv4-watcher.new"
CHECKSUMS="/tmp/wanipv4-watcher-checksums.txt"
PIDFILE="/tmp/wanipv4-watcher.pid"
LOG="/tmp/wanipv4-watcher.log"
ASSET="wanipv4-watcher-linux-mipsle-softfloat"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

say() {
    printf '%s\n' "[WANIPv4-Watcher] $*"
}

fetch() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 90 "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 90 -O "$dest" "$url"
    else
        say "ERROR: neither curl nor wget is available; cannot download the program."
        return 1
    fi
}

sha256_file() {
    target="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$target" | awk '{print $1}'
    elif command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 "$target" | awk '{print $NF}'
    else
        return 1
    fi
}

is_our_process() {
    pid="$1"
    [ -r "/proc/$pid/cmdline" ] || return 1
    cmdline=$(tr '\000' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)
    case "$cmdline" in
        *"/tmp/wanipv4-watcher"*) return 0 ;;
        *) return 1 ;;
    esac
}

stop_process() {
    if [ -r "$PIDFILE" ]; then
        oldpid=$(cat "$PIDFILE" 2>/dev/null || true)
        case "$oldpid" in
            ''|*[!0-9]*) ;;
            *)
                if kill -0 "$oldpid" 2>/dev/null && is_our_process "$oldpid"; then
                    kill "$oldpid" 2>/dev/null || true
                    n=0
                    while kill -0 "$oldpid" 2>/dev/null && [ "$n" -lt 5 ]; do
                        sleep 1
                        n=$((n + 1))
                    done
                    if kill -0 "$oldpid" 2>/dev/null && is_our_process "$oldpid"; then
                        kill -9 "$oldpid" 2>/dev/null || true
                    fi
                fi
                ;;
        esac
        rm -f "$PIDFILE"
    fi
}

case "${1:-start}" in
    stop)
        stop_process
        say "stopped (if it was running)"
        exit 0
        ;;
    status)
        if [ -r "$PIDFILE" ]; then
            p=$(cat "$PIDFILE" 2>/dev/null || true)
            case "$p" in
                ''|*[!0-9]*) ;;
                *) if kill -0 "$p" 2>/dev/null && is_our_process "$p"; then say "running, pid=$p"; exit 0; fi ;;
            esac
        fi
        say "not running"
        exit 1
        ;;
    start) ;;
    *) say "usage: $0 [start|stop|status]"; exit 2 ;;
esac

# Short Padavan Web UI startup snippets pass SMTP settings through environment
# variables. The launcher creates its own private runtime JSON config in /tmp.
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

if [ -n "${WANIPV4_SMTP_USER:-}" ] || [ -n "${WANIPV4_SMTP_AUTH_CODE:-}" ] || [ -n "${WANIPV4_SMTP_TO:-}" ]; then
    if [ -z "${WANIPV4_SMTP_USER:-}" ] || [ -z "${WANIPV4_SMTP_AUTH_CODE:-}" ] || [ -z "${WANIPV4_SMTP_TO:-}" ]; then
        say "ERROR: set WANIPV4_SMTP_USER, WANIPV4_SMTP_AUTH_CODE and WANIPV4_SMTP_TO together."
        exit 1
    fi
    if printf '%s\n' "$WANIPV4_SMTP_USER" "$WANIPV4_SMTP_AUTH_CODE" "$WANIPV4_SMTP_TO" | grep -E 'YOUR_|REPLACE_WITH|example.com' >/dev/null 2>&1; then
        say "ERROR: SMTP variables still contain example placeholders."
        exit 1
    fi

    CONFIG="${WANIPV4_CONFIG:-/tmp/wanipv4-watcher.json}"
    SMTP_USER_JSON=$(json_escape "$WANIPV4_SMTP_USER")
    SMTP_AUTH_JSON=$(json_escape "$WANIPV4_SMTP_AUTH_CODE")
    SMTP_TO_JSON=$(json_escape "$WANIPV4_SMTP_TO")
    cat > "$CONFIG" <<WANIPV4_CONFIG
{
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
  "log_file": "/tmp/wanipv4-watcher.log",
  "smtp": {
    "host": "smtp.qq.com",
    "port": 465,
    "username": "$SMTP_USER_JSON",
    "password": "$SMTP_AUTH_JSON",
    "from": "$SMTP_USER_JSON",
    "to": ["$SMTP_TO_JSON"]
  }
}
WANIPV4_CONFIG
    chmod 600 "$CONFIG"
    unset WANIPV4_SMTP_USER WANIPV4_SMTP_AUTH_CODE WANIPV4_SMTP_TO
    say "generated runtime config at $CONFIG (credentials stored only in RAM)"
elif [ ! -r "$CONFIG" ] && [ "$CONFIG" = "/tmp/wanipv4-watcher.json" ] && [ -r "/etc/storage/wanipv4-watcher.json" ]; then
    # Backward compatibility for the legacy SSH installer.
    CONFIG="/etc/storage/wanipv4-watcher.json"
fi

if [ ! -r "$CONFIG" ]; then
    say "ERROR: config not found at $CONFIG. Set SMTP variables in the Padavan startup script or configure the legacy JSON file."
    exit 1
fi
if grep -E 'YOUR_QQ_MAIL|REPLACE_WITH|YOUR_RECIPIENT_MAIL|example.com' "$CONFIG" >/dev/null 2>&1; then
    say "ERROR: configuration still contains template placeholders. Edit $CONFIG first."
    exit 1
fi
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v openssl >/dev/null 2>&1; then
    say "ERROR: neither sha256sum nor openssl is available; refusing to run an unchecked download."
    exit 1
fi

rm -f "$NEW_BIN" "$CHECKSUMS"
say "downloading current release asset"
if ! fetch "$BASE_URL/$ASSET" "$NEW_BIN"; then
    rm -f "$NEW_BIN"
    say "ERROR: binary download failed; check Internet/GitHub reachability."
    exit 1
fi
if ! fetch "$BASE_URL/checksums.txt" "$CHECKSUMS"; then
    rm -f "$NEW_BIN" "$CHECKSUMS"
    say "ERROR: checksum manifest download failed; refusing to execute the binary."
    exit 1
fi
EXPECTED=$(grep "  $ASSET$" "$CHECKSUMS" | head -n 1 | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
    rm -f "$NEW_BIN" "$CHECKSUMS"
    say "ERROR: the checksum manifest does not contain $ASSET."
    exit 1
fi
if ! ACTUAL=$(sha256_file "$NEW_BIN"); then
    rm -f "$NEW_BIN" "$CHECKSUMS"
    say "ERROR: could not calculate SHA-256; refusing to execute the binary."
    exit 1
fi
if [ "$EXPECTED" != "$ACTUAL" ]; then
    rm -f "$NEW_BIN" "$CHECKSUMS"
    say "ERROR: SHA-256 validation failed; refusing to execute the binary."
    exit 1
fi
chmod 700 "$NEW_BIN"
mv -f "$NEW_BIN" "$BIN"
rm -f "$CHECKSUMS"

if ! "$BIN" -config "$CONFIG" -check-config >> "$LOG" 2>&1; then
    say "ERROR: configuration is invalid. Diagnostic: tail -n 50 $LOG"
    exit 1
fi

stop_process
nohup "$BIN" -config "$CONFIG" >/dev/null 2>&1 </dev/null &
PID=$!
echo "$PID" > "$PIDFILE"
say "started pid=$PID; logs: $LOG"
exit 0
