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
LOG="/tmp/wanipv4-watcher.log"
BOOTLOG="/tmp/wanipv4-watcher-bootstrap.log"
ASSET="wanipv4-watcher-linux-arm64"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

say() { printf '%s\n' "[WANIPv4-Watcher Merlin] $*"; }

fetch() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 90 "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 90 -O "$dest" "$url"
    else
        say "ERROR: curl or wget is required to download the ARM64 binary."
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
        say "ERROR: sha256sum or openssl is required; refusing to run an unchecked binary."
        return 1
    fi
}

is_our_process() {
    pid="$1"
    [ -r "/proc/$pid/cmdline" ] || return 1
    cmdline="$(tr '\000' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
    case "$cmdline" in
        *"/jffs/wanipv4-watcher/wanipv4-watcher"*) return 0 ;;
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
    if grep -E 'YOUR_QQ_MAIL|REPLACE_WITH|YOUR_RECIPIENT_MAIL|example.com' "$CONFIG" >/dev/null 2>&1; then
        say "ERROR: config still contains example placeholders; edit $CONFIG."
        return 1
    fi
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
    if ! "$BIN" -config "$CONFIG" -check-config >> "$BOOTLOG" 2>&1; then
        say "ERROR: configuration validation failed; see $BOOTLOG and $LOG."
        return 1
    fi
    nohup "$BIN" -config "$CONFIG" >/dev/null 2>&1 </dev/null &
    pid=$!
    echo "$pid" > "$PIDFILE"
    sleep 1
    if running_pid >/dev/null 2>&1; then
        say "started pid=$pid; IP checks scheduled for 07:00 and 19:00; Web UI uses LAN TCP port 10005."
        say "logs: $LOG; startup diagnostics: $BOOTLOG"
        return 0
    fi
    rm -f "$PIDFILE"
    say "ERROR: process did not remain running; inspect $BOOTLOG and $LOG."
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
        stop_process
        ensure_config
        ensure_binary
        "$BIN" -config "$CONFIG" -update
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
        printf '\\nWANIPv4-Watcher Merlin\\n1) 安装/修复\\n2) 更新\\n3) 状态\\n4) 卸载\\n5) 日志\\n0) 退出\\n选择: '
        IFS= read -r choice
        case "$choice" in
            1) if command -v curl >/dev/null 2>&1; then curl -fsSL "$REPO_RAW/scripts/install-merlin.sh" -o /tmp/install-merlin.sh && sh /tmp/install-merlin.sh; else say "curl is required"; fi ;;
            2) "$MANAGER" update ;;
            3) "$MANAGER" status || true ;;
            4) if command -v curl >/dev/null 2>&1; then curl -fsSL "$REPO_RAW/scripts/uninstall-merlin.sh" -o /tmp/uninstall-merlin.sh && sh /tmp/uninstall-merlin.sh; fi ;;
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
        tail -n 100 "$LOG" 2>/dev/null || true
        tail -n 100 "$BOOTLOG" 2>/dev/null || true
        ;;
    *)
        say "usage: $0 {start|stop|restart|update|status|guard|install-cron|menu|check-config|once|test-email|logs}"
        exit 2
        ;;
esac
