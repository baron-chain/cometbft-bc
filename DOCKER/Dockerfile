# Specify build arguments
ARG GOLANG_BASE_IMAGE=golang:1.20-alpine
ARG CMTHOME=/cometbft
ARG USER=tmuser
ARG GROUP=tmuser

# Stage 1: Build CometBFT Binary
FROM --platform=$BUILDPLATFORM $GOLANG_BASE_IMAGE AS builder

RUN apk add --no-cache \
    make \
    git

WORKDIR /cometbft
COPY . .
RUN TARGETPLATFORM=$TARGETPLATFORM make build-linux

# Stage 2: Final Image
FROM $GOLANG_BASE_IMAGE

LABEL maintainer="hello@informal.systems" \
      description="Baron Chain node" \
      version="1.0" \
      org.opencontainers.image.source="https://github.com/baron-chain/cometbft-bc"

# Import build arguments
ARG CMTHOME
ARG USER
ARG GROUP

# Set environment variables
ENV CMTHOME=$CMTHOME \
    PROXY_APP=kvstore \
    MONIKER=dockernode \
    CHAIN_ID=dockerchain

# Install dependencies and setup user
RUN apk add --no-cache \
        curl \
        jq \
        bash && \
    addgroup -S $GROUP && \
    adduser -S -G $GROUP $USER -h "$CMTHOME" && \
    mkdir -p "$CMTHOME" && \
    chown -R $USER:$GROUP "$CMTHOME"

# Copy binary and entrypoint
COPY --from=builder --chown=$USER:$GROUP /cometbft/build/cometbft /usr/bin/cometbft
COPY --chown=$USER:$GROUP ./DOCKER/docker-entrypoint.sh /usr/local/bin/

# Configure container
USER $USER
WORKDIR $CMTHOME

# Expose ports (p2p, rpc, prometheus)
EXPOSE 26656 26657 26660

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:26657/status || exit 1

# Set stop signal
STOPSIGNAL SIGTERM

# Define volume for persistence
VOLUME ["$CMTHOME"]

# Set entrypoint and default command
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["node"]
