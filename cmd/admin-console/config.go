/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	internallogger "github.com/kentakayama/AttesTAM/internal/logger"
)

const envConsoleLogLevel = "ATTESTAM_CONSOLE_LOG_LEVEL"

// AppConfig holds runtime settings.
type AppConfig struct {
	Server struct {
		Port int
	}
	TAMAPIBase  string
	TAMAPIDebug bool
	LogLevel    slog.Level
}

func loadConfigFromFlags() AppConfig {
	var logLevel string
	var cfg AppConfig
	flag.IntVar(&cfg.Server.Port, "port", 9090, "HTTP listen port for AttesTAM Console")
	flag.StringVar(&cfg.TAMAPIBase, "tam-api-base", "http://127.0.0.1:8080/", "TAM API base URL")
	flag.BoolVar(&cfg.TAMAPIDebug, "tam-api-debug", false, "Log TAM API HTTP requests and responses to stderr")
	flag.StringVar(&logLevel, "log-level", "info", "minimum log level: debug, info, warn, or error")
	flag.Parse()

	level, err := resolveConsoleLogLevel(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level: %v\n", err)
		os.Exit(1)
	}
	cfg.LogLevel = level

	return cfg
}

func validateConfig(cfg AppConfig) error {
	if cfg.TAMAPIBase == "" {
		return fmt.Errorf("tam-api-base is required")
	}
	return nil
}

func resolvePath(rel string) string {
	candidates := []string{
		rel,
		filepath.Join("cmd", "admin-console", rel),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return rel
}

func resolveConsoleLogLevel(cliValue string) (slog.Level, error) {
	if consoleFlagWasSet("log-level") {
		return internallogger.ParseLevel(cliValue)
	}
	if envValue, ok := os.LookupEnv(envConsoleLogLevel); ok {
		return internallogger.ParseLevel(envValue)
	}
	return internallogger.DefaultLevel, nil
}

func consoleFlagWasSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
