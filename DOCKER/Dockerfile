# syntax=docker/dockerfile:1
ARG GOLANG_BASE_IMAGE=golang:1.20-alpine
ARG CMTHOME=/baronchain
ARG USER=bcuser
ARG GROUP=bcuser
ARG BUILD_VERSION="1.0"
ARG BUILD_DATE="2024"

FROM --platform=$BUILDPLATFORM ${GOLANG_BASE_IMAGE} AS builder
RUN apk add --no-cache make git
WORKDIR /baronchain
COPY . .
RUN TARGETPLATFORM=$TARGETPLATFORM make build-linux

FROM ${GOLANG_BASE_IMAGE}
ARG CMTHOME
ARG USER
ARG GROUP
ARG BUILD_VERSION
ARG BUILD_DATE

ENV RUNTIME_DEPS="curl jq bash" \
    CMTHOME=${CMTHOME} \
    PROXY_APP=kvstore \
    MONIKER=baronnode \
    CHAIN_ID=baronchain

RUN apk add --no-cache ${RUNTIME_DEPS} && \
    addgroup -S ${GROUP} && \
    adduser -S -G ${GROUP} ${USER} -h "${CMTHOME}" && \
    mkdir -p "${CMTHOME}" && \
    chown -R ${USER}:${GROUP} "${CMTHOME}"

COPY --from=builder --chown=${USER}:${GROUP} /baronchain/build/cometbft /usr/bin/baronchain
COPY --chown=${USER}:${GROUP} ./DOCKER/docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER ${USER}
WORKDIR ${CMTHOME}

EXPOSE 26656 26657 26660
VOLUME ["${CMTHOME}"]
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s \
            --timeout=10s \
            --start-period=5s \
            --retries=3 \
            CMD curl -f http://localhost:26657/status || exit 1

LABEL org.opencontainers.image.title="Baron Chain Node" \
      org.opencontainers.image.description="Baron Chain Network Node" \
      org.opencontainers.image.version=${BUILD_VERSION} \
      org.opencontainers.image.created=${BUILD_DATE} \
      org.opencontainers.image.authors="liviu@baronchain.org" \
      org.opencontainers.image.vendor="Baron Chain" \
      org.opencontainers.image.source="https://github.com/baron-chain/cometbft-bc" \
      org.opencontainers.image.documentation="https://docs.baronchain.org"

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["node"]
