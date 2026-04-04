package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"sipbridge/internal/config"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleDatabaseSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleDatabaseSettingsGet(w, r)
	case http.MethodPut:
		s.handleDatabaseSettingsPut(w, r)
	case http.MethodDelete:
		s.handleDatabaseSettingsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabaseSettingsGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var saved *config.DatabaseSpec
	if s.cm != nil {
		saved = s.cm.Current().Spec.Database
	}
	note := "Saved values are written to config YAML. PostgreSQL as the live config store is optional; set DATABASE_URL or individual POSTGRES_* env vars on the process when supported."
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved": saved,
		"env": map[string]bool{
			"config_http_url_set": strings.TrimSpace(os.Getenv("CONFIG_HTTP_URL")) != "",
			"database_url_set":    strings.TrimSpace(os.Getenv("DATABASE_URL")) != "",
		},
		"note": note,
	})
}

func (s *Server) handleDatabaseSettingsPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var req config.DatabaseSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	if err := config.ValidateDatabaseSpec(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	cur := s.cm.Current()
	cur.Spec.Database = &req
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":               true,
		"restart_required": false,
		"message":          "Database settings saved to config. Restart may be required when wiring PostgreSQL at runtime.",
	})
}

func (s *Server) handleDatabaseSettingsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	cur := s.cm.Current()
	cur.Spec.Database = nil
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
