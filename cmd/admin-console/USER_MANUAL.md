# User Manual

## Purpose
This document explains how to run `admin-console` and use its UI/API during local development.

## Quick Flow

1. Start TAM (`go run ./cmd/tam-over-http`) if you want to read real device/manifest data.
2. Start Admin Console (`go run ./cmd/admin-console`).
3. Open `http://localhost:<port>` in a browser.
4. Use these views:
   - `Show Devices`: list devices and inspect per-device WAPP versions.
   - `Show TCs`: list uploaded/fetched manifests.
   - `Upload Manifests`: upload a manifest file to the console API.

## Start the Admin Console

### Local
```bash
go run ./cmd/admin-console
```

### Listen Port / External API Settings
Settings precedence is:

1. environment variables
2. `config.json`
3. built-in defaults

| Setting | Env Var | `config.json` key | Default | Description |
| ---- | ---- | ---- | ---- | ---- |
| Listen port | `PORT` | `server.port` | `8080` | HTTP port for Admin Console |
| External API base URL | `EXTERNAL_API_BASE` | `externalApiBase` | `""` (disabled) | If set, console calls TAM APIs for devices/manifests upload |

Current sample config (`cmd/admin-console/config.json`):

```json
{
  "server": {
    "port": 9090
  },
  "externalApiBase": "http://localhost:8080"
}
```

> [!NOTE]
> With this sample config, `go run ./cmd/admin-console` starts on port `9090` unless `PORT` is set.

## Prerequisites

- Go toolchain (`go run`)
- Browser (Chrome/Safari/Firefox, etc.)
- TAM server (`tam-over-http`) when using external data mode

## External Integration Behavior

When `EXTERNAL_API_BASE` (or `externalApiBase`) is set, the console calls TAM endpoints:

- `GET {base}/admin/getAgents`
- `GET {base}/admin/getManifests`
- `POST {base}/tc-developer/addManifest`

Request header used by console:

- `Accept: application/cbor`

Console behavior:

- If response `Content-Type` starts with `application/cbor`, response is parsed as CBOR and converted to JSON for UI.
- Otherwise, response is treated as JSON.

When `EXTERNAL_API_BASE` is not set:

- `GET /api/devices` reads and decodes `cmd/admin-console/testvector/devices.cbor`.
- `GET /api/manifests` reads and decodes `cmd/admin-console/testvector/manifests.cbor`.
- `POST /api/manifests` returns the uploaded file as downloadable response (`Content-Disposition: attachment`).

## UI Operation Guide

## 1. Show Devices

- Click `Show Devices` in the sidebar.
- Device table is loaded from `GET /api/devices`.
- Click a `Device Name` row to open the detail panel.
- Detail panel shows WAPP list (`name`, `ver`) for the selected device.
- Clicking the selected device again closes the detail panel.

## 2. Show TCs

- Click `Show TCs`.
- Manifest table is loaded from `GET /api/manifests`.
- Columns:
  - `TC Name`
  - `Version`

## 3. Upload Manifests

- Click `Upload Manifests`.
- Select a file and click `Upload`.
- Browser sends `multipart/form-data` to `POST /api/manifests`.
- On success, UI displays `Upload complete.` and refreshes manifest list.

## Run Tests

Project-wide tests:

```bash
go test ./...
```

## Troubleshooting

- `external fetch failed: status 4xx/5xx from external`:
  - Verify TAM is running and `externalApiBase` is correct.
  - Verify TAM endpoints `/admin/getAgents` and `/admin/getManifests` are reachable.
- `Upload failed: file is required`:
  - Ensure the upload form includes `file` field.
- Empty tables in UI:
  - If external mode is enabled, validate TAM data existence.
  - If external mode is disabled, verify testvector files exist under `cmd/admin-console/testvector`.
