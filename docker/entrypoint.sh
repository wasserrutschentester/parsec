#!/bin/sh
set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

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

    # Keep the container running when no parsec command was supplied.
    if [ "$#" -eq 0 ]; then
        exec su-exec "$PUID:$PGID" tail -f /dev/null
    fi

    # Forward all supplied arguments to parsec as PUID:PGID.
    exec su-exec "$PUID:$PGID" /usr/local/bin/parsec "$@"
else
    # If Docker was configured with --user, PUID and PGID cannot be applied
    # because the entrypoint is no longer running as root.
    if [ "$#" -eq 0 ]; then
        exec tail -f /dev/null
    fi

    exec /usr/local/bin/parsec "$@"
fi
