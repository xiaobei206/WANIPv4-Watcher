#!/bin/sh
# Remove only the startup block installed by WANIPv4-Watcher. Keep user config.
set -eu
STORAGE="/etc/storage"
STARTED="$STORAGE/started_script.sh"
TEMP="/tmp/started_script.wanipv4-watcher.$$"

say() { printf '%s\n' "[WANIPv4-Watcher uninstall] $*"; }

if [ -f /tmp/wanipv4-watcher-bootstrap.sh ]; then
    /bin/sh /tmp/wanipv4-watcher-bootstrap.sh stop >/dev/null 2>&1 || true
fi

if [ -f "$STARTED" ] && grep -F '# BEGIN WANIPv4-Watcher' "$STARTED" >/dev/null 2>&1; then
    awk '
        /# BEGIN WANIPv4-Watcher/ { skip = 1; next }
        /# END WANIPv4-Watcher/   { skip = 0; next }
        !skip { print }
    ' "$STARTED" > "$TEMP"
    cat "$TEMP" > "$STARTED"
    rm -f "$TEMP"
    chmod 755 "$STARTED"
    say "removed the WANIPv4-Watcher startup block from $STARTED"
else
    say "startup block was not found; no startup changes were made"
fi

rm -f /tmp/wanipv4-watcher /tmp/wanipv4-watcher.new \
    /tmp/wanipv4-watcher.pid /tmp/wanipv4-watcher-checksums.txt \
    /tmp/wanipv4-watcher-bootstrap.sh /tmp/wanipv4-watcher-bootstrap.log \
    /tmp/wanipv4-watcher.log /tmp/wanipv4-watcher.log.1
say "runtime files removed. Configuration and QQ credentials were kept at $STORAGE/wanipv4-watcher.json"
