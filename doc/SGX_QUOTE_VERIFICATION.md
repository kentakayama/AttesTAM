# SGX Quote Verification

## Purpose
This document describes the current AttesTAM implementation policy for Intel SGX Quote verification.

AttesTAM is primarily a Relying Party in [RFC 9334](https://datatracker.ietf.org/doc/rfc9334/) Rats Architecture terms.
However, in the current implementation, AttesTAM also embeds the Verifier-side functionality needed to verify Intel SGX Quotes.

## Terminology

### Intel SGX Terms and RFC 9334 RATS Terms

This document uses both Intel SGX/DCAP terms and RFC 9334 RATS terms.
The rough correspondence is as follows.

| Intel / SGX term | Rough RATS term | Notes |
| ---- | ---- | ---- |
| SGX Quote | Evidence | The Quote mainly contains the state of the CPU platform, the Quoting Enclave, the Enclave App, and the ECDSA signature produced by the Quoting Enclave. It also carries the PCK Cert chain and the certification data for the ECDSA Attestation Key. |
| Quoting Enclave (QE) | Attesting Environment | The QE collects platform and enclave state and signs the Quote with the ECDSA Attestation Key. |
| ECDSA Attestation key | Attestation Key (AK) | Used by QE and certified by PCE. |
| Provisioning Certification Enclave (PCE) | Endorser | Certifies the AK in the same CPU platform and is itself certified by Intel. |
| Enclave App | Target Environment (TE) | The environment in which the TEEP Agent runs. |
| (verification) collateral | Endorsement | In practice this groups the CRLs, `TCB Info`, and `QE Identity` that are fetched from Intel PCS and passed into Intel QVL. |
| Intel Quote Verification Library (QVL) | N/A | Intel's verification library, used by the Verifier as the implementation of the SGX Quote verification procedure. |
| SGX Root CA Certificate | trust anchor | Certifies Intel's collateral and the PCK Cert chain, and is pinned in Intel QVL. |
| Intel PCS | Endorser | Intel PCS is the source from which the Verifier obtains Intel-generated Endorsements (collateral). |

For Intel SGX Quote verification, the Verifier receives a Quote as Evidence, then uses Quote-derived information such as `FMSPC` (Family-Model-Stepping-Platform-CustomSKU) as a key to obtain Endorsements from the Intel PCS.

## Implementation Policy

AttesTAM currently adopts the following design priorities.

1. The AttesTAM-embedded Verifier uses Intel QVL, but does not use Intel PCCS or Intel QCNL as required runtime components.
2. The Verifier manages collateral in its own Endorsement store, fetched from Intel PCS.
3. The Verifier does **not** appraise the Target Environment by itself.

These points are intentional design choices, not incidental implementation details.

## Data Flow

The internal Verifier first treats the Quote as the primary input object.
From that Quote, it derives identifiers such as `FMSPC` and the PCK CA type, and uses them to determine which Intel PCS collateral set is relevant to this Quote.

In that sense, the trust and data flow are organized as follows:

- the Quote provides the key material used to identify the required collateral set
- Intel PCS is treated as the upstream source of that collateral
- Intel QVL verifies the Quote using the collateral that the Verifier selected for that Quote

For the current AttesTAM model, this can be summarized as:

```text
Quote -> Verifier key extraction -> Intel PCS collateral selection
Verifier -> Intel QVL
```

As shown in the data flow below, this internal Verifier function does **NOT** by itself appraise Target Environment identity values such as `MRENCLAVE` and `MRSIGNER` from the SGX Quote.
That appraisal usually depends on Relying Party-specific policy, so in AttesTAM it belongs conceptually above the Intel QVL-based Quote verification step.

![](./img/sgx-data-flow.svg)

Refer [Conceptual Data Flow of RATS Architecture](https://datatracker.ietf.org/doc/html/rfc9334#figure-1).

### Sequence Diagram

```mermaid
sequenceDiagram
    participant V as Verifier
    participant E@{"type": "database"} as Endorsement store
    participant I as Intel PCS
    participant Q as Intel QVL

    V->>E: lookup collateral for Quote-derived FMSPC
    alt endorsement cache hit
        E-->>V: cached collateral
    else endorsement cache miss
        V->>I: fetch collateral directly
        I-->>V: collateral
        V->>E: store collateral
    end
    V->>Q: tee_verify_quote(quote, collateral)
    Q-->>V: verification result
```

In this sequence, the Verifier first decides which collateral is relevant for the Quote, then owns the Endorsement store lookup, the Intel PCS fetch decision, and the final collateral object passed into Intel QVL.

## Main Design Intent

The current implementation embeds a Verifier-side Intel SGX Quote verification path inside AttesTAM.

That embedded Verifier is responsible for:

- obtaining the collateral needed to verify the SGX Quote
- passing the Quote and collateral into Intel QVL
- determining whether the Quote is valid with respect to Intel's verification logic and the supplied collateral

That embedded Verifier is **not** responsible, by itself, for appraising Target Environment identity values such as `MRENCLAVE` and `MRSIGNER`.
Those checks usually depend on Relying Party-specific policy, so in AttesTAM they belong conceptually above the Intel QVL-based Quote verification step.

### 1. Use Intel QVL, but do not rely on Intel PCCS or Intel QCNL

AttesTAM uses the native Intel DCAP quote verification library (QVL) through `tee_verify_quote()` in [`internal/infra/rats/intel_qvl_verifier.go`](../internal/infra/rats/intel_qvl_verifier.go).

However, AttesTAM does not delegate collateral acquisition policy to Intel QPL / QCNL / PCCS at verification time.
The implementation policy is:

- use Intel QVL for the verification primitive itself
- do not use Intel QCNL for collateral retrieval policy
- do not use Intel PCCS as the normal collateral-management component
- keep collateral lookup, fetch, cache, and reuse decisions in AttesTAM Go code

In practice:

- The TAM process prepares the collateral in Go code.
- The TAM process always passes the prepared collateral to `tee_verify_quote()`.
- Quote verification is performed only within the scope of the collateral explicitly supplied by AttesTAM.
- The collateral fetch path, cache lookup path, and reuse policy are all decided in Go code in the AttesTAM process.
- AttesTAM intentionally does not call `tee_verify_quote()` with `collateral = NULL`, because the Verifier-side Endorsement store is meant to directly control collateral selection and reuse.

This means the verifier path is intentionally split as follows:

- Native verification logic: Intel QVL / `tee_verify_quote()`
- Collateral retrieval, cache control, and reuse policy: AttesTAM Go code

This choice is intended to keep collateral lifecycle control in the AttesTAM Verifier implementation.
For the more common Intel deployment model using QCNL/PCCS and for the detailed comparison with that model, see [Architecture Comparison](#architecture-comparison).

### 2. Derive FMSPC from the Quote, then use Go-managed cache and Intel PCS

AttesTAM derives the FMSPC from the externally supplied Quote and uses that value as the main key for collateral lookup.

The current flow is:

1. The TEEP Agent sends an SGX Quote.
2. AttesTAM extracts `FMSPC` from the Quote.
3. AttesTAM derives the collateral cache key from that Quote, preferring an FMSPC-based key.
4. AttesTAM checks the Go-managed collateral cache.
5. If cached collateral is available, AttesTAM uses it for verification.
6. If cached collateral is missing, or if the cached collateral is no longer acceptable for use, AttesTAM fetches collateral from Intel PCS.
7. The fetched collateral is stored in the Go-managed cache.
8. AttesTAM calls `tee_verify_quote()` with the Quote and the explicitly prepared collateral.

The key point is that collateral management is owned by the TAM, not by Intel PCCS and not implicitly by the QVL library.

> [!NOTE]
> The intended policy is "cache hit and still usable -> reuse" and "cache miss or stale collateral -> refetch from Intel PCS".
> The current implementation already uses the Go-managed cache and Intel PCS fetch path, and it always passes explicit collateral to `tee_verify_quote()`.
> If cache freshness rules evolve further, that logic should remain in AttesTAM Go code rather than being delegated to Intel PCCS or QCNL runtime retrieval.

## Architecture Comparison

### Trade-off Summary

The current AttesTAM policy is not presented as universally better than using Intel QCNL/PCCS.
The goal is to make the trade-offs explicit.

| Viewpoint | Use Intel QCNL/PCCS | Current AttesTAM policy |
| ---- | ---- | ---- |
| Maintenance ownership | More of the collateral retrieval and cache behavior is maintained by Intel-provided components. | AttesTAM must maintain its own Go implementation for collateral retrieval, cache handling, and policy decisions. |
| Configuration flexibility | Intel-specific configuration can support multiple deployment patterns through files such as `/etc/sgx_default_qcnl.conf`. | Behavior is concentrated in one implementation path, which reduces hidden branches and can make environment setup easier to understand. |
| Operational simplicity | A standard Intel-style deployment may fit environments already using QCNL/PCCS. | Fewer Intel-specific runtime components are required in the normal Verifier path. |
| Visibility from Verifier code | Collateral source selection and cache behavior can be harder to see because they may be externalized into QCNL/PCCS configuration and runtime behavior. | Collateral source selection, cache lookup, and reuse policy are visible in the Verifier implementation. |
| Scaling collateral management | PCCS can act as a shared service and shared cache across multiple Verifier hosts. | The Verifier can apply an AttesTAM-specific Endorsement store policy, but distributed/shared cache strategy must be designed and maintained by AttesTAM. |
| Local host behavior | QCNL/QPL can use host-local cache and host-local policy such as `local_cache_only`. | The Verifier directly controls host-local collateral behavior instead of inheriting QCNL policy. |
| Runtime adaptability | Intel components may support different routing and caching topologies without changing Verifier code. | Fewer moving parts can make runtime behavior easier to reason about, but new topologies usually require code changes or explicit AttesTAM design work. |
| Verification input control | Convenient when the Verifier is comfortable delegating collateral management to Intel's runtime stack. | Better fit when the Verifier wants to directly control which collateral is fetched, cached, reused, and passed to Intel QVL. |

In short:

- using Intel QCNL/PCCS reduces the amount of Verifier-owned collateral management code, but pushes more behavior into Intel runtime components and configuration
- the current AttesTAM policy keeps behavior concentrated in the Verifier implementation, but requires AttesTAM to maintain more custom code

### Common Intel DCAP deployment model

A common deployment model is:

- Verifier
- Intel QVL
- Intel QCNL
- Intel PCCS
- Intel PCS

In that model:

- the Verifier calls Intel QVL APIs
- the Verifier commonly passes `collateral = NULL` to `tee_verify_quote()`
- Intel QVL or its surrounding Intel provider stack relies on QCNL/QPL behavior
- QCNL behavior is influenced by `/etc/sgx_default_qcnl.conf`
- QCNL/QPL can maintain a local cache on the Verifier host
- QCNL may obtain collateral through PCCS
- Intel PCCS, commonly deployed as Intel's Node.js reference service, can also maintain its own shared cache
- PCCS may in turn obtain or proxy data from Intel PCS

That structure can be summarized as:

```text
Verifier -> Intel QVL -> Intel QCNL -> Intel PCCS -> Intel PCS
```

When `tee_verify_quote()` is called with `collateral = NULL`, the practical behavior is commonly understood as:

- the Verifier asks Intel QVL to verify the Quote without explicitly supplying collateral
- Intel's surrounding provider stack becomes responsible for deciding how collateral is found
- QCNL/QPL consults its own runtime configuration
- QCNL/QPL may use local cache, local-cache-only mode, retry rules, PCCS, or direct collateral service settings
- Intel QVL verifies the Quote using collateral that was resolved through that external path

That means the verification input path is no longer fully described by the Verifier code alone.

### Typical `collateral = NULL` sequence

```mermaid
sequenceDiagram
    participant V as Verifier
    participant Q as Intel QVL
    participant N as Intel QCNL/QPL
    participant S as Intel PCCS
    participant I as Intel PCS

    V->>Q: tee_verify_quote(quote, NULL)
    Q->>N: resolve collateral
    N->>N: read qcnl config and cache policy
    alt cache hit
        N-->>Q: local cached collateral
    else cache miss
        alt collateral path via PCCS
            N->>S: request collateral
            alt PCCS shared cache hit
                S-->>N: shared cached collateral
            else PCCS shared cache miss
                S->>I: request upstream collateral
                I-->>S: collateral
                S-->>N: collateral
            end
        else direct collateral service
            N->>I: request collateral
            I-->>N: collateral
        end
        N-->>Q: resolved collateral
    end
    Q-->>V: verification result
```

In this sequence, the Verifier sees the Quote and the final result, but the collateral selection and retrieval policy are mostly externalized into the Intel stack and its configuration.

Here, "local cached collateral" refers to the QCNL-managed host-local cache, while "shared cached collateral" refers to the PCCS-side cache held by the PCCS service.

### Why the difference matters

The difference is not only packaging. It changes who owns the verification inputs.

In the common Intel stack:

- collateral source selection is largely delegated to the Intel retrieval stack
- runtime behavior can depend on QCNL configuration
- the Verifier host may need QCNL/QPL-related components and configuration in addition to Intel QVL
- the Verifier host may also need PCCS deployment or access to a PCCS service
- QCNL/QPL can keep host-local cache files
- PCCS can keep a service-side shared cache
- collateral cache behavior, retry policy, local-cache-only mode, and service routing can be controlled outside the Verifier code
- the actual endorsement inputs used by Intel QVL can become harder to inspect from the Verifier implementation itself

In the AttesTAM stack:

- collateral source selection is implemented by the Verifier in Go code
- the endorsement cache is owned by the Verifier
- Intel PCS is contacted directly by the Verifier-side Go implementation
- Intel QVL only consumes the collateral already selected by the Verifier
- the Verifier code explicitly determines when cached collateral is reused and when Intel PCS is queried
- the endorsement path is visible in the Verifier implementation, rather than hidden behind Intel-specific runtime configuration

This distinction matters for two separate reasons.

First, operational dependency:

- if collateral management is delegated to QCNL/PCCS, the machine running the Verifier may need Intel-specific runtime components, configuration, and in many cases PCCS deployment or access

Second, implementation visibility:

- if collateral and policy are controlled through Intel-specific configuration, the Verifier source code no longer fully describes which collateral source, cache policy, and retrieval behavior are actually in effect
- AttesTAM wants to avoid that situation and keep those decisions visible in Go code

## Current Implementation Structure

### Quote verification entrypoint

- [`internal/infra/rats/intel_qvl_verifier.go`](../internal/infra/rats/intel_qvl_verifier.go)

Responsibilities:

- accept the Quote bytes from TAM
- resolve collateral using the Go-managed path
- call `tee_verify_quote()` with explicit collateral
- interpret the Intel QVL result and map it into AttesTAM verifier output

### Intel PCS client

- [`internal/infra/rats/intel_qvl_pcs.go`](../internal/infra/rats/intel_qvl_pcs.go)

Responsibilities:

- extract `FMSPC` from the Quote
- extract the PCK CA identity from the Quote certification data
- call Intel PCS endpoints needed for `sgx_ql_qve_collateral_t`
- construct the collateral object passed into Intel QVL

The current implementation fetches:

- `PCK CRL`
- `PCK CRL issuer chain`
- `Root CA CRL`
- `TCB Info`
- `TCB Info issuer chain`
- `QE Identity`
- `QE Identity issuer chain`

Note that Intel's `sgx_ql_qve_collateral_t` does not contain the trusted Root CA certificate itself.
AttesTAM therefore distinguishes between:

- collateral data that maps naturally to `sgx_ql_qve_collateral_t`
- trust-anchor material such as the Intel SGX Root CA certificate, which is related but is not stored as a field in that collateral structure

### Go-managed collateral cache

- [`internal/infra/rats/intel_qvl_collateral.go`](../internal/infra/rats/intel_qvl_collateral.go)

Responsibilities:

- derive the cache key from the Quote
- persist collateral in a Go-controlled cache directory
- reload cached collateral for later verification

The cache prefers an FMSPC-based key. If FMSPC extraction fails, it falls back to a Quote hash based key.

## Verification Flow

```mermaid
sequenceDiagram
    participant A as TEEP Agent
    participant T as Verifier
    participant C as Endorsement store
    participant P as Intel PCS
    participant Q as Intel QVL

    A->>T: SGX Quote
    T->>T: extract FMSPC and PCK CA from Quote
    T->>C: lookup collateral
    alt cache hit
        C-->>T: cached collateral
    else cache miss
        T->>P: fetch collateral by FMSPC / PCK CA
        P-->>T: collateral set
        T->>C: store collateral
    end
    T->>Q: tee_verify_quote(quote, collateral)
    Q-->>T: verification result
```

This sequence is intentionally different from the common `tee_verify_quote(quote, NULL)` path.

Here:

- the Verifier itself selects the collateral source
- the Verifier itself decides whether cache is reused
- the Verifier itself prepares the `sgx_ql_qve_collateral_t`-equivalent input
- Intel QVL receives an explicit collateral object instead of discovering collateral through QCNL runtime behavior

## Intel DCAP Reference Behavior

This section summarizes reference behavior observed in Intel's open-source DCAP implementation under `confidential-computing.tee.dcap`.
It is included as implementation background, not as a statement that AttesTAM must follow the same runtime structure.

### What the Quote Verification code shows

The main verification flow can be understood at the following level.

1. `tee_verify_quote()` accepts the Quote together with a collateral structure of type `sgx_ql_qve_collateral_t`.
2. In AttesTAM, that collateral is prepared in Go and converted into the C structure in [`internal/infra/rats/intel_qvl_verifier.go`](../internal/infra/rats/intel_qvl_verifier.go).
3. The collateral structure contains items such as `pck_crl`, `root_ca_crl`, `tcb_info`, and `qe_identity`, together with their issuer chains.
4. The trusted Intel SGX Root CA certificate itself is not a field of `sgx_ql_qve_collateral_t`.
5. For the normal SGX `tee_verify_quote()` path, Intel QVL does not simply trust the Root CA certificate embedded in the Quote's PCK certificate chain. It checks that certificate against a pinned Intel Root public key.

So the rough trust model is:

- the Verifier supplies the Quote and collateral to `tee_verify_quote()`
- the collateral carries revocation and TCB-related material, but not the trusted SGX Root CA certificate as a field
- Intel QVL verifies the Quote's certificate chain against a pinned Intel Root public key before accepting that chain as trustworthy

The pinned public key appears in Intel's DCAP source as `INTEL_ROOT_PUB_KEY` in [`ae/QvE/qve/qve.cpp`](https://github.com/intel/confidential-computing.tee.dcap/blob/main/ae/QvE/qve/qve.cpp).
The source file path looks QvE-specific, but this source is shared with the QVL software path as well.

### Why this matters for AttesTAM

This reference behavior is useful for AttesTAM for two reasons.

First, it explains why "fetching the Intel PCS collateral set" is not exactly the same thing as "collecting every trust input involved in verification".

Second, it reinforces the design choice to keep collateral handling explicit in AttesTAM:

- AttesTAM should clearly distinguish collateral fields from trust-anchor material
- AttesTAM should avoid hidden runtime behavior in QCNL / PCCS / host configuration
- AttesTAM should make it clear which inputs are passed to Intel verification logic and why

## Why This Design Is Preferred

### Explicit collateral boundary

AttesTAM wants the verification boundary to be explicit:

- the Quote comes from the TEEP Agent
- the collateral comes from the TAM-controlled path
- Intel QVL verifies only against the collateral that the TAM provides

This makes the source of verification inputs easier to reason about and easier to test.

### Avoid a PCCS runtime dependency

AttesTAM does not want SGX Quote verification to require a separately deployed PCCS instance as part of the normal TAM runtime.

By keeping collateral retrieval in Go code:

- deployment is simpler
- collateral behavior is easier to inspect
- cache policy is controlled in AttesTAM code
- tests can validate collateral selection logic without requiring a live PCCS

### Preserve Intel verification semantics

AttesTAM still uses Intel QVL for the actual Quote verification decision.

This design does not replace Intel’s verification logic. It replaces only the collateral acquisition path.

## Non-Goals

The current implementation is not trying to:

- make Intel PCCS part of the required AttesTAM runtime
- rely on `sgx_ql_get_quote_verification_collateral()` or QCNL runtime retrieval during Quote verification
- hide collateral source selection inside the Intel library stack

## Operational Summary

In short, the current AttesTAM SGX Quote verification policy is:

- use Intel QVL for Quote verification
- do not require Intel PCCS
- extract `FMSPC` from the Quote in Go
- manage collateral lookup and caching in Go
- fetch collateral from Intel PCS when the cache is missing the required entry
- always pass explicit collateral into `tee_verify_quote()`

## Related Files

- [`internal/infra/rats/intel_qvl_verifier.go`](../internal/infra/rats/intel_qvl_verifier.go)
- [`internal/infra/rats/intel_qvl_pcs.go`](../internal/infra/rats/intel_qvl_pcs.go)
- [`internal/infra/rats/intel_qvl_collateral.go`](../internal/infra/rats/intel_qvl_collateral.go)
- [`internal/tam/tam.go`](../internal/tam/tam.go)
