/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentMarshalJSONAlwaysIncludesLastUpdate(t *testing.T) {
	t.Run("zero time becomes zero RFC3339 timestamp", func(t *testing.T) {
		agent := Agent{
			KID:             []byte("dev-1"),
			Attributes:      Attribute{},
			InstalledTCList: []TrustedComponent{},
		}

		raw, err := json.Marshal(agent)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if _, ok := got["last_update"]; !ok {
			t.Fatalf("last_update is missing: %s", string(raw))
		}
		if got["last_update"] != "0001-01-01T00:00:00Z" {
			t.Fatalf("unexpected last_update: %#v", got["last_update"])
		}
	})

	t.Run("non-zero time becomes RFC3339 string", func(t *testing.T) {
		agent := Agent{
			KID:             []byte("dev-1"),
			LastUpdate:      time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC),
			Attributes:      Attribute{},
			InstalledTCList: []TrustedComponent{},
		}

		raw, err := json.Marshal(agent)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if got["last_update"] != "2026-02-18T10:00:00Z" {
			t.Fatalf("unexpected last_update: %#v", got["last_update"])
		}
	})
}
