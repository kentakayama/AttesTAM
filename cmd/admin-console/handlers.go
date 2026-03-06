/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"
	"log"
	"net/http"
)

type indexViewData struct {
	ConnectedTAM string
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := indexViewData{
		ConnectedTAM: conf.TAMAPIBase,
	}

	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	base := conf.TAMAPIBase
	if base == "" {
		http.Error(w, "admin console is misconfigured: tam-api-base is required", http.StatusInternalServerError)
		return
	}

	devices, err := fetchTAMDevices(base)
	if err != nil {
		log.Printf("TAM API fetch failed: %v", err)
		http.Error(w, fmt.Sprintf("TAM API fetch failed: %v", err), http.StatusBadGateway)
		return
	}
	respondJSON(w, devices)
}

func handleListManifestsService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	base := conf.TAMAPIBase
	if base == "" {
		http.Error(w, "admin console is misconfigured: tam-api-base is required", http.StatusInternalServerError)
		return
	}

	manifests, err := fetchTAMManifests(base)
	if err != nil {
		log.Printf("TAM API fetch manifests failed: %v", err)
		http.Error(w, fmt.Sprintf("TAM API fetch failed: %v", err), http.StatusBadGateway)
		return
	}
	respondJSON(w, manifests)
}

func handleRegisterManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	base := conf.TAMAPIBase
	if base == "" {
		http.Error(w, "admin console is misconfigured: tam-api-base is required", http.StatusInternalServerError)
		return
	}

	if err := postTAMManifest(w, r, base); err != nil {
		log.Printf("TAM API post manifest failed: %v", err)
		http.Error(w, fmt.Sprintf("TAM API post failed: %v", err), http.StatusBadGateway)
	}
}
