FROM alpine:3.21

# Docker Buildx automatic variables
ARG TARGETOS
ARG TARGETARCH
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

# Copy the pre-built binary for the specific architecture
COPY dist/parsec-${TARGETOS}-${TARGETARCH} /usr/local/bin/parsec
RUN chmod +x /usr/local/bin/parsec

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
