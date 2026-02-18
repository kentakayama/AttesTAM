package main

import (
	"fmt"
	"log"
	"net/http"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func devicesHandler(w http.ResponseWriter, r *http.Request) {
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

	devices, err := loadTestVectorDevices()
	if err != nil {
		log.Printf("testvector devices load failed: %v", err)
		http.Error(w, fmt.Sprintf("testvector devices load failed: %v", err), http.StatusInternalServerError)
		return
	}
	respondJSON(w, devices)
}

func manifestsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
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
	case http.MethodPost:
		base := conf.TAMAPIBase
		if base != "" {
			if err := postTAMManifest(w, r, base); err != nil {
				log.Printf("TAM API post manifest failed: %v", err)
				http.Error(w, fmt.Sprintf("TAM API post failed: %v", err), http.StatusBadGateway)
			}
			return
		}

		if err := echoUploadedFileAsDownload(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
