#!/bin/sh
set -eux

case "$PUID" in
    ''|*[!0-9]*)
        echo "Error: PUID must be a numeric user ID." >&2
        exit 1
        ;;
esac

case "$PGID" in
    ''|*[!0-9]*)
        echo "Error: PGID must be a numeric group ID." >&2
        exit 1
        ;;
esac

# When the container starts as root, prepare the configuration directory
# and drop privileges to the requested UID and GID.
if [ "$(id -u)" -eq 0 ]; then
    mkdir -p "$HOME/.config/parsec"
    chown -R "$PUID:$PGID" "$HOME/.config/parsec"
fi

# Keep the container running when the internal idle command was supplied.
if [ "${1:-}" = "idle" ]; then
    while true; do
        sleep 1000;
    done
fi

# Forward all supplied arguments to parsec as PUID:PGID.
if [ "$(id -u)" -eq 0 ]; then
    exec su-exec "$PUID:$PGID" env HOME="$HOME" /usr/local/bin/parsec "$@"
else
    # If Docker was configured with --user, PUID and PGID cannot be applied
    # because the entrypoint is no longer running as root.
    exec /usr/local/bin/parsec "$@"
fi
