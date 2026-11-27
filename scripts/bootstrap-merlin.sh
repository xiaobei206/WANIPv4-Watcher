#!/bin/sh
# Runtime controller for WANIPv4-Watcher on ASUSWRT-Merlin.
set -eu
umask 077

REPO="xiaobei206/WANIPv4-Watcher"
REPO_RAW="https://raw.githubusercontent.com/$REPO/main"
CONFIG="/jffs/configs/wanipv4-watcher.json"
APP_DIR="/jffs/wanipv4-watcher"
BIN="$APP_DIR/wanipv4-watcher"
MANAGER="/jffs/scripts/wanipv4-watcher"
PIDFILE="/tmp/wanipv4-watcher.pid"
BOOTLOG="/tmp/wanipv4-watcher-bootstrap.log"
ASSET="wanipv4-watcher-linux-arm64"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
say() { printf '%s\n' "[WANIPv4-Watcher Merlin] $*"; }

NVRAM_BIN="$(command -v nvram 2>/dev/null || true)"
if [ -z "$NVRAM_BIN" ] && [ -x /bin/nvram ]; then NVRAM_BIN="/bin/nvram"; fi
if [ -z "$NVRAM_BIN" ]; then
    say "ERROR: nvram command not found (checked PATH and /bin/nvram)."
    exit 1
fi

fetch() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL --connect-timeout 10 --max-time 120 "$url" -o "$dest"; then
            return 0
        fi
        rm -f "$dest"
    fi
    if command -v wget >/dev/null 2>&1; then
        if wget -q -T 180 -O "$dest" "$url"; then
            return 0
        fi
        rm -f "$dest"
    fi
    say "ERROR: download failed (curl and/or wget); verify network access or configured proxy environment."; return 1
}

sha256_file() {
    target="$1"
    sha_bin="$(command -v sha256sum 2>/dev/null || true)"
    if [ -n "$sha_bin" ]; then
        "$sha_bin" "$target" | awk '{print $1}'
        return $?
    fi
    ssl_bin="$(command -v openssl 2>/dev/null || true)"
    if [ -z "$ssl_bin" ]; then
        for candidate in /usr/sbin/openssl /usr/bin/openssl /bin/openssl /koolshare/bin/openssl; do
            if [ -x "$candidate" ]; then ssl_bin="$candidate"; break; fi
        done
    fi
    if [ -n "$ssl_bin" ]; then
        "$ssl_bin" dgst -sha256 "$target" | awk '{print $NF}'
        return $?
    fi
    say "ERROR: SHA-256 tool missing; refusing to run an unchecked binary."
    return 1
}

is_our_process() {
    pid="$1"
    [ -r "/proc/$pid/cmdline" ] || return 1
    cmdline="$(tr '\000' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
    case "$cmdline" in
        *"/jffs/wanipv4-watcher/wanipv4-watcher"*|*"/tmp/wanipv4-watcher"*) return 0 ;;
        *) return 1 ;;
    esac
}

running_pid() {
    if [ -r "$PIDFILE" ]; then
        candidate="$(cat "$PIDFILE" 2>/dev/null || true)"
        case "$candidate" in
            ''|*[!0-9]*) ;;
            *)
                if kill -0 "$candidate" 2>/dev/null && is_our_process "$candidate"; then
                    printf '%s\n' "$candidate"
                    return 0
                fi
                ;;
        esac
    fi
    return 1
}

stop_process() {
    oldpid="$(running_pid 2>/dev/null || true)"
    if [ -z "$oldpid" ]; then
        rm -f "$PIDFILE"
        say "not running"
        return 0
    fi
    kill "$oldpid" 2>/dev/null || true
    n=0
    while kill -0 "$oldpid" 2>/dev/null && [ "$n" -lt 10 ]; do
        sleep 1
        n=$((n + 1))
    done
    if kill -0 "$oldpid" 2>/dev/null && is_our_process "$oldpid"; then
        kill -9 "$oldpid" 2>/dev/null || true
    fi
    rm -f "$PIDFILE"
    say "stopped pid=$oldpid"
}

download_binary() {
    say "ERROR: permanent JFFS binary is missing; rerun install-merlin.sh to repair."
    return 1
}

ensure_config() {
    if [ ! -r "$CONFIG" ]; then
        say "ERROR: config missing at $CONFIG; rerun install-merlin.sh."
        return 1
    fi
    chmod 600 "$CONFIG"
}

ensure_binary() {
    if [ ! -x "$BIN" ]; then
        download_binary
    fi
}

start_service() {
    if pid="$(running_pid)"; then
        say "already running, pid=$pid"
        return 0
    fi
    ensure_config
    ensure_binary
    : > "$BOOTLOG"
    if ! "$BIN" -config "$CONFIG" -check-config >> "$BOOTLOG" 2>&1; then
        say "ERROR: base configuration validation failed; inspect startup diagnostics at $BOOTLOG and runtime logs in the Web UI RAM buffer."
        return 1
    fi
    nohup "$BIN" -config "$CONFIG" >/dev/null 2>&1 </dev/null &
    pid=$!
    echo "$pid" > "$PIDFILE"
    sleep 1
    if running_pid >/dev/null 2>&1; then
        say "started pid=$pid; complete settings in Web UI at http://$("$NVRAM_BIN" get lan_ipaddr 2>/dev/null):10005."
        say "runtime logs: WebUI RAM buffer; startup diagnostics: $BOOTLOG"
        return 0
    fi
    rm -f "$PIDFILE"
    say "ERROR: process did not remain running; inspect startup diagnostics at $BOOTLOG and runtime logs in the Web UI RAM buffer."
    return 1
}

case "${1:-start}" in
    start)
        start_service
        ;;
    stop)
        stop_process
        ;;
    restart)
        stop_process
        start_service
        ;;
    update)
        ensure_config
        ensure_binary
        # Replace the file while the old process is still mapped; if download or
        # verification fails, leave the current service untouched.
        "$BIN" -config "$CONFIG" -update
        stop_process
        start_service
        ;;
    guard)
        if pid="$(running_pid)"; then
            say "guard: service running, pid=$pid"
        else
            say "guard: service not running; restarting"
            start_service
        fi
        ;;
    install-cron)
        command -v cru >/dev/null 2>&1 || { say "ERROR: cru command not found; watchdog not registered."; exit 1; }
        cru d wanipv4_watcher_guard_0640 2>/dev/null || true
        cru d wanipv4_watcher_guard_1840 2>/dev/null || true
        cru a wanipv4_watcher_guard_0640 "40 6 * * * $MANAGER guard"
        cru a wanipv4_watcher_guard_1840 "40 18 * * * $MANAGER guard"
        say "registered guards at 06:40 and 18:40 router local time."
        ;;
    menu)
        printf '\nWANIPv4-Watcher Merlin\n1) 安装/修复\n2) 更新\n3) 状态\n4) 卸载\n5) 日志\n0) 退出\n选择: '
        IFS= read -r choice
        case "$choice" in
            1) fetch "$REPO_RAW/scripts/install-merlin.sh" /tmp/install-merlin.sh && sh /tmp/install-merlin.sh ;;
            2) "$MANAGER" update ;;
            3) "$MANAGER" status || true ;;
            4) fetch "$REPO_RAW/scripts/uninstall-merlin.sh" /tmp/uninstall-merlin.sh && sh /tmp/uninstall-merlin.sh ;;
            5) "$MANAGER" logs ;;
            *) say "exit" ;;
        esac
        ;;
    status)
        if pid="$(running_pid)"; then
            say "running, pid=$pid"
            exit 0
        fi
        say "not running"
        exit 1
        ;;
    check-config)
        ensure_config
        ensure_binary
        "$BIN" -config "$CONFIG" -check-config
        ;;
    once)
        ensure_config
        ensure_binary
        "$BIN" -config "$CONFIG" -once
        ;;
    test-email)
        ensure_config
        ensure_binary
        "$BIN" -config "$CONFIG" -test-email
        ;;
    logs)
        say "runtime logs are stored only in RAM and are viewed in the Web UI; below are startup diagnostics."
        tail -n 100 "$BOOTLOG" 2>/dev/null || true
        ;;
    *)
        say "usage: $0 {start|stop|restart|update|status|guard|install-cron|menu|check-config|once|test-email|logs}"
        exit 2
        ;;
esac
