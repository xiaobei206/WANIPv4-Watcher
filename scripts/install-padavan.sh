#!/bin/sh
# One-time installer/updater for Padavan. Run as root, usually via SSH.
set -eu

REPO_RAW="https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main"
STORAGE="/etc/storage"
CONFIG="$STORAGE/wanipv4-watcher.json"
STARTED="$STORAGE/started_script.sh"
EXAMPLE="/tmp/wanipv4-watcher-config.example.json"
TEMP_START="/tmp/started_script.wanipv4-watcher.$$"
MARKER="# BEGIN WANIPv4-Watcher"
END_MARKER="# END WANIPv4-Watcher"

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

persist_storage() {
    SAVER=""
    for candidate in /sbin/mtd_storage.sh /usr/sbin/mtd_storage.sh; do
        if [ -x "$candidate" ]; then
            SAVER="$candidate"
            break
        fi
    done
    if [ -z "$SAVER" ]; then
        say "ERROR: cannot find mtd_storage.sh; files have NOT been confirmed saved to flash."
        say "In the Padavan web UI, open System Management and use 'Save /etc/storage/ contents to flash', then apply."
        return 1
    fi
    if "$SAVER" save; then
        say "Padavan /etc/storage save command completed."
    else
        say "ERROR: $SAVER save failed; use the Padavan web UI to save /etc/storage to flash."
        return 1
    fi
}

mkdir -p "$STORAGE"
if [ ! -f "$CONFIG" ]; then
    say "downloading configuration template"
    fetch "$REPO_RAW/config.example.json" "$EXAMPLE"
    cp "$EXAMPLE" "$CONFIG"
    chmod 600 "$CONFIG"
    rm -f "$EXAMPLE"
    say "created $CONFIG; edit the QQ SMTP username, authorization code, sender and recipient."
else
    say "existing config preserved: $CONFIG"
fi

# Refresh our own startup block on every installer run. This repairs older
# installs too, while preserving the user's unrelated startup-script content.
if [ -f "$STARTED" ]; then
    if grep -F "$MARKER" "$STARTED" >/dev/null 2>&1 && ! grep -F "$END_MARKER" "$STARTED" >/dev/null 2>&1; then
        say "ERROR: startup marker is incomplete; refusing to rewrite $STARTED to avoid losing user content."
        exit 1
    fi
    awk '
        /# BEGIN WANIPv4-Watcher/ { skip = 1; next }
        /# END WANIPv4-Watcher/   { skip = 0; next }
        !skip { print }
    ' "$STARTED" > "$TEMP_START"
else
    printf '#!/bin/sh\n' > "$TEMP_START"
fi

cat >> "$TEMP_START" <<'PADAVAN_BOOT'

# BEGIN WANIPv4-Watcher
# Padavan calls started_script.sh after router startup/network initialization.
(
    CONFIG='/etc/storage/wanipv4-watcher.json'
    if [ ! -r "$CONFIG" ] || grep -E 'YOUR_QQ_MAIL|REPLACE_WITH|YOUR_RECIPIENT_MAIL|example.com' "$CONFIG" >/dev/null 2>&1; then
        echo "[WANIPv4-Watcher] SMTP config is missing or still a template; configure it and save /etc/storage to flash." 
        exit 0
    fi

    URL='https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh'
    attempt=1
    while [ "$attempt" -le 12 ]; do
        downloaded=0
        if command -v curl >/dev/null 2>&1; then
            curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o /tmp/wanipv4-watcher-bootstrap.sh && downloaded=1
        elif command -v wget >/dev/null 2>&1; then
            wget -q -T 60 -O /tmp/wanipv4-watcher-bootstrap.sh "$URL" && downloaded=1
        fi
        if [ "$downloaded" = "1" ] && [ -s /tmp/wanipv4-watcher-bootstrap.sh ]; then
            if /bin/sh /tmp/wanipv4-watcher-bootstrap.sh start; then
                exit 0
            fi
        fi
        echo "[WANIPv4-Watcher] startup attempt $attempt failed; retrying in 10 seconds."
        rm -f /tmp/wanipv4-watcher-bootstrap.sh
        sleep 10
        attempt=$((attempt + 1))
    done
    echo "[WANIPv4-Watcher] startup failed after 12 attempts. Check GitHub reachability and /etc/storage/wanipv4-watcher.json."
) >/tmp/wanipv4-watcher-bootstrap.log 2>&1 &
# END WANIPv4-Watcher
PADAVAN_BOOT

chmod 755 "$TEMP_START"
mv -f "$TEMP_START" "$STARTED"
chmod 600 "$CONFIG"
chmod 755 "$STARTED"
say "startup block refreshed in $STARTED; unrelated user script contents were preserved."

# Important: Padavan's /etc/storage changes must be explicitly committed to flash.
# This call also repairs existing installs whose previous startup block was never saved.
persist_storage

say "installed. Edit $CONFIG and fill in the QQ SMTP username, authorization code, sender and recipient."
say "AFTER editing the config, run: /sbin/mtd_storage.sh save"
say "Equivalent web UI action: System Management -> Save /etc/storage/ contents to flash -> Apply."
say "Then start immediately with: curl -fsSL $REPO_RAW/scripts/bootstrap.sh -o /tmp/wanipv4-watcher-bootstrap.sh && sh /tmp/wanipv4-watcher-bootstrap.sh start"
say "Diagnostics: tail -n 80 /tmp/wanipv4-watcher-bootstrap.log; tail -n 80 /tmp/wanipv4-watcher.log"
