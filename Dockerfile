#
# Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
#
# SPDX-License-Identifier: BSD-2-Clause
#

FROM golang:1.25.3-alpine AS build

WORKDIR /src

# Install build tooling and certificate bundle for module downloads.
RUN apk add --no-cache build-base ca-certificates git

# Pre-fetch module dependencies for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree and build the runtime binaries.
COPY . .
RUN GOOS=linux \
    go build -trimpath -o /out/tam-over-http ./cmd/tam-over-http && \
    GOOS=linux \
    go build -trimpath -o /out/admin-console ./cmd/admin-console


FROM alpine:3.20 AS runtime

WORKDIR /app

# Install certificate bundle for outbound TLS calls to verifier endpoints.
RUN apk add --no-cache ca-certificates

# Copy the compiled binaries and runtime resources.
COPY --from=build /out/tam-over-http ./tam-over-http
COPY --from=build /out/admin-console ./admin-console
COPY cmd/admin-console/templates ./templates
COPY cmd/admin-console/static ./static

# Default configuration based on the CLI flags defined in cmd/tam-over-http/main.go.
ENV TAM4WASM_ADDR=":8080" \
    TAM4WASM_CHALLENGE_SERVER="" \
    TAM4WASM_CHALLENGE_CONTENT_TYPE='application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"' \
    TAM4WASM_CHALLENGE_INSECURE_TLS="true" \
    TAM4WASM_CHALLENGE_TIMEOUT="1m0s" \
    ADMIN_CONSOLE_PORT="9090" \
    ADMIN_CONSOLE_TAM_API_BASE="http://127.0.0.1:8080"

EXPOSE 8080 9090

ENTRYPOINT ["/bin/sh", "-c", "\
set -eu; \
./tam-over-http & \
tam_pid=$!; \
trap 'kill \"$tam_pid\" 2>/dev/null || true; wait \"$tam_pid\" 2>/dev/null || true' INT TERM EXIT; \
exec ./admin-console -port \"${ADMIN_CONSOLE_PORT}\" -tam-api-base \"${ADMIN_CONSOLE_TAM_API_BASE}\" \
"]
