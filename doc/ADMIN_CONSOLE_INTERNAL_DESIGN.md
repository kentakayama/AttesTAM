# Admin Console Internal Design

## 1. Purpose And Scope

`cmd/admin-console` provides a browser-based operation console for TAM.

Current scope:
- Show managed devices and their installed trusted components.
- Show managed trusted components (manifests).
- Register a trusted component (manifest upload relay to TAM API).

Out of scope:
- Authentication/authorization for console users.
- Full TAM business logic (handled by TAM server side).

## 2. Architecture Overview

Entry point:
- `cmd/admin-console/main.go`

Main layers:
- HTTP/UI layer:
  - `cmd/admin-console/handlers.go`
  - `cmd/admin-console/http_utils.go`
  - `cmd/admin-console/templates/index.html`
  - `cmd/admin-console/static/app.js`
  - `cmd/admin-console/static/styles.css`
- TAM API client layer:
  - `cmd/admin-console/tam_api.go`
- CBOR decoding / conversion layer:
  - `cmd/admin-console/agent_codec.go`
  - `cmd/admin-console/manifest_codec.go`
  - `cmd/admin-console/cbor_converter.go`
- Data model layer:
  - `cmd/admin-console/types.go`
- Config / local testvector loading:
  - `cmd/admin-console/config.go`
  - `cmd/admin-console/testvector_loader.go`
  - `cmd/admin-console/testvector/*`

Runtime mode:
- TAM API mode: `--tam-api-base` is set.
- Testvector mode: `--tam-api-base` is empty.

## 3. Data Model And Transformation Policy

### 3.1 Internal model (CBOR-oriented)

Defined in `cmd/admin-console/types.go`.

- `Agent.KID`: `[]byte` (CBOR `bstr`)
- `Agent.LastUpdate`: `time.Time`
- `Attribute.Ueid`: `eat.UEID` (byte-string based UEID type)
- `TrustedComponent.Name`: `suit.ComponentID`
- `TrustedComponent.Version`: `uint64`

### 3.2 JSON output policy

Internal model keeps precise raw-typed data, and JSON encoding applies display-oriented conversion.

- `Agent.MarshalJSON`:
  - `kid`: `[]byte` -> string
  - `last_update`: `time.Time` -> RFC3339 string (`formatUpdatedAt`)
- `Attribute.MarshalJSON`:
  - `ueid`: `eat.UEID` -> hex string
- `TrustedComponent.MarshalJSON`:
  - `name`: `suit.ComponentID` -> CBOR diagnostic string
  - `version`: `uint64` as-is

Rationale:
- Preserve correctness and type fidelity internally.
- Provide stable, easy-to-render JSON fields for UI.

## 4. Endpoint Flows

### 4.1 `GET /console/view-managed-devices`

Handler:
- `handleListAgents` (`cmd/admin-console/handlers.go`)

Flow (TAM API mode):
1. `fetchTAMDevices` requests:
   - `GET /AgentService/ListAgents`
   - `POST /AgentService/GetAgentStatus` (delta fetch with in-memory cache)
2. `decodeAgentsFromCBOR` converts status records to internal `[]Agent`.
3. `respondJSON` returns converted JSON.

Flow (testvector mode):
1. `loadTestVectorAgents` reads:
   - `cmd/admin-console/testvector/agentservice_listagents.cbor`
   - `cmd/admin-console/testvector/agentservice_getagentstatus.cbor`
2. Decode with `decodeAgentsFromCBOR`.
3. Map `UpdatedAt` values and return JSON.

### 4.2 `GET /console/view-managed-tcs`

Handler:
- `handleListManifestsService`

Flow (TAM API mode):
1. `fetchTAMManifests` calls `GET /SUITManifestService/ListManifests`.
2. `decodeManifestsFromCBOR` decodes overviews and maps to `[]TrustedComponent`.
3. `respondJSON` returns JSON.

Flow (testvector mode):
1. `loadTestVectorManifests` reads:
   - `cmd/admin-console/testvector/suitmanifestservice_listmanifests.cbor`
2. Decode with `decodeManifestsFromCBOR`.
3. Return JSON.

### 4.3 `POST /console/register-tc`

Handler:
- `handleRegisterManifest`

Flow (TAM API mode):
1. Parse multipart input.
2. Relay uploaded bytes to TAM API:
   - `POST /SUITManifestService/RegisterManifest`
3. Return JSON success response (`{"ok": true}`).

Flow (testvector mode):
1. Parse multipart input.
2. Validate uploaded payload can be fully read.
3. Return JSON success response (`{"ok": true}`) without TAM call.

## 5. Error Handling Policy

- Non-`2xx` from TAM API is converted to `502 Bad Gateway`.
- Invalid method returns `405 Method Not Allowed`.
- Multipart parsing/file acquisition failures return `400 Bad Request`.
- Unsupported TAM response `Content-Type` (non-CBOR where CBOR is expected) is treated as error.
- Decode failures return explicit wrapped errors for troubleshooting.

## 6. Configuration And Modes

Defined in `cmd/admin-console/config.go`.

Primary flags:
- `--port`
- `--tam-api-base`

Behavior:
- If `tam-api-base` is empty, console serves using local CBOR testvectors.
- If set, console acts as a thin TAM API client.

## 7. Test Strategy

Key tests:
- `agent_codec_test.go`:
  - Agent CBOR decode correctness.
  - JSON conversion shape checks.
- `manifest_codec_test.go`:
  - Manifest CBOR decode correctness.
- `tam_api_test.go`:
  - TAM API integration behavior (CBOR contract, error handling, cache/delta behavior).
- `handlers_test.go`:
  - HTTP handler behavior for register endpoint.

Test objective:
- Keep CBOR decoding and JSON conversion contracts stable while refactoring internals.

## 8. Change Guide

When changing display format:
- Update `MarshalJSON` implementations in `cmd/admin-console/types.go`.
- Update UI render assumptions in `cmd/admin-console/static/app.js`.
- Update JSON expectation tests.

When changing TAM CBOR contract handling:
- Update decoders in `agent_codec.go` / `manifest_codec.go`.
- Update TAM API tests and testvectors.

When changing endpoint behavior:
- Update `handlers.go` + `tam_api.go`.
- Reflect external operation changes in `doc/ADMIN_CONSOLE_USER_MANUAL.md`.
