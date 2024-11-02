# syntax=docker/dockerfile:1

# Build arguments
ARG GOLANG_BASE_IMAGE=golang:1.20-alpine
ARG CMTHOME=/cometbft
ARG USER=tmuser
ARG GROUP=tmuser
ARG BUILD_VERSION="1.0"
ARG BUILD_DATE="2024"

# Stage 1: Build CometBFT Binary
FROM --platform=$BUILDPLATFORM ${GOLANG_BASE_IMAGE} AS builder

# Build essentials
RUN apk add --no-cache \
    make \
    git \
    && rm -rf /var/cache/apk/*

WORKDIR /cometbft
COPY . .

# Build for target platform
RUN TARGETPLATFORM=$TARGETPLATFORM make build-linux

# Stage 2: Final Image
FROM ${GOLANG_BASE_IMAGE}

# Import build arguments
ARG CMTHOME
ARG USER
ARG GROUP
ARG BUILD_VERSION
ARG BUILD_DATE

# Runtime dependencies
ENV RUNTIME_DEPS=" \
    curl \
    jq \
    bash \
    "

# Application configuration
ENV CMTHOME=${CMTHOME} \
    PROXY_APP=kvstore \
    MONIKER=dockernode \
    CHAIN_ID=dockerchain

# Set up non-root user and directories
RUN set -eux; \
    apk add --no-cache ${RUNTIME_DEPS} \
    && rm -rf /var/cache/apk/* \
    && addgroup -S ${GROUP} \
    && adduser -S -G ${GROUP} ${USER} -h "${CMTHOME}" \
    && mkdir -p "${CMTHOME}" \
    && chown -R ${USER}:${GROUP} "${CMTHOME}"

# Copy binary and entrypoint
COPY --from=builder --chown=${USER}:${GROUP} /cometbft/build/cometbft /usr/bin/cometbft
COPY --chown=${USER}:${GROUP} ./DOCKER/docker-entrypoint.sh /usr/local/bin/

# Make entrypoint executable
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Switch to non-root user
USER ${USER}
WORKDIR ${CMTHOME}

# Container configuration
EXPOSE 26656 26657 26660
VOLUME ["${CMTHOME}"]
STOPSIGNAL SIGTERM

# Health check configuration
HEALTHCHECK --interval=30s \
            --timeout=10s \
            --start-period=5s \
            --retries=3 \
            CMD curl -f http://localhost:26657/status || exit 1

# Metadata
LABEL org.opencontainers.image.title="Baron Chain Node" \
      org.opencontainers.image.description="Baron Chain CometBFT node" \
      org.opencontainers.image.version=${BUILD_VERSION} \
      org.opencontainers.image.created=${BUILD_DATE} \
      org.opencontainers.image.authors="hello@informal.systems" \
      org.opencontainers.image.vendor="Informal Systems" \
      org.opencontainers.image.source="https://github.com/baron-chain/cometbft-bc" \
      org.opencontainers.image.documentation="https://docs.baron.chain/cometbft"

# Entrypoint and default command
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["node"]
