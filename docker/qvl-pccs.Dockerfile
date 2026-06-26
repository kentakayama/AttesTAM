#
# Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
#
# SPDX-License-Identifier: BSD-2-Clause
#

FROM ubuntu:22.04 AS build

ARG DEBIAN_FRONTEND=noninteractive
ARG GO_VERSION=1.25.3
ARG INTEL_SGX_APT_REPOSITORY="deb [trusted=yes arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main"

SHELL ["/bin/bash", "-euo", "pipefail", "-c"]

WORKDIR /src

# Install Go, build tooling, and Intel DCAP quote verification headers/libs.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl build-essential git && \
    echo "${INTEL_SGX_APT_REPOSITORY}" > /etc/apt/sources.list.d/intel-sgx.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
      libsgx-dcap-quote-verify-dev \
      libsgx-dcap-default-qpl-dev && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz && \
    rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/go/bin:${PATH}"

# Pre-fetch module dependencies for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree and build the runtime binaries.
COPY . .
RUN GOOS=linux CGO_ENABLED=1 \
    go build -tags=intel_qvl -trimpath -o /out/attestam ./cmd/attestam && \
    GOOS=linux CGO_ENABLED=0 \
    go build -trimpath -o /out/admin-console ./cmd/admin-console


FROM ubuntu:22.04 AS runtime

ARG DEBIAN_FRONTEND=noninteractive
ARG INTEL_SGX_APT_REPOSITORY="deb [trusted=yes arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main"
ARG NODE_MAJOR=22

SHELL ["/bin/bash", "-euo", "pipefail", "-c"]

WORKDIR /app

# Install runtime Intel DCAP quote verification libraries and a local PCCS.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl openssl && \
    curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    echo "${INTEL_SGX_APT_REPOSITORY}" > /etc/apt/sources.list.d/intel-sgx.list && \
    apt-get update && \
    printf '#!/bin/sh\nexit 0\n' > /bin/systemctl && \
    chmod +x /bin/systemctl && \
    mkdir -p /run/systemd/system && \
    apt-get install -y --no-install-recommends \
      libsgx-dcap-quote-verify \
      libsgx-dcap-default-qpl \
      sgx-dcap-pccs && \
    rm -f /bin/systemctl && \
    rmdir /run/systemd/system /run/systemd 2>/dev/null || true && \
    rm -rf /var/lib/apt/lists/*

# Copy the compiled binaries and runtime resources.
COPY --from=build /out/attestam ./attestam
COPY --from=build /out/admin-console ./admin-console
COPY docker/sgx_default_qcnl.conf /etc/sgx_default_qcnl.conf
COPY docker/pccs_default.json /opt/intel/sgx-dcap-pccs/config/default.json
COPY cmd/admin-console/templates ./templates
COPY cmd/admin-console/static ./static

RUN cd /opt/intel/sgx-dcap-pccs && \
    npm ci --omit=dev --engine-strict && \
    mkdir -p /opt/intel/sgx-dcap-pccs/ssl_key && \
    openssl req -x509 -newkey rsa:2048 -nodes \
      -keyout /opt/intel/sgx-dcap-pccs/ssl_key/private.pem \
      -out /opt/intel/sgx-dcap-pccs/ssl_key/file.crt \
      -days 3650 \
      -subj "/CN=127.0.0.1" && \
    chown -R pccs:pccs /opt/intel/sgx-dcap-pccs

# Default configuration based on the CLI flags defined in cmd/attestam/main.go.
ENV ATTESTAM_ADDR=":8080" \
    ATTESTAM_CHALLENGE_SERVER="https://localhost:8443" \
    ATTESTAM_CHALLENGE_CONTENT_TYPE='application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"' \
    ATTESTAM_CHALLENGE_INSECURE_TLS="true" \
    ATTESTAM_CHALLENGE_TIMEOUT="1m0s" \
    ATTESTAM_INTEL_QVL_COLLATERAL_CACHE_DIR="/var/cache/attestam/intel-qvl-collateral" \
    LD_LIBRARY_PATH="/usr/lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu" \
    ADMIN_CONSOLE_PORT="9090" \
    ADMIN_CONSOLE_TAM_API_BASE="http://127.0.0.1:8080"

EXPOSE 8080 9090

ENTRYPOINT ["/bin/bash", "-c", "\
set -eu; \
cd /opt/intel/sgx-dcap-pccs; NODE_ENV=production runuser -u pccs -- node pccs_server.js & \
pccs_pid=$!; \
cd /app; \
./attestam & \
tam_pid=$!; \
trap 'kill \"$tam_pid\" \"$pccs_pid\" 2>/dev/null || true; wait \"$tam_pid\" \"$pccs_pid\" 2>/dev/null || true' INT TERM EXIT; \
exec ./admin-console -port \"${ADMIN_CONSOLE_PORT}\" -tam-api-base \"${ADMIN_CONSOLE_TAM_API_BASE}\" \
"]
