/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"flag"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveLogLevelDefault(t *testing.T) {
	resetFlagsForTest(t)
	t.Setenv(envLogLevel, "")

	level, err := resolveLogLevel("info", "log-level", envLogLevel)
	require.NoError(t, err)
	require.Equal(t, slog.LevelInfo, level)
}

func TestResolveLogLevelUsesEnvWhenFlagNotSet(t *testing.T) {
	resetFlagsForTest(t)
	t.Setenv(envLogLevel, "debug")

	level, err := resolveLogLevel("info", "log-level", envLogLevel)
	require.NoError(t, err)
	require.Equal(t, slog.LevelDebug, level)
}

func TestResolveLogLevelPrefersFlagOverEnv(t *testing.T) {
	resetFlagsForTest(t)
	t.Setenv(envLogLevel, "error")

	require.NoError(t, flag.CommandLine.Set("log-level", "warn"))
	level, err := resolveLogLevel("warn", "log-level", envLogLevel)
	require.NoError(t, err)
	require.Equal(t, slog.LevelWarn, level)
}

func TestResolveLogLevelRejectsInvalidValue(t *testing.T) {
	resetFlagsForTest(t)
	t.Setenv(envLogLevel, "trace")

	_, err := resolveLogLevel("info", "log-level", envLogLevel)
	require.Error(t, err)
}

func resetFlagsForTest(t *testing.T) {
	t.Helper()
	orig := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.String("log-level", "info", "")
	t.Cleanup(func() {
		flag.CommandLine = orig
	})
}
