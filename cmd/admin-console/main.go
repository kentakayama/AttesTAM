// Sample TAM Admin Console.
// TAM API integration + command-line flags.
//
// To run (manual test pattern #3):
//   1) Start mock API:  go run ./cmd/mockapi
//   2) Start app:      go run ./cmd/admin-console --port=8080 --tam-api-base=http://localhost:8080
//   3) Open: http://localhost:8080 and click "Show Devices"

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

	log.Printf("Admin Console listening on http://localhost%v (build: %s) tamApiBase=%q", addr, buildTime.Format(time.RFC3339), conf.TAMAPIBase)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
