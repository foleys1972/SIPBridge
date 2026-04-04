package api

import (
	"encoding/json"
	"net/http"

	"sipbridge/internal/config"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleClusterSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleClusterSettingsGet(w, r)
	case http.MethodPut:
		s.handleClusterSettingsPut(w, r)
	case http.MethodDelete:
		s.handleClusterSettingsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClusterSettingsGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var saved *config.ClusterSpec
	if s.cm != nil {
		saved = s.cm.Current().Spec.Cluster
	}
	note := "Saved values are written to config YAML. The SIP stack applies merged cluster limits at process start—restart after changing capacity or overflow redirect so new dialogs use the updated limits."
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved":     saved,
		"effective": s.sipSrv.ClusterLimits(),
		"note":      note,
	})
}

func (s *Server) handleClusterSettingsPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if s.cm.ConfigReadOnly() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	var req config.ClusterSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	if err := config.ValidateClusterSpec(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	cur := s.cm.Current()
	cur.Spec.Cluster = &req
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
		"restart_required": true,
		"message":          "Restart the sipbridge process so the SIP stack picks up merged cluster limits.",
	})
}

func (s *Server) handleClusterSettingsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if s.cm.ConfigReadOnly() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "config is read-only (CONFIG_HTTP_URL)"})
		return
	}
	cur := s.cm.Current()
	cur.Spec.Cluster = nil
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
		"restart_required": true,
		"message":          "Removed spec.cluster from config; restart to rely on environment defaults only.",
	})
}
