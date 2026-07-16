#!/usr/bin/env bash
#-----------------#
# parsec          #
#-----------------#
# parsec CLI wrapper
container_name="parsec"
host_root="/mnt/user/data"
container_root="/data"

# Strip trailing slashes to prevent comparison/translation failures
host_root="${host_root%/}"
container_root="${container_root%/}"

workdir=""
args=()
exec_options=()

if ! docker container inspect "$container_name" >/dev/null 2>&1; then
    printf '%s\n' \
        "The $container_name container does not exist. Create it with the docker run command from the Docker guide." \
        >&2
    exit 1
fi

if [[ "$(docker inspect --format '{{.State.Running}}' "$container_name" 2>/dev/null)" != "true" ]]; then
    if ! docker start "$container_name" >/dev/null; then
        printf 'Error: Failed to start container "%s".\n' "$container_name" >&2
        exit 2
    fi
fi

if [[ "$PWD" == "$host_root" || "$PWD" == "$host_root/"* ]]; then
    workdir="${PWD/#$host_root/$container_root}"
fi

for arg in "$@"; do
    if [[ "$arg" == "$host_root" || "$arg" == "$host_root/"* ]]; then
        args+=("${arg/#$host_root/$container_root}")
    else
        args+=("$arg")
    fi
done

if [[ -t 0 && -t 1 ]]; then
    exec_options=(-it)
else
    exec_options=(-i)
fi

if [[ -n "$workdir" ]]; then
    docker exec "${exec_options[@]}" \
        -w "$workdir" \
        "$container_name" \
        /usr/local/bin/entrypoint.sh "${args[@]}"
else
    docker exec "${exec_options[@]}" \
        "$container_name" \
        /usr/local/bin/entrypoint.sh "${args[@]}"
fi
