# User Manual

## Purpose
This document explains how to run `tam-over-http` and use the currently exposed APIs during local development.

## Quick Flow

1. Start TAM (`go run ./cmd/tam-over-http`).
2. Check admin endpoints:
   - `GET /admin/getAgents`
   - `GET /admin/getManifests`
3. Register SUIT envelope when needed via `POST /tc-developer/addManifest`.
4. Integrate a TEEP Agent that communicates with `POST /tam`.

## Start the Server

### Local
```bash
go run ./cmd/tam-over-http
```

### With Make
```bash
make run
```

### Docker
```bash
docker build -t tam-over-http .
docker run --rm -p 8080:8080 tam-over-http
```

With verifier settings:
```bash
docker run --rm -p 8080:8080 \
  -e TAM4WASM_CHALLENGE_SERVER="https://verifier.example.com" \
  -e TAM4WASM_CHALLENGE_CONTENT_TYPE='application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"' \
  tam-over-http
```

### Command Options

`tam-over-http` accepts CLI flags (also configurable by environment variables):

| Flag | Env Var | Default | Description |
| ---- | ------- | ------- | ----------- |
| `-addr` | `TAM4WASM_ADDR` | `:8080` | Listen address for the HTTP server. |
| `-challenge-server` | `TAM4WASM_CHALLENGE_SERVER` | `https://localhost:8443` | Base URL for the verifier challenge-response endpoint. Leave empty to disable verifier submission. |
| `-challenge-content-type` | `TAM4WASM_CHALLENGE_CONTENT_TYPE` | `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"` | `Content-Type` used when posting attestation payloads to the verifier. |
| `-challenge-insecure-tls` | `TAM4WASM_CHALLENGE_INSECURE_TLS` | `true` | Skip TLS verification when contacting the verifier. Set `false` for stricter environments. |
| `-challenge-timeout` | `TAM4WASM_CHALLENGE_TIMEOUT` | `1m` | Timeout for verifier challenge-response interactions. |

Print live defaults with:
```bash
go run ./cmd/tam-over-http -h
```

## Prerequisites

- `curl` for API calls (or any other HTTP client)
- [`cbor2diag.rb`](https://rubygems.org/gems/cbor-diag/) (or equivalent CBOR diagnostic tool) for readable output

## API Summary

Method | Endpoint | Notes
--|--|--
`POST` | `/tam` | TEEP over HTTP endpoint. Body is empty or TEEP message (COSE/CBOR).
`GET` | `/admin/getAgents` | Returns agent status in CBOR.
`GET` | `/admin/getManifests` | Returns SUIT manifest overviews in CBOR.
`POST` | `/tc-developer/addManifest` | Registers a signed SUIT envelope.

For protocol details, see:
- [`TEEP_MESSAGE_HANDLE.md`](./TEEP_MESSAGE_HANDLE.md)
- [`SUIT_MANIFEST_STORE.md`](./SUIT_MANIFEST_STORE.md)

> [!NOTE]
> Communicating with `/tam` requires a TEEP Agent implementation over HTTP.
> See [`TEEP_MESSAGE_HANDLE.md`](./TEEP_MESSAGE_HANDLE.md), [TEEP Protocol](https://datatracker.ietf.org/doc/html/draft-ietf-teep-protocol), and [TEEP over HTTP](https://datatracker.ietf.org/doc/html/draft-ietf-teep-otrp-over-http).
> For working examples, reference `TestTAMResolveTEEPMessage_AgentAttestation_OK` and `TestTAMResolveTEEPMessage_AgentUpdate_OK` in [`../internal/tam/tam_test.go`](../internal/tam/tam_test.go).

## Get Agent Status (CBOR)

```bash
curl -X GET http://localhost:8080/admin/getAgents \
  -H "Accept: application/cbor" -s | cbor2diag.rb
```

Example output:
```cbor-diag
[
  [
    'dummy-teep-agent-kid-for-dev-123',
    {
      "wapp_list": [
        [<< ['app1.wasm'] >>, 3],
        [<< ['app2.wasm'] >>, 2]
      ],
      "attributes": {256: h'016275696C64696E672D6465762D313233'}
    }
  ]
]
```

## Post SUIT Manifest

```bash
curl -X POST http://localhost:8080/tc-developer/addManifest \
  -H "Content-Type: application/suit-envelope+cose" \
  --data-binary "@./examples/text.0.envelope.cbor"
```

Example output:
```
OK
```

If you want to see the SUIT manifest content, see [text.0.envelope.diag](./examples/text.0.envelope.diag).

## Get Manifest Overviews (CBOR)

```bash
curl -X GET http://localhost:8080/admin/getManifests \
  -H "Accept: application/cbor" -s | cbor2diag.rb
```

Example output:
```cbor-diag
[
  [
    / component: / << ['app1.wasm'] >>,
    / manifest-sequence-number: / 3
  ],
  [
    / component: / << ['app2.wasm'] >>,
    / manifest-sequence-number: / 2
  ]
]
```

## Planned Management APIs

TODO: add API endpoints to manage entities, keys, ...

## Run Tests

Basic tests:
```bash
go test ./...
```

Integration tests with VERAISON (you need to run VERAISON on localhost):
```bash
go test -tags=integration ./...
```

Equivalent Make targets:
```bash
make test
make test-integrated
```

## Troubleshooting

- `415 Unsupported Media Type`:
  - check request headers (`Content-Type` for `POST`, `Accept` for `GET`).
- `400 Bad Request` on `/tc-developer/addManifest`:
  - verify SUIT envelope encoding and signature.
  - verify signer key is pre-registered in TAM.
- Unexpected `204 No Content`:
  - current admin/manifests behavior is demo-oriented and may return no content when no matching records are found.
