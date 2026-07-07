/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	internallogger "github.com/kentakayama/AttesTAM/internal/logger"
)

var (
	tmpl      *template.Template
	buildTime = time.Now()
	conf      AppConfig
	appLogger = internallogger.NewWithWriter(os.Stderr, slog.LevelDebug).With("service", "attestam-console")
)

func main() {
	conf = loadConfigFromFlags()
	if err := validateConfig(conf); err != nil {
		appLogger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	var err error
	tmpl, err = template.ParseFiles(resolvePath(filepath.Join("templates", "index.html")))
	if err != nil {
		appLogger.Error("parse template", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/console/view-managed-devices", handleListAgents)
	mux.HandleFunc("/console/view-managed-tcs", handleListManifestsService)
	mux.HandleFunc("/console/register-tc", handleRegisterManifest)
	mux.Handle("/static/", withNoStore(http.StripPrefix("/static/", http.FileServer(http.Dir(resolvePath("static"))))))

	addr := fmt.Sprintf(":%d", conf.Server.Port)

	appLogger.Debug("AttesTAM Console listening", "addr", "http://127.0.0.1"+addr, "build", buildTime.Format(time.RFC3339), "tam_api_base", conf.TAMAPIBase)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		appLogger.Error("console server failed", "err", err)
		os.Exit(1)
	}
}

func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setNoStoreHeaders(w)
		next.ServeHTTP(w, r)
	})
}
