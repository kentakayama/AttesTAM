package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig holds runtime settings.
type AppConfig struct {
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
	TAMAPIBase string `json:"tamApiBase"`
}

func loadConfig(path string, out *AppConfig) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(out)
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
