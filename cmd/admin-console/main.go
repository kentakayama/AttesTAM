// Admin Console for Building Security Service Provider
// TAM API integration + config file support (config.json).
// Precedence of settings:
//   1) Environment variables (PORT, TAM_API_BASE)
//   2) config.json values
//   3) built-in defaults
//
// To run (manual test pattern #3):
//   1) Start mock API:  go run ./cmd/mockapi
//   2) Start app:      go run .
//   3) Open: http://localhost:8080 and click "Show Devices"

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	tmpl      *template.Template
	buildTime = time.Now()
	conf      AppConfig
)

func main() {
	// defaults
	conf.Server.Port = 8080
	conf.TAMAPIBase = ""

	// Load config.json if present
	loadConfig(resolvePath("config.json"), &conf)

	// Env overrides
	if v := os.Getenv("TAM_API_BASE"); v != "" {
		conf.TAMAPIBase = v
	}
	if v := os.Getenv("PORT"); v != "" {
		// Keep as string for ListenAndServe; we still print configured port
		// but not converting for simplicity.
	}

	var err error
	tmpl, err = template.ParseFiles(resolvePath(filepath.Join("templates", "index.html")))
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/devices", devicesHandler)
	mux.HandleFunc("/api/manifests", manifestsHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(resolvePath("static")))))

	addr := fmt.Sprintf(":%d", conf.Server.Port)
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	log.Printf("Admin Console listening on http://localhost%v (build: %s) tamApiBase=%q", addr, buildTime.Format(time.RFC3339), conf.TAMAPIBase)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
