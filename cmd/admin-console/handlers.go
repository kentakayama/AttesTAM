package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	base := conf.TAMAPIBase
	if base != "" {
		devices, err := fetchTAMDevices(base)
		if err != nil {
			log.Printf("TAM API fetch failed: %v", err)
			http.Error(w, fmt.Sprintf("TAM API fetch failed: %v", err), http.StatusBadGateway)
			return
		}
		respondJSON(w, devices)
		return
	}

	devices, err := loadTestVectorAgents()
	if err != nil {
		log.Printf("testvector agents load failed: %v", err)
		http.Error(w, fmt.Sprintf("testvector agents load failed: %v", err), http.StatusInternalServerError)
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
	if base != "" {
		manifests, err := fetchTAMManifests(base)
		if err != nil {
			log.Printf("TAM API fetch manifests failed: %v", err)
			http.Error(w, fmt.Sprintf("TAM API fetch failed: %v", err), http.StatusBadGateway)
			return
		}
		respondJSON(w, manifests)
		return
	}

	manifests, err := loadTestVectorManifests()
	if err != nil {
		log.Printf("testvector manifests load failed: %v", err)
		http.Error(w, fmt.Sprintf("testvector manifests load failed: %v", err), http.StatusInternalServerError)
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
	if base != "" {
		if err := postTAMManifest(w, r, base); err != nil {
			log.Printf("TAM API post manifest failed: %v", err)
			http.Error(w, fmt.Sprintf("TAM API post failed: %v", err), http.StatusBadGateway)
		}
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := echoUploadedFileAsDownload(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		return
	}

	http.Error(w, "multipart/form-data is required", http.StatusBadRequest)
}
