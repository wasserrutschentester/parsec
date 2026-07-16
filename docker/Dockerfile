# 1. Build stage
FROM golang:1.25-alpine AS builder

# Accept build-time variables
ARG VERSION=dev

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy the repository into the container
WORKDIR /app
COPY . .

# Build parsec from the main package with static linking
# -buildvcs=false prevents Go from adding +dirty suffix from git status

RUN CGO_ENABLED=0 go build \
    -mod=vendor \
    -tags no_update \
    -buildvcs=false \
    -trimpath \
    -ldflags "-s -w -X codeberg.org/upPollo/parsec/cmd.Version=${VERSION}" \
    -o /parsec .

# 2. Runtime stage
FROM alpine:3.21

ARG VERSION=dev

LABEL org.opencontainers.image.title="parsec" \
      org.opencontainers.image.description="parse, check and create releases to send far into space" \
      org.opencontainers.image.url="https://codeberg.org/upPollo/parsec" \
      org.opencontainers.image.source="https://codeberg.org/upPollo/parsec" \
      org.opencontainers.image.documentation="https://codeberg.org/upPollo/parsec/src/branch/main/docs/README.md" \
      org.opencontainers.image.licenses="WTFPL" \
      org.opencontainers.image.vendor="upPollo" \
      org.opencontainers.image.version="${VERSION}"


# Install runtime dependencies
RUN apk add --no-cache \
    mediainfo \
    mkvtoolnix \
    ca-certificates \
    tzdata \
    su-exec \
    tini

# Copy the binary from the builder stage
COPY --from=builder /parsec /usr/local/bin/parsec

# Copy the container entrypoint
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh

# Create the home and configuration directories
RUN mkdir -p /home/parsec/.config/parsec && \
    chmod 755 /usr/local/bin/entrypoint.sh

# Set the default runtime UID, GID, and home directory
ENV PUID=1000 \
    PGID=1000 \
    HOME=/home/parsec

WORKDIR /home/parsec

# Set entrypoint
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/entrypoint.sh"]
