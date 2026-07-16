# Docker

`parsec` can be run as a temporary Docker container for each command or from a persistent, idle container using `docker exec`. Its configuration is stored on the host and mounted into the container.

## Pull the Image

Pull the latest stable image:

```bash
docker pull uppollo/parsec:latest
```

Use `uppollo/parsec:nightly` for nightly builds or a version tag such as `uppollo/parsec:v0.4.2` to pin a specific release.

## Build the Image Locally

We provide a convenient Makefile target that natively builds the binary for your architecture and creates the Docker image:

```bash
git clone https://codeberg.org/upPollo/parsec.git
cd parsec
make docker
```

## Configuration

> [!NOTE]
> The configuration and media directory paths used in this guide (e.g., `/mnt/user/appdata/parsec` and `/mnt/user/data`) are common defaults for Unraid. If you are on a standard Linux distribution (like Ubuntu, Debian, or Arch) or macOS, you should adjust these paths to locations suitable for your system (e.g., `~/.config/parsec` or `/opt/parsec` for configuration, and `/media/data` or `~/media` for media storage).

Create the configuration directory:

```bash
mkdir -p /mnt/user/appdata/parsec
```

Create the default configuration file:

```bash
docker run --rm -it \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  uppollo/parsec:latest \
  config init
```

This creates:

```text
/mnt/user/appdata/parsec/config.toml
```

Edit the file and add the required settings and API keys:

```bash
nano /mnt/user/appdata/parsec/config.toml
```

Validate the configuration:

```bash
docker run --rm \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  uppollo/parsec:latest \
  config validate
```

For a full explanation of every configuration option, see the [Configuration guide](config.md).

### User and Group IDs

The container supports the `PUID` and `PGID` environment variables. These control the user and group IDs used when Parsec accesses mounted files and directories.

For Unraid, the usual values are:

```bash
-e PUID=99 \
-e PGID=100 \
```

On other systems, you can use the IDs of the current user:

```bash
-e PUID="$(id -u)" \
-e PGID="$(id -g)" \
```

Do not combine `PUID` and `PGID` with Docker’s `--user` option. The container must start as root so the entrypoint can apply the requested IDs and then drop privileges before running `parsec`.

## Running `parsec` with temporary interactive containers

Mount the configuration directory with every `docker run` command:

```bash
-v /mnt/user/appdata/parsec:/home/parsec/.config/parsec
```

Media files must also be mounted into the container. For example, this mount:

```bash
-v /path/to/data:/data
```

makes the host directory `/path/to/data` available inside the container as `/data`.

See the [Check](checks.md), [Identify](identify.md), and [Rename](rename.md) guides for command-specific options.

Add [`PUID` and `PGID`](#user-and-group-ids) as required to the `docker run` commands.

### `check`

```bash
docker run --rm \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  -v /path/to/data:/data:ro \
  uppollo/parsec:latest \
  check /data
```

### `identify`

```bash
docker run --rm -it \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  -v /path/to/data:/data:ro \
  uppollo/parsec:latest \
  identify /data/Loki.S01E01.mkv
```

### `rename`

Use a writable media mount when renaming files:

```bash
docker run --rm -it \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  -v /path/to/data:/data \
  uppollo/parsec:latest \
  rename /data/Movie.2023.1080p.mkv
```

## Running `parsec` with a persistent CLI container

Instead of creating a temporary container for every command, you can keep an idle container running and execute `parsec` with `docker exec`.

### Create the container using Docker Compose

Docker Compose is the recommended way to manage the persistent CLI container. Create a `compose.yaml` file (or use the one in `docker/compose.yaml`):

```yaml
services:
  parsec:
    image: uppollo/parsec:latest
    container_name: parsec
    restart: unless-stopped
    environment:
      - PUID=99  # Adjust to match your user ID (99 is Unraid default)
      - PGID=100 # Adjust to match your group ID (100 is Unraid default)
    volumes:
      - /mnt/user/appdata/parsec:/home/parsec/.config/parsec
      - /mnt/user/data:/data
```

Start the container in the background:

```bash
docker compose up -d
```

This mounts `/mnt/user/appdata/parsec` as the configuration directory and `/mnt/user/data` as the `/data` directory inside the container.

### Alternative: Create the container using `docker run`

If you prefer to start the container using standard `docker run` commands instead of Docker Compose:

```bash
docker run -d \
  --name parsec \
  --restart unless-stopped \
  -e PUID=99 \
  -e PGID=100 \
  -v /mnt/user/appdata/parsec:/home/parsec/.config/parsec \
  -v /mnt/user/data:/data \
  uppollo/parsec:latest
```

### Execute a `parsec` command

To execute a command use:

```bash
docker exec -it parsec entrypoint.sh \
  check /data/example.mkv
```

Do not use:

```bash
docker exec -it parsec parsec check /data/example.mkv
```

This bypasses `entrypoint.sh`, so the command runs as root instead of using the configured `PUID` and `PGID`.

### Optional: Add a wrapping script to the host

To use `parsec [command]` directly on the host instead of typing the full `docker exec` command, add the following shell script:

Copy the [parsec-wrapper.sh](../docker/parsec-wrapper.sh) script into a directory in your `PATH`, renaming it to `parsec` (without the `.sh` extension) and marking it as executable:

```bash
cp docker/parsec-wrapper.sh ~/.local/bin/parsec
chmod +x ~/.local/bin/parsec
```

Ensure your chosen directory is in your shell's `PATH`. If needed, you can add it by editing your shell configuration (e.g. `~/.bashrc` or `~/.zshrc`):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

The script starts the container when it is stopped. When the current directory is below `/mnt/user/data`, it uses the corresponding directory below `/data` as the working directory inside the container. Absolute arguments below `/mnt/user/data` are translated in the same way.

For example:

```zsh
cd /mnt/user/data/movies
parsec check .
parsec identify Movie.2023.1080p.mkv --dry-run
parsec rename /mnt/user/data/movies/Movie.2023.1080p.mkv --dry-run
```

Change `host_root`, `container_root`, and the matching volume mount together if your media is stored somewhere else.

If you want to use [shell completion](completion.md), it now works the same as if parsec was installed on your host system directly 

## Updating

To update, pull the latest image:

```bash
docker pull uppollo/parsec:latest
```

Temporary `docker run` commands use the newly pulled image immediately.

When using the persistent CLI container, remove the old container and recreate it with the command from [persistent CLI container](#running-parsec-with-a-persistent-cli-container).

Show the version contained in the image:

```bash
docker run --rm uppollo/parsec:latest --version
```
