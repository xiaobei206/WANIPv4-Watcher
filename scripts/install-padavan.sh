#!/bin/sh
# One-time installer for Padavan. Run as root, usually via SSH.
set -eu

REPO_RAW="https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main"
STORAGE="/etc/storage"
CONFIG="$STORAGE/wanipv4-watcher.json"
STARTED="$STORAGE/started_script.sh"
EXAMPLE="/tmp/wanipv4-watcher-config.example.json"
MARKER="# BEGIN WANIPv4-Watcher"

say() { printf '%s\n' "[WANIPv4-Watcher installer] $*"; }

fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 60 "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 60 -O "$2" "$1"
    else
        say "ERROR: neither curl nor wget is available."
        exit 1
    fi
}

mkdir -p "$STORAGE"
if [ ! -f "$CONFIG" ]; then
    say "downloading configuration template"
    fetch "$REPO_RAW/config.example.json" "$EXAMPLE"
    cp "$EXAMPLE" "$CONFIG"
    chmod 600 "$CONFIG"
    rm -f "$EXAMPLE"
    say "created $CONFIG; edit the QQ SMTP username, authorization code, sender and recipient first."
else
    say "existing config preserved: $CONFIG"
fi

if [ ! -f "$STARTED" ]; then
    printf '#!/bin/sh\n' > "$STARTED"
fi
if grep -F "$MARKER" "$STARTED" >/dev/null 2>&1; then
    say "startup hook already exists; no duplicate added."
else
    cat >> "$STARTED" <<'PADAVAN_BOOT'

# BEGIN WANIPv4-Watcher
# Padavan calls this script after router startup/network initialization.
(
    URL='https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh'
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o /tmp/wanipv4-watcher-bootstrap.sh
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 60 -O /tmp/wanipv4-watcher-bootstrap.sh "$URL"
    else
        exit 1
    fi
    [ -s /tmp/wanipv4-watcher-bootstrap.sh ] && /bin/sh /tmp/wanipv4-watcher-bootstrap.sh start
) >/tmp/wanipv4-watcher-bootstrap.log 2>&1 &
# END WANIPv4-Watcher
PADAVAN_BOOT
    chmod 755 "$STARTED"
    say "startup hook appended to $STARTED (existing contents were preserved)."
fi

say "installed. Edit $CONFIG, then run: /bin/sh $STARTED"
say "After configuring SMTP, inspect startup diagnostics with: tail -n 80 /tmp/wanipv4-watcher-bootstrap.log"
say "Then test mail with: /tmp/wanipv4-watcher -config $CONFIG -test-email"
say "If the executable is not present yet, run: /bin/sh /tmp/wanipv4-watcher-bootstrap.sh start"
