/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

var (
	tmpl      *template.Template
	buildTime = time.Now()
	conf      AppConfig
)

func main() {
	conf = loadConfigFromFlags()
	if err := validateConfig(conf); err != nil {
		log.Fatal(err)
	}

	var err error
	tmpl, err = template.ParseFiles(resolvePath(filepath.Join("templates", "index.html")))
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/console/view-managed-devices", handleListAgents)
	mux.HandleFunc("/console/view-managed-tcs", handleListManifestsService)
	mux.HandleFunc("/console/register-tc", handleRegisterManifest)
	mux.Handle("/static/", withNoStore(http.StripPrefix("/static/", http.FileServer(http.Dir(resolvePath("static"))))))

	addr := fmt.Sprintf(":%d", conf.Server.Port)

	log.Printf("AttesTAM Console listening on http://127.0.0.1%v (build: %s) tamApiBase=%q", addr, buildTime.Format(time.RFC3339), conf.TAMAPIBase)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setNoStoreHeaders(w)
		next.ServeHTTP(w, r)
	})
}
