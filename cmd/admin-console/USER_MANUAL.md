# User Manual

## Purpose
This document explains how to run `admin-console` and use its UI/API during local development.

## Quick Flow

1. Start TAM (`go run ./cmd/tam-over-http`) if you want to read real device/manifest data.
2. Start Admin Console (`go run ./cmd/admin-console`).
3. Open `http://localhost:<port>` in a browser.
4. Use these views:
   - `Managed Devices`: list agents and inspect per-agent installed TC versions.
   - `Managed TCs`: list uploaded/fetched manifests.
   - `Register TC`: upload a manifest file to the console API.

## Start the Admin Console

### Local
```bash
go run ./cmd/admin-console
```

### Listen Port / TAM API Settings
Use command-line flags:

| Setting | Flag | Default | Description |
| ---- | ---- | ---- | ---- |
| Listen port | `--port` | `9090` | HTTP port for Admin Console |
| TAM API base URL | `--tam-api-base` | `""` (disabled) | If set, console calls TAM APIs for devices/manifests upload |

Example:

```bash
go run ./cmd/admin-console --port=9090 --tam-api-base=http://localhost:8080
```

## Prerequisites

- Go toolchain (`go run`)
- Browser (Chrome/Safari/Firefox, etc.)
- TAM server (`tam-over-http`) when using TAM API mode

## TAM API Integration Behavior

When `--tam-api-base` is set, the console calls TAM endpoints:

- `POST {base}/AgentService/ListAgents` (empty request body)
- `POST {base}/AgentService/GetAgentStatus` (request body: list of KIDs from ListAgents)
- `POST {base}/SUITManifestService/ListManifests` (empty request body)
- `POST {base}/SUITManifestService/RegisterManifest`

Request header used by console:

- `Accept: application/cbor`

Console behavior:

- If response `Content-Type` starts with `application/cbor`, response is parsed as CBOR and converted to JSON for UI.
- Otherwise, response is treated as JSON.

When `--tam-api-base` is not set:

- `GET /api/agents` reads and combines:
  - `cmd/admin-console/testvector/agentservice_listagents.cbor`
  - `cmd/admin-console/testvector/agentservice_getagentstatus.cbor`
- `GET /api/manifests/service` reads and decodes `cmd/admin-console/testvector/suitmanifestservice_listmanifests.cbor`.
- `POST /api/manifests/register` returns the uploaded file as downloadable response (`Content-Disposition: attachment`).

## UI Operation Guide

## 1. Managed Devices

- Click `Managed Devices` in the sidebar.
- Agent table is loaded from `GET /api/agents`.
- Click an `Agent KID` row to open the detail panel.
- Detail panel shows installed TC list (`name`, `ver`) for the selected agent.
- Clicking the selected agent again closes the detail panel.

## 2. Managed TCs

- Click `Managed TCs`.
- Manifest table is loaded from `GET /api/manifests/service`.
- Columns:
  - `TC Name`
  - `Version`

## 3. Register TC

- Click `Register TC`.
- Select a file and click `Upload`.
- Browser sends `multipart/form-data` to `POST /api/manifests/register`.
- On success, UI displays `Upload complete.` and refreshes manifest list.

## Run Tests

Project-wide tests:

```bash
go test ./...
```

## Troubleshooting

- `TAM API fetch failed: status 4xx/5xx from TAM API`:
  - Verify TAM is running and `--tam-api-base` is correct.
  - Verify TAM endpoints `/AgentService/ListAgents`, `/AgentService/GetAgentStatus`, and `/SUITManifestService/ListManifests` are reachable.
- `Upload failed: file is required`:
  - Ensure the upload form includes `file` field.
- Empty tables in UI:
  - If external mode is enabled, validate TAM data existence.
  - If external mode is disabled, verify testvector files exist under `cmd/admin-console/testvector`.
