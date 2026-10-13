package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type RuntimeConfig struct {
	BackendURL    string   `json:"backendUrl"`
	AdminPassword string   `json:"adminPassword"`
	AdminKey      string   `json:"adminKey"`
	MediaCommand  []string `json:"mediaCommand"`
}

func runtimeConfig(dir string) RuntimeConfig {
	var c RuntimeConfig
	data, e := os.ReadFile(filepath.Join(dir, "runtime.json"))
	if e == nil {
		json.Unmarshal(data, &c)
	}
	if c.BackendURL == "" {
		c.BackendURL = "http://127.0.0.1:18771"
	}
	return c
}
func RuntimeBackend(dir string) Backend {
	c := runtimeConfig(dir)
	return &SFTPGo{URL: c.BackendURL, AdminKey: c.AdminKey, AdminPassword: c.AdminPassword}
}
func RuntimeMedia(dir string) []string { return runtimeConfig(dir).MediaCommand }
