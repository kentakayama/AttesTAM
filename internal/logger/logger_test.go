/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{name: "empty defaults to info", input: "", want: slog.LevelInfo},
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info uppercase", input: "INFO", want: slog.LevelInfo},
		{name: "warn alias", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseLevelRejectsUnknownValue(t *testing.T) {
	_, err := ParseLevel("trace")
	require.Error(t, err)
}

func TestNewNamedWithWriterFormatsSemiStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewNamedWithWriter("attestam", &buf, slog.LevelDebug)

	logger.Error("failed to create server", "err", "bind: address already in use", "addr", ":8080")

	logged := buf.String()
	require.Contains(t, logged, "[ERROR] attestam: failed to create server")
	require.Contains(t, logged, `err="bind: address already in use"`)
	require.Contains(t, logged, "addr=:8080")
	require.True(t, strings.HasSuffix(logged, "\n"))
}

func TestRenamePreservesFormatAndChangesName(t *testing.T) {
	var buf bytes.Buffer
	base := NewNamedWithWriter("attestam", &buf, slog.LevelDebug)
	verifier := Rename(base, "attestam-verifier")

	verifier.Warn("TLS verification is disabled", "base_url", "https://localhost:8443")

	logged := buf.String()
	require.Contains(t, logged, "[WARN] attestam-verifier: TLS verification is disabled")
	require.Contains(t, logged, "base_url=https://localhost:8443")
}
