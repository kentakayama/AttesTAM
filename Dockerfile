#
# Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
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

# Copy the full source tree and build the tam-over-http binary.
COPY . .
RUN GOOS=linux \
    go build -trimpath -o /out/tam-over-http ./cmd/tam-over-http


FROM alpine:3.20 AS runtime

WORKDIR /app

# Install certificate bundle for outbound TLS calls to verifier endpoints.
RUN apk add --no-cache ca-certificates

# Copy the compiled binary and runtime resources.
COPY --from=build /out/tam-over-http ./tam-over-http

# Default configuration based on the CLI flags defined in cmd/tam-over-http/main.go.
ENV TAM4WASM_ADDR=":8080" \
    TAM4WASM_CHALLENGE_SERVER="" \
    TAM4WASM_CHALLENGE_CONTENT_TYPE='application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"' \
    TAM4WASM_CHALLENGE_INSECURE_TLS="true" \
    TAM4WASM_CHALLENGE_TIMEOUT="1m0s"

EXPOSE 8080

ENTRYPOINT ["./tam-over-http"]
