# TAM External Design

From terminology of [RFC 9397: Trusted Execution Environment Provisioning (TEEP) Architecture](https://datatracker.ietf.org/doc/html/rfc9397#name-terminology), we use TC Developer (Trusted Component Developer), TEEP Agent, TAM Admin (TAM Administrator), Device Admin (Device Administrator).
They communicates with our TAM to ....

```mermaid
---
title: TAM External Design
---

flowchart TB

TCDeveloper([TC Developer]) -- SUIT Manifest --> TAM
TAM -- Agent Status<br/>SUIT Manifest Overviews --> TAMAdmin([TAM Admin])
TAM -- Agent Status --> DeviceManager([Device Admin])
TEEPAgent([TEEP Agent]) <-- TEEP Message --> TAM
TAM[TAM]
```

Method | Endpoint | Requester | Input | Output | Reference
--|--|--|--|--|--
POST | `/tam` | TEEP Agent | empty<br/>QueryResponse<br/>Success<br/>Error | 200: QueryRequest<br/>200: Update / QueryRequest<br/>204: empty<br/>204: empty | [TEEP_MESSAGE_HANDLE](TEEP_MESSAGE_HANDLE.md)
POST | `/SUITManifestService/RegisterManifest` | TC Developer | SUIT Manifest | 200: OK | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
GET | `/SUITManifestService/ListManifests` | TAM Admin | none | 200: `[overview of SUIT Manifest]` (CBOR) | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
GET | `/AgentService/GetAgentList` | TAM Admin/<br/>Device Admin | none | 200: Agent status (CBOR) | [TEEP_AGENT_STATUS](TEEP_AGENT_STATUS.md)
POST | `/AgentService/GetAgentStatus` | TAM Admin/<br/>Device Admin | `agent-kids` | 200: Agent status (CBOR) | [TEEP_AGENT_STATUS](TEEP_AGENT_STATUS.md)

> [!NOTE]
> Current `/*Service/` endpoints return fixed demo-oriented records. Request-specific filtering and role-based authorization are still TODO.
