/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

// To keep the CBOR-based backend server free from any CBOR→JSON translation
// responsibility, this admin console defines its own JSON-specific DTOs
// (MarshalJSON targets).
//
// CBOR decoding MUST use the backend protocol definitions:
// internal/tam/agent_status.go
// Therefore, these DTOs should stay aligned with those definitions as much
// as possible to avoid semantic drift.
//
// In short:
//   - Backend: authoritative for CBOR wire format and protocol semantics.
//   - Frondend (this code): responsible for JSON representation for the web UI.

package main

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/veraison/eat"
)

type Agent struct {
	KID             []byte             `json:"kid"`
	LastUpdate      time.Time          `json:"last_update"`
	Attributes      Attribute          `json:"attribute"`
	InstalledTCList []TrustedComponent `json:"installed-tc"`
}

func (a Agent) MarshalJSON() ([]byte, error) {
	type alias struct {
		KID             string             `json:"kid"`
		LastUpdate      string             `json:"last_update"`
		Attributes      Attribute          `json:"attribute"`
		InstalledTCList []TrustedComponent `json:"installed-tc"`
	}
	out := alias{
		KID:             string(a.KID),
		LastUpdate:      formatUpdatedAt(a.LastUpdate),
		Attributes:      a.Attributes,
		InstalledTCList: a.InstalledTCList,
	}
	return json.Marshal(out)
}

type Attribute struct {
	Ueid eat.UEID `json:"ueid"`
}

func (a Attribute) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ueid string `json:"ueid"`
	}
	return json.Marshal(alias{
		Ueid: hex.EncodeToString([]byte(a.Ueid)),
	})
}

type TrustedComponent struct {
	Name    suit.ComponentID `json:"name"`
	Version uint64           `json:"version"`
}

func (w TrustedComponent) MarshalJSON() ([]byte, error) {
	type alias struct {
		Name    string `json:"name"`
		Version uint64 `json:"version"`
	}
	return json.Marshal(alias{
		Name:    componentIDDisplayName(w.Name),
		Version: w.Version,
	})
}

func componentIDDisplayName(id suit.ComponentID) string {
	if len(id) == 0 {
		return ""
	}
	return id.CBORDiagString(0)
}
