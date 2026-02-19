/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"flag"
	"os"
	"path/filepath"
)

// AppConfig holds runtime settings.
type AppConfig struct {
	Server struct {
		Port int
	}
	TAMAPIBase string
}

func loadConfigFromFlags() AppConfig {
	var cfg AppConfig
	flag.IntVar(&cfg.Server.Port, "port", 9090, "HTTP listen port for admin console")
	flag.StringVar(&cfg.TAMAPIBase, "tam-api-base", "", "TAM API base URL (e.g. http://localhost:8080)")
	flag.Parse()

	return cfg
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
