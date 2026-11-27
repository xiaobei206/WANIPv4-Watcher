#!/bin/sh
# Remove only WANIPv4-Watcher's Merlin startup block and runtime files.
set -eu

SCRIPTS="/jffs/scripts"
CONFIG="/jffs/configs/wanipv4-watcher.json"
MANAGER="$SCRIPTS/wanipv4-watcher"
STARTED="$SCRIPTS/services-start"
TEMP="/tmp/services-start.wanipv4-watcher-uninstall.$$"
MARKER="# BEGIN WANIPv4-Watcher-Merlin"
END_MARKER="# END WANIPv4-Watcher-Merlin"

say() { printf '%s\n' "[WANIPv4-Watcher Merlin uninstall] $*"; }

if [ "$(id -u)" -ne 0 ]; then
    say "ERROR: please run as administrator/root over SSH."
    exit 1
fi

if [ -f "$STARTED" ] && grep -F "$MARKER" "$STARTED" >/dev/null 2>&1 &&
   ! grep -F "$END_MARKER" "$STARTED" >/dev/null 2>&1; then
    say "ERROR: incomplete startup markers; refusing to rewrite services-start."
    exit 1
fi

if [ -x "$MANAGER" ]; then
    "$MANAGER" stop >/dev/null 2>&1 || true
fi

if [ -f "$STARTED" ] && grep -F "$MARKER" "$STARTED" >/dev/null 2>&1; then
    awk '
        /# BEGIN WANIPv4-Watcher-Merlin/ { skip = 1; next }
        /# END WANIPv4-Watcher-Merlin/   { skip = 0; next }
        !skip { print }
    ' "$STARTED" > "$TEMP"
    chmod 755 "$TEMP"
    mv -f "$TEMP" "$STARTED"
    say "removed only the WANIPv4-Watcher block from services-start."
else
    say "no WANIPv4-Watcher startup block found."
fi

rm -f "$MANAGER" /tmp/wanipv4-watcher /tmp/wanipv4-watcher.new \
    /tmp/wanipv4-watcher-checksums.txt /tmp/wanipv4-watcher.pid \
    /tmp/wanipv4-watcher.log /tmp/wanipv4-watcher.log.1 \
    /tmp/wanipv4-watcher-bootstrap.log /tmp/wanipv4-watcher-merlin.new \
    /tmp/services-start.wanipv4-watcher.*
say "runtime files removed. Configuration/credentials were kept at $CONFIG."
say "to delete them too, review the file and then run: rm -f $CONFIG"
