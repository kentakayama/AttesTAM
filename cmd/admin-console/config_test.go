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

func TestResolveConsoleLogLevelDefault(t *testing.T) {
	resetConsoleFlagsForTest(t)
	t.Setenv(envConsoleLogLevel, "")

	level, err := resolveConsoleLogLevel("info")
	require.NoError(t, err)
	require.Equal(t, slog.LevelInfo, level)
}

func TestResolveConsoleLogLevelUsesEnvWhenFlagNotSet(t *testing.T) {
	resetConsoleFlagsForTest(t)
	t.Setenv(envConsoleLogLevel, "debug")

	level, err := resolveConsoleLogLevel("info")
	require.NoError(t, err)
	require.Equal(t, slog.LevelDebug, level)
}

func TestResolveConsoleLogLevelPrefersFlagOverEnv(t *testing.T) {
	resetConsoleFlagsForTest(t)
	t.Setenv(envConsoleLogLevel, "error")

	require.NoError(t, flag.CommandLine.Set("log-level", "warn"))
	level, err := resolveConsoleLogLevel("warn")
	require.NoError(t, err)
	require.Equal(t, slog.LevelWarn, level)
}

func TestResolveConsoleLogLevelRejectsInvalidValue(t *testing.T) {
	resetConsoleFlagsForTest(t)
	t.Setenv(envConsoleLogLevel, "trace")

	_, err := resolveConsoleLogLevel("info")
	require.Error(t, err)
}

func resetConsoleFlagsForTest(t *testing.T) {
	t.Helper()
	orig := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.String("log-level", "info", "")
	t.Cleanup(func() {
		flag.CommandLine = orig
	})
}
