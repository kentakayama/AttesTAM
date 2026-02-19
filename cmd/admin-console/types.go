package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kentakayama/tam-over-http/internal/suit"
)

type Agent struct {
	KID             string             `json:"kid"`
	LastUpdate      string             `json:"last_update,omitempty"`
	Attributes      Attribute          `json:"attribute"`
	InstalledTCList []TrustedComponent `json:"installed-tc"`
}

type Attribute struct {
	Ueid string `json:"ueid"`
}

type TrustedComponent struct {
	Name    suit.ComponentID `json:"name"`
	Version int              `json:"version"`
}

func (w TrustedComponent) MarshalJSON() ([]byte, error) {
	type alias struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	return json.Marshal(alias{
		Name:    componentIDDisplayName(w.Name),
		Version: w.Version,
	})
}

func (w *TrustedComponent) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name    json.RawMessage `json:"name"`
		Version int             `json:"version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	w.Version = raw.Version
	if len(raw.Name) == 0 || string(raw.Name) == "null" {
		w.Name = nil
		return nil
	}

	var id suit.ComponentID
	if err := json.Unmarshal(raw.Name, &id); err == nil {
		w.Name = id
		return nil
	}

	var s string
	if err := json.Unmarshal(raw.Name, &s); err == nil {
		w.Name = toComponentID(s)
		return nil
	}

	var list []any
	if err := json.Unmarshal(raw.Name, &list); err == nil {
		w.Name = toComponentID(list)
		return nil
	}
	return fmt.Errorf("invalid installed tc item name")
}

func componentIDDisplayName(id suit.ComponentID) string {
	if len(id) == 0 {
		return ""
	}
	if len(id) == 1 {
		diag := id[0].CBORDiagString(0)
		if strings.HasPrefix(diag, "'") && strings.HasSuffix(diag, "'") && len(diag) >= 2 {
			return diag[1 : len(diag)-1]
		}
		return diag
	}
	return id.CBORDiagString(0)
}
