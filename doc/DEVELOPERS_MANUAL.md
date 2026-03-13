# Developers Manual

## Development Workflow

```bash
make run-demo         # Start server locally in insecure demo mode (evaluation only; not for production)
make test             # Run unit tests (go test ./...)
make test-integrated  # Run integration-tagged tests (requires provisioned VERAISON server)

# Equivalent direct Go commands:
go run ./cmd/attestam -insecure-demo-mode
go test ./...
go test -tags=integration ./...
```

Notes:
- The embedded TAM private key is a public insecure demo key and is only used through explicit insecure demo/test flows.
- Outside `-insecure-demo-mode`, startup requires `-tam-teep-private-key-path` or `ATTESTAM_TAM_TEEP_PRIVATE_KEY_PATH`.

## Troubleshooting

### Server startup failure

- `tam-api-base is required`:
  - Do not pass an empty `--tam-api-base`.
  - If TAM is not on the default endpoint, start the console with `--tam-api-base=http://<tam-host>:<tam-port>/`.
- `parse template: ...`:
  - Ensure `templates/index.html` is available from the current working directory (or run from repository root).
- `listen tcp ...: bind: address already in use`:
  - Another process is already using the port (default `9090`).
  - Use a different port, for example: `go run ./cmd/admin-console --port=19090`.

### HTTP error

- `500 admin console is misconfigured: tam-api-base is required`:
  - Start admin-console with a valid `--tam-api-base`.
- `502 TAM API fetch failed: ...` / `502 TAM API post failed: ...`:
  - Verify TAM is running and reachable.
  - Verify `--tam-api-base` is correct.
  - Verify TAM endpoints `/AgentService/ListAgents`, `/AgentService/GetAgentStatus`, `/SUITManifestService/ListManifests`, and `/SUITManifestService/RegisterManifest` are reachable.
- If upload fails with a message that includes `status 400 from TAM API`:
  - Verify SUIT envelope encoding and signature.
  - Verify signer key is pre-registered in TAM.
  - Verify that you are not uploading a manifest whose sequence number is the same as, or older than, a manifest already registered in TAM.
- Empty tables in UI:
  - Validate that TAM has device or manifest data to return.
