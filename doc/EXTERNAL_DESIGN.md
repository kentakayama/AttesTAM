## TAM's External Design

```mermaid
---
title: Conceptual Design of 
---

flowchart TB

TCDeveloper([TC Developer]) -- GET SUIT Manifests<br/>POST SUIT Manifest --> HTTPServer
DeviceManager([Device Manager]) -- GET AgentStatus --> HTTPServer
TEEPAgent([TEEP Agent]) -- POST TEEP Message --> HTTPServer

subgraph TAM Server
    HTTPServer[HTTP Server]
    TAMManager[TAM Manager]
    TAMoverHTTP[TAM Message Handler]
    DBMS[DBMS]

    HTTPServer -- TEEP Message --> TAMoverHTTP
    HTTPServer -- Command/Query --> TAMManager
    TAMManager --> DBMS    
    TAMoverHTTP --> DBMS
    DBMS --> DB
end

```

Method | Endpoint | Requester | Input | Output | Reference
--|--|--|--|--|--
GET | `/tc-developer/getManifests` | TC Developer | `{TBD}` | 200: `[overview of SUIT Manifest]`
POST | `/tc-developer/addManifest` | TC Developer | SUIT Manifest | 200: OK
GET | `/admin/getAgents` | TAM Admin | `{TBD}` | 200: `{TBD}` (status of Agents under this TAM)
POST | `/tam` | TEEP Agent | empty<br/>QueryResponse<br/>Success<br/>Error | 200: QueryRequest<br/>200: Update / QueryRequest<br/>204: empty<br/>204: empty | [TEEP_MESSAGE_HANDLE](TEEP_MESSAGE_HANDLE.md)
