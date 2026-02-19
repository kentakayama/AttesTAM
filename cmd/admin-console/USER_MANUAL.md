# User Manual

## Purpose
This document explains how to run `admin-console` and use its UI.

## Quick Flow

1. Start TAM (`go run ./cmd/tam-over-http`) if you want to read real device/manifest data.
2. Start Admin Console (`go run ./cmd/admin-console`).
3. Open `http://localhost:<port>` in a browser.
4. Use these views:
   - `View Managed Devices`: list agents and inspect per-agent installed TC versions.
   - `View Managed TCs`: list uploaded/fetched manifests.
   - `Register TC`: upload a manifest file to the console API.

## Start the Admin Console

### Prerequisites

- Go toolchain (`go run`)
- Browser (Chrome/Safari/Firefox, etc.)
- TAM server (`tam-over-http`) when using TAM API mode

### Command
```bash
go run ./cmd/admin-console
```

#### Listen Port / TAM API Settings
Use command-line flags:

| Setting | Flag | Default | Description |
| ---- | ---- | ---- | ---- |
| Listen port | `--port` | `9090` | HTTP port for Admin Console |
| TAM API base URL | `--tam-api-base` | `""` (disabled) | If set, console calls TAM APIs for devices/manifests upload |

Example:

```bash
go run ./cmd/admin-console --port=9090 --tam-api-base=http://localhost:8080
```

## UI Operation Guide

### View Managed Devices

- Click `View Managed Devices` in the sidebar.
- Agent table is loaded from `GET /console/view-managed-devices`.
- Click an `Agent KID` row to open the detail panel.
- Detail panel shows installed TC list (`name`, `version`) for the selected agent.
- Clicking the selected agent again closes the detail panel.

#### Behind the Scenes

- Console endpoint: `GET /console/view-managed-devices`
- When `--tam-api-base` is set:
  - Console calls `GET {base}/AgentService/ListAgents`.
  - Console compares each `UpdatedAt` with in-memory cache.
  - Console calls `POST {base}/AgentService/GetAgentStatus` only for changed/new KIDs.
  - Console removes agents that no longer appear in `ListAgents` from the in-memory cache.
- When `--tam-api-base` is not set:
  - Instead of calling TAM APIs, console reads and combines testvectors:
    - `cmd/admin-console/testvector/agentservice_listagents.cbor`
    - `cmd/admin-console/testvector/agentservice_getagentstatus.cbor`

### View Managed TCs

- Click `View Managed TCs`.
- Manifest table is loaded from `GET /console/view-managed-tcs`.
- Columns:
  - `TC Name`
  - `Version`

#### Behind the Scenes

- Console endpoint: `GET /console/view-managed-tcs`
- When `--tam-api-base` is set:
  - Console calls `GET {base}/SUITManifestService/ListManifests`.
- When `--tam-api-base` is not set:
  - Instead of calling TAM APIs, console reads `cmd/admin-console/testvector/suitmanifestservice_listmanifests.cbor`.

### Register TC

- Click `Register TC`.
- Select a file and click `Upload`.
- Browser sends `multipart/form-data` to `POST /console/register-tc`.
- On success, UI displays `Upload complete.` and refreshes manifest list.

#### Behind the Scenes

- Console endpoint: `POST /console/register-tc`
- When `--tam-api-base` is set:
  - Console forwards the uploaded payload to `POST {base}/SUITManifestService/RegisterManifest`.
- When `--tam-api-base` is not set:
  - Instead of calling TAM APIs, console validates multipart upload and returns JSON success response.

## Troubleshooting

- `TAM API fetch failed: status 4xx/5xx from TAM API`:
  - Verify TAM is running and `--tam-api-base` is correct.
  - Verify TAM endpoints `/AgentService/ListAgents`, `/AgentService/GetAgentStatus`, and `/SUITManifestService/ListManifests` are reachable.
- `Upload failed: file is required`:
  - Ensure the upload form includes `file` field.
- Empty tables in UI:
  - If external mode is enabled, validate TAM data existence.
  - If external mode is disabled, verify testvector files exist under `cmd/admin-console/testvector`.
